package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	jsoniter "github.com/json-iterator/go"
	"github.com/mostynb/zstdpool-freelist"
	"github.com/mr-tron/base58"
	jsonparsed "github.com/rpcpool/yellowstone-faithful/jsonparsed"
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
	Slot    uint64 `json:"slot"`
	Options struct {
		Commitment                     *rpc.CommitmentType  `json:"commitment,omitempty"` // default: "finalized"
		Encoding                       *solana.EncodingType `json:"encoding,omitempty"`   // default: "json"
		MaxSupportedTransactionVersion *uint64              `json:"maxSupportedTransactionVersion,omitempty"`
		TransactionDetails             *string              `json:"transactionDetails,omitempty"` // default: "full"
		Rewards                        *bool                `json:"rewards,omitempty"`
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
	if req.Options.Encoding != nil && *req.Options.Encoding == solana.EncodingJSONParsed {
		if !jsonparsed.IsEnabled() {
			return fmt.Errorf("encoding=jsonParsed is not enabled on this server")
		}
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
			out.Options.TransactionDetails = &transactionDetails
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

func defaultTransactionDetails() string {
	return "full"
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
		if req.Options.Encoding != nil && *req.Options.Encoding == solana.EncodingJSONParsed {
			if !jsonparsed.IsEnabled() {
				return fmt.Errorf("encoding=jsonParsed is not enabled on this server")
			}
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

func encodeTransactionResponseBasedOnWantedEncoding(
	encoding solana.EncodingType,
	tx solana.Transaction,
	meta any,
) (any, any, error) {
	switch encoding {
	case solana.EncodingBase58, solana.EncodingBase64, solana.EncodingBase64Zstd:
		txBuf, err := tx.MarshalBinary()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal transaction: %w", err)
		}
		tOut, err := encodeBytesResponseBasedOnWantedEncoding(encoding, txBuf)
		return tOut, meta, err
	case solana.EncodingJSONParsed:
		if !jsonparsed.IsEnabled() {
			return nil, nil, fmt.Errorf("unsupported encoding")
		}

		{
			unwrappedMeta, ok := meta.(*confirmed_block.TransactionStatusMeta)
			if ok {
				{
					tables := map[solana.PublicKey]solana.PublicKeySlice{}
					writable := byteSlicesToKeySlice(unwrappedMeta.LoadedWritableAddresses)
					readonly := byteSlicesToKeySlice(unwrappedMeta.LoadedReadonlyAddresses)
					for _, addr := range tx.Message.AddressTableLookups {
						numTakeWritable := len(addr.WritableIndexes)
						numTakeReadonly := len(addr.ReadonlyIndexes)
						tableKey := addr.AccountKey
						{
							// now need to rebuild the address table taking into account the indexes, and put the keys into the tables
							maxIndex := 0
							for _, indexB := range addr.WritableIndexes {
								index := int(indexB)
								if index > maxIndex {
									maxIndex = index
								}
							}
							for _, indexB := range addr.ReadonlyIndexes {
								index := int(indexB)
								if index > maxIndex {
									maxIndex = index
								}
							}
							tables[tableKey] = make([]solana.PublicKey, maxIndex+1)
						}
						if numTakeWritable > 0 {
							writableForTable := writable[:numTakeWritable]
							for i, indexB := range addr.WritableIndexes {
								index := int(indexB)
								tables[tableKey][index] = writableForTable[i]
							}
							writable = writable[numTakeWritable:]
						}
						if numTakeReadonly > 0 {
							readableForTable := readonly[:numTakeReadonly]
							for i, indexB := range addr.ReadonlyIndexes {
								index := int(indexB)
								tables[tableKey][index] = readableForTable[i]
							}
							readonly = readonly[numTakeReadonly:]
						}
					}
					err := tx.Message.SetAddressTables(tables)
					if err != nil {
						return nil, nil, fmt.Errorf("failed to set address tables: %w", err)
					}
				}
				if tx.Message.IsVersioned() {
					err := tx.Message.ResolveLookups()
					if err != nil {
						panic(err)
					}
				}
			}
		}

		parsedInstructions := make([]json.RawMessage, 0)

		for _, inst := range tx.Message.Instructions {
			parsedInstructionJSON, err := compiledInstructionsToJsonParsed(tx, inst, meta)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to compile instruction: %w", err)
			}
			parsedInstructions = append(parsedInstructions, parsedInstructionJSON)
		}

		resp, err := jsonparsed.FromTransaction(tx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert transaction to jsonparsed.Transaction: %w", err)
		}
		resp.Message.Instructions = parsedInstructions

		{
			// now try to encode unwrappedMeta:
			unwrappedMeta, ok := meta.(*confirmed_block.TransactionStatusMeta)
			if ok {
				// convert meta to json:
				metaJSON, err := toMapAny(unwrappedMeta)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to marshal meta: %w", err)
				}
				for innerIndex, insts := range unwrappedMeta.InnerInstructions {
					inner := make([]solana.CompiledInstruction, len(insts.Instructions))
					for j, inst := range insts.Instructions {
						inner[j] = solana.CompiledInstruction{
							ProgramIDIndex: uint16(inst.ProgramIdIndex),
							Accounts:       byeSliceToUint16Slice(inst.Accounts),
							Data:           clone(inst.Data),
						}
					}
					for instIndex, inst := range inner {
						parsedInstructionJSON, err := compiledInstructionsToJsonParsed(tx, inst, unwrappedMeta)
						if err != nil {
							return nil, nil, fmt.Errorf("failed to compile instruction: %w", err)
						}
						// now replace the inner instruction with the parsed instruction:
						{
							if _, ok := metaJSON["inner_instructions"]; !ok {
								metaJSON["inner_instructions"] = []any{}
							} else {
								innerInstructions, ok := metaJSON["inner_instructions"].([]any)
								if ok && len(innerInstructions) > innerIndex {
									relevantInner := innerInstructions[innerIndex].(map[string]any)
									{
										_, ok := relevantInner["instructions"].([]any)
										if ok {
											metaJSON["inner_instructions"].([]any)[innerIndex].(map[string]any)["instructions"].([]any)[instIndex] = parsedInstructionJSON
										}
									}
								}
							}
						}
					}
				}
				return resp, metaJSON, nil
			}
		}

		return resp, meta, nil
	case solana.EncodingJSON:
		return tx, meta, nil
	default:
		return nil, nil, fmt.Errorf("unsupported encoding")
	}
}

func clone[T any](in []T) []T {
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func byeSliceToUint16Slice(in []byte) []uint16 {
	out := make([]uint16, len(in))
	for i, v := range in {
		out[i] = uint16(v)
	}
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

func toUniqueSorted(accountIndexes []uint16) []uint16 {
	seen := make(map[uint16]struct{})
	var out []uint16
	for _, v := range accountIndexes {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}

func encodeBytesResponseBasedOnWantedEncoding(
	encoding solana.EncodingType,
	buf []byte,
) ([]any, error) {
	switch encoding {
	case solana.EncodingBase58:
		return []any{base58.Encode(buf), encoding}, nil
	case solana.EncodingBase64:
		return []any{base64.StdEncoding.EncodeToString(buf), encoding}, nil
	case solana.EncodingBase64Zstd:
		enc, err := zstdEncoderPool.Get(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get zstd encoder: %w", err)
		}
		defer zstdEncoderPool.Put(enc)
		return []any{base64.StdEncoding.EncodeToString(enc.EncodeAll(buf, nil)), encoding}, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
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
