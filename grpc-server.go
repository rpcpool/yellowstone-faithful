package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sort"
	"sync"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip" // Install the gzip compressor
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const maxSlotsToStream uint64 = 100

// ListeAndServe starts listening on the configured address and serves the RPC API.
func (me *MultiEpoch) ListenAndServeGRPC(ctx context.Context, listenOn string) error {
	lis, err := net.Listen("tcp", listenOn)
	if err != nil {
		return fmt.Errorf("failed to create listener for gRPC server: %w", err)
	}

	grpcServer := grpc.NewServer()
	old_faithful_grpc.RegisterOldFaithfulServer(grpcServer, me)

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve gRPC server: %w", err)
	}
	return nil
}

func (me *MultiEpoch) GetVersion(context.Context, *old_faithful_grpc.VersionRequest) (*old_faithful_grpc.VersionResponse, error) {
	// faithfulVersion["version"] = GitTag
	// faithfulVersion["commit"] = GitCommit
	// faithfulVersion["epochs"] = me.GetEpochNumbers()
	resp := &old_faithful_grpc.VersionResponse{
		Version: func() string {
			if GitTag == "" {
				return GitCommit
			}
			return GitTag
		}(),
	}
	return resp, nil
}

func (multi *MultiEpoch) GetBlock(ctx context.Context, params *old_faithful_grpc.BlockRequest) (*old_faithful_grpc.BlockResponse, error) {
	// find the epoch that contains the requested slot
	slot := params.Slot
	epochNumber := slottools.CalcEpochForSlot(slot)
	epochHandler, err := multi.GetEpoch(epochNumber)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Epoch %d is not available", epochNumber)
	}

	block, _, err := epochHandler.GetBlock(WithSubrapghPrefetch(ctx, true), slot)
	if err != nil {
		if errors.Is(err, compactindexsized.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "Slot %d was skipped, or missing in long-term storage", slot)
		} else {
			return nil, status.Errorf(codes.Internal, "Failed to get block: %v", err)
		}
	}

	tim := newTimer(getRequestIDFromContext(ctx))
	tim.time("GetBlock")
	{
		prefetcherFromCar := func() error {
			parentIsInPreviousEpoch := slottools.CalcEpochForSlot(uint64(block.Meta.Parent_slot)) != slottools.CalcEpochForSlot(slot)
			if slot == 0 {
				parentIsInPreviousEpoch = true
			}
			if slot > 1 && block.Meta.Parent_slot == 0 {
				parentIsInPreviousEpoch = true
			}

			var blockCid, parentBlockCid cid.Cid
			wg := new(errgroup.Group)
			wg.Go(func() (err error) {
				blockCid, err = epochHandler.FindCidFromSlot(ctx, slot)
				if err != nil {
					return err
				}
				return nil
			})
			wg.Go(func() (err error) {
				if parentIsInPreviousEpoch {
					return nil
				}
				parentBlockCid, err = epochHandler.FindCidFromSlot(ctx, uint64(block.Meta.Parent_slot))
				if err != nil {
					return err
				}
				return nil
			})
			err = wg.Wait()
			if err != nil {
				return err
			}
			if slot == 0 {
				klog.V(4).Infof("car start to slot(0)::%s", blockCid)
			} else {
				klog.V(4).Infof(
					"slot(%d)::%s to slot(%d)::%s",
					uint64(block.Meta.Parent_slot),
					parentBlockCid,
					slot,
					blockCid,
				)
			}
			{
				var blockOffset, parentOffset uint64
				wg := new(errgroup.Group)
				wg.Go(func() (err error) {
					offsetAndSize, err := epochHandler.FindOffsetAndSizeFromCid(ctx, blockCid)
					if err != nil {
						return err
					}
					blockOffset = offsetAndSize.Offset
					return nil
				})
				wg.Go(func() (err error) {
					if parentIsInPreviousEpoch {
						// get car file header size
						parentOffset = epochHandler.carHeaderSize
						return nil
					}
					offsetAndSize, err := epochHandler.FindOffsetAndSizeFromCid(ctx, parentBlockCid)
					if err != nil {
						return err
					}
					parentOffset = offsetAndSize.Offset
					return nil
				})
				err = wg.Wait()
				if err != nil {
					return err
				}

				length := blockOffset - parentOffset
				MiB := uint64(1024 * 1024)
				maxPrefetchSize := MiB * 10 // let's cap prefetching size
				if length > maxPrefetchSize {
					length = maxPrefetchSize
				}

				start := parentOffset

				klog.V(4).Infof("prefetching CAR: start=%d length=%d (parent_offset=%d)", start, length, parentOffset)
				carSection, err := epochHandler.ReadAtFromCar(ctx, start, length)
				if err != nil {
					return err
				}
				dr := bytes.NewReader(carSection)
				br := bufio.NewReader(dr)

				gotCid, data, err := util.ReadNode(br)
				if err != nil {
					return fmt.Errorf("failed to read first node: %w", err)
				}
				if !parentIsInPreviousEpoch && !gotCid.Equals(parentBlockCid) {
					return fmt.Errorf("CID mismatch: expected %s, got %s", parentBlockCid, gotCid)
				}
				epochHandler.GetCache().PutRawCarObject(gotCid, data)

				for {
					gotCid, data, err = util.ReadNode(br)
					if err != nil {
						if errors.Is(err, io.EOF) {
							break
						}
						return fmt.Errorf("failed to read node: %w", err)
					}
					if gotCid.Equals(blockCid) {
						break
					}
					epochHandler.GetCache().PutRawCarObject(gotCid, data)
				}
			}
			return nil
		}
		if epochHandler.lassieFetcher == nil {
			err := prefetcherFromCar()
			if err != nil {
				klog.Errorf("failed to prefetch from car: %v", err)
			}
		}
	}

	allTransactionNodes := make([][]*ipldbindcode.Transaction, len(block.Entries))
	mu := &sync.Mutex{}
	var lastEntryHash solana.Hash
	{
		wg := new(errgroup.Group)
		wg.SetLimit(runtime.NumCPU() * 2)
		// get entries from the block
		for entryIndex, entry := range block.Entries {
			entryIndex := entryIndex
			entryCid := entry.(cidlink.Link).Cid
			wg.Go(func() error {
				// get the entry by CID
				entryNode, err := epochHandler.GetEntryByCid(ctx, entryCid)
				if err != nil {
					klog.Errorf("failed to decode Entry: %v", err)
					return err
				}

				if entryIndex == len(block.Entries)-1 {
					lastEntryHash = solana.HashFromBytes(entryNode.Hash)
				}

				twg := new(errgroup.Group)
				twg.SetLimit(runtime.NumCPU())
				// get the transactions from the entry
				allTransactionNodes[entryIndex] = make([]*ipldbindcode.Transaction, len(entryNode.Transactions))
				for txI := range entryNode.Transactions {
					txI := txI
					tx := entryNode.Transactions[txI]
					twg.Go(func() error {
						// get the transaction by CID
						tcid := tx.(cidlink.Link).Cid
						txNode, err := epochHandler.GetTransactionByCid(ctx, tcid)
						if err != nil {
							klog.Errorf("failed to decode Transaction %s: %v", tcid, err)
							return nil
						}
						mu.Lock()
						allTransactionNodes[entryIndex][txI] = txNode
						mu.Unlock()
						return nil
					})
				}
				return twg.Wait()
			})
		}
		err = wg.Wait()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get entries: %v", err)
		}
	}
	tim.time("get entries")

	resp := &old_faithful_grpc.BlockResponse{
		Slot: uint64(block.Slot),
	}

	var allTransactions []*old_faithful_grpc.Transaction
	hasRewards := !block.Rewards.(cidlink.Link).Cid.Equals(DummyCID)
	if hasRewards {
		rewardsNode, err := epochHandler.GetRewardsByCid(ctx, block.Rewards.(cidlink.Link).Cid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get Rewards: %v", err)
		}
		rewardsBuf, err := tooling.LoadDataFromDataFrames(&rewardsNode.Data, epochHandler.GetDataFrameByCid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to load Rewards dataFrames: %v", err)
		}

		uncompressedRawRewards, err := tooling.DecompressZstd(rewardsBuf)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to decompress Rewards: %v", err)
		}
		resp.Rewards = uncompressedRawRewards
	}
	tim.time("get rewards")
	{
		for _, transactionNode := range mergeTxNodeSlices(allTransactionNodes) {
			txResp := new(old_faithful_grpc.Transaction)

			// response.Slot = uint64(transactionNode.Slot)
			// if blocktime != 0 {
			// 	response.Blocktime = &blocktime
			// }

			{
				pos, ok := transactionNode.GetPositionIndex()
				if ok {
					txResp.Index = ptrToUint64(uint64(pos))
				}
				txResp.Transaction, txResp.Meta, err = getTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
				}
			}

			allTransactions = append(allTransactions, txResp)
		}
	}

	sort.Slice(allTransactions, func(i, j int) bool {
		if allTransactions[i].Index == nil || allTransactions[j].Index == nil {
			return false
		}
		return *allTransactions[i].Index < *allTransactions[j].Index
	})
	tim.time("get transactions")
	resp.Transactions = allTransactions
	blocktime := uint64(block.Meta.Blocktime)
	if blocktime != 0 {
		resp.BlockTime = int64(blocktime)
	}
	resp.Blockhash = lastEntryHash[:]
	resp.ParentSlot = uint64(block.Meta.Parent_slot)

	if slot == 0 {
		genesis := epochHandler.GetGenesis()
		if genesis != nil {
			blockZeroBlocktime := uint64(genesis.Config.CreationTime.Unix())
			resp.BlockTime = int64(blockZeroBlocktime)
		}
		resp.ParentSlot = uint64(0)

		zeroBlockHeight := uint64(0)
		resp.BlockHeight = zeroBlockHeight

		blockZeroBlockHash := lastEntryHash
		resp.PreviousBlockhash = blockZeroBlockHash[:] // NOTE: this is what solana RPC does. Should it be nil instead? Or should it be the genesis hash?
	}

	{
		blockHeight, ok := block.GetBlockHeight()
		if ok {
			resp.BlockHeight = blockHeight
		}
	}
	{
		// get parent slot
		parentSlot := uint64(block.Meta.Parent_slot)
		if (parentSlot != 0 || slot == 1) && slottools.CalcEpochForSlot(parentSlot) == epochNumber {
			// NOTE: if the parent is in the same epoch, we can get it from the same epoch handler as the block;
			// otherwise, we need to get it from the previous epoch (TODO: implement this)
			parentBlock, _, err := epochHandler.GetBlock(WithSubrapghPrefetch(ctx, false), parentSlot)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to get parent block: %v", err)
			}

			if len(parentBlock.Entries) > 0 {
				lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1]
				parentEntryNode, err := epochHandler.GetEntryByCid(ctx, lastEntryCidOfParent.(cidlink.Link).Cid)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "Failed to get parent entry: %v", err)
				}
				parentEntryHash := solana.HashFromBytes(parentEntryNode.Hash)
				resp.PreviousBlockhash = parentEntryHash[:]
			}
		} else {
			if slot != 0 {
				klog.V(4).Infof("parent slot is in a different epoch, not implemented yet (can't get previousBlockhash)")
			}
		}
	}
	tim.time("get parent block")

	return resp, nil
}

