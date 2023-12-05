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

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rpcpool/yellowstone-faithful/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	hugecache "github.com/rpcpool/yellowstone-faithful/huge-cache"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

type Epoch struct {
	epoch          uint64
	isFilecoinMode bool // true if the epoch is in Filecoin mode (i.e. Lassie mode)
	config         *Config
	// contains indexes and block data for the epoch
	lassieFetcher           *lassieWrapper
	localCarReader          *carv2.Reader
	remoteCarReader         ReaderAtCloser
	remoteCarHeaderSize     uint64
	cidToOffsetAndSizeIndex *indexes.CidToOffsetAndSize_Reader
	slotToCidIndex          *indexes.SlotToCid_Reader
	sigToCidIndex           *indexes.SigToCid_Reader
	sigExists               *bucketteer.Reader
	gsfaReader              *gsfa.GsfaReader
	onClose                 []func() error
	allCache                *hugecache.Cache
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

func NewEpochFromConfig(
	config *Config,
	c *cli.Context,
	allCache *hugecache.Cache,
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

	if isCarMode {
		// The CAR-mode requires a cid-to-offset index.
		cidToOffsetAndSizeIndexFile, err := openIndexStorage(
			c.Context,
			string(config.Indexes.CidToOffsetAndSize.URI),
			DebugMode,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to open cid-to-offset index file: %w", err)
		}
		ep.onClose = append(ep.onClose, cidToOffsetAndSizeIndexFile.Close)

		cidToOffsetIndex, err := indexes.OpenWithReader_CidToOffsetAndSize(cidToOffsetAndSizeIndexFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open cid-to-offset index: %w", err)
		}
		if config.Indexes.CidToOffsetAndSize.URI.IsRemoteWeb() {
			cidToOffsetIndex.Prefetch(true)
		}
		ep.cidToOffsetAndSizeIndex = cidToOffsetIndex

		if ep.Epoch() != cidToOffsetIndex.Meta().Epoch {
			return nil, fmt.Errorf("epoch mismatch in cid-to-offset-and-size index: expected %d, got %d", ep.Epoch(), cidToOffsetIndex.Meta().Epoch)
		}
		lastRootCid = cidToOffsetIndex.Meta().RootCid
	}

	{
		slotToCidIndexFile, err := openIndexStorage(
			c.Context,
			string(config.Indexes.SlotToCid.URI),
			DebugMode,
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

		if ep.Epoch() != slotToCidIndex.Meta().Epoch {
			return nil, fmt.Errorf("epoch mismatch in slot-to-cid index: expected %d, got %d", ep.Epoch(), slotToCidIndex.Meta().Epoch)
		}
		if lastRootCid != cid.Undef && !lastRootCid.Equals(slotToCidIndex.Meta().RootCid) {
			return nil, fmt.Errorf("root CID mismatch in slot-to-cid index: expected %s, got %s", lastRootCid, slotToCidIndex.Meta().RootCid)
		}
		lastRootCid = slotToCidIndex.Meta().RootCid
	}

	{
		sigToCidIndexFile, err := openIndexStorage(
			c.Context,
			string(config.Indexes.SigToCid.URI),
			DebugMode,
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

		if ep.Epoch() != sigToCidIndex.Meta().Epoch {
			return nil, fmt.Errorf("epoch mismatch in sig-to-cid index: expected %d, got %d", ep.Epoch(), sigToCidIndex.Meta().Epoch)
		}

		if !lastRootCid.Equals(sigToCidIndex.Meta().RootCid) {
			return nil, fmt.Errorf("root CID mismatch in sig-to-cid index: expected %s, got %s", lastRootCid, sigToCidIndex.Meta().RootCid)
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
		localCarReader, remoteCarReader, err := openCarStorage(c.Context, string(config.Data.Car.URI))
		if err != nil {
			return nil, fmt.Errorf("failed to open CAR file: %w", err)
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
			// read 10 bytes from the CAR file to get the header size
			headerSizeBuf, err := readSectionFromReaderAt(remoteCarReader, 0, 10)
			if err != nil {
				return nil, fmt.Errorf("failed to read CAR header: %w", err)
			}
			// decode as uvarint
			headerSize, n := binary.Uvarint(headerSizeBuf)
			if n <= 0 {
				return nil, fmt.Errorf("failed to decode CAR header size")
			}
			ep.remoteCarHeaderSize = uint64(n) + headerSize
		}
	}
	{
		sigExistsFile, err := openIndexStorage(
			c.Context,
			string(config.Indexes.SigExists.URI),
			DebugMode,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to open sig-exists index file: %w", err)
		}
		ep.onClose = append(ep.onClose, sigExistsFile.Close)

		sigExists, err := bucketteer.NewReader(sigExistsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open sig-exists index: %w", err)
		}
		ep.onClose = append(ep.onClose, sigExists.Close)

		{
			// warm up the cache
			for i := 0; i < 100_000; i++ {
				sigExists.Has(newRandomSignature())
			}
		}

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
		klog.Errorf("failed to get subgraph from lassie: %v", err)
		return err
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
		klog.Errorf("failed to get node from lassie: %v", err)
		return nil, err
	}
	// Find CAR file offset for CID in index.
	offset, err := s.FindOffsetFromCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find offset for CID %s: %v", wantedCid, err)
		// not found or error
		return nil, err
	}
	return s.GetNodeByOffset(ctx, wantedCid, offset)
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
		klog.Errorf("failed to get data reader: %v", err)
		return nil, err
	}
	dr.Seek(int64(offset), io.SeekStart)
	data := make([]byte, length)
	_, err = io.ReadFull(dr, data)
	if err != nil {
		klog.Errorf("failed to read node: %v", err)
		return nil, err
	}
	return data, nil
}

func (s *Epoch) GetNodeByOffset(ctx context.Context, wantedCid cid.Cid, offsetAndSize *indexes.OffsetAndSize) ([]byte, error) {
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
		return readNodeFromReaderAt(s.remoteCarReader, wantedCid, offset, length)
	}
	// Get reader and seek to offset, then read node.
	dr, err := s.localCarReader.DataReader()
	if err != nil {
		klog.Errorf("failed to get data reader: %v", err)
		return nil, err
	}
	dr.Seek(int64(offset), io.SeekStart)
	br := bufio.NewReader(dr)

	return readNodeWithKnownSize(br, wantedCid, length)
}

func readNodeWithKnownSize(br *bufio.Reader, wantedCid cid.Cid, length uint64) ([]byte, error) {
	section := make([]byte, length)
	_, err := io.ReadFull(br, section)
	if err != nil {
		klog.Errorf("failed to read section: %v", err)
		return nil, err
	}
	return parseNodeFromSection(section, wantedCid)
}

func parseNodeFromSection(section []byte, wantedCid cid.Cid) ([]byte, error) {
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
	if !gotCid.Equals(wantedCid) {
		klog.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}
	return data[cidLen:], nil
}

func (ser *Epoch) FindCidFromSlot(ctx context.Context, slot uint64) (o cid.Cid, e error) {
	startedAt := time.Now()
	defer func() {
		klog.Infof("Found CID for slot %d in %s: %s", slot, time.Since(startedAt), o)
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
		klog.Infof("Found CID for signature %s in %s: %s", sig, time.Since(startedAt), o)
	}()
	return ser.sigToCidIndex.Get(sig)
}

func (ser *Epoch) FindOffsetFromCid(ctx context.Context, cid cid.Cid) (os *indexes.OffsetAndSize, e error) {
	startedAt := time.Now()
	defer func() {
		klog.Infof("Found offset and size for CID %s in %s: o=%d s=%d", cid, time.Since(startedAt), os.Offset, os.Size)
	}()

	// try from cache
	if osi, err, has := ser.GetCache().GetCidToOffsetAndSize(cid); err != nil {
		return nil, err
	} else if has {
		return osi, nil
	}
	found, err := ser.cidToOffsetAndSizeIndex.Get(cid)
	if err != nil {
		return nil, err
	}
	// TODO: use also the size.
	ser.GetCache().PutCidToOffsetAndSize(cid, found)
	return found, nil
}

func (ser *Epoch) GetBlock(ctx context.Context, slot uint64) (*ipldbindcode.Block, error) {
	// get the slot by slot number
	wantedCid, err := ser.FindCidFromSlot(ctx, slot)
	if err != nil {
		klog.Errorf("failed to find CID for slot %d: %v", slot, err)
		return nil, err
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
		klog.Errorf("failed to find node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as a Block node.
	decoded, err := iplddecoders.DecodeBlock(data)
	if err != nil {
		klog.Errorf("failed to decode block: %v", err)
		return nil, err
	}
	return decoded, nil
}

func (ser *Epoch) GetEntryByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Entry, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as an Entry node.
	decoded, err := iplddecoders.DecodeEntry(data)
	if err != nil {
		klog.Errorf("failed to decode entry: %v", err)
		return nil, err
	}
	return decoded, nil
}

func (ser *Epoch) GetTransactionByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Transaction, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as a Transaction node.
	decoded, err := iplddecoders.DecodeTransaction(data)
	if err != nil {
		klog.Errorf("failed to decode transaction: %v", err)
		return nil, err
	}
	return decoded, nil
}

func (ser *Epoch) GetDataFrameByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as a DataFrame node.
	decoded, err := iplddecoders.DecodeDataFrame(data)
	if err != nil {
		klog.Errorf("failed to decode data frame: %v", err)
		return nil, err
	}
	return decoded, nil
}

func (ser *Epoch) GetRewardsByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Rewards, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as a Rewards node.
	decoded, err := iplddecoders.DecodeRewards(data)
	if err != nil {
		klog.Errorf("failed to decode rewards: %v", err)
		return nil, err
	}
	return decoded, nil
}

func (ser *Epoch) GetTransaction(ctx context.Context, sig solana.Signature) (*ipldbindcode.Transaction, error) {
	// get the CID by signature
	wantedCid, err := ser.FindCidFromSignature(ctx, sig)
	if err != nil {
		klog.Errorf("failed to find CID for signature %s: %v", sig, err)
		return nil, err
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
		klog.Errorf("failed to get node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as a Transaction node.
	decoded, err := iplddecoders.DecodeTransaction(data)
	if err != nil {
		klog.Errorf("failed to decode transaction: %v", err)
		return nil, err
	}
	return decoded, nil
}
