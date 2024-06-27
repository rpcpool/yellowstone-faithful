package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/sourcegraph/jsonrpc2"
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
	// - if multiple epochs, use sigToEpoch to find the epoch number
	// - if sigToEpoch is not available, linear search through all epochs

	if epochs := multi.GetEpochNumbers(); len(epochs) == 1 {
		return epochs[0], nil
	}

	numbers := multi.GetEpochNumbers()
	// sort from highest to lowest:
	sort.Slice(numbers, func(i, j int) bool {
		return numbers[i] > numbers[j]
	})

	buckets := multi.getAllBucketteers()

	// Search all epochs in parallel:
	wg := NewFirstResponse(ctx, multi.options.EpochSearchConcurrency)
	for i := range numbers {
		epochNumber := numbers[i]
		wg.Spawn(func() (any, error) {
			bucket, ok := buckets[epochNumber]
			if !ok {
				return nil, nil
			}
			has, err := bucket.Has(sig)
			if err != nil {
				return 0, fmt.Errorf("failed to check if signature exists in bucket: %w", err)
			}
			if !has {
				return nil, nil
			}
			epoch, err := multi.GetEpoch(epochNumber)
			if err != nil {
				return nil, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
			}
			if _, err := epoch.FindCidFromSignature(ctx, sig); err == nil {
				return epochNumber, nil
			}
			// Not found in this epoch.
			return nil, nil
		})
	}
	switch result := wg.Wait().(type) {
	case nil:
		// All epochs were searched, but the signature was not found.
		return 0, ErrNotFound
	case error:
		// An error occurred while searching one of the epochs.
		return 0, result
	case uint64:
		// The signature was found in one of the epochs.
		return result, nil
	default:
		return 0, fmt.Errorf("unexpected result: (%T) %v", result, result)
	}
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

	startedEpochLookupAt := time.Now()
	epochNumber, err := multi.findEpochNumberFromSignature(ctx, sig)
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

	transactionNode, transactionCid, err := epochHandler.GetTransaction(WithSubrapghPrefetch(ctx, true), sig)
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

	var response GetTransactionResponse

	response.Slot = ptrToUint64(uint64(transactionNode.Slot))
	{
		block, _, err := epochHandler.GetBlock(ctx, uint64(transactionNode.Slot))
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get block: %w", err)
		}
		blocktime := uint64(block.Meta.Blocktime)
		if blocktime != 0 {
			response.Blocktime = &blocktime
		}
	}

	{
		pos, ok := transactionNode.GetPositionIndex()
		if ok {
			response.Position = uint64(pos)
		}
		tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to decode transaction: %w", err)
		}
		response.Signatures = tx.Signatures
		if tx.Message.IsVersioned() {
			response.Version = tx.Message.GetVersion() - 1
		} else {
			response.Version = "legacy"
		}

		encodedTx, encodedMeta, err := encodeTransactionResponseBasedOnWantedEncoding(*params.Options.Encoding, tx, meta)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to encode transaction: %w", err)
		}
		response.Transaction = encodedTx
		response.Meta = encodedMeta
	}

	// reply with the data
	err = conn.Reply(
		ctx,
		req.ID,
		response,
		func(m map[string]any) map[string]any {
			return adaptTransactionMetaToExpectedOutput(m)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}
