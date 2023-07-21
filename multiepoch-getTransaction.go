package main

import (
	"context"

	"github.com/sourcegraph/jsonrpc2"
)

func (ser *MultiEpoch) handleGetTransaction(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	panic("not implemented")
}
