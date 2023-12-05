package main

import (
	"context"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

func (multi *MultiEpoch) handleGetGenesisHash(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	// Epoch 0 contains the genesis config.
	epochNumber := uint64(0)
	epochHandler, err := multi.GetEpoch(epochNumber)
	if err != nil {
		// If epoch 0 is not available, then the genesis config is not available.
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: fmt.Sprintf("Epoch %d is not available", epochNumber),
		}, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
	}

	genesis := epochHandler.GetGenesis()
	if genesis == nil {
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: "Genesis is not available",
		}, fmt.Errorf("genesis is nil")
	}

	genesisHash := genesis.Hash

	err = conn.ReplyRaw(
		ctx,
		req.ID,
		genesisHash.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}
