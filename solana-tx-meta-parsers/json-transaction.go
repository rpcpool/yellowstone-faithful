package solanatxmetaparsers

import (
	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
)

func TransactionToUi(
	tx *solana.Transaction,
	format solana.EncodingType,
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
			// .addressTableLookups
			if tx.Message.IsVersioned() {
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
						ins.String("data", base58.Encode(instruction.Data))
						ins.Null("stackHeight")
					}
					arr.AddObject(ins)
				}
			})
			// .recentBlockhash
			objMessage.String("recentBlockhash", tx.Message.RecentBlockhash.String())
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