func (multi *MultiEpoch) GetTransaction(ctx context.Context, params *old_faithful_grpc.TransactionRequest) (*old_faithful_grpc.TransactionResponse, error) {
	if multi.CountEpochs() == 0 {
		return nil, status.Errorf(codes.Internal, "no epochs available")
	}

	sig := solana.SignatureFromBytes(params.Signature)

	startedEpochLookupAt := time.Now()
	epochNumber, err := multi.findEpochNumberFromSignature(ctx, sig)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// solana just returns null here in case of transaction not found: {"jsonrpc":"2.0","result":null,"id":1}
			return nil, status.Errorf(codes.NotFound, "Transaction not found")
		}
		return nil, status.Errorf(codes.Internal, "Failed to get epoch for signature %s: %v", sig, err)
	}
	klog.V(4).Infof("Found signature %s in epoch %d in %s", sig, epochNumber, time.Since(startedEpochLookupAt))

	epochHandler, err := multi.GetEpoch(uint64(epochNumber))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Epoch %d is not available from this RPC", epochNumber)
	}

	transactionNode, _, err := epochHandler.GetTransaction(WithSubrapghPrefetch(ctx, true), sig)
	if err != nil {
		if errors.Is(err, compactindexsized.ErrNotFound) {
			// NOTE: solana just returns null here in case of transaction not found: {"jsonrpc":"2.0","result":null,"id":1}
			return nil, status.Errorf(codes.NotFound, "Transaction not found")
		}
		return nil, status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
	}

	response := &old_faithful_grpc.TransactionResponse{
		Transaction: &old_faithful_grpc.Transaction{},
	}
	response.Slot = uint64(transactionNode.Slot)
	{
		blocktimeIndex := epochHandler.GetBlocktimeIndex()
		if blocktimeIndex != nil {
			blocktime, err := blocktimeIndex.Get(uint64(transactionNode.Slot))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to get blocktime: %v", err)
			}
			response.BlockTime = int64(blocktime)
		} else {
			return nil, status.Errorf(codes.Internal, "Failed to get blocktime: blocktime index is nil")
		}
	}

	{
		pos, ok := transactionNode.GetPositionIndex()
		if ok {
			response.Index = ptrToUint64(uint64(pos))
		}
		response.Transaction.Transaction, response.Transaction.Meta, err = getTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
		}
	}

	return response, nil
}

