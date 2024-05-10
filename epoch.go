package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/multiformats/go-multiaddr"

	"github.com/anjor/carlet"
	"github.com/davecgh/go-spew/spew"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	carv1 "github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rpcpool/yellowstone-faithful/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	deprecatedbucketter "github.com/rpcpool/yellowstone-faithful/deprecated/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	hugecache "github.com/rpcpool/yellowstone-faithful/huge-cache"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/radiance/genesis"
	splitcarfetcher "github.com/rpcpool/yellowstone-faithful/split-car-fetcher"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

type Epoch struct {
	epoch          uint64
	isFilecoinMode bool // true if the epoch is in Filecoin mode (i.e. Lassie mode)
	config         *Config
	// genesis:
	genesis *GenesisContainer
	// contains indexes and block data for the epoch
	lassieFetcher               *lassieWrapper
	localCarReader              *carv2.Reader
	remoteCarReader             ReaderAtCloser
	carHeaderSize               uint64
	rootCid                     cid.Cid
	cidToOffsetAndSizeIndex     *indexes.CidToOffsetAndSize_Reader
	deprecated_cidToOffsetIndex *indexes.Deprecated_CidToOffset_Reader
	slotToCidIndex              *indexes.SlotToCid_Reader
	sigToCidIndex               *indexes.SigToCid_Reader
	sigExists                   SigExistsIndex
	gsfaReader                  *gsfa.GsfaReader
	onClose                     []func() error
	allCache                    *hugecache.Cache
}

func (r *Epoch) GetCache() *hugecache.Cache {
	return r.allCache
}

func (e *Epoch) Epoch() uint64 {
	return e.epoch
}

func (e *Epoch) IsFilecoinMode() bool {
	return e.isFilecoinMode
}

// IsCarMode returns true if the epoch is in CAR mode.
// This means that the data is going to be fetched from a CAR file (local or remote).
func (e *Epoch) IsCarMode() bool {
	return !e.isFilecoinMode
}

func (e *Epoch) Close() error {
	multiErr := make([]error, 0)
	for _, fn := range e.onClose {
		if err := fn(); err != nil {
			multiErr = append(multiErr, err)
		}
	}
	return errors.Join(multiErr...)
}

func (e *Epoch) GetGenesis() *GenesisContainer {
	return e.genesis
}

