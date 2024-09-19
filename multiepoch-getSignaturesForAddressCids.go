package main

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/sourcegraph/jsonrpc2"
)

func countTransactionCids(v gsfa.EpochToTransactionCids) int {
	var count int
	for _, txs := range v {
		count += len(txs)
	}
	return count
}

func (multi *MultiEpoch) handleGetSignaturesForAddressCids(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	// - parse and validate request
	// - get list of epochs (from most recent to oldest)
	// - iterate until we find the requested number of signatures
	// - expand the signatures with tx data
	signaturesOnly := multi.options.GsfaOnlySignatures

	params, err := parseGetSignaturesForAddressParams(req.Params)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "Invalid params",
		}, fmt.Errorf("failed to parse params: %v", err)
	}
	pk := params.Address
	limit := params.Limit

	gsfaIndexes, _ := multi.getGsfaReadersInEpochDescendingOrder()
	if len(gsfaIndexes) == 0 {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "getSignaturesForAddress method is not enabled",
		}, fmt.Errorf("no gsfa indexes found")
	}

	gsfaMulti, err := gsfa.NewGsfaReaderMultiepoch(gsfaIndexes)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to create gsfa multiepoch reader: %w", err)
	}

	// Get the transactions:
	foundTransactions, err := gsfaMulti.GetCids(
		ctx,
		pk,
		limit,
		func(epochNum uint64, oas linkedlog.OffsetAndSizeAndBlocktime) (cid.Cid, error) {
			epoch, err := multi.GetEpoch(epochNum)
			if err != nil {
				return cid.Cid{}, fmt.Errorf("failed to get epoch %d: %w", epochNum, err)
			}
			raw, err := epoch.GetNodeByOffsetAndSize(ctx, nil, &indexes.OffsetAndSize{
				Offset: oas.Offset,
				Size:   oas.Size,
			})
			if err != nil {
				return cid.Cid{}, fmt.Errorf("failed to get signature: %w", err)
			}
			cb := cid.V1Builder{MhLength: -1, MhType: uint64(multicodec.Sha2_256), Codec: uint64(multicodec.DagCbor)}
			c, err := cb.Sum(raw)
			if err != nil {
				return cid.Cid{}, fmt.Errorf("failed to get cid: %w", err)
			}
			return c, nil
		},
	)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to get signatures: %w", err)
	}

	if len(foundTransactions) == 0 {
		err = conn.ReplyRaw(
			ctx,
			req.ID,
			[]map[string]any{},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to reply: %w", err)
		}
		return nil, nil
	}

	// The response is an array of objects: [{sigCid: string}]
	response := make([]map[string]any, countTransactionCids(foundTransactions))
	numBefore := 0
	for ei := range foundTransactions {
		epoch := ei
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get epoch %d: %w", epoch, err)
		}

		sigs := foundTransactions[ei]
		for i := range sigs {
			ii := numBefore + i
			c := sigs[i]
			err := func() error {
				response[ii] = map[string]any{
					"sigCid": c.String(),
				}
				if signaturesOnly {
					return nil
				}

				return nil
			}()
			if err != nil {
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to get tx data: %w", err)
			}
		}
		numBefore += len(sigs)
	}
	// reply with the data
	err = conn.ReplyRaw(
		ctx,
		req.ID,
		response,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}

	return nil, nil
}
