package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/gsfa/offsetstore"
	metalatest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-latest"
	metaoldest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-oldest"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"github.com/sourcegraph/jsonrpc2"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

type GetSignaturesForAddressParams struct {
	Address solana.PublicKey
	Limit   int
	Before  *solana.Signature
	Until   *solana.Signature
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
			if before, ok := m["before"]; ok {
				if before, ok := before.(string); ok {
					sig, err := solana.SignatureFromBase58(before)
					if err != nil {
						klog.Errorf("failed to parse signature from base58: %v", err)
						return nil, err
					}
					out.Before = &sig
				}
			}
			if after, ok := m["until"]; ok {
				if after, ok := after.(string); ok {
					sig, err := solana.SignatureFromBase58(after)
					if err != nil {
						klog.Errorf("failed to parse signature from base58: %v", err)
						return nil, err
					}
					out.Until = &sig
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

func (ser *rpcServer) handleGetSignaturesForAddress(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
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
	signaturesOnly := ser.options.GsfaOnlySignatures

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

	sigs, err := ser.gsfaReader.GetBeforeUntil(
		ctx,
		pk,
		limit,
		params.Before,
		params.Until,
	)
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

	var blockTimeCache struct {
		m  map[uint64]uint64
		mu sync.Mutex
	}
	blockTimeCache.m = make(map[uint64]uint64)
	getBlockTime := func(slot uint64) uint64 {
		blockTimeCache.mu.Lock()
		defer blockTimeCache.mu.Unlock()
		if blockTime, ok := blockTimeCache.m[slot]; ok {
			return blockTime
		}
		block, err := ser.GetBlock(ctx, slot)
		if err != nil {
			klog.Errorf("failed to get block time for slot %d: %v", slot, err)
			return 0
		}
		blockTimeCache.m[slot] = uint64(block.Meta.Blocktime)
		return uint64(block.Meta.Blocktime)
	}

	wg := new(errgroup.Group)
	wg.SetLimit(runtime.NumCPU() * 2)
	// The response is an array of objects: [{signature: string}]
	response := make([]map[string]any, len(sigs))
	for i := range sigs {
		ii := i
		sig := sigs[ii]
		wg.Go(func() error {
			response[ii] = map[string]any{
				"signature": sig.String(),
			}
			if signaturesOnly {
				return nil
			}
			transactionNode, err := ser.GetTransaction(ctx, sig)
			if err != nil {
				klog.Errorf("failed to get tx %s: %v", sig, err)
				return nil
			}
			if transactionNode != nil {
				{
					tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, ser.GetDataFrameByCid)
					if err == nil {
						switch metaValue := meta.(type) {
						case *confirmed_block.TransactionStatusMeta:
							response[ii]["err"] = metaValue.Err
						case *metalatest.TransactionStatusMeta:
							response[ii]["err"] = metaValue.Status
						case *metaoldest.TransactionStatusMeta:
							response[ii]["err"] = metaValue.Status
						}

						if _, ok := response[ii]["err"]; ok {
							response[ii]["err"], _ = parseTransactionError(response[ii]["err"])
						}

						memoData := getMemoInstructionDataFromTransaction(&tx)
						if memoData != nil {
							response[ii]["memo"] = string(memoData)
						}
					}

					if _, ok := response[ii]["memo"]; !ok {
						response[ii]["memo"] = nil
					}
					if _, ok := response[ii]["err"]; !ok {
						response[ii]["err"] = nil
					}
				}
				slot := uint64(transactionNode.Slot)
				response[ii]["slot"] = slot
				response[ii]["blockTime"] = getBlockTime(slot)
				response[ii]["confirmationStatus"] = "finalized" // TODO: is this correct?
			}
			return nil
		})
	}
	if err := wg.Wait(); err != nil {
		klog.Errorf("failed to get txs: %v", err)
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			})
		return
	}

	// reply with the data
	err = conn.ReplyNoMod(
		ctx,
		req.ID,
		response,
	)
	if err != nil {
		klog.Errorf("failed to reply: %v", err)
	}
}

func getMemoInstructionDataFromTransaction(tx *solana.Transaction) []byte {
	for _, instruction := range tx.Message.Instructions {
		prog, err := tx.ResolveProgramIDIndex(instruction.ProgramIDIndex)
		if err != nil {
			continue
		}
		if prog.IsAnyOf(memoProgramIDV1, memoProgramIDV2) {
			return instruction.Data
		}
	}
	return nil
}

var (
	memoProgramIDV1 = solana.MPK("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo")
	memoProgramIDV2 = solana.MPK("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")
)

func parseTransactionError(v any) (map[string]any, error) {
	// marshal to json
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// then unmarshal to map
	var m map[string]any
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	// get the "err" field
	errValue, ok := m["err"]
	if !ok {
		return nil, nil
	}
	// try to parse base64
	errValueStr, ok := errValue.(string)
	if !ok {
		return nil, nil
	}
	b, err = base64.StdEncoding.DecodeString(errValueStr)
	if err != nil {
		return nil, err
	}
	///
	{
		dec := bin.NewBinDecoder(b)
		transactionErrorType, err := dec.ReadUint32(bin.LE)
		if err != nil {
			return nil, err
		}
		errorCode, err := dec.ReadUint8()
		if err != nil {
			return nil, err
		}
		transactionErrorTypeName, ok := TransactionErrorType_name[int32(transactionErrorType)]
		if !ok {
			return nil, fmt.Errorf("unknown transaction error type: %d", transactionErrorType)
		}
		transactionErrorTypeName = bin.ToPascalCase(transactionErrorTypeName)

		switch TransactionErrorType(transactionErrorType) {
		case TransactionErrorType_INSTRUCTION_ERROR:

			instructionErrorType, err := dec.ReadUint32(bin.LE)
			if err != nil {
				return nil, err
			}

			instructionErrorTypeName, ok := InstructionErrorType_name[int32(instructionErrorType)]
			if !ok {
				return nil, fmt.Errorf("unknown instruction error type: %d", instructionErrorType)
			}
			instructionErrorTypeName = bin.ToPascalCase(instructionErrorTypeName)

			switch InstructionErrorType(instructionErrorType) {
			case InstructionErrorType_CUSTOM:
				customErrorType, err := dec.ReadUint32(bin.LE)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					transactionErrorTypeName: []any{
						errorCode,
						map[string]any{
							instructionErrorTypeName: customErrorType,
						},
					},
				}, nil
			}

			return map[string]any{
				transactionErrorTypeName: []any{
					errorCode,
					instructionErrorTypeName,
				},
			}, nil
		default:
			return map[string]any{
				transactionErrorTypeName: []any{
					errorCode,
				},
			}, nil
		}
	}
}