func NewEpochFromConfig(
	config *Config,
	c *cli.Context,
	allCache *hugecache.Cache,
	minerInfo *splitcarfetcher.MinerInfoCache,
) (*Epoch, error) {
	if config == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	isLassieMode := config.IsFilecoinMode()
	isCarMode := !isLassieMode

	ep := &Epoch{
		epoch:          *config.Epoch,
		isFilecoinMode: isLassieMode,
		config:         config,
		onClose:        make([]func() error, 0),
		allCache:       allCache,
	}
	var lastRootCid cid.Cid
	{
		// if epoch is 0, then try loading the genesis from the config:
		if *config.Epoch == 0 {
			genesisConfig, ha, err := genesis.ReadGenesisFromFile(string(config.Genesis.URI))
			if err != nil {
				return nil, fmt.Errorf("failed to read genesis: %w", err)
			}
			ep.genesis = &GenesisContainer{
				Hash:   solana.HashFromBytes(ha[:]),
				Config: genesisConfig,
			}
		}
	}
	if isCarMode {
		if config.IsDeprecatedIndexes() {
			// The CAR-mode requires a cid-to-offset index.
			cidToOffsetIndexFile, err := openIndexStorage(
				c.Context,
				string(config.Indexes.CidToOffset.URI),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to open cid-to-offset index file: %w", err)
			}
			ep.onClose = append(ep.onClose, cidToOffsetIndexFile.Close)

			cidToOffsetIndex, err := indexes.Deprecated_OpenWithReader_CidToOffset(cidToOffsetIndexFile)
			if err != nil {
				return nil, fmt.Errorf("failed to open cid-to-offset index: %w", err)
			}
			if config.Indexes.CidToOffsetAndSize.URI.IsRemoteWeb() {
				cidToOffsetIndex.Prefetch(true)
			}
			ep.deprecated_cidToOffsetIndex = cidToOffsetIndex
		} else {
			// The CAR-mode requires a cid-to-offset index.
			cidToOffsetAndSizeIndexFile, err := openIndexStorage(
				c.Context,
				string(config.Indexes.CidToOffsetAndSize.URI),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to open cid-to-offset index file: %w", err)
			}
			ep.onClose = append(ep.onClose, cidToOffsetAndSizeIndexFile.Close)

			cidToOffsetAndSizeIndex, err := indexes.OpenWithReader_CidToOffsetAndSize(cidToOffsetAndSizeIndexFile)
			if err != nil {
				return nil, fmt.Errorf("failed to open cid-to-offset index: %w", err)
			}
			if config.Indexes.CidToOffsetAndSize.URI.IsRemoteWeb() {
				cidToOffsetAndSizeIndex.Prefetch(true)
			}
			ep.cidToOffsetAndSizeIndex = cidToOffsetAndSizeIndex

			if ep.Epoch() != cidToOffsetAndSizeIndex.Meta().Epoch {
				return nil, fmt.Errorf("epoch mismatch in cid-to-offset-and-size index: expected %d, got %d", ep.Epoch(), cidToOffsetAndSizeIndex.Meta().Epoch)
			}
			lastRootCid = cidToOffsetAndSizeIndex.Meta().RootCid
		}
	}

	{
		slotToCidIndexFile, err := openIndexStorage(
			c.Context,
			string(config.Indexes.SlotToCid.URI),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to open slot-to-cid index file: %w", err)
		}
		ep.onClose = append(ep.onClose, slotToCidIndexFile.Close)

		slotToCidIndex, err := indexes.OpenWithReader_SlotToCid(slotToCidIndexFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open slot-to-cid index: %w", err)
		}
		if config.Indexes.SlotToCid.URI.IsRemoteWeb() {
			slotToCidIndex.Prefetch(true)
		}
		ep.slotToCidIndex = slotToCidIndex

		if !slotToCidIndex.IsDeprecatedOldVersion() {
			if ep.Epoch() != slotToCidIndex.Meta().Epoch {
				return nil, fmt.Errorf("epoch mismatch in slot-to-cid index: expected %d, got %d", ep.Epoch(), slotToCidIndex.Meta().Epoch)
			}
			if lastRootCid != cid.Undef && !lastRootCid.Equals(slotToCidIndex.Meta().RootCid) {
				return nil, fmt.Errorf("root CID mismatch in slot-to-cid index: expected %s, got %s", lastRootCid, slotToCidIndex.Meta().RootCid)
			}
			lastRootCid = slotToCidIndex.Meta().RootCid
		}
	}

	{
		sigToCidIndexFile, err := openIndexStorage(
			c.Context,
			string(config.Indexes.SigToCid.URI),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to open sig-to-cid index file: %w", err)
		}
		ep.onClose = append(ep.onClose, sigToCidIndexFile.Close)

		sigToCidIndex, err := indexes.OpenWithReader_SigToCid(sigToCidIndexFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open sig-to-cid index: %w", err)
		}
		if config.Indexes.SigToCid.URI.IsRemoteWeb() {
			sigToCidIndex.Prefetch(true)
		}
		ep.sigToCidIndex = sigToCidIndex

		if !sigToCidIndex.IsDeprecatedOldVersion() {
			if ep.Epoch() != sigToCidIndex.Meta().Epoch {
				return nil, fmt.Errorf("epoch mismatch in sig-to-cid index: expected %d, got %d", ep.Epoch(), sigToCidIndex.Meta().Epoch)
			}
			if !lastRootCid.Equals(sigToCidIndex.Meta().RootCid) {
				return nil, fmt.Errorf("root CID mismatch in sig-to-cid index: expected %s, got %s", lastRootCid, sigToCidIndex.Meta().RootCid)
			}
		}
	}

	{
		if !config.Indexes.Gsfa.URI.IsZero() {
			gsfaIndex, err := gsfa.NewGsfaReader(string(config.Indexes.Gsfa.URI))
			if err != nil {
				return nil, fmt.Errorf("failed to open gsfa index: %w", err)
			}
			ep.onClose = append(ep.onClose, gsfaIndex.Close)
			ep.gsfaReader = gsfaIndex

			if gsfaIndex.Version() >= 2 {
				gotIndexEpoch, ok := gsfaIndex.Meta().GetUint64(indexmeta.MetadataKey_Epoch)
				if !ok {
					return nil, fmt.Errorf("the gsfa index does not have the epoch metadata")
				}
				if ep.Epoch() != gotIndexEpoch {
					return nil, fmt.Errorf("epoch mismatch in gsfa index: expected %d, got %d", ep.Epoch(), gotIndexEpoch)
				}

				gotRootCid, ok := gsfaIndex.Meta().GetCid(indexmeta.MetadataKey_RootCid)
				if !ok {
					return nil, fmt.Errorf("the gsfa index does not have the root CID metadata")
				}
				if !lastRootCid.Equals(gotRootCid) {
					return nil, fmt.Errorf("root CID mismatch in gsfa index: expected %s, got %s", lastRootCid, gotRootCid)
				}
			}
		}
	}

	if isLassieMode {
		fetchProviderAddrInfos, err := ParseFilecoinProviders(config.Data.Filecoin.Providers...)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Filecoin providers: %w", err)
		}
		ls, err := newLassieWrapper(c, fetchProviderAddrInfos)
		if err != nil {
			return nil, fmt.Errorf("newLassieWrapper: %w", err)
		}
		ep.lassieFetcher = ls

		if !lastRootCid.Equals(config.Data.Filecoin.RootCID) {
			return nil, fmt.Errorf("root CID mismatch in lassie: expected %s, got %s", lastRootCid, config.Data.Filecoin.RootCID)
		}
		// TODO: check epoch.
	}

	if isCarMode {
		var localCarReader *carv2.Reader
		var remoteCarReader ReaderAtCloser
		var err error
		if config.IsCarFromPieces() {

			metadata, err := splitcarfetcher.MetadataFromYaml(string(config.Data.Car.FromPieces.Metadata.URI))
			if err != nil {
				return nil, fmt.Errorf("failed to read pieces metadata: %w", err)
			}

			isFromDeals := !config.Data.Car.FromPieces.Deals.URI.IsZero()

			if isFromDeals {
				dealRegistry, err := splitcarfetcher.DealsFromCSV(string(config.Data.Car.FromPieces.Deals.URI))
				if err != nil {
					return nil, fmt.Errorf("failed to read deals: %w", err)
				}

				scr, err := splitcarfetcher.NewSplitCarReader(
					metadata.CarPieces,
					func(piece carlet.CarFile) (splitcarfetcher.ReaderAtCloserSize, error) {
						minerID, ok := dealRegistry.GetMinerByPieceCID(piece.CommP)
						if !ok {
							return nil, fmt.Errorf("failed to find miner for piece CID %s", piece.CommP)
						}
						klog.V(3).Infof("piece CID %s is stored on miner %s", piece.CommP, minerID)
						minerInfo, err := minerInfo.GetProviderInfo(c.Context, minerID)
						if err != nil {
							return nil, fmt.Errorf("failed to get miner info for miner %s, for piece %s: %w", minerID, piece.CommP, err)
						}
						if len(minerInfo.Multiaddrs) == 0 {
							return nil, fmt.Errorf("miner %s has no multiaddrs", minerID)
						}
						klog.V(3).Infof("miner info: %s", spew.Sdump(minerInfo))
						// extract the IP address from the multiaddr:
						split := multiaddr.Split(minerInfo.Multiaddrs[0])
						if len(split) < 2 {
							return nil, fmt.Errorf("invalid multiaddr: %s", minerInfo.Multiaddrs[0])
						}
						component0 := split[0].(*multiaddr.Component)
						component1 := split[1].(*multiaddr.Component)

						var ip string
						// TODO: use the appropriate port (80, better if 443 with TLS)
						port := "80"

						if component0.Protocol().Code == multiaddr.P_IP4 {
							ip = component0.Value()
						} else if component1.Protocol().Code == multiaddr.P_IP4 {
							ip = component1.Value()
						} else {
							return nil, fmt.Errorf("invalid multiaddr: %s", minerInfo.Multiaddrs[0])
						}
						minerIP := fmt.Sprintf("%s:%s", ip, port)
						klog.V(3).Infof("piece CID %s is stored on miner %s (%s)", piece.CommP, minerID, minerIP)
						formattedURL := fmt.Sprintf("http://%s/piece/%s", minerIP, piece.CommP.String())

						{
							rfspc, _, err := splitcarfetcher.NewRemoteHTTPFileAsIoReaderAt(
								c.Context,
								formattedURL,
							)
							if err != nil {
								return nil, fmt.Errorf("failed to create remote file split car reader from %q: %w", formattedURL, err)
							}

							return &readCloserWrapper{
								rac:        rfspc,
								name:       formattedURL,
								size:       rfspc.Size(),
								isSplitCar: true,
							}, nil
						}
					})
				if err != nil {
					return nil, fmt.Errorf("failed to open CAR file from pieces: %w", err)
				}
				remoteCarReader = scr
			} else {
				// is from pieceToURL mapping:
				scrFromURLs, err := splitcarfetcher.NewSplitCarReader(
					metadata.CarPieces,
					func(piece carlet.CarFile) (splitcarfetcher.ReaderAtCloserSize, error) {
						pieceURL, ok := config.Data.Car.FromPieces.PieceToURI[piece.CommP]
						if !ok {
							return nil, fmt.Errorf("failed to find URL for piece CID %s", piece.CommP)
						}

						{
							formattedURL := pieceURL.URI.String()
							rfspc, _, err := splitcarfetcher.NewRemoteHTTPFileAsIoReaderAt(
								c.Context,
								formattedURL,
							)
							if err != nil {
								return nil, fmt.Errorf("failed to create remote file split car reader from %q: %w", formattedURL, err)
							}

							return &readCloserWrapper{
								rac:        rfspc,
								name:       formattedURL,
								size:       rfspc.Size(),
								isSplitCar: true,
							}, nil
						}
					})
				if err != nil {
					return nil, fmt.Errorf("failed to open CAR file from pieces: %w", err)
				}
				remoteCarReader = scrFromURLs
			}
		} else {
			localCarReader, remoteCarReader, err = openCarStorage(c.Context, string(config.Data.Car.URI))
			if err != nil {
				return nil, fmt.Errorf("failed to open CAR file: %w", err)
			}
		}
		if localCarReader != nil {
			ep.onClose = append(ep.onClose, localCarReader.Close)
		}
		if remoteCarReader != nil {
			ep.onClose = append(ep.onClose, remoteCarReader.Close)
		}
		ep.localCarReader = localCarReader
		ep.remoteCarReader = remoteCarReader
		if remoteCarReader != nil {
			// determine the header size so that we know where the data starts:
			headerSizeBuf, err := readSectionFromReaderAt(remoteCarReader, 0, 10)
			if err != nil {
				return nil, fmt.Errorf("failed to read CAR header: %w", err)
			}
			// decode as uvarint
			headerSize, n := binary.Uvarint(headerSizeBuf)
			if n <= 0 {
				return nil, fmt.Errorf("failed to decode CAR header size")
			}
			ep.carHeaderSize = uint64(n) + headerSize
		}
		if localCarReader != nil {
			// determine the header size so that we know where the data starts:
			dr, err := localCarReader.DataReader()
			if err != nil {
				return nil, fmt.Errorf("failed to get local CAR data reader: %w", err)
			}
			header, err := carreader.ReadHeader(dr)
			if err != nil {
				return nil, fmt.Errorf("failed to read local CAR header: %w", err)
			}
			var buf bytes.Buffer
			if err = carv1.WriteHeader(header, &buf); err != nil {
				return nil, fmt.Errorf("failed to encode local CAR header: %w", err)
			}
			headerSize := uint64(buf.Len())
			ep.carHeaderSize = headerSize
		}
		if remoteCarReader == nil && localCarReader == nil {
			return nil, fmt.Errorf("no CAR reader available")
		}
	}
	{
		sigExistsFile, err := openIndexStorage(
			c.Context,
			string(config.Indexes.SigExists.URI),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to open sig-exists index file: %w", err)
		}
		ep.onClose = append(ep.onClose, sigExistsFile.Close)

		if config.IsDeprecatedIndexes() {
			sigExists, err := deprecatedbucketter.NewReader(sigExistsFile)
			if err != nil {
				return nil, fmt.Errorf("failed to open (deprecated) sig-exists index: %w", err)
			}
			ep.onClose = append(ep.onClose, sigExists.Close)
			ep.sigExists = sigExists
		} else {
			sigExists, err := bucketteer.NewReader(sigExistsFile)
			if err != nil {
				return nil, fmt.Errorf("failed to open sig-exists index: %w", err)
			}
			ep.onClose = append(ep.onClose, sigExists.Close)
			ep.sigExists = sigExists

			gotEpoch, ok := sigExists.Meta().GetUint64(indexmeta.MetadataKey_Epoch)
			if !ok {
				return nil, fmt.Errorf("the sig-exists index does not have the epoch metadata")
			}
			if ep.Epoch() != gotEpoch {
				return nil, fmt.Errorf("epoch mismatch in sig-exists index: expected %d, got %d", ep.Epoch(), gotEpoch)
			}

			gotRootCid, ok := sigExists.Meta().GetCid(indexmeta.MetadataKey_RootCid)
			if !ok {
				return nil, fmt.Errorf("the sig-exists index does not have the root CID metadata")
			}

			if !lastRootCid.Equals(gotRootCid) {
				return nil, fmt.Errorf("root CID mismatch in sig-exists index: expected %s, got %s", lastRootCid, gotRootCid)
			}
		}
	}

	ep.rootCid = lastRootCid

	return ep, nil
}

