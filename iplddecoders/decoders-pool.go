package iplddecoders

import (
	"sync"

	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
)

var transactionPool = &sync.Pool{
	New: func() any {
		return &ipldbindcode.Transaction{}
	},
}

func GetTransaction() *ipldbindcode.Transaction {
	return transactionPool.Get().(*ipldbindcode.Transaction)
}

func PutTransaction(t *ipldbindcode.Transaction) {
	if t == nil {
		return
	}
	t.Reset() // Reset the transaction to its initial state.
	transactionPool.Put(t)
}

var entryPool = &sync.Pool{
	New: func() any {
		return &ipldbindcode.Entry{}
	},
}

func GetEntry() *ipldbindcode.Entry {
	return entryPool.Get().(*ipldbindcode.Entry)
}

func PutEntry(e *ipldbindcode.Entry) {
	if e == nil {
		return
	}
	e.Reset() // Reset the entry to its initial state.
	entryPool.Put(e)
}

var blockPool = &sync.Pool{
	New: func() any {
		return &ipldbindcode.Block{}
	},
}

func GetBlock() *ipldbindcode.Block {
	return blockPool.Get().(*ipldbindcode.Block)
}

func PutBlock(b *ipldbindcode.Block) {
	if b == nil {
		return
	}
	b.Reset() // Reset the block to its initial state.
	blockPool.Put(b)
}

var subsetPool = &sync.Pool{
	New: func() any {
		return &ipldbindcode.Subset{}
	},
}

func GetSubset() *ipldbindcode.Subset {
	return subsetPool.Get().(*ipldbindcode.Subset)
}

func PutSubset(s *ipldbindcode.Subset) {
	if s == nil {
		return
	}
	s.Reset() // Reset the subset to its initial state.
	subsetPool.Put(s)
}

var epochPool = &sync.Pool{
	New: func() any {
		return &ipldbindcode.Epoch{}
	},
}

func GetEpoch() *ipldbindcode.Epoch {
	return epochPool.Get().(*ipldbindcode.Epoch)
}

func PutEpoch(e *ipldbindcode.Epoch) {
	if e == nil {
		return
	}
	e.Reset() // Reset the epoch to its initial state.
	epochPool.Put(e)
}

var rewardsPool = &sync.Pool{
	New: func() any {
		return &ipldbindcode.Rewards{}
	},
}

func GetRewards() *ipldbindcode.Rewards {
	return rewardsPool.Get().(*ipldbindcode.Rewards)
}

func PutRewards(r *ipldbindcode.Rewards) {
	if r == nil {
		return
	}
	r.Reset() // Reset the rewards to its initial state.
	rewardsPool.Put(r)
}

var dataFramePool = &sync.Pool{
	New: func() any {
		return &ipldbindcode.DataFrame{}
	},
}

func GetDataFrame() *ipldbindcode.DataFrame {
	return dataFramePool.Get().(*ipldbindcode.DataFrame)
}

func PutDataFrame(df *ipldbindcode.DataFrame) {
	if df == nil {
		return
	}
	df.Reset() // Reset the data frame to its initial state.
	dataFramePool.Put(df)
}

func PutAny(obj ipldbindcode.Node) {
	switch v := obj.(type) {
	case *ipldbindcode.Transaction:
		PutTransaction(v)
	case *ipldbindcode.Entry:
		PutEntry(v)
	case *ipldbindcode.Block:
		PutBlock(v)
	case *ipldbindcode.Subset:
		PutSubset(v)
	case *ipldbindcode.Epoch:
		PutEpoch(v)
	case *ipldbindcode.Rewards:
		PutRewards(v)
	case *ipldbindcode.DataFrame:
		PutDataFrame(v)
	default:
		panic("unknown type for PutAny")
	}
}