func (multi *MultiEpoch) Get(ser old_faithful_grpc.OldFaithful_GetServer) error {
	ctx := ser.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := ser.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		id := req.GetId()

		switch req.Request.(type) {
		case *old_faithful_grpc.GetRequest_Block:
			params := req.GetBlock()
			resp, err := multi.GetBlock(ctx, params)
			if err != nil {
				gerr, ok := status.FromError(err)
				if ok {
					err := ser.Send(&old_faithful_grpc.GetResponse{
						Id: id,
						Response: &old_faithful_grpc.GetResponse_Error{
							Error: &old_faithful_grpc.GetResponseError{
								Code: func() old_faithful_grpc.GetResponseErrorCode {
									switch gerr.Code() {
									case codes.NotFound:
										return old_faithful_grpc.GetResponseErrorCode_NOT_FOUND
									default:
										return old_faithful_grpc.GetResponseErrorCode_INTERNAL
									}
								}(),
								Message: gerr.Message(),
							},
						},
					})
					if err != nil {
						return status.Errorf(codes.Internal, "request %d; failed to send block error response: %v", id, err)
					}
					continue
				}
				return status.Errorf(codes.Internal, "request %d; failed to get block: %v", id, err)
			}
			if err := ser.Send(&old_faithful_grpc.GetResponse{
				Id:       id,
				Response: &old_faithful_grpc.GetResponse_Block{Block: resp},
			}); err != nil {
				return status.Errorf(codes.Internal, "request %d; failed to send block response: %v", id, err)
			}
		case *old_faithful_grpc.GetRequest_Transaction:
			params := req.GetTransaction()
			resp, err := multi.GetTransaction(ctx, params)
			if err != nil {
				gerr, ok := status.FromError(err)
				if ok {
					err := ser.Send(&old_faithful_grpc.GetResponse{
						Id: id,
						Response: &old_faithful_grpc.GetResponse_Error{
							Error: &old_faithful_grpc.GetResponseError{
								Code: func() old_faithful_grpc.GetResponseErrorCode {
									switch gerr.Code() {
									case codes.NotFound:
										return old_faithful_grpc.GetResponseErrorCode_NOT_FOUND
									default:
										return old_faithful_grpc.GetResponseErrorCode_INTERNAL
									}
								}(),
								Message: gerr.Message(),
							},
						},
					})
					if err != nil {
						return status.Errorf(codes.Internal, "request %d; failed to send transaction error response: %v", id, err)
					}
					continue
				}
				return status.Errorf(codes.Internal, "request %d; failed to get transaction: %v", id, err)
			}
			if err := ser.Send(&old_faithful_grpc.GetResponse{
				Id:       id,
				Response: &old_faithful_grpc.GetResponse_Transaction{Transaction: resp},
			}); err != nil {
				return status.Errorf(codes.Internal, "request %d; failed to send transaction response: %v", id, err)
			}
		case *old_faithful_grpc.GetRequest_Version:
			params := req.GetVersion()
			resp, err := multi.GetVersion(ctx, params)
			if err != nil {
				gerr, ok := status.FromError(err)
				if ok {
					err := ser.Send(&old_faithful_grpc.GetResponse{
						Id: id,
						Response: &old_faithful_grpc.GetResponse_Error{
							Error: &old_faithful_grpc.GetResponseError{
								Code: func() old_faithful_grpc.GetResponseErrorCode {
									switch gerr.Code() {
									case codes.NotFound:
										return old_faithful_grpc.GetResponseErrorCode_NOT_FOUND
									default:
										return old_faithful_grpc.GetResponseErrorCode_INTERNAL
									}
								}(),
								Message: gerr.Message(),
							},
						},
					})
					if err != nil {
						return status.Errorf(codes.Internal, "request %d; failed to send version error response: %v", id, err)
					}
					continue
				}
				return status.Errorf(codes.Internal, "request %d; failed to get version: %v", id, err)
			}
			if err := ser.Send(&old_faithful_grpc.GetResponse{
				Id:       id,
				Response: &old_faithful_grpc.GetResponse_Version{Version: resp},
			}); err != nil {
				return status.Errorf(codes.Internal, "request %d; failed to send version response: %v", id, err)
			}
		case *old_faithful_grpc.GetRequest_BlockTime:
			params := req.GetBlockTime()
			resp, err := multi.GetBlockTime(ctx, params)
			if err != nil {
				gerr, ok := status.FromError(err)
				if ok {
					err := ser.Send(&old_faithful_grpc.GetResponse{
						Id: id,
						Response: &old_faithful_grpc.GetResponse_Error{
							Error: &old_faithful_grpc.GetResponseError{
								Code: func() old_faithful_grpc.GetResponseErrorCode {
									switch gerr.Code() {
									case codes.NotFound:
										return old_faithful_grpc.GetResponseErrorCode_NOT_FOUND
									}
									return old_faithful_grpc.GetResponseErrorCode_INTERNAL
								}(),
								Message: gerr.Message(),
							},
						},
					})
					if err != nil {
						return status.Errorf(codes.Internal, "request %d; failed to send blocktime error response: %v", id, err)
					}
					continue
				}
				return status.Errorf(codes.Internal, "request %d; failed to get blocktime: %v", id, err)
			}
			if err := ser.Send(&old_faithful_grpc.GetResponse{
				Id:       id,
				Response: &old_faithful_grpc.GetResponse_BlockTime{BlockTime: resp},
			}); err != nil {
				return status.Errorf(codes.Internal, "request %d; failed to send blocktime response: %v", id, err)
			}
		default:
			return status.Errorf(codes.InvalidArgument, "unknown request type %T", req.Request)
		}
	}
}