func ParseFilecoinProviders(vs ...string) ([]peer.AddrInfo, error) {
	providerAddrInfos := make([]peer.AddrInfo, 0, len(vs))

	for _, v := range vs {
		providerAddrInfo, err := peer.AddrInfoFromString(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse provider address %q: %w", v, err)
		}
		providerAddrInfos = append(providerAddrInfos, *providerAddrInfo)
	}
	return providerAddrInfos, nil
}

func newRandomSignature() [64]byte {
	var sig [64]byte
	rand.Read(sig[:])
	return sig
}

func (r *Epoch) Config() *Config {
	return r.config
}

func (s *Epoch) GetMostRecentAvailableBlock(ctx context.Context) (*ipldbindcode.Block, error) {
	// get root object, then get the last subset, then the last block.
	rootCid := s.rootCid
	rootNode, err := s.GetNodeByCid(ctx, rootCid)
	if err != nil {
		return nil, fmt.Errorf("failed to get root node: %w", err)
	}
	epochNode, err := iplddecoders.DecodeEpoch(rootNode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode epoch node: %w", err)
	}
	if len(epochNode.Subsets) == 0 {
		return nil, fmt.Errorf("no subsets found")
	}
	subsetNode, err := s.GetNodeByCid(ctx, epochNode.Subsets[len(epochNode.Subsets)-1].(cidlink.Link).Cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get subset node: %w", err)
	}
	subset, err := iplddecoders.DecodeSubset(subsetNode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode subset node: %w", err)
	}
	if len(subset.Blocks) == 0 {
		return nil, fmt.Errorf("no blocks found")
	}
	blockNode, err := s.GetNodeByCid(ctx, subset.Blocks[len(subset.Blocks)-1].(cidlink.Link).Cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get block node: %w", err)
	}
	block, err := iplddecoders.DecodeBlock(blockNode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode block node: %w", err)
	}
	return block, nil
}

