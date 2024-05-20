package main

import (
	"encoding/base64"

	"github.com/mr-tron/base58"
)

func ptrToUint64(v uint64) *uint64 {
	return &v
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
	{
		_, ok := m["blockTime"]
		if !ok {
			m["blockTime"] = nil
		}
	}
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
								_, ok = uiTokenAmount["uiAmount"]
								if !ok {
									uiTokenAmount["uiAmount"] = nil
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
								_, ok = uiTokenAmount["uiAmount"]
								if !ok {
									uiTokenAmount["uiAmount"] = nil
								}
								_, ok = uiTokenAmount["amount"]
								if !ok {
									uiTokenAmount["amount"] = "0"
								}
								_, ok = uiTokenAmount["uiAmountString"]
								if !ok {
									uiTokenAmount["uiAmountString"] = "0"
								}
							}
						}
					}
				}
			}
		}

		delete(meta, "returnDataNone")

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
		if returnData, ok := meta["returnData"].(map[string]any); ok {
			if data, ok := returnData["data"].(string); ok {
				returnData["data"] = []any{data, "base64"}
			}

			if programId, ok := returnData["programId"].(string); ok {
				decoded, err := base64.StdEncoding.DecodeString(programId)
				if err == nil {
					returnData["programId"] = base58.Encode(decoded)
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
			// If doesn't have `index`, then set it to 0
			if _, ok := innerInstruction["index"]; !ok {
				innerInstruction["index"] = 0
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
					} else {
						instruction["accounts"] = []any{}
					}
					// if data, ok := instruction["data"]; ok {
					// 	// as string
					// 	dataStr, ok := data.(string)
					// 	if ok {
					// 		decoded, err := base64.StdEncoding.DecodeString(dataStr)
					// 		if err == nil {
					// 			// TODO: the data in the `innerInstructions` is always base58 encoded (even if the transaction is base64 encoded)
					// 			// instruction["data"] = base58.Encode(decoded)
					// 			_ = decoded
					// 		}
					// 	}
					// }
				}
			}
			meta["innerInstructions"].([]any)[i] = innerInstruction
		}
	}
	return m
}