func (multi *MultiEpoch) GetBlockTime(ctx context.Context, params *old_faithful_grpc.BlockTimeRequest) (*old_faithful_grpc.BlockTimeResponse, error) {
	slot := params.Slot
	epochNumber := slottools.CalcEpochForSlot(slot)
	epochHandler, err := multi.GetEpoch(epochNumber)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Epoch %d is not available", epochNumber)
	}

	blocktimeIndex := epochHandler.GetBlocktimeIndex()
	if blocktimeIndex != nil {
		blocktime, err := blocktimeIndex.Get(slot)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get block: %v", err)
		}
		return &old_faithful_grpc.BlockTimeResponse{
			BlockTime: blocktime,
		}, nil
	} else {
		return nil, status.Errorf(codes.Internal, "Failed to get block: blocktime index is not available")
	}
}

func (multi *MultiEpoch) StreamBlocks(params *old_faithful_grpc.StreamBlocksRequest, ser old_faithful_grpc.OldFaithful_StreamBlocksServer) error {
	ctx := ser.Context()

	startSlot := params.StartSlot
	endSlot := startSlot + maxSlotsToStream

	if params.EndSlot != nil {
		endSlot = *params.EndSlot
	}

	filterFunc := func(block *old_faithful_grpc.BlockResponse) bool {
		if params.Filter == nil || len(params.Filter.AccountInclude) == 0 {
			return true
		}

		return blockContainsAccounts(block, params.Filter.AccountInclude)
	}

	for slot := startSlot; slot <= endSlot; slot++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		block, err := multi.GetBlock(ctx, &old_faithful_grpc.BlockRequest{Slot: slot})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				continue // is this the right thing to do?
			}
			return err
		}

		if filterFunc(block) {
			if err := ser.Send(block); err != nil {
				return err
			}
		}
	}

	return nil
}

