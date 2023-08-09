package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/patrickmn/go-cache"
	"github.com/rpcpool/yellowstone-faithful/compactindex"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

type Epoch struct {
	epoch          uint64
	isFilecoinMode bool // true if the epoch is in Filecoin mode (i.e. Lassie mode)
	// contains indexes and block data for the epoch
	lassieFetcher    *lassieWrapper
	localCarReader   *carv2.Reader
	remoteCarReader  ReaderAtCloser
	cidToOffsetIndex *compactindex.DB
	slotToCidIndex   *compactindex36.DB
	sigToCidIndex    *compactindex36.DB
	gsfaReader       *gsfa.GsfaReader
	cidToNodeCache   *cache.Cache // TODO: prevent OOM
	onClose          []func() error
	slotToCidCache   *cache.Cache
	cidToOffsetCache *cache.Cache
}

func (r *Epoch) getSlotToCidFromCache(slot uint64) (cid.Cid, error, bool) {
	if v, ok := r.slotToCidCache.Get(fmt.Sprint(slot)); ok {
		return v.(cid.Cid), nil, true
	}
	return cid.Undef, nil, false
}

func (r *Epoch) putSlotToCidInCache(slot uint64, c cid.Cid) {
	r.slotToCidCache.Set(fmt.Sprint(slot), c, cache.DefaultExpiration)
}

func (r *Epoch) getCidToOffsetFromCache(c cid.Cid) (uint64, error, bool) {
	if v, ok := r.cidToOffsetCache.Get(c.String()); ok {
		return v.(uint64), nil, true
	}
	return 0, nil, false
}

func (r *Epoch) putCidToOffsetInCache(c cid.Cid, offset uint64) {
	r.cidToOffsetCache.Set(c.String(), offset, cache.DefaultExpiration)
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

func NewEpochFromConfig(config *Config, c *cli.Context) (*Epoch, error) {
	if config == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	isLassieMode := config.IsFilecoinMode()
	isCarMode := !isLassieMode

	ep := &Epoch{
		epoch:          *config.Epoch,
		isFilecoinMode: isLassieMode,
		onClose:        make([]func() error, 0),
	}

	if isCarMode {
		// The CAR-mode requires a cid-to-offset index.
		cidToOffsetIndexFile, err := openIndexStorage(c.Context, string(config.Indexes.CidToOffset.URI))
		if err != nil {
			return nil, fmt.Errorf("failed to open cid-to-offset index file: %w", err)
		}
		ep.onClose = append(ep.onClose, cidToOffsetIndexFile.Close)

		cidToOffsetIndex, err := compactindex.Open(cidToOffsetIndexFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open cid-to-offset index: %w", err)
		}
		if config.Indexes.CidToOffset.URI.IsRemoteWeb() {
			cidToOffsetIndex.Prefetch(true)
		}
		ep.cidToOffsetIndex = cidToOffsetIndex
	}

	{
		slotToCidIndexFile, err := openIndexStorage(c.Context, string(config.Indexes.SlotToCid.URI))
		if err != nil {
			return nil, fmt.Errorf("failed to open slot-to-cid index file: %w", err)
		}
		ep.onClose = append(ep.onClose, slotToCidIndexFile.Close)

		slotToCidIndex, err := compactindex36.Open(slotToCidIndexFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open slot-to-cid index: %w", err)
		}
		if config.Indexes.SlotToCid.URI.IsRemoteWeb() {
			slotToCidIndex.Prefetch(true)
		}
		ep.slotToCidIndex = slotToCidIndex
	}

	{
		sigToCidIndexFile, err := openIndexStorage(c.Context, string(config.Indexes.SigToCid.URI))
		if err != nil {
			return nil, fmt.Errorf("failed to open sig-to-cid index file: %w", err)
		}
		ep.onClose = append(ep.onClose, sigToCidIndexFile.Close)

		sigToCidIndex, err := compactindex36.Open(sigToCidIndexFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open sig-to-cid index: %w", err)
		}
		if config.Indexes.SigToCid.URI.IsRemoteWeb() {
			sigToCidIndex.Prefetch(true)
		}
		ep.sigToCidIndex = sigToCidIndex
	}

	{
		if !config.Indexes.Gsfa.URI.IsZero() {
			gsfaIndex, err := gsfa.NewGsfaReader(string(config.Indexes.Gsfa.URI))
			if err != nil {
				return nil, fmt.Errorf("failed to open gsfa index: %w", err)
			}
			ep.onClose = append(ep.onClose, gsfaIndex.Close)
			ep.gsfaReader = gsfaIndex
		}
	}

	if isLassieMode {
		ls, err := newLassieWrapper(c)
		if err != nil {
			return nil, fmt.Errorf("newLassieWrapper: %w", err)
		}
		ep.lassieFetcher = ls
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
	}

	{
		ca := cache.New(30*time.Second, 1*time.Minute)
		ep.cidToNodeCache = ca
	}
	{
		ca := cache.New(30*time.Second, 1*time.Minute)
		ep.slotToCidCache = ca
	}
	{
		ca := cache.New(30*time.Second, 1*time.Minute)
		ep.cidToOffsetCache = ca
	}

	return ep, nil
}

func (r *Epoch) getNodeFromCache(c cid.Cid) (v []byte, err error, has bool) {
	if v, ok := r.cidToNodeCache.Get(c.String()); ok {
		return v.([]byte), nil, true
	}
	return nil, nil, false
}

func (r *Epoch) putNodeInCache(c cid.Cid, data []byte) {
	r.cidToNodeCache.Set(c.String(), data, cache.DefaultExpiration)
}

func (s *Epoch) prefetchSubgraph(ctx context.Context, wantedCid cid.Cid) error {
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

func (s *Epoch) GetNodeByCid(ctx context.Context, wantedCid cid.Cid) ([]byte, error) {
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

func (s *Epoch) GetNodeByOffset(ctx context.Context, wantedCid cid.Cid, offset uint64) ([]byte, error) {
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

func (ser *Epoch) FindCidFromSlot(ctx context.Context, slot uint64) (cid.Cid, error) {
	// try from cache
	if c, err, has := ser.getSlotToCidFromCache(slot); err != nil {
		return cid.Undef, err
	} else if has {
		return c, nil
	}
	found, err := findCidFromSlot(ser.slotToCidIndex, slot)
	if err != nil {
		return cid.Undef, err
	}
	ser.putSlotToCidInCache(slot, found)
	return found, nil
}

func (ser *Epoch) FindCidFromSignature(ctx context.Context, sig solana.Signature) (cid.Cid, error) {
	return findCidFromSignature(ser.sigToCidIndex, sig)
}

func (ser *Epoch) FindOffsetFromCid(ctx context.Context, cid cid.Cid) (uint64, error) {
	// try from cache
	if offset, err, has := ser.getCidToOffsetFromCache(cid); err != nil {
		return 0, err
	} else if has {
		return offset, nil
	}
	found, err := findOffsetFromCid(ser.cidToOffsetIndex, cid)
	if err != nil {
		return 0, err
	}
	ser.putCidToOffsetInCache(cid, found)
	return found, nil
}

func (ser *Epoch) GetBlock(ctx context.Context, slot uint64) (*ipldbindcode.Block, error) {
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