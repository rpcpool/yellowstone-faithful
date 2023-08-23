package main

import (
	"context"
	"errors"

	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/sourcegraph/jsonrpc2"
	"k8s.io/klog/v2"
)

func (ser *deprecatedRPCServer) handleGetTransaction(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
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
		if errors.Is(err, compactindex36.ErrNotFound) {
			conn.ReplyNoMod(
				ctx,
				req.ID,
				nil, // NOTE: solana just returns null here in case of transaction not found
			)
			return
		}
		klog.Errorf("failed to get Transaction: %v", err)
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
		pos, ok := transactionNode.GetPositionIndex()
		if ok {
			response.Position = uint64(pos)
		}
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
		response.Signatures = tx.Signatures
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
		func(m map[string]any) map[string]any {
			return adaptTransactionMetaToExpectedOutput(m)
		},
	)
	if err != nil {
		klog.Errorf("failed to reply: %v", err)
	}
}