func blockContainsAccounts(block *old_faithful_grpc.BlockResponse, accounts []string) bool {
	accountSet := make(map[string]struct{}, len(accounts))
	for _, acc := range accounts {
		accountSet[acc] = struct{}{}
	}

	for _, tx := range block.Transactions {
		decoder := bin.NewBinDecoder(tx.GetTransaction())
		solTx, err := solana.TransactionFromDecoder(decoder)
		if err != nil {
			klog.Errorf("Failed to decode transaction: %v", err)
			continue
		}

		for _, acc := range solTx.Message.AccountKeys {
			if _, exists := accountSet[acc.String()]; exists {
				return true
			}
		}

		meta, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(tx.Meta)
		if err != nil {
			klog.Errorf("Failed to parse transaction meta: %v", err)
		}

		loadedAccounts := meta.GetLoadedAccounts()
		keys := byteSlicesToKeySlice(loadedAccounts)
		for _, key := range keys {
			if _, exists := accountSet[key.String()]; exists {
				return true
			}
		}
	}

	return false
}

func (multi *MultiEpoch) StreamTransactions(params *old_faithful_grpc.StreamTransactionsRequest, ser old_faithful_grpc.OldFaithful_StreamTransactionsServer) error {
	ctx := ser.Context()

	ctx, overallCancel := context.WithTimeout(ctx, 60*time.Second)
	defer overallCancel()

	startSlot := params.StartSlot
	endSlot := startSlot + maxSlotsToStream

	if params.EndSlot != nil {
		endSlot = *params.EndSlot
	}
	gsfaReader, epochNums := multi.getGsfaReadersInEpochDescendingOrderForSlotRange(ctx, startSlot, endSlot)

	gsfaReadersLoaded := true
	if len(epochNums) == 0 {
		klog.V(2).Info("No gsfa readers were loaded")
		gsfaReadersLoaded = false
	}

	return multi.processSlotTransactions(ctx, ser, startSlot, endSlot, params.Filter, gsfaReader, gsfaReadersLoaded)
}

