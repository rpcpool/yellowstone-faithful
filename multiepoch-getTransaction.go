package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/telemetry"
	"github.com/sourcegraph/jsonrpc2"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

type SigExistsIndex interface {
	Has(sig [64]byte) (bool, error)
}

func (multi *MultiEpoch) getAllBucketteers() map[uint64]SigExistsIndex {
	multi.mu.RLock()
	defer multi.mu.RUnlock()
	bucketteers := make(map[uint64]SigExistsIndex)
	for _, epoch := range multi.epochs {
		if epoch.sigExists != nil {
			bucketteers[epoch.Epoch()] = epoch.sigExists
		}
	}
	return bucketteers
}

func (multi *MultiEpoch) findEpochNumberFromSignature(ctx context.Context, sig solana.Signature) (uint64, error) {
	// FLOW:
	// - if one epoch, just return that epoch
	// - if multiple epochs, use hot tier strategy for optimal performance
	// - Hot Tier: Search last 30 epochs in parallel (handles 90% of requests)
	// - Cold Tier: Search remaining epochs sequentially (handles 10% of requests)
	ttok := time.Now()
	defer func() {
		klog.V(4).Infof("findEpochNumberFromSignature took %s", time.Since(ttok))
	}()

	if epochs := multi.GetEpochNumbers(); len(epochs) == 1 {
		return epochs[0], nil
	}

	numbers := multi.GetEpochNumbers()
	// sort from highest to lowest:
	sort.Slice(numbers, func(i, j int) bool {
		return numbers[i] > numbers[j]
	})

	buckets := multi.getAllBucketteers()

	// Hot tier configuration - search most recent epochs in parallel
	hotTierLimit := multi.options.HotTierLimit
	if hotTierLimit <= 0 {
		hotTierLimit = 30 // default
	}
	if len(numbers) < hotTierLimit {
		hotTierLimit = len(numbers)
	}

	// Helper function to search a single epoch
	searchSingleEpoch := func(epochNumber uint64) (uint64, error) {
		bucket, ok := buckets[epochNumber]
		if !ok {
			return 0, ErrNotFound
		}
		has, err := bucket.Has(sig)
		if err != nil {
			return 0, fmt.Errorf("failed to check if signature exists in bucket: %w", err)
		}
		if !has {
			return 0, ErrNotFound
		}
		epoch, err := multi.GetEpoch(epochNumber)
		if err != nil {
			return 0, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
		}
		if _, err := epoch.FindCidFromSignature(ctx, sig); err == nil {
			return epochNumber, nil
		}
		return 0, ErrNotFound
	}

	// Helper function to search epochs in parallel
	searchEpochsParallel := func(epochNumbers []uint64) (uint64, error) {
		if len(epochNumbers) == 0 {
			return 0, ErrNotFound
		}
		
		jobGroup := NewJobGroup[uint64]()
		for _, epochNumber := range epochNumbers {
			epochNumber := epochNumber // capture for closure
			jobGroup.Add(func(ctx context.Context) (uint64, error) {
				if ctx.Err() != nil {
					return 0, ctx.Err()
				}
				return searchSingleEpoch(epochNumber)
			})
		}
		val, err := jobGroup.RunWithConcurrency(ctx, multi.options.EpochSearchConcurrency)
		if err != nil {
			errs, ok := err.(ErrorSlice)
			if !ok {
				return 0, err
			}
			if errs.All(func(err error) bool {
				return errors.Is(err, ErrNotFound)
			}) {
				return 0, ErrNotFound
			}
			return 0, err
		}
		return val, nil
	}

	// HOT TIER: Search last 30 epochs in parallel (handles 90% of requests)
	if len(numbers) > 0 {
		hotTierEpochs := numbers
		if len(hotTierEpochs) > hotTierLimit {
			hotTierEpochs = hotTierEpochs[:hotTierLimit]
		}
		klog.V(5).Infof("Searching hot tier: %d most recent epochs in parallel (%d-%d)", len(hotTierEpochs), hotTierEpochs[len(hotTierEpochs)-1], hotTierEpochs[0])
		if result, err := searchEpochsParallel(hotTierEpochs); err == nil {
			return result, nil
		} else if !errors.Is(err, ErrNotFound) {
			return 0, err
		}
	}

	// COLD TIER: Search remaining epochs sequentially (handles 10% of requests)
	if len(numbers) > hotTierLimit {
		coldTierEpochs := numbers[hotTierLimit:]
		klog.V(5).Infof("Searching cold tier: %d remaining epochs sequentially (%d-%d)", len(coldTierEpochs), coldTierEpochs[len(coldTierEpochs)-1], coldTierEpochs[0])
		for _, epochNumber := range coldTierEpochs {
			if result, err := searchSingleEpoch(epochNumber); err == nil {
				return result, nil
			} else if !errors.Is(err, ErrNotFound) {
				return 0, err
			}
		}
	}

	return 0, ErrNotFound
}

