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
	// - if multiple epochs, use tiered search strategy to minimize disk I/O
	// - Tier 1: Search last-3 to last-10 epochs (e.g., 991-998 for 1000 total epochs)
	// - Tier 2: Search last-10 to last-50 epochs (e.g., 951-990 for 1000 total epochs)
	// - Tier 3: Search remaining epochs if not found
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

	// Define tier thresholds from configuration
	// We expect this to be handled upstream by the HOT TIER
	tier1Start := 0  // Start from last-3
	tier1End := multi.options.Tier1EpochLimit
	if tier1End <= 0 {
		tier1End = 5 // default: last-10
	}
	tier2Start := multi.options.Tier1EpochLimit
	if tier2Start <= 0 {
		tier2Start = 10 // default: last-10
	}
	tier2End := multi.options.Tier2EpochLimit
	if tier2End <= 0 {
		tier2End = 30 // default: last-50
	}

	// Helper function to search a subset of epochs with specific concurrency
	searchEpochs := func(epochNumbers []uint64, concurrency int) (uint64, error) {
		if len(epochNumbers) == 0 {
			return 0, ErrNotFound
		}
		
		// Use provided concurrency, or default to EpochSearchConcurrency
		if concurrency <= 0 {
			concurrency = multi.options.EpochSearchConcurrency
		}
		
		jobGroup := NewJobGroup[uint64]()
		for _, epochNumber := range epochNumbers {
			epochNumber := epochNumber // capture for closure
			jobGroup.Add(func(ctx context.Context) (uint64, error) {
				if ctx.Err() != nil {
					return 0, ctx.Err()
				}
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
				// Not found in this epoch.
				return 0, ErrNotFound
			})
		}
		val, err := jobGroup.RunWithConcurrency(ctx, concurrency)
		if err != nil {
			errs, ok := err.(ErrorSlice)
			if !ok {
				// An error occurred while searching one of the epochs.
				return 0, err
			}
			// All epochs were searched, but the signature was not found.
			if errs.All(func(err error) bool {
				return errors.Is(err, ErrNotFound)
			}) {
				return 0, ErrNotFound
			}
			return 0, err
		}
		// The signature was found in one of the epochs.
		return val, nil
	}

	// Tier 1: Search most recent epochs
	if len(numbers) > 0 {
		var tier1Epochs []uint64
		if tier1Start > 0 {
			// Start from last-N (e.g., last-3)
			if len(numbers) >= tier1Start {
				tier1Epochs = numbers[tier1Start-1:]
			}
		} else {
			// Start from the very beginning (most recent)
			tier1Epochs = numbers
		}
		
		// Limit to tier1End epochs
		if len(tier1Epochs) > tier1End {
			tier1Epochs = tier1Epochs[:tier1End]
		}
		
		if len(tier1Epochs) > 0 {
			klog.V(5).Infof("Searching tier 1: %d most recent epochs (%d-%d)", len(tier1Epochs), tier1Epochs[len(tier1Epochs)-1], tier1Epochs[0])
			if result, err := searchEpochs(tier1Epochs, multi.options.Tier1Concurrency); err == nil {
				return result, nil
			} else if !errors.Is(err, ErrNotFound) {
				return 0, err
			}
		}
	}

	// Tier 2: Search next batch of epochs (no overlap with tier 1)
	if len(numbers) > tier1End {
		tier2Epochs := numbers[tier1End:]
		if len(tier2Epochs) > (tier2End - tier1End) {
			tier2Epochs = tier2Epochs[:(tier2End - tier1End)]
		}
		if len(tier2Epochs) > 0 {
			klog.V(5).Infof("Searching tier 2: %d epochs (%d-%d)", len(tier2Epochs), tier2Epochs[len(tier2Epochs)-1], tier2Epochs[0])
			if result, err := searchEpochs(tier2Epochs, multi.options.Tier2Concurrency); err == nil {
				return result, nil
			} else if !errors.Is(err, ErrNotFound) {
				return 0, err
			}
		}
	}

	// Tier 3: Search all remaining epochs (no overlap with previous tiers)
	if len(numbers) > tier2End {
		tier3Epochs := numbers[tier2End:]
		klog.V(5).Infof("Searching tier 3: remaining %d epochs (%d-%d)", len(tier3Epochs), tier3Epochs[len(tier3Epochs)-1], tier3Epochs[0])
		if result, err := searchEpochs(tier3Epochs, multi.options.EpochSearchConcurrency); err == nil {
			return result, nil
		} else if !errors.Is(err, ErrNotFound) {
			return 0, err
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
