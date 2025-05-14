package solanatxmetaparsers

import (
	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
)

func TransactionToUi(
	tx solana.Transaction,
	format solana.EncodingType,
) (*jsonbuilder.OrderedJSONObject, error) {
	obj := jsonbuilder.NewObject()
	{
		// .message
		obj.ObjectFunc("message", func(obj *jsonbuilder.OrderedJSONObject) {
			//.accountKeys
			obj.ArrayFunc("accountKeys", func(arr *jsonbuilder.ArrayBuilder) {
				for _, key := range tx.Message.AccountKeys {
					arr.AddString(key.String())
				}
			})
			// .header
			obj.ObjectFunc("header", func(obj *jsonbuilder.OrderedJSONObject) {
				obj.Uint("numRequiredSignatures", uint64(tx.Message.Header.NumRequiredSignatures))
				obj.Uint("numReadonlySignedAccounts", uint64(tx.Message.Header.NumReadonlySignedAccounts))
				obj.Uint("numReadonlyUnsignedAccounts", uint64(tx.Message.Header.NumReadonlyUnsignedAccounts))
			})
			// .instructions
			obj.ArrayFunc("instructions", func(arr *jsonbuilder.ArrayBuilder) {
				for _, instruction := range tx.Message.Instructions {
					ins := jsonbuilder.NewObject()
					{
						ins.Uint("programIdIndex", uint64(instruction.ProgramIDIndex))
						ins.ArrayFunc("accounts", func(arr *jsonbuilder.ArrayBuilder) {
							for _, account := range instruction.Accounts {
								arr.AddUint(uint64(account))
							}
						})
						ins.String("data", base58.Encode(instruction.Data))
						ins.Null("stackHeight")
					}
					arr.AddObject(ins)
				}
			})
			// .recentBlockhash
			obj.String("recentBlockhash", tx.Message.RecentBlockhash.String())
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
