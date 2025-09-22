package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	jsoniter "github.com/json-iterator/go"
	"github.com/mostynb/zstdpool-freelist"
	"github.com/mr-tron/base58"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
	"github.com/rpcpool/yellowstone-faithful/jsonparsed"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/valyala/fasthttp"
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

// Reply sends a raw response without any processing (no camelCase conversion, etc).
func (c *requestContext) Reply(
	ctx context.Context,
	id jsonrpc2.ID,
	result any,
) error {
	resRaw, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(result)
	if err != nil {
		return err
	}
	{
		// if result has Put method, call it:
		if puttable, ok := result.(jsonbuilder.Recyclable); ok {
			puttable.Put()
		}
	}
	raw := json.RawMessage(resRaw)
	resp := &jsonrpc2.Response{
		ID:     id,
		Result: &raw,
	}
	replyJSON(c.ctx, http.StatusOK, resp)
	return err
}

func (c *requestContext) ReplyRawMessage(
	ctx context.Context,
	id jsonrpc2.ID,
	result json.RawMessage,
) {
	resp := &jsonrpc2.Response{
		ID:     id,
		Result: &result,
	}
	replyJSON(c.ctx, http.StatusOK, resp)
}

func putValueIntoContext(ctx context.Context, key, value any) context.Context {
	return context.WithValue(ctx, key, value)
}

func getValueFromContext(ctx context.Context, key any) any {
	return ctx.Value(key)
}

// WithSubrapghPrefetch sets the prefetch flag in the context
// to enable prefetching of subgraphs.
func WithSubrapghPrefetch(ctx context.Context, yesNo bool) context.Context {
	return putValueIntoContext(ctx, "prefetch", yesNo)
}

type GetBlockRequest struct {
	Slot    uint64 `json:"slot"`
	Options struct {
		Commitment                     *rpc.CommitmentType         `json:"commitment,omitempty"` // default: "finalized"
		Encoding                       *solana.EncodingType        `json:"encoding,omitempty"`   // default: "json"
		MaxSupportedTransactionVersion *uint64                     `json:"maxSupportedTransactionVersion,omitempty"`
		TransactionDetails             *rpc.TransactionDetailsType `json:"transactionDetails,omitempty"` // default: "full"
		Rewards                        *bool                       `json:"rewards,omitempty"`
	} `json:"options,omitempty"`
}

// Validate validates the request.
func (req *GetBlockRequest) Validate() error {
	if req.Options.Encoding != nil && !isAnyEncodingOf(
		*req.Options.Encoding,
		solana.EncodingBase58,
		solana.EncodingBase64,
		solana.EncodingBase64Zstd,
		solana.EncodingJSON,
		solana.EncodingJSONParsed,
	) {
		return fmt.Errorf("unsupported encoding")
	}
	if req.Options.Encoding != nil && *req.Options.Encoding == solana.EncodingJSONParsed && !jsonparsed.IsEnabled() {
		return fmt.Errorf("encoding=jsonParsed is not enabled on this server")
	}
	if req.Options.TransactionDetails != nil &&
		*req.Options.TransactionDetails != rpc.TransactionDetailsFull &&
		*req.Options.TransactionDetails != rpc.TransactionDetailsNone &&
		*req.Options.TransactionDetails != rpc.TransactionDetailsSignatures &&
		*req.Options.TransactionDetails != rpc.TransactionDetailsAccounts {
		return fmt.Errorf("unsupported transaction details: %s", *req.Options.TransactionDetails)
	}
	return nil
}