func (s *Epoch) GetFirstAvailableBlock(ctx context.Context) (*ipldbindcode.Block, error) {
	// get root object, then get the first subset, then the first block.
	rootCid := s.rootCid
	rootNode, err := s.GetNodeByCid(ctx, rootCid)
	if err != nil {
		return nil, fmt.Errorf("failed to get root node: %w", err)
	}
	epochNode, err := iplddecoders.DecodeEpoch(rootNode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode epoch node: %w", err)
	}
	if len(epochNode.Subsets) == 0 {
		return nil, fmt.Errorf("no subsets found")
	}
	subsetNode, err := s.GetNodeByCid(ctx, epochNode.Subsets[0].(cidlink.Link).Cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get subset node: %w", err)
	}
	subset, err := iplddecoders.DecodeSubset(subsetNode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode subset node: %w", err)
	}
	if len(subset.Blocks) == 0 {
		return nil, fmt.Errorf("no blocks found")
	}
	blockNode, err := s.GetNodeByCid(ctx, subset.Blocks[0].(cidlink.Link).Cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get block node: %w", err)
	}
	block, err := iplddecoders.DecodeBlock(blockNode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode block node: %w", err)
	}
	return block, nil
}

func (s *Epoch) prefetchSubgraph(ctx context.Context, wantedCid cid.Cid) error {
	if s.lassieFetcher != nil {
		// Fetch the subgraph from lassie
		sub, err := s.lassieFetcher.GetSubgraph(ctx, wantedCid)
		if err == nil {
			// put in cache
			return sub.Each(ctx, func(c cid.Cid, data []byte) error {
				s.GetCache().PutRawCarObject(c, data)
				return nil
			})
		}
		return fmt.Errorf("failed to get subgraph from lassie for CID %s: %w", wantedCid, err)
	}
	return nil
}