func (multi *MultiEpoch) processSlotTransactions(
	ctx context.Context,
	ser old_faithful_grpc.OldFaithful_StreamTransactionsServer,
	startSlot uint64,
	endSlot uint64,
	filter *old_faithful_grpc.StreamTransactionsFilter,
	gsfaReader *gsfa.GsfaReaderMultiepoch,
	gsfaReadersLoaded bool,
) error {

	filterOutTxn := func(tx solana.Transaction, meta any) bool {
		if filter == nil {
			return true
		}

		if filter.Vote != nil && !(*filter.Vote) && IsSimpleVoteTransaction(&tx) { // If vote is false, we should filter out vote transactions
			return false
		}

		if filter.Failed != nil && !(*filter.Failed) { // If failed is false, we should filter out failed transactions
			err := getErr(meta)
			if err != nil {
				return false
			}
		}

		if !gsfaReadersLoaded { // Only needed if gsfaReaders not loaded, otherwise handled in the main branch
			hasOne := false
			for _, acc := range filter.AccountInclude {
				pkey := solana.MustPublicKeyFromBase58(acc)
				ok, err := tx.HasAccount(pkey)
				if err != nil {
					klog.V(2).Infof("Failed to check if transaction %v has account %s", tx, acc)
					return false
				}
				if ok {
					hasOne = true
					break // Found at least one included account, no need to check others
				}
			}
			if !hasOne { // If none of the included accounts are present, filter out the transaction
				return false
			}
		}

		for _, acc := range filter.AccountExclude {
			pkey := solana.MustPublicKeyFromBase58(acc)
			ok, err := tx.HasAccount(pkey)
			if err != nil {
				klog.V(2).Infof("Failed to check if transaction %v has account %s", tx, acc)
				return false
			}
			if ok { // If any excluded account is present, filter out the transaction
				return false
			}
		}

		for _, acc := range filter.AccountRequired {
			pkey := solana.MustPublicKeyFromBase58(acc)
			ok, err := tx.HasAccount(pkey)
			if err != nil {
				klog.V(2).Infof("Failed to check if transaction %v has account %s", tx, acc)
				return false
			}
			if !ok { // If any required account is missing, filter out the transaction
				return false
			}
		}

		return true
	}

	if filter == nil || len(filter.AccountInclude) == 0 || !gsfaReadersLoaded {

		for slot := startSlot; slot <= endSlot; slot++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			block, err := multi.GetBlock(ctx, &old_faithful_grpc.BlockRequest{Slot: slot})
			if err != nil {
				if status.Code(err) == codes.NotFound {
					return nil
				}
				return err
			}

			for _, tx := range block.Transactions {
				decoder := bin.NewBinDecoder(tx.GetTransaction())
				txn, err := solana.TransactionFromDecoder(decoder)
				if err != nil {
					return status.Errorf(codes.Internal, "Failed to decode transaction: %v", err)
				}

				meta, err := solanatxmetaparsers.ParseAnyTransactionStatusMeta(tx.Meta)
				if err != nil {
					return status.Errorf(codes.Internal, "Failed to parse transaction meta: %v", err)
				}

				if !filterOutTxn(*txn, meta) {

					txResp := new(old_faithful_grpc.TransactionResponse)
					txResp.Transaction = new(old_faithful_grpc.Transaction)

					{
						txResp.Transaction.Transaction = tx.Transaction
						txResp.Transaction.Meta = tx.Meta
						txResp.Transaction.Index = tx.Index

						epochNumber := slottools.CalcEpochForSlot(slot)
						epochHandler, err := multi.GetEpoch(epochNumber)
						if err != nil {
							return status.Errorf(codes.NotFound, "Epoch %d is not available", epochNumber)
						}
						blocktimeIndex := epochHandler.GetBlocktimeIndex()
						if blocktimeIndex != nil {
							blocktime, err := blocktimeIndex.Get(uint64(slot))
							if err != nil {
								return status.Errorf(codes.Internal, "Failed to get blocktime: %v", err)
							}
							txResp.BlockTime = int64(blocktime)
						} else {
							return status.Errorf(codes.Internal, "Failed to get blocktime: blocktime index is nil")
						}
					}

					if err := ser.Send(txResp); err != nil {
						return err
					}
				}
			}
		}
		return nil
	} else {

		const batchSize = 100
		buffer := newTxBuffer(uint64(startSlot), uint64(endSlot))
		errChan := make(chan error, len(filter.AccountInclude))

		var wg sync.WaitGroup

		const maxConcurrentAccounts = 10
		sem := make(chan struct{}, maxConcurrentAccounts)

		for _, account := range filter.AccountInclude {
			sem <- struct{}{} // Acquire token
			wg.Add(1)

			go func(acc string) {
				defer func() {
					<-sem // Release token
					wg.Done()
				}()

				pKey := solana.MustPublicKeyFromBase58(acc)

				queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				startTime := time.Now()
				klog.V(2).Infof("Starting GSFA query for account %s, from slot %d to %d", pKey.String(), startSlot, endSlot)

				epochToTxns, err := gsfaReader.GetBeforeUntilSlot(
					queryCtx,
					pKey,
					batchSize,
					endSlot+1, //  Before (exclusive)
					startSlot, // Until (inclusive)
					func(epochNum uint64, oas linkedlog.OffsetAndSizeAndSlot) (*ipldbindcode.Transaction, error) {
						fnStartTime := time.Now()
						defer func() {
							klog.V(4).Infof("GSFA transaction lookup for epoch %d took %s",
								epochNum, time.Since(fnStartTime))
						}()
						epoch, err := multi.GetEpoch(epochNum)
						if err != nil {
							return nil, fmt.Errorf("failed to get epoch %d: %w", epochNum, err)
						}
						raw, err := epoch.GetNodeByOffsetAndSize(ctx, nil, &indexes.OffsetAndSize{
							Offset: oas.Offset,
							Size:   oas.Size,
						})
						if err != nil {
							return nil, fmt.Errorf("failed to get signature: %w", err)
						}
						decoded, err := iplddecoders.DecodeTransaction(raw)
						if err != nil {
							return nil, fmt.Errorf("error while decoding transaction from nodex at offset %d: %w", oas.Offset, err)
						}
						return decoded, nil
					},
				)
				if err != nil {
					errChan <- err
					return
				}
				duration := time.Since(startTime)
				klog.V(2).Infof("GSFA query completed for account %s, from slot %d to %d took %s", pKey.String(), startSlot, endSlot, duration)

				for epochNumber, txns := range epochToTxns {
					epochHandler, err := multi.GetEpoch(epochNumber)
					if err != nil {
						errChan <- status.Errorf(codes.NotFound, "Epoch %d is not available", epochNumber)
						return
					}

					for _, txn := range txns {
						if txn == nil {
							klog.V(2).Infof("Skipping nil transaction from epoch %d", epochNumber)
							continue
						}

						txStartTime := time.Now()
						tx, meta, err := parseTransactionAndMetaFromNode(txn, epochHandler.GetDataFrameByCid)
						klog.V(3).Infof("Parsing transaction for account %s took %s", pKey.String(), time.Since(txStartTime))
						if err != nil {
							errChan <- status.Errorf(codes.Internal, "Failed to parse transaction from node: %v", err)
							return
						}

						if !filterOutTxn(tx, meta) {
							txResp := new(old_faithful_grpc.TransactionResponse)
							txResp.Transaction = new(old_faithful_grpc.Transaction)
							{
								pos, ok := txn.GetPositionIndex()
								if ok {
									txResp.Index = ptrToUint64(uint64(pos))
									txResp.Transaction.Index = ptrToUint64(uint64(pos))
								}
								txResp.Transaction.Transaction, txResp.Transaction.Meta, err = getTransactionAndMetaFromNode(txn, epochHandler.GetDataFrameByCid)
								if err != nil {
									errChan <- status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
									return
								}
								txResp.Slot = uint64(txn.Slot)

								blocktimeIndex := epochHandler.GetBlocktimeIndex()
								if blocktimeIndex != nil {
									blocktime, err := blocktimeIndex.Get(uint64(txn.Slot))
									if err != nil {
										errChan <- status.Errorf(codes.Internal, "Failed to get blocktime: %v", err)
										return
									}
									txResp.BlockTime = int64(blocktime)
								} else {
									errChan <- status.Errorf(codes.Internal, "Failed to get blocktime: blocktime index is nil")
									return
								}
							}

							buffer.add(txResp.Slot, *txResp.Index, txResp)
						}
					}
				}
			}(account)
		}

		// Wait for all processing to complete
		wg.Wait()

		// Flush after all processing is done
		klog.V(2).Infof("Starting buffer flush with %d slots of transactions", len(buffer.items))
		flushStartTime := time.Now()
		if err := buffer.flush(ser); err != nil {
			return err
		}
		klog.V(2).Infof("Buffer flush completed in %s", time.Since(flushStartTime))

		// Handle any errors
		klog.V(2).Infof("Checking for errors from goroutines")
		errCheckStartTime := time.Now()
		select {
		case err := <-errChan:
			if err != nil {
				klog.Infof("Received error from error channel: %v", err)
				return err
			}
			klog.V(3).Infof("Received nil error from error channel")
		case <-ctx.Done():
			klog.V(2).Infof("Context done while checking errors")
			return ctx.Err()
		default:
			klog.V(3).Infof("No errors found in channel")
		}
		klog.V(3).Infof("Error check completed in %s", time.Since(errCheckStartTime))

		// If we got here with no transactions (buffer is empty), send an empty response
		if len(buffer.items) == 0 {
			klog.V(2).Infof("No transactions found for the requested accounts, sending empty response")
			emptyResp := &old_faithful_grpc.TransactionResponse{
				Slot: startSlot,
				// Include other required fields as needed
			}
			if err := ser.Send(emptyResp); err != nil {
				return err
			}
		}

		return nil
	}
}