func (multi *MultiEpoch) handleGetTransaction(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	if multi.CountEpochs() == 0 {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "no epochs available",
		}, fmt.Errorf("no epochs available")
	}

	params, err := parseGetTransactionRequest(req.Params)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "Invalid params",
		}, fmt.Errorf("failed to parse params: %w", err)
	}
	if err := params.Validate(); err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: err.Error(),
		}, fmt.Errorf("failed to validate params: %w", err)
	}

	sig := params.Signature

	// Start span for finding epoch from signature
	epochLookupCtx, epochLookupSpan := telemetry.StartSpan(ctx, "GetTransaction_FindEpochFromSignature")
	epochLookupSpan.SetAttributes(attribute.String("signature", sig.String()))
	startedEpochLookupAt := time.Now()
	epochNumber, err := multi.findEpochNumberFromSignature(epochLookupCtx, sig)
	epochLookupSpan.End()
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// solana just returns null here in case of transaction not found: {"jsonrpc":"2.0","result":null,"id":1}
			return &jsonrpc2.Error{
				Code:    CodeNotFound,
				Message: "Transaction not found",
			}, fmt.Errorf("failed to find epoch number from signature %s: %w", sig, err)
		}
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to get epoch for signature %s: %w", sig, err)
	}
	klog.V(4).Infof("Found signature %s in epoch %d in %s", sig, epochNumber, time.Since(startedEpochLookupAt))

	epochHandler, err := multi.GetEpoch(uint64(epochNumber))
	if err != nil {
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: fmt.Sprintf("Epoch %d is not available from this RPC", epochNumber),
		}, fmt.Errorf("failed to get handler for epoch %d: %w", epochNumber, err)
	}

	// Start span for getting transaction from epoch
	txRetrievalCtx, txRetrievalSpan := telemetry.StartSpan(ctx, "GetTransaction_GetTransactionFromEpoch")
	txRetrievalSpan.SetAttributes(
		attribute.Int64("epoch", int64(epochNumber)),
		attribute.String("signature", sig.String()),
	)
	transactionNode, transactionCid, err := epochHandler.GetTransaction(WithSubrapghPrefetch(txRetrievalCtx, true), sig)
	txRetrievalSpan.End()
	if err != nil {
		if errors.Is(err, compactindexsized.ErrNotFound) {
			// NOTE: solana just returns null here in case of transaction not found: {"jsonrpc":"2.0","result":null,"id":1}
			return &jsonrpc2.Error{
				Code:    CodeNotFound,
				Message: "Transaction not found",
			}, fmt.Errorf("transaction %s not found", sig)
		}
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to get Transaction: %w", err)
	}
	{
		conn.ctx.Response.Header.Set("DAG-Root-CID", transactionCid.String())
	}

	// Start span for parsing transaction and metadata
	_, parseSpan := telemetry.StartSpan(ctx, "GetTransaction_ParseTransactionMeta")
	tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
	parseSpan.End()
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to decode transaction: %w", err)
	}
	out := solanatxmetaparsers.NewEncodedTransactionWithStatusMeta(
		tx,
		meta,
	)

	response, err := out.ToUi(*params.Options.Encoding)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to encode transaction: %w", err)
	}

	response.Value("slot", transactionNode.Slot)
	{
		// Start span for getting block time
		_, blocktimeSpan := telemetry.StartSpan(ctx, "GetTransaction_GetBlockTime")
		blocktimeSpan.SetAttributes(attribute.Int64("slot", int64(transactionNode.Slot)))
		blocktimeIndex := epochHandler.GetBlocktimeIndex()
		if blocktimeIndex != nil {
			blocktime, err := blocktimeIndex.Get(uint64(transactionNode.Slot))
			blocktimeSpan.End()
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to get block: %v", err)
			}
			if blocktime == 0 {
				response.Value("blockTime", nil)
			} else {
				response.Value("blockTime", blocktime)
			}
		} else {
			blocktimeSpan.End()
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get blocktime: blocktime index is not available")
		}
	}

	{
		pos, ok := transactionNode.GetPositionIndex()
		if ok {
			response.Value("position", pos)
		}
	}
	// reply with the data
	err = conn.Reply(
		ctx,
		req.ID,
		response,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}