func (s *Epoch) GetNodeByCid(ctx context.Context, wantedCid cid.Cid) ([]byte, error) {
	{
		// try from cache
		data, err, has := s.GetCache().GetRawCarObject(wantedCid)
		if err != nil {
			return nil, err
		}
		if has {
			return data, nil
		}
	}
	if s.lassieFetcher != nil {
		// Fetch the node from lassie.
		data, err := s.lassieFetcher.GetNodeByCid(ctx, wantedCid)
		if err == nil {
			// put in cache
			s.GetCache().PutRawCarObject(wantedCid, data)
			return data, nil
		}
		return nil, fmt.Errorf("failed to get node from lassie for CID %s: %w", wantedCid, err)
	}
	// Find CAR file oas for CID in index.
	oas, err := s.FindOffsetAndSizeFromCid(ctx, wantedCid)
	if err != nil {
		// not found or error
		return nil, fmt.Errorf("failed to find offset for CID %s: %w", wantedCid, err)
	}
	return s.GetNodeByOffsetAndSize(ctx, &wantedCid, oas)
}

func (s *Epoch) ReadAtFromCar(ctx context.Context, offset uint64, length uint64) ([]byte, error) {
	if s.localCarReader == nil {
		// try remote reader
		if s.remoteCarReader == nil {
			return nil, fmt.Errorf("no CAR reader available")
		}
		return readSectionFromReaderAt(s.remoteCarReader, offset, length)
	}
	// Get reader and seek to offset, then read node.
	dr, err := s.localCarReader.DataReader()
	if err != nil {
		return nil, fmt.Errorf("failed to get data reader: %w", err)
	}
	dr.Seek(int64(offset), io.SeekStart)
	data := make([]byte, length)
	_, err = io.ReadFull(dr, data)
	if err != nil {
		return nil, fmt.Errorf("failed to read node from CAR: %w", err)
	}
	return data, nil
}

