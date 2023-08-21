package main

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/mr-tron/base58"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/sourcegraph/jsonrpc2"
	"k8s.io/klog/v2"
)

func ptrToUint64(v uint64) *uint64 {
	return &v
}

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

// byteSliceAsIntegerSlice converts a byte slice to an integer slice.
func byteSliceAsIntegerSlice(b []byte) []uint64 {
	var ret []uint64
	for i := 0; i < len(b); i++ {
		ret = append(ret, uint64(b[i]))
	}
	return ret
}

// adaptTransactionMetaToExpectedOutput adapts the transaction meta to the expected output
// as per what solana RPC server returns.
func adaptTransactionMetaToExpectedOutput(m map[string]any) map[string]any {
	meta, ok := m["meta"].(map[string]any)
	if !ok {
		return m
	}
	{
		if _, ok := meta["err"]; ok {
			meta["err"], _ = parseTransactionError(meta["err"])
		} else {
			meta["err"] = nil
		}
	}
	{
		if _, ok := meta["loadedAddresses"]; !ok {
			meta["loadedAddresses"] = map[string]any{
				"readonly": []any{},
				"writable": []any{},
			}
		}
		{
			// if has loadedReadonlyAddresses and is []string, then use that for loadedAddresses.readonly
			if loadedReadonlyAddresses, ok := meta["loadedReadonlyAddresses"].([]any); ok {
				// the address list is base64 encoded; decode and encode to base58
				for i, addr := range loadedReadonlyAddresses {
					addrStr, ok := addr.(string)
					if ok {
						decoded, err := base64.StdEncoding.DecodeString(addrStr)
						if err == nil {
							loadedReadonlyAddresses[i] = base58.Encode(decoded)
						}
					}
				}
				meta["loadedAddresses"].(map[string]any)["readonly"] = loadedReadonlyAddresses
				delete(meta, "loadedReadonlyAddresses")
			}
			// if has loadedWritableAddresses and is []string, then use that for loadedAddresses.writable
			if loadedWritableAddresses, ok := meta["loadedWritableAddresses"].([]any); ok {
				// the address list is base64 encoded; decode and encode to base58
				for i, addr := range loadedWritableAddresses {
					addrStr, ok := addr.(string)
					if ok {
						decoded, err := base64.StdEncoding.DecodeString(addrStr)
						if err == nil {
							loadedWritableAddresses[i] = base58.Encode(decoded)
						}
					}
				}
				meta["loadedAddresses"].(map[string]any)["writable"] = loadedWritableAddresses
				delete(meta, "loadedWritableAddresses")
			}
			// remove loadedReadonlyAddresses and loadedWritableAddresses
		}
		if preTokenBalances, ok := meta["preTokenBalances"]; !ok {
			meta["preTokenBalances"] = []any{}
		} else {
			// in preTokenBalances.[].uiTokenAmount.decimals if not present, set to 0
			preTokenBalances, ok := preTokenBalances.([]any)
			if ok {
				for _, preTokenBalanceAny := range preTokenBalances {
					preTokenBalance, ok := preTokenBalanceAny.(map[string]any)
					if ok {
						uiTokenAmountAny, ok := preTokenBalance["uiTokenAmount"]
						if ok {
							uiTokenAmount, ok := uiTokenAmountAny.(map[string]any)
							if ok {
								_, ok := uiTokenAmount["decimals"]
								if !ok {
									uiTokenAmount["decimals"] = 0
								}
							}
						}
					}
				}
			}
		}
		if postTokenBalances, ok := meta["postTokenBalances"]; !ok {
			meta["postTokenBalances"] = []any{}
		} else {
			// in postTokenBalances.[].uiTokenAmount.decimals if not present, set to 0
			postTokenBalances, ok := postTokenBalances.([]any)
			if ok {
				for _, postTokenBalanceAny := range postTokenBalances {
					postTokenBalance, ok := postTokenBalanceAny.(map[string]any)
					if ok {
						uiTokenAmountAny, ok := postTokenBalance["uiTokenAmount"]
						if ok {
							uiTokenAmount, ok := uiTokenAmountAny.(map[string]any)
							if ok {
								_, ok := uiTokenAmount["decimals"]
								if !ok {
									uiTokenAmount["decimals"] = 0
								}
							}
						}
					}
				}
			}
		}
		if _, ok := meta["returnDataNone"]; !ok {
			// TODO: what is this?
			meta["returnDataNone"] = nil
		}
		if _, ok := meta["rewards"]; !ok {
			meta["rewards"] = []any{}
		}
		if _, ok := meta["status"]; !ok {
			eee, ok := meta["err"]
			if ok {
				if eee == nil {
					meta["status"] = map[string]any{
						"Ok": nil,
					}
				} else {
					meta["status"] = map[string]any{
						"Err": eee,
					}
				}
			}
		}
		{
			// TODO: is this correct?
			// if doesn't have err, but has status and it is empty, then set status to Ok
			if _, ok := meta["err"]; !ok || meta["err"] == nil {
				if status, ok := meta["status"].(map[string]any); ok {
					if len(status) == 0 {
						meta["status"] = map[string]any{
							"Ok": nil,
						}
					}
				}
			}
		}
	}
	{
		innerInstructionsAny, ok := meta["innerInstructions"]
		if !ok {
			meta["innerInstructions"] = []any{}
			return m
		}
		innerInstructions, ok := innerInstructionsAny.([]any)
		if !ok {
			return m
		}
		for i, innerInstructionAny := range innerInstructions {
			innerInstruction, ok := innerInstructionAny.(map[string]any)
			if !ok {
				continue
			}
			instructionsAny, ok := innerInstruction["instructions"]
			if !ok {
				continue
			}
			instructions, ok := instructionsAny.([]any)
			if !ok {
				continue
			}
			for _, instructionAny := range instructions {
				instruction, ok := instructionAny.(map[string]any)
				if !ok {
					continue
				}
				{
					if accounts, ok := instruction["accounts"]; ok {
						// as string
						accountsStr, ok := accounts.(string)
						if ok {
							decoded, err := base64.StdEncoding.DecodeString(accountsStr)
							if err == nil {
								instruction["accounts"] = byteSliceAsIntegerSlice(decoded)
							}
						}
					}
					if data, ok := instruction["data"]; ok {
						// as string
						dataStr, ok := data.(string)
						if ok {
							decoded, err := base64.StdEncoding.DecodeString(dataStr)
							if err == nil {
								// TODO: the data in the `innerInstructions` is always base58 encoded (even if the transaction is base64 encoded)
								instruction["data"] = base58.Encode(decoded)
							}
						}
					}
				}
			}
			meta["innerInstructions"].([]any)[i] = innerInstruction
		}
	}
	return m
}
