package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/nodetools"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/telemetry"
	"github.com/valyala/bytebufferpool"
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
	go func() {
		<-ctx.Done()
		klog.Info("gRPC server shutting down...")
		defer klog.Info("gRPC server shut down")
		
		// Create a timeout context for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		// Use a channel to signal when GracefulStop completes
		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()
		
		// Wait for either graceful shutdown to complete or timeout
		select {
		case <-done:
			klog.Info("gRPC server gracefully stopped")
		case <-shutdownCtx.Done():
			klog.Warning("gRPC server graceful shutdown timed out, forcing stop")
			grpcServer.Stop()
		}
	}()

	klog.Infof("gRPC server starting with telemetry enabled on %s", listenOn)
	
	// Start the server in a goroutine so we can handle shutdown properly
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- grpcServer.Serve(lis)
	}()
	
	// Wait for either server error or context cancellation
	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("failed to serve gRPC server: %w", err)
		}
		return nil
	case <-ctx.Done():
		klog.Info("gRPC server context cancelled, shutting down...")
		return ctx.Err()
	}
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
	ctx, span := telemetry.StartSpan(ctx, "GetBlock")
	defer span.End()
	span.SetAttributes(attribute.Int64("slot", int64(params.Slot)))

	nogc := DontGC(ctx)

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

	/////////////////////////////////
	tim := newTimer(getRequestIDFromContext(ctx))

	childCid, err := epochHandler.FindCidFromSlot(ctx, slot)
	if err != nil {
		if errors.Is(err, compactindexsized.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "Slot %d was skipped, or missing in long-term storage", slot)
		} else {
			return nil, status.Errorf(codes.Internal, "Failed to get block: %v", err)
		}
	}
	// Find CAR file oasChild for CID in index.
	oasChild, err := epochHandler.FindOffsetAndSizeFromCid(ctx, childCid)
	if err != nil {
		// not found or error
		return nil, fmt.Errorf("failed to find offset for CID %s: %w", childCid, err)
	}
	childData, err := epochHandler.GetNodeByOffsetAndSizeBuffer(ctx, &childCid, oasChild)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get block: %v", err)
	}
	block, err := iplddecoders.DecodeBlock(childData.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode block: %w", err)
	}
	if uint64(block.Slot) != slot {
		return nil, fmt.Errorf("expected slot %d, got %d", slot, block.Slot)
	}
	bytebufferpool.Put(childData) // return the buffer to the pool

	parentSlot := block.GetParentSlot()
	parentIsInPreviousEpoch := slottools.ParentIsInPreviousEpoch(parentSlot, (slot))
	// TODO: the parent object might be in the previous epoch, so we need to handle that case.

	offsetParent, parentCid, err := func() (uint64, cid.Cid, error) {
		if parentSlot == 0 {
			return epochHandler.carHeaderSize, cid.Cid{}, nil // genesis has no parent
		}
		if parentIsInPreviousEpoch {
			return epochHandler.carHeaderSize, cid.Cid{}, nil
		}
		parentCid, err := epochHandler.FindCidFromSlot(ctx, parentSlot)
		if err != nil {
			if errors.Is(err, compactindexsized.ErrNotFound) {
				return 0, cid.Cid{}, fmt.Errorf("parent slot %d was skipped, or missing in long-term storage", parentSlot)
			}
		}
		if parentCid == cid.Undef {
			return 0, cid.Cid{}, fmt.Errorf("parent CID for slot %d is undefined", parentSlot)
		}
		parentOas, err := epochHandler.FindOffsetAndSizeFromCid(ctx, parentCid)
		if err != nil {
			return 0, cid.Cid{}, fmt.Errorf("failed to find offset for parent CID %s: %w", parentCid, err)
		}
		offsetParent := parentOas.Offset
		return offsetParent, parentCid, nil
	}()
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get block")
	}
	totalSize := oasChild.Offset + oasChild.Size - offsetParent

	if totalSize > GiB*2 {
		return nil, status.Error(codes.Internal, "Internal error")
	}
	reader, err := epochHandler.GetEpochReaderAt()
	if err != nil {
		return nil, fmt.Errorf("failed to get epoch reader: %w", err)
	}
	// TODO: save this info immediately so for next getBlock(thisBlock) we know immediately where to read in the CAR file,
	// and whether the parent is in the previous epoch or not.
	section, err := carreader.ReadIntoBuffer(offsetParent, totalSize, reader)
	if err != nil {
		slog.Error("failed to read node from CAR", "error", err)
		return nil, fmt.Errorf("failed to read node from CAR: %w", err)
	}
	tim.time("read section from CAR")

	nodes, err := nodetools.SplitIntoDataAndCids(section.Bytes())
	if err != nil {
		slog.Error("failed to split section into nodes", "error", err)
		return nil, status.Errorf(codes.Internal, "Internal error")
	}
	defer nodes.Put() // return the nodes to the pool
	nodes.SortByCid()
	bytebufferpool.Put(section) // return the buffer to the pool
	tim.time("nodes")

	parsedNodes, err := nodes.ToParsedAndCidSlice()
	if err != nil {
		slog.Error("failed to convert nodes to parsed nodes", "error", err)
		return nil, status.Errorf(codes.Internal, "Internal error")
	}
	defer parsedNodes.Put() // return the parsed nodes to the pool
	// parsedNodes.SortByCid() // NOTE: already sorted by CIDs in SplitIntoDataAndCids; ToParsedAndCidSlice maintains the same order.
	tim.time("parsedNodes")

	blocktime := uint64(block.Meta.Blocktime)

	lastEntryCid := block.Entries[len(block.Entries)-1]
	lastEntry, err := parsedNodes.EntryByCid(lastEntryCid.(cidlink.Link).Cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get last entry: %w", err)
	}
	lastEntryHash := solana.HashFromBytes(lastEntry.Hash)
	tim.time("get entries")

	resp := old_faithful_grpc.GetBlockResponse()
	resp.Slot = uint64(block.Slot)
	if !nogc {
		defer old_faithful_grpc.PutBlockResponse(resp) // return to pool
	}

	hasRewards := block.HasRewards()
	rewardsCid := block.Rewards.(cidlink.Link).Cid
	if hasRewards {
		uncompressedRewards, err := nodetools.GetRawRewards(parsedNodes, rewardsCid)
		if err != nil {
			slog.Error(
				"failed to parse block rewards",
				"block", slot,
				"rewards_cid", rewardsCid.String(),
				"error", err,
			)
			return nil, status.Errorf(codes.Internal, "Failed to get rewards: %v", err)
		} else {
			resp.Rewards = uncompressedRewards
			{
				actualRewards, err := solanablockrewards.ParseRewards(uncompressedRewards)
				if err == nil && actualRewards.NumPartitions != nil {
					resp.NumPartitions = &actualRewards.NumPartitions.NumPartitions
				}
			}
		}
	} else {
		klog.V(4).Infof("rewards not requested or not available")
	}
	tim.time("get rewards")

	allTransactions := make([]*old_faithful_grpc.Transaction, 0, parsedNodes.CountTransactions())

	{
		_, buildTxSpan := telemetry.StartSpan(ctx, "BuildTransactionsResponse")

		for _, transactionNode := range parsedNodes.SortedTransactions() {
			{
				tx, meta, err := nodetools.GetRawTransactionAndMeta(parsedNodes, transactionNode)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
				}
				txResp := old_faithful_grpc.GetTransaction()
				txResp.Transaction = tx
				txResp.Meta = meta

				pos, ok := transactionNode.GetPositionIndex()
				if ok {
					txResp.Index = ptrToUint64(uint64(pos))
				}
				allTransactions = append(allTransactions, txResp)
				if !nogc {
					defer old_faithful_grpc.PutTransaction(txResp) // return to pool
				}
			}
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
			parentBlock, err := parsedNodes.BlockByCid(parentCid)
			if err != nil {
				telemetry.RecordError(parentSpan, err, "Failed to get parent block")
				parentSpan.End()
				return nil, status.Errorf(codes.Internal, "Failed to get parent block: %v", err)
			}
			if len(parentBlock.Entries) > 0 {
				lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1].(cidlink.Link).Cid

				parentEntryNode, err := epochHandler.GetEntryByCid(parentSpanCtx, lastEntryCidOfParent)
				if err != nil {
					telemetry.RecordError(parentSpan, err, "Failed to get parent entry")
					parentSpan.End()
					return nil, status.Errorf(codes.Internal, "Failed to get parent entry: %v", err)
				}
				parentEntryHash := solana.HashFromBytes(parentEntryNode.Hash)
				resp.PreviousBlockhash = parentEntryHash[:]
			}
		} else {
			// TODO: handle the case when the parent is in a different epoch.
			if slot != 0 {
				klog.V(4).Infof("parent slot is in a different epoch, not implemented yet (can't get previousBlockhash)")
				// is previous epoch available?
				// if yes, get the parent block from there
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
	klog.V(5).Infof("Found signature %s in epoch %d in %s", sig, epochNumber, time.Since(startedEpochLookupAt))

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

func (multi *MultiEpoch) StreamBlocks(params *old_faithful_grpc.StreamBlocksRequest, ser old_faithful_grpc.OldFaithful_StreamBlocksServer) error {
	ctx := ser.Context()

	startSlot := params.StartSlot
	endSlot := startSlot + maxSlotsToStream

	if params.EndSlot != nil {
		endSlot = *params.EndSlot
	}

	var accountInclude []solana.PublicKey
	if params.Filter != nil && len(params.Filter.AccountInclude) > 0 {
		var err error
		accountInclude, err = stringSliceToPublicKeySlice(params.Filter.AccountInclude)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "Failed to parse accountInclude: %v", err)
		}
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

		block, err := multi.GetBlock(WithDontGC(ctx), &old_faithful_grpc.BlockRequest{Slot: slot})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				continue // is this the right thing to do?
			}
			return err
		}
		defer old_faithful_grpc.PutBlockResponse(block) // return to pool

		if filterFunc(block) {
			if err := ser.Send(block); err != nil {
				return err
			}
		}
	}

	return nil
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

	// Validate that at least one filter is provided
	if !hasValidTransactionFilter(params.Filter) {
		return status.Errorf(codes.InvalidArgument, "At least one filter must be specified (vote, failed, account_include, account_exclude, or account_required)")
	}

	// Validate account addresses early to catch invalid ones before processing
	if err := validateAccountFilters(params.Filter); err != nil {
		klog.Errorf("Invalid account filter provided: %v", err)
		return status.Errorf(codes.InvalidArgument, "Invalid account filter: %v", err)
	}

	gsfaReader, epochNums := multi.getGsfaReadersInEpochDescendingOrderForSlotRange(ctx, startSlot, endSlot)
	wantedEpochs := slottools.CalcEpochsForSlotRange(startSlot, endSlot)
	klog.V(4).Infof("Streaming transactions from slots %d to %d, epochs %v", startSlot, endSlot, wantedEpochs)

	gsfaReadersLoaded := true
	if len(epochNums) == 0 {
		klog.V(4).Info("The requested slot range does not have any GSFA readers loaded, will use the default method")
		gsfaReadersLoaded = false
	} else {
		klog.V(4).Infof("Using GSFA readers for epochs: %v; wanted epochs: %v", epochNums, wantedEpochs)
		if len(epochNums) < len(wantedEpochs) {
			klog.V(4).Infof("Not all epochs in the requested slot range have GSFA readers loaded")
		}
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
	klog.V(4).Infof("Processing StreamTransactions request from slot %d to %d with filter", startSlot, endSlot)
	compilableFilter, err := fromStreamTransactionsFilter(filter)
	if err != nil {
		klog.Errorf("Failed to parse StreamTransactions filter: %v", err)
		return status.Errorf(codes.InvalidArgument, "Failed to parse filter: %v", err)
	}
	klog.V(4).Info("Successfully parsed StreamTransactions filter, compiling exclusion rules")
	filterOut, err := compilableFilter.CompileExclusion()
	if err != nil {
		klog.Errorf("Failed to compile StreamTransactions filter: %v", err)
		return status.Errorf(codes.Internal, "Failed to compile filter: %v", err)
	}
	klog.V(4).Info("Successfully compiled StreamTransactions filter exclusion rules")

	if 1 == 0+1 {
		klog.V(4).Infof("Using the old faithful method for streaming transactions from slots %d to %d", startSlot, endSlot)

		for slot := startSlot; slot <= endSlot; slot++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			block, err := multi.GetBlock(WithDontGC(ctx), &old_faithful_grpc.BlockRequest{Slot: slot})
			if err != nil {
				if status.Code(err) == codes.NotFound {
					continue // This block is not available, skip it (either skipped or not available)
				}
				return err
			}
			defer old_faithful_grpc.PutBlockResponse(block) // return to pool

			for _, tx := range block.Transactions {
				txn, err := solana.TransactionFromBytes(tx.GetTransaction())
				if err != nil {
					return status.Errorf(codes.Internal, "Failed to decode transaction: %v", err)
				}

				meta, err := func() (*solanatxmetaparsers.TransactionStatusMetaContainer, error) {
					if len(tx.Meta) == 0 {
						return nil, nil // No meta available, return nil
					}
					return solanatxmetaparsers.ParseTransactionStatusMetaContainer(tx.Meta)
				}()
				if err != nil {
					return status.Errorf(codes.Internal, "Failed to parse transaction meta: %v", err)
				}

				shouldExclude := filterOut.Do(txn, meta)
				klog.V(5).Infof("Filter evaluation for transaction: shouldExclude=%v", shouldExclude)
				if !shouldExclude {

					txResp := new(old_faithful_grpc.TransactionResponse)
					txResp.Transaction = new(old_faithful_grpc.Transaction)

					{
						txResp.Transaction.Transaction = tx.Transaction
						txResp.Transaction.Meta = tx.Meta
						txResp.Transaction.Index = tx.Index
						txResp.Slot = block.Slot
						txResp.Index = tx.Index

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
	}

	klog.V(4).Infof("Using GSFA reader for streaming transactions from slots %d to %d", startSlot, endSlot)

	const batchSize = 100
	buffer := newTxBuffer(uint64(startSlot), uint64(endSlot))

	wg := new(errgroup.Group)

	const maxConcurrentAccounts = 10
	wg.SetLimit(maxConcurrentAccounts)

	for accI := range compilableFilter.AccountInclude {
		account := compilableFilter.AccountInclude[accI]
		wg.Go(func() error {
			return func(pKey solana.PK) error {
				queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				startTime := time.Now()
				klog.V(5).Infof("Starting GSFA query for account %s, from slot %d to %d", pKey.String(), startSlot, endSlot)

				epochToTxns, err := gsfaReader.GetBeforeUntilSlot(
					queryCtx,
					pKey,
					batchSize,
					endSlot+1, //  Before (exclusive)
					startSlot, // Until (inclusive)
					func(epochNum uint64, oas linkedlog.OffsetAndSizeAndSlot) (*ipldbindcode.Transaction, error) {
						fnStartTime := time.Now()
						defer func() {
							klog.V(5).Infof("GSFA transaction lookup for epoch %d took %s",
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
					return err
				}
				duration := time.Since(startTime)
				klog.V(4).Infof("GSFA query completed for account %s, from slot %d to %d took %s", pKey.String(), startSlot, endSlot, duration)

				for epochNumber, txns := range epochToTxns {
					epochHandler, err := multi.GetEpoch(epochNumber)
					if err != nil {
						return status.Errorf(codes.NotFound, "Epoch %d is not available", epochNumber)
					}

					for _, txn := range txns {
						if txn == nil {
							klog.V(5).Infof("Skipping nil transaction from epoch %d", epochNumber)
							continue
						}

						txStartTime := time.Now()
						tx, meta, err := nodetools.ParseTransactionAndMetaFromNode(txn, epochHandler.GetDataFrameByCid)
						klog.V(5).Infof("Parsing transaction for account %s took %s", pKey.String(), time.Since(txStartTime))
						if err != nil {
							return status.Errorf(codes.Internal, "Failed to parse transaction from node: %v", err)
						}

						shouldExcludeTx := filterOut.Do(tx, meta)
						klog.V(5).Infof("Filter evaluation for GSFA transaction: shouldExclude=%v", shouldExcludeTx)
						if !shouldExcludeTx {
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
									return status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
								}
								txResp.Slot = uint64(txn.Slot)

								blocktimeIndex := epochHandler.GetBlocktimeIndex()
								if blocktimeIndex != nil {
									blocktime, err := blocktimeIndex.Get(uint64(txn.Slot))
									if err != nil {
										return status.Errorf(codes.Internal, "Failed to get blocktime: %v", err)
									}
									txResp.BlockTime = int64(blocktime)
								} else {
									return status.Errorf(codes.Internal, "Failed to get blocktime: blocktime index is nil")
								}
							}

							buffer.add(txResp.Slot, *txResp.Index, txResp)
						} else {
							klog.V(5).Infof("Transaction for account %s at slot %d was filtered out", pKey.String(), txn.Slot)
						}
					}
				}
				klog.V(5).Infof("Completed processing transactions for account %s", pKey.String())
				return nil
			}(account)
		})
	}

	// Wait for all processing to complete

	// Handle any errors
	klog.V(5).Infof("Checking for errors from goroutines")
	errCheckStartTime := time.Now()
	select {
	case err := <-wgWaitToChannel(wg):
		if err != nil {
			klog.Errorf("Received error from error channel: %v", err)
			return err
		}
		klog.V(5).Infof("Received nil error from error channel")
	case <-ctx.Done():
		klog.V(5).Infof("Context done while checking errors")
		return ctx.Err()
	}
	klog.V(5).Infof("Error check completed in %s", time.Since(errCheckStartTime))
	// Flush after all processing is done
	klog.V(2).Infof("Starting buffer flush with %d slots of transactions", len(buffer.items))
	flushStartTime := time.Now()
	if err := buffer.flush(ser); err != nil {
		return err
	}
	klog.V(4).Infof("Buffer flush completed in %s", time.Since(flushStartTime))

	// If we got here with no transactions (buffer is empty), send an empty response
	if len(buffer.items) == 0 {
		klog.V(4).Infof("No transactions found for the requested accounts, sending empty response")
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

func wgWaitToChannel(wg *errgroup.Group) <-chan error {
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		err := wg.Wait()
		if err != nil {
			errChan <- err
		} else {
			errChan <- nil
		}
	}()
	return errChan
}

type txBuffer struct {
	items     map[uint64]map[uint64]*old_faithful_grpc.TransactionResponse // slot -> index -> tx
	mu        sync.Mutex
	startSlot uint64
	endSlot   uint64
}

func newTxBuffer(startSlot, endSlot uint64) *txBuffer {
	return &txBuffer{
		items:     make(map[uint64]map[uint64]*old_faithful_grpc.TransactionResponse),
		startSlot: startSlot,
		endSlot:   endSlot,
	}
}

func (b *txBuffer) add(slot, idx uint64, tx *old_faithful_grpc.TransactionResponse) {
	addStartTime := time.Now()
	b.mu.Lock()
	lockTime := time.Since(addStartTime)
	if lockTime > 100*time.Millisecond {
		klog.V(5).Infof("txBuffer.add lock acquisition took %s", lockTime)
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
	klog.V(5).Infof("Flushing buffer with %d slots containing %d total transactions", len(b.items), totalTxs)

	slots := make([]uint64, 0, len(b.items))
	for slot := range b.items {
		slots = append(slots, slot)
	}
	// sort from highest to lowest slot
	sort.Slice(slots, func(i, j int) bool {
		return slots[i] > slots[j]
	})
	klog.V(5).Infof("Sorted slots for flushing: %v", slots)

	for _, slot := range slots {
		if slot < b.startSlot || slot > b.endSlot {
			klog.V(5).Infof("Skipping slot %d, outside of range [%d, %d]", slot, b.startSlot, b.endSlot)
			continue
		}
		klog.V(5).Infof("Processing slot %d with %d transactions", slot, len(b.items[slot]))
		// Send all transactions for this slot in index order
		txMap, exists := b.items[slot]
		if !exists {
			klog.V(5).Infof("No transactions found for slot %d, skipping", slot)
			continue
		}
		// Get all indices and sort them
		indices := make([]uint64, 0, len(txMap))
		for idx := range txMap {
			indices = append(indices, idx)
		}
		sort.Slice(indices, func(i, j int) bool {
			return indices[i] < indices[j]
		})
		klog.V(5).Infof("Sending %d transactions for slot %d in order", len(indices), slot)

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
				klog.V(5).Infof("Skipping nil transaction response at slot %d, index %d", slot, idx)
				continue
			}

			sendStart := time.Now()
			if err := ser.Send(txResp); err != nil {
				return err
			}
			if time.Since(sendStart) > 100*time.Millisecond {
				klog.V(5).Infof("Sending transaction took %s", time.Since(sendStart))
			}
		}

		// Clean up processed slot
		delete(b.items, slot)
	}
	return nil
}

// hasValidTransactionFilter checks if the filter contains at least one valid constraint.
// Returns true if the filter has at least one meaningful constraint set.
func hasValidTransactionFilter(filter *old_faithful_grpc.StreamTransactionsFilter) bool {
	if filter == nil {
		return false
	}

	// Check if any filter field is set
	return filter.Vote != nil ||
		filter.Failed != nil ||
		len(filter.AccountInclude) > 0 ||
		len(filter.AccountExclude) > 0 ||
		len(filter.AccountRequired) > 0
}

// validateAccountFilters validates that all account addresses in filters are valid base58 public keys
func validateAccountFilters(filter *old_faithful_grpc.StreamTransactionsFilter) error {
	if filter == nil {
		return nil
	}

	if err := validateAccountSlice(filter.AccountInclude, "account_include"); err != nil {
		return err
	}
	if err := validateAccountSlice(filter.AccountExclude, "account_exclude"); err != nil {
		return err
	}
	if err := validateAccountSlice(filter.AccountRequired, "account_required"); err != nil {
		return err
	}

	return nil
}

// validateAccountSlice validates that all account addresses in the slice are valid base58 public keys
func validateAccountSlice(accounts []string, fieldName string) error {
	for i, account := range accounts {
		if _, err := solana.PublicKeyFromBase58(account); err != nil {
			return fmt.Errorf("invalid %s[%d] '%s': %w", fieldName, i, account, err)
		}
	}
	return nil
}
