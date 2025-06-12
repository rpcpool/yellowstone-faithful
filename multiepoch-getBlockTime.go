package main

import (
	"context"
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/slottools"
	"github.com/sourcegraph/jsonrpc2"
)

func (multi *MultiEpoch) handleGetBlockTime(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	blockNum, err := parseGetBlockTimeRequest(req.Params)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "Invalid params",
		}, fmt.Errorf("failed to parse params: %w", err)
	}

	// find the epoch that contains the requested slot
	epochNumber := slottools.CalcEpochForSlot(blockNum)
	epochHandler, err := multi.GetEpoch(epochNumber)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: fmt.Sprintf("Epoch %d is not available", epochNumber),
		}, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
	}
	{
		blocktimeIndex := epochHandler.GetBlocktimeIndex()
		if blocktimeIndex != nil {
			blockTime, err := blocktimeIndex.Get(blockNum)
			if err != nil {
				return &jsonrpc2.Error{
					Code:    CodeNotFound,
					Message: fmt.Sprintf("Slot %d was skipped, or missing in long-term storage", blockNum),
				}, fmt.Errorf("failed to get blocktime: %w", err)
			}
			err = conn.Reply(
				ctx,
				req.ID,
				func() any {
					if blockTime != 0 {
						return blockTime
					}
					return nil
				}(),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to reply: %w", err)
			}
			return nil, nil
		} else {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Failed to get block",
			}, fmt.Errorf("failed to get blocktime: blocktime index is nil")
		}
	}
}
