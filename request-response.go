package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	jsoniter "github.com/json-iterator/go"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2"
)

type requestContext struct {
	ctx *fasthttp.RequestCtx
}

// ReplyWithError(ctx context.Context, id ID, respErr *Error) error {
func (c *requestContext) ReplyWithError(ctx context.Context, id jsonrpc2.ID, respErr *jsonrpc2.Error) error {
	resp := &jsonrpc2.Response{
		ID:    id,
		Error: respErr,
	}
	replyJSON(c.ctx, http.StatusOK, resp)
	return nil
}

func toMapAny(v any) (map[string]any, error) {
	b, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// MapToCamelCase converts a map[string]interface{} to a map[string]interface{} with camelCase keys
func MapToCamelCase(m map[string]any) map[string]any {
	newMap := make(map[string]any)
	for k, v := range m {
		newMap[toLowerCamelCase(k)] = MapToCamelCaseAny(v)
	}
	return newMap
}

func MapToCamelCaseAny(m any) any {
	if m == nil {
		return nil
	}
	if m, ok := m.(map[string]any); ok {
		return MapToCamelCase(m)
	}
	// if array, convert each element
	if m, ok := m.([]any); ok {
		for i, v := range m {
			m[i] = MapToCamelCaseAny(v)
		}
	}
	return m
}

func toLowerCamelCase(v string) string {
	pascal := bin.ToPascalCase(v)
	if len(pascal) == 0 {
		return ""
	}
	if len(pascal) == 1 {
		return strings.ToLower(pascal)
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// Reply sends a response to the client with the given result.
// The result fields keys are converted to camelCase.
// If remapCallback is not nil, it is called with the result map[string]interface{}.
func (c *requestContext) Reply(
	ctx context.Context,
	id jsonrpc2.ID,
	result interface{},
	remapCallback func(map[string]any) map[string]any,
) error {
	mm, err := toMapAny(result)
	if err != nil {
		return err
	}
	result = MapToCamelCaseAny(mm)
	if remapCallback != nil {
		if mp, ok := result.(map[string]any); ok {
			result = remapCallback(mp)
		}
	}
	resRaw, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(result)
	if err != nil {
		return err
	}
	raw := json.RawMessage(resRaw)
	resp := &jsonrpc2.Response{
		ID:     id,
		Result: &raw,
	}
	replyJSON(c.ctx, http.StatusOK, resp)
	return err
}

// ReplyRaw sends a raw response without any processing (no camelCase conversion, etc).
func (c *requestContext) ReplyRaw(
	ctx context.Context,
	id jsonrpc2.ID,
	result interface{},
) error {
	resRaw, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(result)
	if err != nil {
		return err
	}
	raw := json.RawMessage(resRaw)
	resp := &jsonrpc2.Response{
		ID:     id,
		Result: &raw,
	}
	replyJSON(c.ctx, http.StatusOK, resp)
	return err
}

func putValueIntoContext(ctx context.Context, key, value interface{}) context.Context {
	return context.WithValue(ctx, key, value)
}

func getValueFromContext(ctx context.Context, key interface{}) interface{} {
	return ctx.Value(key)
}

// WithSubrapghPrefetch sets the prefetch flag in the context
// to enable prefetching of subgraphs.
func WithSubrapghPrefetch(ctx context.Context, yesNo bool) context.Context {
	return putValueIntoContext(ctx, "prefetch", yesNo)
}

type GetBlockRequest struct {
	Slot uint64 `json:"slot"`
	// TODO: add more params
}

func parseGetBlockRequest(raw *json.RawMessage) (*GetBlockRequest, error) {
	var params []any
	if err := json.Unmarshal(*raw, &params); err != nil {
		klog.Errorf("failed to unmarshal params: %v", err)
		return nil, err
	}
	slotRaw, ok := params[0].(float64)
	if !ok {
		klog.Errorf("first argument must be a number, got %T", params[0])
		return nil, nil
	}

	return &GetBlockRequest{
		Slot: uint64(slotRaw),
	}, nil
}

type GetTransactionRequest struct {
	Signature solana.Signature `json:"signature"`
	// TODO: add more params
}

func parseGetTransactionRequest(raw *json.RawMessage) (*GetTransactionRequest, error) {
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

	sig, err := solana.SignatureFromBase58(sigRaw)
	if err != nil {
		klog.Errorf("failed to convert signature from base58: %v", err)
		return nil, err
	}
	return &GetTransactionRequest{
		Signature: sig,
	}, nil
}
