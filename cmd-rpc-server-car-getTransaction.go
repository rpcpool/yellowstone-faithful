package main

import (
	"context"

	"github.com/sourcegraph/jsonrpc2"
	"k8s.io/klog/v2"
)

func ptrToUint64(v uint64) *uint64 {
	return &v
}

func (ser *rpcServer) handleGetTransaction(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
	params, err := parseGetTransactionRequest(req.Params)
	if err != nil {
		klog.Errorf("failed to parse params: %v", err)
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeInvalidParams,
				Message: "Invalid params",
			})
		return
	}

	sig := params.Signature

	transactionNode, err := ser.GetTransaction(WithSubrapghPrefetch(ctx, true), sig)
	if err != nil {
		klog.Errorf("failed to decode Transaction: %v", err)
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			})
		return
	}

	var response GetTransactionResponse

	response.Slot = ptrToUint64(uint64(transactionNode.Slot))
	{
		block, err := ser.GetBlock(ctx, uint64(transactionNode.Slot))
		if err != nil {
			klog.Errorf("failed to decode block: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				})
			return
		}
		blocktime := uint64(block.Meta.Blocktime)
		if blocktime != 0 {
			response.Blocktime = &blocktime
		}
	}

	{
		tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, ser.GetDataFrameByCid)
		if err != nil {
			klog.Errorf("failed to decode transaction: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				})
			return
		}
		if tx.Message.IsVersioned() {
			response.Version = tx.Message.GetVersion() - 1
		} else {
			response.Version = "legacy"
		}
		response.Meta = meta

		b64Tx, err := tx.ToBase64()
		if err != nil {
			klog.Errorf("failed to encode transaction: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				})
			return
		}

		response.Transaction = []any{b64Tx, "base64"}
	}

	// reply with the data
	err = conn.Reply(
		ctx,
		req.ID,
		response,
		nil,
	)
	if err != nil {
		klog.Errorf("failed to reply: %v", err)
	}
}
