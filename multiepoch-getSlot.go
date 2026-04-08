package main

import (
	"context"
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/errctx"
	"github.com/sourcegraph/jsonrpc2"
)

func (multi *MultiEpoch) handleGetSlot(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	// TODO: parse params?
	lastBlock, err := multi.GetMostRecentAvailableBlock(ctx)
	if err != nil {
		return NewInternalError(), errctx.Wrap(fmt.Errorf("failed to get first available block: %w", err), "GetSlot_GetMostRecentAvailableBlock")
	}

	slotNumber := uint64(lastBlock.Slot)
	err = conn.Reply(
		ctx,
		req.ID,
		slotNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}
