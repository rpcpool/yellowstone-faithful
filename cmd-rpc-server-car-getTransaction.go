package main

import (
	"context"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
	"k8s.io/klog/v2"
)

func (ser *rpcServer) getTransaction(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
	params, err := parseGetTransactionRequest(req.Params)
	if err != nil {
		klog.Errorf("failed to parse params: %v", err)
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeInvalidParams,
				Message: "invalid params",
			})
		return
	}

	sig := params.Signature

	transactionNode, err := ser.GetTransaction(ctx, sig)
	if err != nil {
		klog.Errorf("failed to decode Transaction: %v", err)
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "internal error",
			})
		return
	}

	var response GetTransactionResponse

	response.Slot = uint64(transactionNode.Slot)
	{
		block, err := ser.GetBlock(ctx, uint64(transactionNode.Slot))
		if err != nil {
			klog.Errorf("failed to decode block: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
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
					Message: "internal error",
				})
			return
		}
		if tx.Message.IsVersioned() {
			// TODO: use the actual version
			response.Version = fmt.Sprintf("%d", tx.Message.GetVersion())
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
					Message: "internal error",
				})
			return
		}

		response.Transaction = []any{b64Tx, "base64"}
	}

	// reply with the data
	err = conn.Reply(ctx, req.ID, response)
	if err != nil {
		klog.Errorf("failed to reply: %v", err)
	}
}
