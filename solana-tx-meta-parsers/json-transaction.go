package solanatxmetaparsers

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
)

func TransactionToUi(
	tx *solana.Transaction,
	format solana.EncodingType,
	details rpc.TransactionDetailsType,
) (*jsonbuilder.OrderedJSONObject, error) {
	obj := jsonbuilder.NewObject()
	{
		// .message
		obj.ObjectFunc("message", func(objMessage *jsonbuilder.OrderedJSONObject) {
			//.accountKeys
			objMessage.ArrayFunc("accountKeys", func(arr *jsonbuilder.ArrayBuilder) {
				for _, key := range tx.Message.AccountKeys {
					arr.AddString(key.String())
				}
			})
			// .addressTableLookups (v0 only; v1 carries a transactionConfig instead)
			if tx.Message.GetVersion() == solana.MessageVersionV0 {
				objMessage.ArrayFunc("addressTableLookups", func(arr *jsonbuilder.ArrayBuilder) {
					for _, lookup := range tx.Message.AddressTableLookups {
						objLookup := jsonbuilder.NewObject()
						{
							objLookup.String("accountKey", lookup.AccountKey.String())
							objLookup.ArrayFunc("writableIndexes", func(arr *jsonbuilder.ArrayBuilder) {
								for _, index := range lookup.WritableIndexes {
									arr.AddUint(uint64(index))
								}
							})
							objLookup.ArrayFunc("readonlyIndexes", func(arr *jsonbuilder.ArrayBuilder) {
								for _, index := range lookup.ReadonlyIndexes {
									arr.AddUint(uint64(index))
								}
							})
						}
						arr.AddObject(objLookup)
					}
				})
			}
			// .header
			objMessage.ObjectFunc("header", func(obj *jsonbuilder.OrderedJSONObject) {
				obj.Uint("numRequiredSignatures", uint64(tx.Message.Header.NumRequiredSignatures))
				obj.Uint("numReadonlySignedAccounts", uint64(tx.Message.Header.NumReadonlySignedAccounts))
				obj.Uint("numReadonlyUnsignedAccounts", uint64(tx.Message.Header.NumReadonlyUnsignedAccounts))
			})
			// .instructions
			objMessage.ArrayFunc("instructions", func(arr *jsonbuilder.ArrayBuilder) {
				for _, instruction := range tx.Message.Instructions {
					ins := jsonbuilder.NewObject()
					{
						ins.Uint("programIdIndex", uint64(instruction.ProgramIDIndex))
						ins.ArrayFunc("accounts", func(arr *jsonbuilder.ArrayBuilder) {
							for _, account := range instruction.Accounts {
								arr.AddUint(uint64(account))
							}
						})
						ins.Base58("data", (instruction.Data))
						if stackHeight := topLevelStackHeight(tx); stackHeight != nil {
							ins.Uint("stackHeight", uint64(*stackHeight))
						} else {
							ins.Null("stackHeight")
						}
					}
					arr.AddObject(ins)
				}
			})
			// .recentBlockhash
			objMessage.String("recentBlockhash", tx.Message.RecentBlockhash.String())
			// .transactionConfig (v1 only)
			if tx.Message.GetVersion() == solana.MessageVersionV1 {
				objMessage.Object("transactionConfig", transactionConfigToUi(tx.Message.TransactionConfig))
			}
		})
		// .signatures
		obj.ArrayFunc("signatures", func(arr *jsonbuilder.ArrayBuilder) {
			for _, sig := range tx.Signatures {
				arr.AddString(sig.String())
			}
		})
	}

	return obj, nil
}

// transactionVersionToUi returns the RPC `version` value: "legacy" or the
// numeric message version (0, 1).
func transactionVersionToUi(tx *solana.Transaction) any {
	switch tx.Message.GetVersion() {
	case solana.MessageVersionLegacy:
		return "legacy"
	case solana.MessageVersionV1:
		return 1
	default:
		return 0
	}
}

// topLevelStackHeight is the stackHeight reported on top-level instructions.
// Scoped to v1: Agave 4.3 source sets 1 for every version, but legacy/v0 stay
// null here until that is confirmed against live Agave output.
func topLevelStackHeight(tx *solana.Transaction) *uint32 {
	if tx.Message.GetVersion() != solana.MessageVersionV1 {
		return nil
	}
	one := uint32(1)
	return &one
}

// transactionConfigToUi mirrors Agave's UiTransactionConfig: all four keys are
// always present, unset fields are null.
func transactionConfigToUi(cfg solana.TransactionConfig) *jsonbuilder.OrderedJSONObject {
	obj := jsonbuilder.NewObject()
	optUint := func(key string, v *uint64) {
		if v == nil {
			obj.Null(key)
		} else {
			obj.Uint(key, *v)
		}
	}
	optUint32 := func(key string, v *uint32) {
		if v == nil {
			obj.Null(key)
		} else {
			obj.Uint(key, uint64(*v))
		}
	}
	optUint("priorityFee", cfg.PriorityFee)
	optUint32("computeUnitLimit", cfg.ComputeUnitLimit)
	optUint32("loadedAccountsDataSizeLimit", cfg.LoadedAccountsDataSizeLimit)
	optUint32("heapSize", cfg.HeapSize)
	return obj
}
