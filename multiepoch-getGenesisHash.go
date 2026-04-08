package main

import (
	"context"
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/errctx"
	"github.com/sourcegraph/jsonrpc2"
)

func (multi *MultiEpoch) handleGetGenesisHash(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	// Epoch 0 contains the genesis config.
	epochNumber := uint64(0)
	epochHandler, err := multi.GetEpoch(epochNumber)
	if err != nil {
		// If epoch 0 is not available, then the genesis config is not available.
		return NewSlotWasSkippedOrMissingError(0), errctx.Wrap(fmt.Errorf("failed to get epoch %d: %w", epochNumber, err), "GetGenesisHash_GetEpochHandler")
	}

	genesis := epochHandler.GetGenesis()
	if genesis == nil {
		return NewSlotWasSkippedOrMissingError(0), fmt.Errorf("genesis is nil")
	}

	genesisHash := genesis.Hash

	err = conn.Reply(
		ctx,
		req.ID,
		genesisHash.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}
