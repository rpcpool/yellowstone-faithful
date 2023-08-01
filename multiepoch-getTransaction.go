package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	sigtoepoch "github.com/rpcpool/yellowstone-faithful/sig-to-epoch"
	"github.com/sourcegraph/jsonrpc2"
)

func (ser *MultiEpoch) handleGetTransaction(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	if ser.sigToEpoch == nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "getTransaction method is not enabled",
		}, fmt.Errorf("the sig-to-epoch index was not provided")
	}

	params, err := parseGetTransactionRequest(req.Params)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "Invalid params",
		}, fmt.Errorf("failed to parse params: %v", err)
	}

	sig := params.Signature

	epochNumber, err := ser.sigToEpoch.Get(ctx, sig)
	if err != nil {
		if sigtoepoch.IsNotFound(err) {
			return nil, fmt.Errorf("not found epoch for signature %s", sig)
		}
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to get epoch for signature %s: %v", sig, err)
	}

	epochHandler, err := ser.GetEpoch(uint64(epochNumber))
	if err != nil {
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: fmt.Sprintf("Epoch %d is not available from this RPC", epochNumber),
		}, fmt.Errorf("failed to get handler for epoch %d: %w", epochNumber, err)
	}

	transactionNode, err := epochHandler.GetTransaction(WithSubrapghPrefetch(ctx, true), sig)
	if err != nil {
		if errors.Is(err, compactindex36.ErrNotFound) {
			// NOTE: solana just returns null here in case of transaction not found
			return &jsonrpc2.Error{
				Code:    CodeNotFound,
				Message: "Transaction not found",
			}, fmt.Errorf("transaction %s not found", sig)
		}
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to get Transaction: %v", err)
	}

	var response GetTransactionResponse

	response.Slot = ptrToUint64(uint64(transactionNode.Slot))
	{
		block, err := epochHandler.GetBlock(ctx, uint64(transactionNode.Slot))
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get block: %v", err)
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
		tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to decode transaction: %v", err)
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
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to encode transaction: %v", err)
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
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}
