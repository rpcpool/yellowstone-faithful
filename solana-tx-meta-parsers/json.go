package solanatxmetaparsers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go"
	jsoniter "github.com/json-iterator/go"
	"github.com/mr-tron/base58"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
	jsonparsed "github.com/rpcpool/yellowstone-faithful/jsonparsed"
)

//	pub enum UiTransactionEncoding {
//	    Binary, // Legacy. Retained for RPC backwards compatibility
//	    Base64,
//	    Base58,
//	    Json,
//	    JsonParsed,
//	}

// #[serde(rename_all = "camelCase")]
// pub struct EncodedTransactionWithStatusMeta {
//     pub transaction: EncodedTransaction,
//     pub meta: Option<UiTransactionStatusMeta>,
//     #[serde(default, skip_serializing_if = "Option::is_none")]
//     pub version: Option<TransactionVersion>,
// }

type EncodedTransactionWithStatusMeta struct {
	Transaction solana.Transaction
	Meta        *TransactionStatusMetaContainer
}

func NewEncodedTransactionWithStatusMeta(
	tx solana.Transaction, meta *TransactionStatusMetaContainer,
) *EncodedTransactionWithStatusMeta {
	return &EncodedTransactionWithStatusMeta{
		Transaction: tx,
		Meta:        meta,
	}
}

// EncodedTransactionWithStatusMeta.ToUi
func (final *EncodedTransactionWithStatusMeta) ToUi(encoding solana.EncodingType) (*jsonbuilder.OrderedJSONObject, error) {
	// #[serde(rename_all = "camelCase")]
	// pub struct EncodedTransactionWithStatusMeta {
	//     pub transaction: EncodedTransaction,
	//     pub meta: Option<UiTransactionStatusMeta>,
	//     #[serde(default, skip_serializing_if = "Option::is_none")]
	//     pub version: Option<TransactionVersion>,
	// }
	txBinBuf, err := final.Transaction.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}
	resp := jsonbuilder.NewObject()

	if final.Meta != nil {
		if final.Meta.IsSerde() {
			metaSerde := final.Meta.GetSerde()
			rawJsonMeta, err := SerdeTransactionStatusMetaToUi(metaSerde)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize (serde) transaction status meta: %w", err)
			}
			resp.Raw("meta", rawJsonMeta)
		}
		if final.Meta.IsProtobuf() {
			metaProtobuf := final.Meta.GetProtobuf()
			rawJsonMeta, err := ProtobufTransactionStatusMetaToUi(metaProtobuf)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize (protobuf) transaction status meta: %w", err)
			}
			resp.Raw("meta", rawJsonMeta)
		}
	} else {
		resp.Null("meta")
	}
	{
		switch encoding {
		case solana.EncodingBase64:
			resp.Value(
				"transaction",
				[]any{
					base64.StdEncoding.EncodeToString(txBinBuf),
					"base64",
				})
		// TODO:
		// case solana.EncodingBinary:
		// 	resp.Value(
		// 		"transaction",
		// 		base58.Encode(txBinBuf),
		// 	)
		case solana.EncodingBase58:
			// NOTE: EncodingBinary is the legacy encoding.
			resp.Value(
				"transaction",
				[]any{
					base58.Encode(txBinBuf),
					"base58",
				})
		case solana.EncodingJSON:
			txAs, err := TransactionToUi(final.Transaction, encoding)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize transaction: %w", err)
			}
			resp.Object(
				"transaction",
				txAs,
			)
		case solana.EncodingJSONParsed:
			{
				if !jsonparsed.IsEnabled() {
					return nil, fmt.Errorf("unsupported encoding jsonParsed: jsonparsed is not enabled")
				}

				{
					writable, readonly := final.Meta.GetLoadedAccounts()
					{
						{
							tables := map[solana.PublicKey]solana.PublicKeySlice{}
							for _, addr := range final.Transaction.Message.AddressTableLookups {
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
							err := final.Transaction.Message.SetAddressTables(tables)
							if err != nil {
								return nil, fmt.Errorf("failed to set address tables: %w", err)
							}
						}
						if final.Transaction.Message.IsVersioned() {
							err := final.Transaction.Message.ResolveLookups()
							if err != nil {
								return nil, fmt.Errorf("failed to resolve lookups: %w", err)
							}
						}
					}
				}

				parsedInstructions := make([]json.RawMessage, 0)

				for _, inst := range final.Transaction.Message.Instructions {
					parsedInstructionJSON, err := compiledInstructionsToJsonParsed(final.Transaction, inst, final.Meta)
					if err != nil {
						return nil, fmt.Errorf("failed to compile instruction: %w", err)
					}
					parsedInstructions = append(parsedInstructions, parsedInstructionJSON)
				}

				parsedTx, err := jsonparsed.FromTransaction(final.Transaction)
				if err != nil {
					return nil, fmt.Errorf("failed to convert transaction to jsonparsed.Transaction: %w", err)
				}
				parsedTx.Message.Instructions = parsedInstructions

				resp.Value(
					"transaction",
					final.Transaction,
				)

				// TODO: must parse also inner instructions?????
			}
		default:
			return nil, fmt.Errorf("unknown encoding: %d", encoding)
		}
	}

	// {
	// 	version := final.Transaction.Message.GetVersion()
	// 	if version == solana.MessageVersionLegacy {
	// 		resp.String("version", "legacy")
	// 	} else {
	// 		resp.Uint8("version", 0)
	// 	}
	// }
	return resp, nil
}

// #[repr(u8)]
// pub enum TransactionVersion {
//     #[default]
//     Legacy = u8::MAX,
//     V0 = 0,
// }

func byeSliceToUint16Slice(in []byte) []uint16 {
	out := make([]uint16, len(in))
	for i, v := range in {
		out[i] = uint16(v)
	}
	return out
}

func clone[T any](in []T) []T {
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func compiledInstructionsToJsonParsed(
	tx solana.Transaction,
	inst solana.CompiledInstruction,
	meta *TransactionStatusMetaContainer,
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
				writable, readonly := meta.GetLoadedAccounts()
				return &jsonparsed.LoadedAddresses{
					Writable: func() []solana.PublicKey {
						return writable
					}(),
					Readonly: func() []solana.PublicKey {
						return readonly
					}(),
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

func byteSlicesToKeySlice(keys [][]byte) []solana.PublicKey {
	var out []solana.PublicKey
	for _, key := range keys {
		var k solana.PublicKey
		copy(k[:], key)
		out = append(out, k)
	}
	return out
}
