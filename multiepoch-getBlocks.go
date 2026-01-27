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

	startEpochNumber := slottools.CalcEpochForSlot(params.StartSlot)
	endEpochNumber := slottools.CalcEpochForSlot(params.EndSlot)

	klog.Infof("GetBlocks request for slots %d to %d (epochs %d to %d)", params.StartSlot, params.EndSlot, startEpochNumber, endEpochNumber)

	// Load all the requested epochs
	var epochs []*Epoch
	_, epochLookupSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlocks_EpochLookups")
	for epochNumber := startEpochNumber; epochNumber <= endEpochNumber; epochNumber++ {
		epoch, err := multi.GetEpoch(epochNumber)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Failed to get epoch",
			}, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
		}
		if epoch == nil {
			return &jsonrpc2.Error{
				Code:    CodeNotFound,
				Message: fmt.Sprintf("Epoch %d not found", epochNumber),
			}, fmt.Errorf("epoch %d not found", epochNumber)
		}
		epochs = append(epochs, epoch)
	}
	epochLookupSpan.End()
	tim.time("loadEpochs")

	// Find the index of the start slot in the first epoch using binary search
	var resultBlocks []uint64
	_, blockSearchSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlocks_BlockSearch")

	startIdx := findSlotIndexBinarySearch(params.StartSlot, epochs[0].blocks)
	endIdx := findSlotIndexBinarySearch(params.EndSlot, epochs[len(epochs)-1].blocks)
	// startIdx is now the index of the first block >= StartSlot in epochs[0].blocks
	if len(epochs) == 1 {
		// If we are only in one epoch, we can directly slice the blocks
		resultBlocks = epochs[0].blocks[startIdx:endIdx]
	} else {
		// Otherwise, we need to gather blocks from multiple epochs
		resultBlocks = append(resultBlocks, epochs[0].blocks[startIdx:]...)
		for i := 1; i < len(epochs)-1; i++ {
			resultBlocks = append(resultBlocks, epochs[i].blocks...)
		}
		resultBlocks = append(resultBlocks, epochs[len(epochs)-1].blocks[:endIdx]...)
	}
	tim.time("findBlocks")

	blockSearchSpan.End()

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