func (s *Epoch) GetNodeByOffsetAndSize(ctx context.Context, wantedCid *cid.Cid, offsetAndSize *indexes.OffsetAndSize) ([]byte, error) {
	if offsetAndSize == nil {
		return nil, fmt.Errorf("offsetAndSize must not be nil")
	}
	if offsetAndSize.Size == 0 {
		return nil, fmt.Errorf("offsetAndSize.Size must not be 0")
	}
	offset := offsetAndSize.Offset
	length := offsetAndSize.Size
	if s.localCarReader == nil {
		// try remote reader
		if s.remoteCarReader == nil {
			return nil, fmt.Errorf("no CAR reader available")
		}
		return readNodeFromReaderAtWithOffsetAndSize(s.remoteCarReader, wantedCid, offset, length)
	}
	// Get reader and seek to offset, then read node.
	dr, err := s.localCarReader.DataReader()
	if err != nil {
		return nil, fmt.Errorf("failed to get local CAR data reader: %w", err)
	}
	dr.Seek(int64(offset), io.SeekStart)
	br := bufio.NewReader(dr)

	return readNodeWithKnownSize(br, wantedCid, length)
}

func (s *Epoch) getNodeSize(ctx context.Context, offset uint64) (uint64, error) {
	if s.localCarReader == nil {
		// try remote reader
		if s.remoteCarReader == nil {
			return 0, fmt.Errorf("no CAR reader available")
		}
		return readNodeSizeFromReaderAtWithOffset(s.remoteCarReader, offset)
	}
	// Get reader and seek to offset, then read node.
	dr, err := s.localCarReader.DataReader()
	if err != nil {
		return 0, fmt.Errorf("failed to get local CAR data reader: %w", err)
	}
	return readNodeSizeFromReaderAtWithOffset(dr, offset)
}