func parseGetBlockRequest(raw *json.RawMessage) (*GetBlockRequest, error) {
	var params []any
	if err := fasterJson.Unmarshal(*raw, &params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal params: %w", err)
	}
	if len(params) < 1 {
		return nil, fmt.Errorf("params must have at least one argument")
	}
	slotRaw, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("first argument must be a number, got %T", params[0])
	}

	out := &GetBlockRequest{
		Slot: uint64(slotRaw),
	}

	if len(params) > 1 {
		optionsRaw, ok := params[1].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("second argument must be an object, got %T", params[1])
		}
		if commitmentRaw, ok := optionsRaw["commitment"]; ok {
			commitment, ok := commitmentRaw.(string)
			if !ok {
				return nil, fmt.Errorf("commitment must be a string, got %T", commitmentRaw)
			}
			commitmentType := rpc.CommitmentType(commitment)
			out.Options.Commitment = &commitmentType
		} else {
			commitmentType := defaultCommitment()
			out.Options.Commitment = &commitmentType
		}
		if encodingRaw, ok := optionsRaw["encoding"]; ok {
			encoding, ok := encodingRaw.(string)
			if !ok {
				return nil, fmt.Errorf("encoding must be a string, got %T", encodingRaw)
			}
			encodingType := solana.EncodingType(encoding)
			out.Options.Encoding = &encodingType
		} else {
			encodingType := defaultEncoding()
			out.Options.Encoding = &encodingType
		}
		if maxSupportedTransactionVersionRaw, ok := optionsRaw["maxSupportedTransactionVersion"]; ok {
			// TODO: add support for this, and validate the value.
			maxSupportedTransactionVersion, ok := maxSupportedTransactionVersionRaw.(float64)
			if !ok {
				return nil, fmt.Errorf("maxSupportedTransactionVersion must be a number, got %T", maxSupportedTransactionVersionRaw)
			}
			maxSupportedTransactionVersionUint64 := uint64(maxSupportedTransactionVersion)
			out.Options.MaxSupportedTransactionVersion = &maxSupportedTransactionVersionUint64
		}
		if transactionDetailsRaw, ok := optionsRaw["transactionDetails"]; ok {
			// TODO: add support for this, and validate the value.
			transactionDetails, ok := transactionDetailsRaw.(string)
			if !ok {
				return nil, fmt.Errorf("transactionDetails must be a string, got %T", transactionDetailsRaw)
			}
			out.Options.TransactionDetails = (*rpc.TransactionDetailsType)(&transactionDetails)
		} else {
			transactionDetails := defaultTransactionDetails()
			out.Options.TransactionDetails = &transactionDetails
		}
		if rewardsRaw, ok := optionsRaw["rewards"]; ok {
			rewards, ok := rewardsRaw.(bool)
			if !ok {
				return nil, fmt.Errorf("rewards must be a boolean, got %T", rewardsRaw)
			}
			out.Options.Rewards = &rewards
		} else {
			rewards := true
			out.Options.Rewards = &rewards
		}
	} else {
		// set defaults:
		commitmentType := defaultCommitment()
		out.Options.Commitment = &commitmentType
		encodingType := defaultEncoding()
		out.Options.Encoding = &encodingType
		transactionDetails := defaultTransactionDetails()
		out.Options.TransactionDetails = &transactionDetails
		rewards := true
		out.Options.Rewards = &rewards
	}

	return out, nil
}

func defaultCommitment() rpc.CommitmentType {
	return rpc.CommitmentFinalized
}

func defaultEncoding() solana.EncodingType {
	return solana.EncodingJSON
}

func defaultTransactionDetails() rpc.TransactionDetailsType {
	return rpc.TransactionDetailsFull
}

type GetTransactionRequest struct {
	Signature solana.Signature `json:"signature"`
	Options   struct {
		Encoding                       *solana.EncodingType `json:"encoding,omitempty"` // default: "json"
		MaxSupportedTransactionVersion *uint64              `json:"maxSupportedTransactionVersion,omitempty"`
		Commitment                     *rpc.CommitmentType  `json:"commitment,omitempty"`
	} `json:"options,omitempty"`
}

// Validate validates the request.
func (req *GetTransactionRequest) Validate() error {
	if req.Signature.IsZero() {
		return fmt.Errorf("signature is required")
	}
	if req.Options.Encoding != nil && !isAnyEncodingOf(
		*req.Options.Encoding,
		solana.EncodingBase58,
		solana.EncodingBase64,
		solana.EncodingBase64Zstd,
		solana.EncodingJSON,
		solana.EncodingJSONParsed,
	) {
		return fmt.Errorf("unsupported encoding")
	}
	{
		if req.Options.Encoding != nil &&
			*req.Options.Encoding == solana.EncodingJSONParsed &&
			!jsonparsed.IsEnabled() {
			return fmt.Errorf("encoding=jsonParsed is not enabled on this server")
		}
	}
	return nil
}

func isAnyEncodingOf(s solana.EncodingType, anyOf ...solana.EncodingType) bool {
	for _, v := range anyOf {
		if s == v {
			return true
		}
	}
	return false
}

