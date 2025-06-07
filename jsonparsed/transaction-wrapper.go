package jsonparsed

import (
	"encoding/json"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

type Transaction struct {
	Message    Message            `json:"message"`
	Signatures []solana.Signature `json:"signatures"`
}

type Message struct {
	AccountKeys     []AccountKey      `json:"accountKeys"`
	Instructions    []json.RawMessage `json:"instructions"`
	RecentBlockhash string            `json:"recentBlockhash"`
}

type AccountKey struct {
	Pubkey   string `json:"pubkey"`
	Signer   bool   `json:"signer"`
	Source   string `json:"source"`
	Writable bool   `json:"writable"`
}

func FromTransaction(solTx solana.Transaction) (Transaction, error) {
	tx := Transaction{
		Message: Message{
			AccountKeys:  make([]AccountKey, len(solTx.Message.AccountKeys)),
			Instructions: make([]json.RawMessage, len(solTx.Message.Instructions)),
		},
		Signatures: solTx.Signatures,
	}
	for i, accKey := range solTx.Message.AccountKeys {
		isWr, err := solTx.IsWritable(accKey)
		if err != nil {
			return tx, fmt.Errorf("failed to check if account key #%d is writable: %w", i, err)
		}
		tx.Message.AccountKeys[i] = AccountKey{
			Pubkey:   accKey.String(),
			Signer:   solTx.IsSigner(accKey),
			Source:   "transaction", // TODO: what is this?
			Writable: isWr,
		}
	}
	for i, inst := range solTx.Message.Instructions {
		tx.Message.Instructions[i] = json.RawMessage(inst.Data)
	}
	tx.Message.RecentBlockhash = solTx.Message.RecentBlockhash.String()
	return tx, nil
}

// {
//       "message": {
//         "accountKeys": [
//           {
//             "pubkey": "GdnSyH3YtwcxFvQrVVJMm1JhTS4QVX7MFsX56uJLUfiZ",
//             "signer": true,
//             "source": "transaction",
//             "writable": true
//           },
//           {
//             "pubkey": "sCtiJieP8B3SwYnXemiLpRFRR8KJLMtsMVN25fAFWjW",
//             "signer": false,
//             "source": "transaction",
//             "writable": true
//           },
//           {
//             "pubkey": "SysvarS1otHashes111111111111111111111111111",
//             "signer": false,
//             "source": "transaction",
//             "writable": false
//           },
//           {
//             "pubkey": "SysvarC1ock11111111111111111111111111111111",
//             "signer": false,
//             "source": "transaction",
//             "writable": false
//           },
//           {
//             "pubkey": "Vote111111111111111111111111111111111111111",
//             "signer": false,
//             "source": "transaction",
//             "writable": false
//           }
//         ],
//         "instructions": [
//           {
//             "parsed": {
//               "info": {
//                 "clockSysvar": "SysvarC1ock11111111111111111111111111111111",
//                 "slotHashesSysvar": "SysvarS1otHashes111111111111111111111111111",
//                 "vote": {
//                   "hash": "EYEnTi2GEy7ApyWm63hvpi6c69Kvfcsc3TtdFu92yLxr",
//                   "slots": [
//                     431996
//                   ],
//                   "timestamp": null
//                 },
//                 "voteAccount": "sCtiJieP8B3SwYnXemiLpRFRR8KJLMtsMVN25fAFWjW",
//                 "voteAuthority": "GdnSyH3YtwcxFvQrVVJMm1JhTS4QVX7MFsX56uJLUfiZ"
//               },
//               "type": "vote"
//             },
//             "program": "vote",
//             "programId": "Vote111111111111111111111111111111111111111",
//             "stackHeight": null
//           }
//         ],
//         "recentBlockhash": "G9jx9FCto47ebxHgXBomE14hvG1WiwGD8LL3p7pEt1JX"
//       },
//       "signatures": [
//         "55y2u7sCd8mZ5LqtdrWnqJ6WBxVojXGXBd5KVuJFZrMJiC6bzziMdaPB3heNWqK9JpB5KfXSY4wTzf1AbyNSwUPd"
//       ]
//     }