type txBuffer struct {
	items       map[uint64]map[uint64]*old_faithful_grpc.TransactionResponse // slot -> index -> tx
	mu          sync.Mutex
	startSlot   uint64
	endSlot     uint64
	currentSlot uint64
}

func newTxBuffer(startSlot, endSlot uint64) *txBuffer {
	return &txBuffer{
		items:       make(map[uint64]map[uint64]*old_faithful_grpc.TransactionResponse),
		startSlot:   startSlot,
		endSlot:     endSlot,
		currentSlot: startSlot,
	}
}

func (b *txBuffer) add(slot, idx uint64, tx *old_faithful_grpc.TransactionResponse) {
	addStartTime := time.Now()
	b.mu.Lock()
	lockTime := time.Since(addStartTime)
	if lockTime > 100*time.Millisecond {
		klog.V(2).Infof("txBuffer.add lock acquisition took %s", lockTime)
	}
	defer b.mu.Unlock()

	if _, exists := b.items[slot]; !exists {
		b.items[slot] = make(map[uint64]*old_faithful_grpc.TransactionResponse)
	}
	b.items[slot][idx] = tx
}

func (b *txBuffer) flush(ser old_faithful_grpc.OldFaithful_StreamTransactionsServer) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	totalTxs := 0
	for _, txMap := range b.items {
		totalTxs += len(txMap)
	}
	klog.V(2).Infof("Flushing buffer with %d slots containing %d total transactions", len(b.items), totalTxs)

	for b.currentSlot <= b.endSlot {
		// Send all transactions for this slot in index order
		if txMap, exists := b.items[b.currentSlot]; exists {
			// Get all indices and sort them
			indices := make([]uint64, 0, len(txMap))
			for idx := range txMap {
				indices = append(indices, idx)
			}
			sort.Slice(indices, func(i, j int) bool {
				return indices[i] < indices[j]
			})

			// Send transactions in order
			for _, idx := range indices {
				// Check context before each send for early cancellation
				select {
				case <-ser.Context().Done():
					return ser.Context().Err()
				default:
					// Continue with send
				}

				txResp := txMap[idx]
				if txResp == nil {
					klog.V(2).Infof("Skipping nil transaction response at slot %d, index %d", b.currentSlot, idx)
					continue
				}

				sendStart := time.Now()
				if err := ser.Send(txResp); err != nil {
					return err
				}
				if time.Since(sendStart) > 100*time.Millisecond {
					klog.V(2).Infof("Sending transaction took %s", time.Since(sendStart))
				}
			}
		}

		// Clean up processed slot
		delete(b.items, b.currentSlot)
		b.currentSlot++
	}
	return nil
}