func parseGetTransactionRequest(raw *json.RawMessage) (*GetTransactionRequest, error) {
	var params []any
	if err := fasterJson.Unmarshal(*raw, &params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal params: %w", err)
	}
	if len(params) < 1 {
		return nil, fmt.Errorf("params must have at least one argument")
	}
	sigRaw, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("first argument must be a string, got %T", params[0])
	}

	sig, err := solana.SignatureFromBase58(sigRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signature from base58: %w", err)
	}

	out := &GetTransactionRequest{
		Signature: sig,
	}

	if len(params) > 1 {
		optionsRaw, ok := params[1].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("second argument must be an object, got %T", params[1])
		}
		if encodingRaw, ok := optionsRaw["encoding"]; ok {
			encoding, ok := encodingRaw.(string)
			if !ok {
				return nil, fmt.Errorf("encoding must be a string, got %T", encodingRaw)
			}
			encodingType := solana.EncodingType(encoding)
			out.Options.Encoding = &encodingType
		} else {
			encodingType := defaultEncoding()
			out.Options.Encoding = &encodingType
		}
		if maxSupportedTransactionVersionRaw, ok := optionsRaw["maxSupportedTransactionVersion"]; ok {
			// TODO: add support for this, and validate the value.
			maxSupportedTransactionVersion, ok := maxSupportedTransactionVersionRaw.(float64)
			if !ok {
				return nil, fmt.Errorf("maxSupportedTransactionVersion must be a number, got %T", maxSupportedTransactionVersionRaw)
			}
			maxSupportedTransactionVersionUint64 := uint64(maxSupportedTransactionVersion)
			out.Options.MaxSupportedTransactionVersion = &maxSupportedTransactionVersionUint64
		}
		if commitmentRaw, ok := optionsRaw["commitment"]; ok {
			commitment, ok := commitmentRaw.(string)
			if !ok {
				return nil, fmt.Errorf("commitment must be a string, got %T", commitmentRaw)
			}
			commitmentType := rpc.CommitmentType(commitment)
			out.Options.Commitment = &commitmentType
		}
	} else {
		// set defaults:
		encodingType := defaultEncoding()
		out.Options.Encoding = &encodingType
	}

	return out, nil
}

var zstdEncoderPool = zstdpool.NewEncoderPool()

func compiledInstructionsToJsonParsed(
	tx solana.Transaction,
	inst solana.CompiledInstruction,
	meta any,
) (json.RawMessage, error) {
	programId, err := tx.ResolveProgramIDIndex(inst.ProgramIDIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve program ID index: %w", err)
	}
	keys := tx.Message.AccountKeys
	instrParams := jsonparsed.Parameters{
		ProgramID: programId,
		Instruction: jsonparsed.CompiledInstruction{
			ProgramIDIndex: uint8(inst.ProgramIDIndex),
			Accounts: func() []uint8 {
				out := make([]uint8, len(inst.Accounts))
				for i, v := range inst.Accounts {
					out[i] = uint8(v)
				}
				return out
			}(),
			Data: inst.Data,
		},
		AccountKeys: jsonparsed.AccountKeys{
			StaticKeys: func() []solana.PublicKey {
				return clone(keys)
			}(),
			// TODO: test this:
			DynamicKeys: func() *jsonparsed.LoadedAddresses {
				switch vv := meta.(type) {
				case *confirmed_block.TransactionStatusMeta:
					return &jsonparsed.LoadedAddresses{
						Writable: func() []solana.PublicKey {
							return byteSlicesToKeySlice(vv.LoadedWritableAddresses)
						}(),
						Readonly: func() []solana.PublicKey {
							return byteSlicesToKeySlice(vv.LoadedReadonlyAddresses)
						}(),
					}
				default:
					return nil
				}
			}(),
		},
		StackHeight: func() *uint32 {
			// TODO: get the stack height from somewhere
			return nil
		}(),
	}

	parsedInstructionJSON, err := instrParams.ParseInstruction()
	if err != nil || parsedInstructionJSON == nil || !strings.HasPrefix(strings.TrimSpace(string(parsedInstructionJSON)), "{") {
		nonParseadInstructionJSON := map[string]any{
			"accounts": func() []string {
				out := make([]string, len(inst.Accounts))
				for i, v := range inst.Accounts {
					out[i] = tx.Message.AccountKeys[v].String()
				}
				return out
			}(),
			"data":        base58.Encode(inst.Data),
			"programId":   programId.String(),
			"stackHeight": nil,
		}
		asRaw, _ := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(nonParseadInstructionJSON)
		return asRaw, nil
	} else {
		return parsedInstructionJSON, nil
	}
}

func clone[T any](in []T) []T {
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func byteSlicesToKeySlice(keys [][]byte) []solana.PublicKey {
	var out []solana.PublicKey
	for _, key := range keys {
		var k solana.PublicKey
		copy(k[:], key)
		out = append(out, k)
	}
	return out
}

func parseGetBlockTimeRequest(raw *json.RawMessage) (uint64, error) {
	var params []any
	if err := fasterJson.Unmarshal(*raw, &params); err != nil {
		return 0, fmt.Errorf("failed to unmarshal params: %w", err)
	}
	if len(params) < 1 {
		return 0, fmt.Errorf("params must have at least one argument")
	}
	blockRaw, ok := params[0].(float64)
	if !ok {
		return 0, fmt.Errorf("first argument must be a number, got %T", params[0])
	}
	return uint64(blockRaw), nil
}
