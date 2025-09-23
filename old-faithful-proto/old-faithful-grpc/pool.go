package old_faithful_grpc

import sync "sync"

var _BlockResponsePool = sync.Pool{
	New: func() any {
		return new(BlockResponse)
	},
}

var _TransactionPool = sync.Pool{
	New: func() any {
		return new(Transaction)
	},
}

var _TransactionResponsePool = sync.Pool{
	New: func() any {
		return new(TransactionResponse)
	},
}

func GetBlockResponse() *BlockResponse {
	return _BlockResponsePool.Get().(*BlockResponse)
}

func PutBlockResponse(br *BlockResponse) {
	br.Reset()
	_BlockResponsePool.Put(br)
}

func GetTransactionResponse() *TransactionResponse {
	return _TransactionResponsePool.Get().(*TransactionResponse)
}

func PutTransactionResponse(tr *TransactionResponse) {
	tr.Reset()
	_TransactionResponsePool.Put(tr)
}

func GetTransaction() *Transaction {
	return _TransactionPool.Get().(*Transaction)
}

func PutTransaction(t *Transaction) {
	t.Reset()
	_TransactionPool.Put(t)
}
