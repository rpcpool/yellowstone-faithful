package main

import (
	"context"
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/errctx"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	"github.com/sourcegraph/jsonrpc2"
)

func (multi *MultiEpoch) handleGetBlockTime(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	blockNum, err := parseGetBlockTimeRequest(req.Params)
	if err != nil {
		return NewInvalidParamsError(InvalidParamsString), fmt.Errorf("failed to parse params: %w", err)
	}

	// find the epoch that contains the requested slot
	epochNumber := slottools.CalcEpochForSlot(blockNum)
	epochHandler, err := multi.GetEpoch(epochNumber)
	if err != nil {
		return NewSlotWasSkippedOrMissingError(blockNum), errctx.Wrap(fmt.Errorf("failed to get epoch %d: %w", epochNumber, err), "GetBlockTime_GetEpochHandler")
	}
	{
		blocktimeIndex := epochHandler.GetBlocktimeIndex()
		if blocktimeIndex != nil {
			blockTime, err := blocktimeIndex.Get(blockNum)
			if err != nil {
				return NewSlotWasSkippedOrMissingError(blockNum), errctx.Wrap(fmt.Errorf("failed to get blocktime: %w", err), "GetBlockTime_GetBlocktime")
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
			return NewInternalError(), errctx.Wrap(fmt.Errorf("failed to get blocktime: blocktime index is nil"), "GetBlockTime_GetBlocktimeIndexNil")
		}
	}
}
