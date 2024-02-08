package main

import (
	"context"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

func (multi *MultiEpoch) handleGetFirstAvailableBlock(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	firstBlock, err := multi.GetFirstAvailableBlock(ctx)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: "Internal error",
		}, fmt.Errorf("failed to get first available block: %w", err)
	}

	slotNumber := uint64(firstBlock.Slot)
	err = conn.ReplyRaw(
		ctx,
		req.ID,
		slotNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}
