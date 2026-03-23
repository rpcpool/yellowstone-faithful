package main

import (
	"context"
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/slottools"
	"github.com/rpcpool/yellowstone-faithful/telemetry"
	"github.com/sourcegraph/jsonrpc2"
	"k8s.io/klog/v2"
)

func (multi *MultiEpoch) handleGetBlocks(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	rpcSpanCtx, rpcSpan := telemetry.StartSpan(ctx, "jsonrpc.GetBlocks")
	defer rpcSpan.End()

	tim := newTimer(getRequestIDFromContext(rpcSpanCtx))
	params, err := parseGetBlocksRequest(req.Params)
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
	tim.time("parseGetBlockRequest")

	klog.Infof("GetBlocks request for slots %d to %d", params.StartSlot, params.EndSlot)

	resultBlocks, rpcErr, err := multi.getBlocksInRange(rpcSpanCtx, params.StartSlot, params.EndSlot)
	if rpcErr != nil || err != nil {
		return rpcErr, err
	}
	tim.time("findBlocks")

	err = conn.Reply(
		ctx,
		req.ID,
		func() any {
			if len(resultBlocks) > 0 {
				return resultBlocks
			}
			return nil
		}(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}

	return nil, nil
}

func (multi *MultiEpoch) handleGetBlocksWithLimit(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	rpcSpanCtx, rpcSpan := telemetry.StartSpan(ctx, "jsonrpc.GetBlocksWithLimit")
	defer rpcSpan.End()

	tim := newTimer(getRequestIDFromContext(rpcSpanCtx))
	params, err := parseGetBlocksWithLimitRequest(req.Params)
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
	tim.time("parseGetBlocksWithLimitRequest")

	endSlot := params.StartSlot + params.Limit - 1
	klog.Infof("GetBlocksWithLimit request for slots %d to %d (limit %d)", params.StartSlot, endSlot, params.Limit)

	resultBlocks, rpcErr, err := multi.getBlocksInRange(rpcSpanCtx, params.StartSlot, endSlot)
	if rpcErr != nil || err != nil {
		return rpcErr, err
	}
	tim.time("findBlocks")

	// Truncate to the requested limit.
	if uint64(len(resultBlocks)) > params.Limit {
		resultBlocks = resultBlocks[:params.Limit]
	}

	err = conn.Reply(
		ctx,
		req.ID,
		func() any {
			if len(resultBlocks) > 0 {
				return resultBlocks
			}
			return nil
		}(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}

	return nil, nil
}

// getBlocksInRange returns all confirmed block slots in [startSlot, endSlot].
func (multi *MultiEpoch) getBlocksInRange(ctx context.Context, startSlot, endSlot uint64) ([]uint64, *jsonrpc2.Error, error) {
	startEpochNumber := slottools.CalcEpochForSlot(startSlot)
	endEpochNumber := slottools.CalcEpochForSlot(endSlot)

	var epochs []*Epoch
	_, epochLookupSpan := telemetry.StartSpan(ctx, "GetBlocks_EpochLookups")
	for epochNumber := startEpochNumber; epochNumber <= endEpochNumber; epochNumber++ {
		epoch, err := multi.GetEpoch(epochNumber)
		if err != nil {
			epochLookupSpan.End()
			return nil, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Failed to get epoch",
			}, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
		}
		if epoch == nil {
			epochLookupSpan.End()
			return nil, &jsonrpc2.Error{
				Code:    CodeNotFound,
				Message: fmt.Sprintf("Epoch %d not found", epochNumber),
			}, fmt.Errorf("epoch %d not found", epochNumber)
		}
		epochs = append(epochs, epoch)
	}
	epochLookupSpan.End()

	var resultBlocks []uint64
	_, blockSearchSpan := telemetry.StartSpan(ctx, "GetBlocks_BlockSearch")

	startIdx := findSlotIndexBinarySearch(startSlot, epochs[0].blocks)
	endIdx := findSlotIndexBinarySearch(endSlot, epochs[len(epochs)-1].blocks)
	if len(epochs) == 1 {
		resultBlocks = epochs[0].blocks[startIdx:endIdx]
	} else {
		resultBlocks = append(resultBlocks, epochs[0].blocks[startIdx:]...)
		for i := 1; i < len(epochs)-1; i++ {
			resultBlocks = append(resultBlocks, epochs[i].blocks...)
		}
		resultBlocks = append(resultBlocks, epochs[len(epochs)-1].blocks[:endIdx]...)
	}

	blockSearchSpan.End()
	return resultBlocks, nil, nil
}

// findSlotIndexBinarySearch returns the index of the first block >= slot in blocks.
// If slot is not found, returns the insertion point.
func findSlotIndexBinarySearch(slot uint64, blocks []uint64) int {
	lo, hi := 0, len(blocks)-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		if blocks[mid] < slot {
			lo = mid + 1
		} else if blocks[mid] > slot {
			hi = mid - 1
		} else {
			return mid
		}
	}
	return lo
}
