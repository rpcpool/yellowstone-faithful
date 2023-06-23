package main

import (
	"context"
	"encoding/json"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/gsfa/offsetstore"
	"github.com/sourcegraph/jsonrpc2"
	"k8s.io/klog/v2"
)

type GetSignaturesForAddressParams struct {
	Address solana.PublicKey `json:"address"`
	Limit   int              `json:"limit"`
	// TODO: add more params
}

func parseGetSignaturesForAddressParams(raw *json.RawMessage) (*GetSignaturesForAddressParams, error) {
	var params []any
	if err := json.Unmarshal(*raw, &params); err != nil {
		klog.Errorf("failed to unmarshal params: %v", err)
		return nil, err
	}
	sigRaw, ok := params[0].(string)
	if !ok {
		klog.Errorf("first argument must be a string")
		return nil, nil
	}

	out := &GetSignaturesForAddressParams{}
	pk, err := solana.PublicKeyFromBase58(sigRaw)
	if err != nil {
		klog.Errorf("failed to parse pubkey from base58: %v", err)
		return nil, err
	}
	out.Address = pk

	if len(params) > 1 {
		// the second param should be a map[string]interface{}
		// with the optional params
		if m, ok := params[1].(map[string]interface{}); ok {
			if limit, ok := m["limit"]; ok {
				if limit, ok := limit.(float64); ok {
					out.Limit = int(limit)
				}
			}
		}
	}
	if out.Limit <= 0 || out.Limit > 1000 {
		// default limit
		out.Limit = 1000
	}
	return out, nil
}

func (ser *rpcServer) getSignaturesForAddress(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
	if ser.gsfaReader == nil {
		klog.Errorf("gsfaReader is nil")
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "getSignaturesForAddress method is not enabled",
			})
		return
	}

	params, err := parseGetSignaturesForAddressParams(req.Params)
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
	pk := params.Address
	limit := params.Limit

	sigs, err := ser.gsfaReader.Get(context.Background(), pk, limit)
	if err != nil {
		if offsetstore.IsNotFound(err) {
			klog.Infof("No signatures found for address: %s", pk)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Not found",
				})
			return
		}
	}

	// The response is an array of objects: [{signature: string}]
	response := make([]map[string]string, len(sigs))
	for i, sig := range sigs {
		response[i] = map[string]string{
			"signature": sig.String(),
		}
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
