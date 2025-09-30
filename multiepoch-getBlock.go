package main

import (
	"context"

	jsoniter "github.com/json-iterator/go"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/sourcegraph/jsonrpc2"
)

var fasterJson = jsoniter.ConfigCompatibleWithStandardLibrary

type MyContextKey string

const requestIDKey = MyContextKey("requestID")

func setRequestIDToContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func getRequestIDFromContext(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}

func (multi *MultiEpoch) handleGetBlock(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	return multi.handleGetBlock_car(ctx, conn, req)
}

func mergeTxNodeSlices(slices [][]*ipldbindcode.Transaction) []*ipldbindcode.Transaction {
	var out []*ipldbindcode.Transaction
	for _, slice := range slices {
		out = append(out, slice...)
	}
	return out
}