func readNodeSizeFromReaderAtWithOffset(reader io.ReaderAt, offset uint64) (uint64, error) {
	// read MaxVarintLen64 bytes
	lenBuf := make([]byte, binary.MaxVarintLen64)
	_, err := reader.ReadAt(lenBuf, int64(offset))
	if err != nil {
		return 0, err
	}
	// read uvarint
	dataLen, n := binary.Uvarint(lenBuf)
	dataLen += uint64(n)
	if dataLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return 0, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}
	return dataLen, nil
}

func readNodeWithKnownSize(br *bufio.Reader, wantedCid *cid.Cid, length uint64) ([]byte, error) {
	section := make([]byte, length)
	_, err := io.ReadFull(br, section)
	if err != nil {
		return nil, fmt.Errorf("failed to read section from CAR with length %d: %w", length, err)
	}
	return parseNodeFromSection(section, wantedCid)
}

func parseNodeFromSection(section []byte, wantedCid *cid.Cid) ([]byte, error) {
	// read an uvarint from the buffer
	gotLen, usize := binary.Uvarint(section)
	if usize <= 0 {
		return nil, fmt.Errorf("failed to decode uvarint")
	}
	if gotLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return nil, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}
	data := section[usize:]
	cidLen, gotCid, err := cid.CidFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to read cid: %w", err)
	}
	// verify that the CID we read matches the one we expected.
	if wantedCid != nil && !gotCid.Equals(*wantedCid) {
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}
	return data[cidLen:], nil
}

func (ser *Epoch) FindCidFromSlot(ctx context.Context, slot uint64) (o cid.Cid, e error) {
	startedAt := time.Now()
	defer func() {
		klog.V(4).Infof("Found CID for slot %d in %s: %s", slot, time.Since(startedAt), o)
	}()

	// try from cache
	if c, err, has := ser.GetCache().GetSlotToCid(slot); err != nil {
		return cid.Undef, err
	} else if has {
		return c, nil
	}
	found, err := ser.slotToCidIndex.Get(slot)
	if err != nil {
		return cid.Undef, err
	}
	ser.GetCache().PutSlotToCid(slot, found)
	return found, nil
}

func (ser *Epoch) FindCidFromSignature(ctx context.Context, sig solana.Signature) (o cid.Cid, e error) {
	startedAt := time.Now()
	defer func() {
		klog.V(4).Infof("Found CID for signature %s in %s: %s", sig, time.Since(startedAt), o)
	}()
	return ser.sigToCidIndex.Get(sig)
}

func (ser *Epoch) FindOffsetAndSizeFromCid(ctx context.Context, cid cid.Cid) (os *indexes.OffsetAndSize, e error) {
	startedAt := time.Now()
	defer func() {
		if os != nil {
			klog.V(4).Infof("Found offset and size for CID %s in %s: o=%d s=%d", cid, time.Since(startedAt), os.Offset, os.Size)
		} else {
			klog.V(4).Infof("Offset and size for CID %s in %s: not found", cid, time.Since(startedAt))
		}
	}()

	// try from cache
	if osi, err, has := ser.GetCache().GetCidToOffsetAndSize(cid); err != nil {
		return nil, err
	} else if has {
		return osi, nil
	}

	if ser.config.IsDeprecatedIndexes() {
		offset, err := ser.deprecated_cidToOffsetIndex.Get(cid)
		if err != nil {
			return nil, err
		}

		klog.V(4).Infof("Found offset for CID %s in %s: %d", cid, time.Since(startedAt), offset)

		size, err := ser.getNodeSize(ctx, offset)
		if err != nil {
			return nil, err
		}

		klog.V(4).Infof("Found size for CID %s in %s: %d", cid, time.Since(startedAt), size)

		found := &indexes.OffsetAndSize{
			Offset: offset,
			Size:   size,
		}
		ser.GetCache().PutCidToOffsetAndSize(cid, found)
		return found, nil
	}

	found, err := ser.cidToOffsetAndSizeIndex.Get(cid)
	if err != nil {
		return nil, err
	}
	ser.GetCache().PutCidToOffsetAndSize(cid, found)
	return found, nil
}

