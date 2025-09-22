package txpool

import (
	"sync"

	"github.com/gagliardetto/solana-go"
)

var transactionPool = &sync.Pool{
	New: func() any {
		return &solana.Transaction{}
	},
}

func Get() *solana.Transaction {
	tx := transactionPool.Get().(*solana.Transaction)
	return tx
}

func Put(tx *solana.Transaction) {
	reset(tx)
	transactionPool.Put(tx)
}

func reset(tx *solana.Transaction) {
	tx.Signatures = tx.Signatures[:0] // Reset signatures slice
	tx.Message = solana.Message{}
}
