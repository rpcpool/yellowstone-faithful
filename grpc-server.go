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
	"github.com/rpcpool/yellowstone-faithful/dummycid"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/telemetry"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	"go.opentelemetry.io/otel/attribute"
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
	// Initialize telemetry
	cleanup, err := telemetry.InitTelemetry(ctx, "yellowstone-faithful")
	if err != nil {
		klog.Warningf("Failed to initialize telemetry: %v", err)
	} else {
		defer cleanup()
	}

	lis, err := net.Listen("tcp", listenOn)
	if err != nil {
		return fmt.Errorf("failed to create listener for gRPC server: %w", err)
	}

	// Create gRPC server with telemetry interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(telemetry.TracingUnaryInterceptor),
		grpc.StreamInterceptor(telemetry.TracingStreamInterceptor),
	)
	old_faithful_grpc.RegisterOldFaithfulServer(grpcServer, me)

	klog.Infof("gRPC server starting with telemetry enabled on %s", listenOn)
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
	// Create a span for this operation
	ctx, span := telemetry.StartSpan(ctx, "GetBlock")
	defer span.End()
	span.SetAttributes(attribute.Int64("slot", int64(params.Slot)))

	// find the epoch that contains the requested slot
	slot := params.Slot
	epochNumber := slottools.CalcEpochForSlot(slot)
	span.SetAttributes(attribute.Int64("epoch_number", int64(epochNumber)))

	// Get epoch handler
	_, epochLookupSpan := telemetry.StartSpan(ctx, "EpochLookup")
	epochHandler, err := multi.GetEpoch(epochNumber)
	epochLookupSpan.End()

	if err != nil {
		telemetry.RecordError(span, err, "Epoch not available")
		return nil, status.Errorf(codes.NotFound, "Epoch %d is not available", epochNumber)
	}

	// Get block from epoch handler
	blockCtx, blockSpan := telemetry.StartSpan(ctx, "GetBlockFromEpoch")
	block, _, err := epochHandler.GetBlock(WithSubrapghPrefetch(blockCtx, true), slot)
	blockSpan.End()

	if err != nil {
		telemetry.RecordError(span, err, "Failed to get block")
		if errors.Is(err, compactindexsized.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "Slot %d was skipped, or missing in long-term storage", slot)
		} else {
			return nil, status.Errorf(codes.Internal, "Failed to get block: %v", err)
		}
	}

	// Keep existing timer for backward compatibility
	tim := newTimer(getRequestIDFromContext(ctx))
	tim.time("GetBlock")
	{
		// Wrapper span for all CAR prefetch operations
		_, carPrefetchWrapperSpan := telemetry.StartSpan(ctx, "CarPrefetchOperations")
		defer carPrefetchWrapperSpan.End()

		prefetcherFromCar := func() error {
			// Create a span for the CAR prefetching operation
			prefetchCtx, prefetchSpan := telemetry.StartDiskIOSpan(ctx, "prefetch_car", map[string]string{
				"slot": fmt.Sprintf("%d", slot),
			})
			defer prefetchSpan.End()

			parentIsInPreviousEpoch := slottools.CalcEpochForSlot(uint64(block.Meta.Parent_slot)) != slottools.CalcEpochForSlot(slot)
			if slot == 0 {
				parentIsInPreviousEpoch = true
			}
			if slot > 1 && block.Meta.Parent_slot == 0 {
				parentIsInPreviousEpoch = true
			}

			var blockCid, parentBlockCid cid.Cid
			wg := new(errgroup.Group)

			// Finding CIDs - often a source of seek time
			_, findCidsSpan := telemetry.StartDiskIOSpan(prefetchCtx, "find_cids", map[string]string{
				"parent_is_in_previous_epoch": fmt.Sprintf("%v", parentIsInPreviousEpoch),
			})

			wg.Go(func() (err error) {
				ctxBlock, blockCidSpan := telemetry.StartDiskIOSpan(prefetchCtx, "find_block_cid", map[string]string{
					"slot": fmt.Sprintf("%d", slot),
				})
				defer blockCidSpan.End()

				blockCid, err = epochHandler.FindCidFromSlot(ctxBlock, slot)
				if err != nil {
					telemetry.RecordError(blockCidSpan, err, "Failed to find CID from slot")
					return err
				}
				return nil
			})
			wg.Go(func() (err error) {
				if parentIsInPreviousEpoch {
					return nil
				}
				ctxParent, parentCidSpan := telemetry.StartDiskIOSpan(prefetchCtx, "find_parent_cid", map[string]string{
					"parent_slot": fmt.Sprintf("%d", block.Meta.Parent_slot),
				})
				defer parentCidSpan.End()

				parentBlockCid, err = epochHandler.FindCidFromSlot(ctxParent, uint64(block.Meta.Parent_slot))
				if err != nil {
					telemetry.RecordError(parentCidSpan, err, "Failed to find parent CID from slot")
					return err
				}
				return nil
			})
			err = wg.Wait()
			findCidsSpan.End()

			if err != nil {
				telemetry.RecordError(prefetchSpan, err, "Failed to find CIDs")
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

				// Find offsets - this can involve disk seeking
				_, findOffsetsSpan := telemetry.StartDiskIOSpan(prefetchCtx, "find_offsets", map[string]string{
					"parent_is_in_previous_epoch": fmt.Sprintf("%v", parentIsInPreviousEpoch),
				})

				wg.Go(func() (err error) {
					ctxOffset, blockOffsetSpan := telemetry.StartDiskIOSpan(prefetchCtx, "find_block_offset", map[string]string{
						"block_cid": blockCid.String(),
					})
					defer blockOffsetSpan.End()

					offsetAndSize, err := epochHandler.FindOffsetAndSizeFromCid(ctxOffset, blockCid)
					if err != nil {
						telemetry.RecordError(blockOffsetSpan, err, "Failed to find offset and size from CID")
						return err
					}
					blockOffset = offsetAndSize.Offset
					blockOffsetSpan.SetAttributes(attribute.Int64("offset", int64(blockOffset)))
					return nil
				})
				wg.Go(func() (err error) {
					if parentIsInPreviousEpoch {
						// get car file header size
						parentOffset = epochHandler.carHeaderSize
						return nil
					}
					ctxParentOffset, parentOffsetSpan := telemetry.StartDiskIOSpan(prefetchCtx, "find_parent_offset", map[string]string{
						"parent_cid": parentBlockCid.String(),
					})
					defer parentOffsetSpan.End()

					offsetAndSize, err := epochHandler.FindOffsetAndSizeFromCid(ctxParentOffset, parentBlockCid)
					if err != nil {
						telemetry.RecordError(parentOffsetSpan, err, "Failed to find parent offset and size from CID")
						return err
					}
					parentOffset = offsetAndSize.Offset
					parentOffsetSpan.SetAttributes(attribute.Int64("offset", int64(parentOffset)))
					return nil
				})
				err = wg.Wait()
				findOffsetsSpan.End()

				if err != nil {
					telemetry.RecordError(prefetchSpan, err, "Failed to find offsets")
					return err
				}

				length := blockOffset - parentOffset
				MiB := uint64(1024 * 1024)
				maxPrefetchSize := MiB * 10 // let's cap prefetching size
				if length > maxPrefetchSize {
					length = maxPrefetchSize
				}

				start := parentOffset
				prefetchSpan.SetAttributes(
					attribute.Int64("read_start", int64(start)),
					attribute.Int64("read_length", int64(length)),
				)

				klog.V(4).Infof("prefetching CAR: start=%d length=%d (parent_offset=%d)", start, length, parentOffset)

				// This is the actual disk read operation - likely significant seek time here
				readCtx, readSpan := telemetry.StartDiskIOSpan(prefetchCtx, "read_car_section", map[string]string{
					"start":  fmt.Sprintf("%d", start),
					"length": fmt.Sprintf("%d", length),
				})
				carSection, err := epochHandler.ReadAtFromCar(readCtx, start, length)
				readSpan.End()

				if err != nil {
					telemetry.RecordError(prefetchSpan, err, "Failed to read CAR section")
					return err
				}
				dr := bytes.NewReader(carSection)
				br := bufio.NewReader(dr)

				// Processing the read data - deserializing and caching
				_, processSpan := telemetry.StartSpan(prefetchCtx, "process_car_data")
				defer processSpan.End()

				gotCid, data, err := util.ReadNode(br)
				if err != nil {
					telemetry.RecordError(processSpan, err, "Failed to read first node")
					return fmt.Errorf("failed to read first node: %w", err)
				}
				if !parentIsInPreviousEpoch && !gotCid.Equals(parentBlockCid) {
					err := fmt.Errorf("CID mismatch: expected %s, got %s", parentBlockCid, gotCid)
					telemetry.RecordError(processSpan, err, "CID mismatch")
					return err
				}
				epochHandler.GetCache().PutRawCarObject(gotCid, data)

				nodesProcessed := 1
				for {
					gotCid, data, err = util.ReadNode(br)
					if err != nil {
						if errors.Is(err, io.EOF) {
							break
						}
						telemetry.RecordError(processSpan, err, "Failed to read node")
						return fmt.Errorf("failed to read node: %w", err)
					}
					nodesProcessed++
					if gotCid.Equals(blockCid) {
						break
					}
					epochHandler.GetCache().PutRawCarObject(gotCid, data)
				}
				processSpan.SetAttributes(attribute.Int64("nodes_processed", int64(nodesProcessed)))
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

	_, entriesSpan := telemetry.StartSpan(ctx, "ProcessEntries")
	allTransactionNodes := make([][]*ipldbindcode.Transaction, len(block.Entries))
	mu := &sync.Mutex{}
	var lastEntryHash solana.Hash
	entriesSpan.SetAttributes(attribute.Int64("entry_count", int64(len(block.Entries))))
	{
		wg := new(errgroup.Group)
		wg.SetLimit(runtime.NumCPU() * 2)
		// get entries from the block
		for entryIndex, entry := range block.Entries {
			entryIndex := entryIndex
			entryCid := entry.(cidlink.Link).Cid
			wg.Go(func() error {
				// Create a span for entry processing
				entryCtx, entrySpan := telemetry.StartSpan(ctx, "ProcessEntry")
				entrySpan.SetAttributes(
					attribute.Int64("entry_index", int64(entryIndex)),
					attribute.String("entry_cid", entryCid.String()),
				)
				defer entrySpan.End()

				// get the entry by CID
				entryFetchStart := time.Now()
				entryNode, err := epochHandler.GetEntryByCid(entryCtx, entryCid)
				entrySpan.SetAttributes(attribute.Int64("entry_fetch_ms", time.Since(entryFetchStart).Milliseconds()))
				if err != nil {
					klog.Errorf("failed to decode Entry: %v", err)
					telemetry.RecordError(entrySpan, err, "Failed to decode entry")
					return err
				}

				if entryIndex == len(block.Entries)-1 {
					lastEntryHash = solana.HashFromBytes(entryNode.Hash)
				}

				entrySpan.SetAttributes(attribute.Int64("transaction_count", int64(len(entryNode.Transactions))))
				twg := new(errgroup.Group)
				twg.SetLimit(runtime.NumCPU())
				// get the transactions from the entry
				allTransactionNodes[entryIndex] = make([]*ipldbindcode.Transaction, len(entryNode.Transactions))

				// Create a span for transaction processing within this entry
				txsCtx, txsSpan := telemetry.StartSpan(entryCtx, "ProcessEntryTransactions")
				txsSpan.SetAttributes(attribute.Int64("transaction_count", int64(len(entryNode.Transactions))))
				defer txsSpan.End()

				for txI := range entryNode.Transactions {
					txI := txI
					tx := entryNode.Transactions[txI]
					twg.Go(func() error {
						// Create a span for individual transaction processing
						txCtx, txSpan := telemetry.StartSpan(txsCtx, "ProcessTransaction")
						txSpan.SetAttributes(attribute.Int64("tx_index", int64(txI)))
						defer txSpan.End()

						// get the transaction by CID
						tcid := tx.(cidlink.Link).Cid
						txSpan.SetAttributes(attribute.String("tx_cid", tcid.String()))

						txFetchStart := time.Now()
						txNode, err := epochHandler.GetTransactionByCid(txCtx, tcid)
						txSpan.SetAttributes(attribute.Int64("tx_fetch_ms", time.Since(txFetchStart).Milliseconds()))

						if err != nil {
							klog.Errorf("failed to decode Transaction %s: %v", tcid, err)
							telemetry.RecordError(txSpan, err, "Failed to decode transaction")
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
			telemetry.RecordError(entriesSpan, err, "Failed to get entries")
			return nil, status.Errorf(codes.Internal, "Failed to get entries: %v", err)
		}
	}
	entriesSpan.End()
	tim.time("get entries")

	resp := &old_faithful_grpc.BlockResponse{
		Slot: uint64(block.Slot),
	}

	var allTransactions []*old_faithful_grpc.Transaction
	hasRewards := !block.Rewards.(cidlink.Link).Cid.Equals(dummycid.DummyCID)
	if hasRewards {
		rewardsSpanCtx, rewardsSpan := telemetry.StartSpan(ctx, "RewardsProcessing")
		rewardsNode, err := epochHandler.GetRewardsByCid(rewardsSpanCtx, block.Rewards.(cidlink.Link).Cid)
		if err != nil {
			telemetry.RecordError(rewardsSpan, err, "Failed to get RewardsByCid")
			rewardsSpan.End()
			return nil, status.Errorf(codes.Internal, "Failed to get Rewards: %v", err)
		}
		rewardsBuf, err := tooling.LoadDataFromDataFrames(&rewardsNode.Data, epochHandler.GetDataFrameByCid)
		if err != nil {
			telemetry.RecordError(rewardsSpan, err, "Failed to load Rewards dataFrames")
			rewardsSpan.End()
			return nil, status.Errorf(codes.Internal, "Failed to load Rewards dataFrames: %v", err)
		}

		uncompressedRawRewards, err := tooling.DecompressZstd(rewardsBuf)
		if err != nil {
			telemetry.RecordError(rewardsSpan, err, "Failed to decompress Rewards")
			rewardsSpan.End()
			return nil, status.Errorf(codes.Internal, "Failed to decompress Rewards: %v", err)
		}
		rewardsSpan.SetAttributes(
			attribute.Int("rewards_compressed_size", len(rewardsBuf)),
			attribute.Int("rewards_uncompressed_size", len(uncompressedRawRewards)),
		)
		resp.Rewards = uncompressedRawRewards
		rewardsSpan.End()
	}
	tim.time("get rewards")
	{
		_, buildTxSpan := telemetry.StartSpan(ctx, "BuildTransactionsResponse")
		for _, transactionNode := range mergeTxNodeSlices(allTransactionNodes) {
			_, txBuildSpan := telemetry.StartSpan(ctx, "TransactionNodeToGRPC")
			txResp := new(old_faithful_grpc.Transaction)

			// response.Slot = uint64(transactionNode.Slot)
			// if blocktime != 0 {
			// 	response.Blocktime = &blocktime
			// }

			{
				pos, ok := transactionNode.GetPositionIndex()
				if ok {
					txResp.Index = ptrToUint64(uint64(pos))
					txBuildSpan.SetAttributes(attribute.Int64("index", int64(pos)))
				}
				var err error
				txResp.Transaction, txResp.Meta, err = getTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
				if err != nil {
					telemetry.RecordError(txBuildSpan, err, "Failed to getTransactionAndMetaFromNode")
					txBuildSpan.End()
					buildTxSpan.End()
					return nil, status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
				}
			}

			allTransactions = append(allTransactions, txResp)
			txBuildSpan.End()
		}
		buildTxSpan.SetAttributes(attribute.Int("num_transactions", len(allTransactions)))
		buildTxSpan.End()
	}

	_, sortSpan := telemetry.StartSpan(ctx, "SortTransactions")
	sort.Slice(allTransactions, func(i, j int) bool {
		if allTransactions[i].Index == nil || allTransactions[j].Index == nil {
			return false
		}
		return *allTransactions[i].Index < *allTransactions[j].Index
	})
	sortSpan.SetAttributes(attribute.Int("num_sorted_transactions", len(allTransactions)))
	sortSpan.End()
	tim.time("get transactions")
	resp.Transactions = allTransactions
	blocktime := uint64(block.Meta.Blocktime)
	if blocktime != 0 {
		resp.BlockTime = int64(blocktime)
	}
	resp.Blockhash = lastEntryHash[:]
	resp.ParentSlot = uint64(block.Meta.Parent_slot)

	// Span for block metadata processing
	_, blockMetaSpan := telemetry.StartSpan(ctx, "ProcessBlockMetadata")
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
	blockMetaSpan.End()
	{
		// get parent slot
		parentSlot := uint64(block.Meta.Parent_slot)
		parentSpanCtx, parentSpan := telemetry.StartSpan(ctx, "GetParentBlockForHash")
		parentSpan.SetAttributes(attribute.Int64("parent_slot", int64(parentSlot)))
		if (parentSlot != 0 || slot == 1) && slottools.CalcEpochForSlot(parentSlot) == epochNumber {
			// NOTE: if the parent is in the same epoch, we can get it from the same epoch handler as the block;
			// otherwise, we need to get it from the previous epoch (TODO: implement this)
			parentBlock, _, err := epochHandler.GetBlock(WithSubrapghPrefetch(parentSpanCtx, false), parentSlot)
			if err != nil {
				telemetry.RecordError(parentSpan, err, "Failed to get parent block")
				parentSpan.End()
				return nil, status.Errorf(codes.Internal, "Failed to get parent block: %v", err)
			}

			if len(parentBlock.Entries) > 0 {
				lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1]
				parentEntryNode, err := epochHandler.GetEntryByCid(parentSpanCtx, lastEntryCidOfParent.(cidlink.Link).Cid)
				if err != nil {
					telemetry.RecordError(parentSpan, err, "Failed to get parent entry")
					parentSpan.End()
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
		parentSpan.End()
	}
	tim.time("get parent block")

	// Final response preparation span
	_, responseSpan := telemetry.StartSpan(ctx, "PrepareResponse")
	responseSpan.SetAttributes(
		attribute.Int("total_transactions", len(resp.Transactions)),
		attribute.Int64("block_slot", int64(resp.Slot)),
	)
	responseSpan.End()

	return resp, nil
}

func (multi *MultiEpoch) GetTransaction(ctx context.Context, params *old_faithful_grpc.TransactionRequest) (*old_faithful_grpc.TransactionResponse, error) {
	// Create a span for this operation
	ctx, span := telemetry.StartSpan(ctx, "GetTransaction")
	defer span.End()

	if multi.CountEpochs() == 0 {
		telemetry.RecordError(span, status.Error(codes.Internal, "no epochs available"), "No epochs available")
		return nil, status.Errorf(codes.Internal, "no epochs available")
	}

	sig := solana.SignatureFromBytes(params.Signature)
	span.SetAttributes(attribute.String("signature", sig.String()))

	startedEpochLookupAt := time.Now()
	_, findEpochSpan := telemetry.StartSpan(ctx, "FindEpochFromSignature")
	epochNumber, err := multi.findEpochNumberFromSignature(ctx, sig)
	findEpochSpan.SetAttributes(
		attribute.Int64("lookup_duration_ms", time.Since(startedEpochLookupAt).Milliseconds()),
	)
	findEpochSpan.End()

	if err != nil {
		telemetry.RecordError(span, err, "Failed to find epoch for signature")
		if errors.Is(err, ErrNotFound) {
			// solana just returns null here in case of transaction not found: {"jsonrpc":"2.0","result":null,"id":1}
			return nil, status.Errorf(codes.NotFound, "Transaction not found")
		}
		return nil, status.Errorf(codes.Internal, "Failed to get epoch for signature %s: %v", sig, err)
	}

	span.SetAttributes(attribute.Int64("epoch_number", int64(epochNumber)))
	klog.V(4).Infof("Found signature %s in epoch %d in %s", sig, epochNumber, time.Since(startedEpochLookupAt))

	_, getEpochSpan := telemetry.StartSpan(ctx, "GetEpochHandler")
	epochHandler, err := multi.GetEpoch(uint64(epochNumber))
	getEpochSpan.End()

	if err != nil {
		telemetry.RecordError(span, err, "Epoch not available")
		return nil, status.Errorf(codes.NotFound, "Epoch %d is not available from this RPC", epochNumber)
	}

	_, getTxSpan := telemetry.StartSpan(ctx, "GetTransactionFromEpoch")
	transactionNode, _, err := epochHandler.GetTransaction(WithSubrapghPrefetch(ctx, true), sig)
	getTxSpan.End()

	if err != nil {
		telemetry.RecordError(span, err, "Failed to get transaction")
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

func stringSliceToPublicKeySlice(accounts []string) (solana.PublicKeySlice, error) {
	if len(accounts) == 0 {
		return nil, nil
	}

	publicKeys := make(solana.PublicKeySlice, len(accounts))
	for i, acc := range accounts {
		pk, err := solana.PublicKeyFromBase58(acc)
		if err != nil {
			return nil, fmt.Errorf("failed to parse account %s: %w", acc, err)
		}
		publicKeys[i] = pk
	}
	return publicKeys, nil
}

func (multi *MultiEpoch) StreamBlocks(params *old_faithful_grpc.StreamBlocksRequest, ser old_faithful_grpc.OldFaithful_StreamBlocksServer) error {
	ctx := ser.Context()

	startSlot := params.StartSlot
	endSlot := startSlot + maxSlotsToStream

	if params.EndSlot != nil {
		endSlot = *params.EndSlot
	}
	accountInclude, err := stringSliceToPublicKeySlice(params.Filter.AccountInclude)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "Failed to parse accountInclude: %v", err)
	}

	filterFunc := func(block *old_faithful_grpc.BlockResponse) bool {
		if params.Filter == nil || len(params.Filter.AccountInclude) == 0 {
			return true
		}

		return blockContainsAccounts(block, accountInclude)
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

func blockContainsAccounts(block *old_faithful_grpc.BlockResponse, accounts solana.PublicKeySlice) bool {
	for _, tx := range block.Transactions {
		decoder := bin.NewBinDecoder(tx.GetTransaction())
		solTx, err := solana.TransactionFromDecoder(decoder)
		if err != nil {
			klog.Errorf("Failed to decode transaction: %v", err)
			continue
		}

		if accounts.ContainsAny(solTx.Message.AccountKeys) {
			return true
		}

		meta, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(tx.Meta)
		if err != nil {
			klog.Errorf("Failed to parse transaction meta: %v", err)
		}

		writable, readonly := meta.GetLoadedAccounts()
		if writable.ContainsAny(accounts) || readonly.ContainsAny(accounts) {
			return true
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
		klog.V(2).Infof("No gsfa readers were loaded for slots %d-%d, falling back to block-by-block scanning", startSlot, endSlot)
		gsfaReadersLoaded = false
	} else {
		klog.V(2).Infof("Loaded %d gsfa readers for epochs %v", len(epochNums), epochNums)
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
	filterOutTxn := func(tx solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer) bool {
		if filter == nil {
			return true
		}

		if filter.Vote != nil && !(*filter.Vote) && IsSimpleVoteTransaction(&tx) { // If vote is false, we should filter out vote transactions
			return false
		}

		if filter.Failed != nil && !(*filter.Failed) { // If failed is false, we should filter out failed transactions
			if meta != nil && meta.IsErr() {
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
		klog.V(2).Infof("Using block-by-block scanning for slots %d-%d (filter=%v, gsfaLoaded=%v)", 
			startSlot, endSlot, filter != nil, gsfaReadersLoaded)

		for slot := startSlot; slot <= endSlot; slot++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			block, err := multi.GetBlock(ctx, &old_faithful_grpc.BlockRequest{Slot: slot})
			if err != nil {
				if status.Code(err) == codes.NotFound {
					continue // Skip this slot and continue to next one
				}
				return err
			}

			for _, tx := range block.Transactions {
				decoder := bin.NewBinDecoder(tx.GetTransaction())
				txn, err := solana.TransactionFromDecoder(decoder)
				if err != nil {
					return status.Errorf(codes.Internal, "Failed to decode transaction: %v", err)
				}

				meta, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(tx.Meta)
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