func (ser *Epoch) GetBlock(ctx context.Context, slot uint64) (*ipldbindcode.Block, cid.Cid, error) {
	// get the slot by slot number
	wantedCid, err := ser.FindCidFromSlot(ctx, slot)
	if err != nil {
		return nil, cid.Cid{}, fmt.Errorf("failed to find CID for slot %d: %w", slot, err)
	}
	{
		doPrefetch := getValueFromContext(ctx, "prefetch")
		if doPrefetch != nil && doPrefetch.(bool) {
			// prefetch the block
			ser.prefetchSubgraph(ctx, wantedCid)
		}
	}
	// get the block by CID
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		return nil, cid.Cid{}, fmt.Errorf("failed to get node by cid %s: %w", wantedCid, err)
	}
	// try parsing the data as a Block node.
	decoded, err := iplddecoders.DecodeBlock(data)
	if err != nil {
		return nil, cid.Cid{}, fmt.Errorf("failed to decode block with CID %s: %w", wantedCid, err)
	}
	return decoded, wantedCid, nil
}

func (ser *Epoch) GetEntryByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Entry, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		return nil, fmt.Errorf("failed to find node by cid %s: %w", wantedCid, err)
	}
	// try parsing the data as an Entry node.
	decoded, err := iplddecoders.DecodeEntry(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode entry with CID %s: %w", wantedCid, err)
	}
	return decoded, nil
}

func (ser *Epoch) GetTransactionByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Transaction, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		return nil, fmt.Errorf("failed to find node by cid %s: %w", wantedCid, err)
	}
	// try parsing the data as a Transaction node.
	decoded, err := iplddecoders.DecodeTransaction(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode transaction with CID %s: %w", wantedCid, err)
	}
	return decoded, nil
}

func (ser *Epoch) GetDataFrameByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		return nil, fmt.Errorf("failed to find node by cid %s: %w", wantedCid, err)
	}
	// try parsing the data as a DataFrame node.
	decoded, err := iplddecoders.DecodeDataFrame(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode data frame with CID %s: %w", wantedCid, err)
	}
	return decoded, nil
}

func (ser *Epoch) GetRewardsByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Rewards, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		return nil, fmt.Errorf("failed to find node by cid %s: %w", wantedCid, err)
	}
	// try parsing the data as a Rewards node.
	decoded, err := iplddecoders.DecodeRewards(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode rewards with CID %s: %w", wantedCid, err)
	}
	return decoded, nil
}

func (ser *Epoch) GetTransaction(ctx context.Context, sig solana.Signature) (*ipldbindcode.Transaction, cid.Cid, error) {
	// get the CID by signature
	wantedCid, err := ser.FindCidFromSignature(ctx, sig)
	if err != nil {
		return nil, cid.Cid{}, fmt.Errorf("failed to find CID for signature %s: %w", sig, err)
	}
	{
		doPrefetch := getValueFromContext(ctx, "prefetch")
		if doPrefetch != nil && doPrefetch.(bool) {
			// prefetch the block
			ser.prefetchSubgraph(ctx, wantedCid)
		}
	}
	// get the transaction by CID
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		return nil, cid.Cid{}, fmt.Errorf("failed to get node by cid %s: %w", wantedCid, err)
	}
	// try parsing the data as a Transaction node.
	decoded, err := iplddecoders.DecodeTransaction(data)
	if err != nil {
		return nil, cid.Cid{}, fmt.Errorf("failed to decode transaction with CID %s: %w", wantedCid, err)
	}
	return decoded, wantedCid, nil
}
