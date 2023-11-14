package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/patrickmn/go-cache"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/urfave/cli/v2"
	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2"
)

func newCmd_rpcServerCar() *cli.Command {
	var listenOn string
	var gsfaOnlySignatures bool
	return &cli.Command{
		Name:        "rpc-server-car",
		Description: "Start a Solana JSON RPC that exposes getTransaction and getBlock",
		ArgsUsage:   "<car-filepath-or-url> <cid-to-offset-index-filepath-or-url> <slot-to-cid-index-filepath-or-url> <sig-to-cid-index-filepath-or-url> <gsfa-index-dir>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "listen",
				Usage:       "Listen address",
				Value:       ":8899",
				Destination: &listenOn,
			},
			&cli.BoolFlag{
				Name:        "gsfa-only-signatures",
				Usage:       "gSFA: only return signatures",
				Value:       false,
				Destination: &gsfaOnlySignatures,
			},
		},
		Action: func(c *cli.Context) error {
			carFilepath := c.Args().Get(0)
			if carFilepath == "" {
				return cli.Exit("Must provide a CAR filepath", 1)
			}
			cidToOffsetIndexFilepath := c.Args().Get(1)
			if cidToOffsetIndexFilepath == "" {
				return cli.Exit("Must provide a CID-to-offset index filepath/url", 1)
			}
			slotToCidIndexFilepath := c.Args().Get(2)
			if slotToCidIndexFilepath == "" {
				return cli.Exit("Must provide a slot-to-CID index filepath/url", 1)
			}
			sigToCidIndexFilepath := c.Args().Get(3)
			if sigToCidIndexFilepath == "" {
				return cli.Exit("Must provide a signature-to-CID index filepath/url", 1)
			}

			cidToOffsetIndexFile, err := openIndexStorage(
				c.Context,
				cidToOffsetIndexFilepath,
				DebugMode,
			)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer cidToOffsetIndexFile.Close()

			cidToOffsetIndex, err := compactindexsized.Open(cidToOffsetIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			slotToCidIndexFile, err := openIndexStorage(
				c.Context,
				slotToCidIndexFilepath,
				DebugMode,
			)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer slotToCidIndexFile.Close()

			slotToCidIndex, err := compactindex36.Open(slotToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			sigToCidIndexFile, err := openIndexStorage(
				c.Context,
				sigToCidIndexFilepath,
				DebugMode,
			)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer sigToCidIndexFile.Close()

			sigToCidIndex, err := compactindex36.Open(sigToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			localCarReader, remoteCarReader, err := openCarStorage(c.Context, carFilepath)
			if err != nil {
				return fmt.Errorf("failed to open CAR file: %w", err)
			}

			var gsfaIndex *gsfa.GsfaReader
			gsfaIndexDir := c.Args().Get(4)
			if gsfaIndexDir != "" {
				gsfaIndex, err = gsfa.NewGsfaReader(gsfaIndexDir)
				if err != nil {
					return fmt.Errorf("failed to open gsfa index: %w", err)
				}
				defer gsfaIndex.Close()
			}

			options := &RpcServerOptions{
				ListenOn:           listenOn,
				GsfaOnlySignatures: gsfaOnlySignatures,
			}

			return createAndStartRPCServer_withCar(
				c.Context,
				options,
				localCarReader,
				remoteCarReader,
				cidToOffsetIndex,
				slotToCidIndex,
				sigToCidIndex,
				gsfaIndex,
			)
		},
	}
}

// createAndStartRPCServer_withCar creates and starts a JSON RPC server.
// Data:
//   - Nodes: the node data is read from a CAR file (which can be a local file or a remote URL).
//   - Indexes: the indexes are read from files (which can be a local file or a remote URL).
//
// The server is backed by a CAR file (meaning that it can only serve the content of the CAR file).
// It blocks until the server is stopped.
// It returns an error if the server fails to start or stops unexpectedly.
// It returns nil if the server is stopped gracefully.
func createAndStartRPCServer_withCar(
	ctx context.Context,
	options *RpcServerOptions,
	carReader *carv2.Reader,
	remoteCarReader ReaderAtCloser,
	cidToOffsetIndex *compactindexsized.DB,
	slotToCidIndex *compactindex36.DB,
	sigToCidIndex *compactindex36.DB,
	gsfaReader *gsfa.GsfaReader,
) error {
	if options == nil {
		panic("options cannot be nil")
	}
	listenOn := options.ListenOn
	ca := cache.New(30*time.Second, 1*time.Minute)
	handler := &deprecatedRPCServer{
		localCarReader:   carReader,
		remoteCarReader:  remoteCarReader,
		cidToOffsetIndex: cidToOffsetIndex,
		slotToCidIndex:   slotToCidIndex,
		sigToCidIndex:    sigToCidIndex,
		gsfaReader:       gsfaReader,
		cidToBlockCache:  ca,
		options:          options,
	}

	h := newRPCHandler_fast(handler)
	h = fasthttp.CompressHandler(h)

	klog.Infof("RPC server listening on %s", listenOn)
	return fasthttp.ListenAndServe(listenOn, h)
}

func createAndStartRPCServer_lassie(
	ctx context.Context,
	options *RpcServerOptions,
	lassieWr *lassieWrapper,
	slotToCidIndex *compactindex36.DB,
	sigToCidIndex *compactindex36.DB,
	gsfaReader *gsfa.GsfaReader,
) error {
	if options == nil {
		panic("options cannot be nil")
	}
	listenOn := options.ListenOn
	ca := cache.New(30*time.Second, 1*time.Minute)
	handler := &deprecatedRPCServer{
		lassieFetcher:   lassieWr,
		slotToCidIndex:  slotToCidIndex,
		sigToCidIndex:   sigToCidIndex,
		gsfaReader:      gsfaReader,
		cidToBlockCache: ca,
		options:         options,
	}

	h := newRPCHandler_fast(handler)
	h = fasthttp.CompressHandler(h)

	klog.Infof("RPC server listening on %s", listenOn)
	return fasthttp.ListenAndServe(listenOn, h)
}

type RpcServerOptions struct {
	ListenOn           string
	GsfaOnlySignatures bool
}

type deprecatedRPCServer struct {
	lassieFetcher    *lassieWrapper
	localCarReader   *carv2.Reader
	remoteCarReader  ReaderAtCloser
	cidToOffsetIndex *compactindexsized.DB
	slotToCidIndex   *compactindex36.DB
	sigToCidIndex    *compactindex36.DB
	gsfaReader       *gsfa.GsfaReader
	cidToBlockCache  *cache.Cache // TODO: prevent OOM
	options          *RpcServerOptions
}

func getCidCacheKey(off int64, p []byte) string {
	return fmt.Sprintf("%d-%d", off, len(p))
}

func (r *deprecatedRPCServer) getNodeFromCache(c cid.Cid) (v []byte, err error, has bool) {
	if v, ok := r.cidToBlockCache.Get(c.String()); ok {
		return v.([]byte), nil, true
	}
	return nil, nil, false
}

func (r *deprecatedRPCServer) putNodeInCache(c cid.Cid, data []byte) {
	r.cidToBlockCache.Set(c.String(), data, cache.DefaultExpiration)
}

func (s *deprecatedRPCServer) prefetchSubgraph(ctx context.Context, wantedCid cid.Cid) error {
	if s.lassieFetcher != nil {
		// Fetch the subgraph from lassie
		sub, err := s.lassieFetcher.GetSubgraph(ctx, wantedCid)
		if err == nil {
			// put in cache
			return sub.Each(ctx, func(c cid.Cid, data []byte) error {
				s.putNodeInCache(c, data)
				return nil
			})
		}
		klog.Errorf("failed to get subgraph from lassie: %v", err)
		return err
	}
	return nil
}

func (s *deprecatedRPCServer) GetNodeByCid(ctx context.Context, wantedCid cid.Cid) ([]byte, error) {
	{
		// try from cache
		data, err, has := s.getNodeFromCache(wantedCid)
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
			s.putNodeInCache(wantedCid, data)
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

func (s *deprecatedRPCServer) ReadAtFromCar(ctx context.Context, offset uint64, length uint64) ([]byte, error) {
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

func (s *deprecatedRPCServer) GetNodeByOffset(ctx context.Context, wantedCid cid.Cid, offset uint64) ([]byte, error) {
	if s.localCarReader == nil {
		// try remote reader
		if s.remoteCarReader == nil {
			return nil, fmt.Errorf("no CAR reader available")
		}
		return readNodeFromReaderAt(s.remoteCarReader, wantedCid, offset)
	}
	// Get reader and seek to offset, then read node.
	dr, err := s.localCarReader.DataReader()
	if err != nil {
		klog.Errorf("failed to get data reader: %v", err)
		return nil, err
	}
	dr.Seek(int64(offset), io.SeekStart)
	br := bufio.NewReader(dr)

	gotCid, data, err := util.ReadNode(br)
	if err != nil {
		klog.Errorf("failed to read node: %v", err)
		return nil, err
	}
	// verify that the CID we read matches the one we expected.
	if !gotCid.Equals(wantedCid) {
		klog.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}
	return data, nil
}

func (ser *deprecatedRPCServer) FindCidFromSlot(ctx context.Context, slot uint64) (cid.Cid, error) {
	return findCidFromSlot(ser.slotToCidIndex, slot)
}

func (ser *deprecatedRPCServer) FindCidFromSignature(ctx context.Context, sig solana.Signature) (cid.Cid, error) {
	return findCidFromSignature(ser.sigToCidIndex, sig)
}

func (ser *deprecatedRPCServer) FindOffsetFromCid(ctx context.Context, cid cid.Cid) (uint64, error) {
	return findOffsetFromCid(ser.cidToOffsetIndex, cid)
}

func (ser *deprecatedRPCServer) GetBlock(ctx context.Context, slot uint64) (*ipldbindcode.Block, error) {
	// get the slot by slot number
	wantedCid, err := ser.FindCidFromSlot(ctx, slot)
	if err != nil {
		klog.Errorf("failed to find CID for slot %d: %v", slot, err)
		return nil, err
	}
	klog.Infof("found CID for slot %d: %s", slot, wantedCid)
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

func (ser *deprecatedRPCServer) GetEntryByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Entry, error) {
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

func (ser *deprecatedRPCServer) GetTransactionByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Transaction, error) {
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

func (ser *deprecatedRPCServer) GetDataFrameByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
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

func (ser *deprecatedRPCServer) GetRewardsByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Rewards, error) {
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

func (ser *deprecatedRPCServer) GetTransaction(ctx context.Context, sig solana.Signature) (*ipldbindcode.Transaction, error) {
	// get the CID by signature
	wantedCid, err := ser.FindCidFromSignature(ctx, sig)
	if err != nil {
		klog.Errorf("failed to find CID for signature %s: %v", sig, err)
		return nil, err
	}
	klog.Infof("found CID for signature %s: %s", sig, wantedCid)
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

// jsonrpc2.RequestHandler interface
func (ser *deprecatedRPCServer) Handle(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
	switch req.Method {
	case "getBlock":
		ser.handleGetBlock(ctx, conn, req)
	case "getTransaction":
		ser.handleGetTransaction(ctx, conn, req)
	case "getSignaturesForAddress":
		ser.handleGetSignaturesForAddress(ctx, conn, req)
	default:
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeMethodNotFound,
				Message: "Method not found",
			})
	}
}
