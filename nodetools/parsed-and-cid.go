package nodetools

import (
	"fmt"
	"sort"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
)

type (
	ParsedAndCidSlice []*ParsedAndCid
	ParsedAndCid      struct {
		Cid  cid.Cid
		Data ipldbindcode.Node
	}
)

// Reset resets the ParsedAndCid to its zero value.
func (p *ParsedAndCid) Reset() {
	if p == nil {
		return
	}
	p.Cid = cid.Undef // Reset the CID to avoid memory leaks.
	if p.Data != nil {
		iplddecoders.PutAny(p.Data) // recycle the Data
		p.Data = nil
	}
}

// Put recycles the ParsedAndCid, releasing it back to the pool.
func (p *ParsedAndCid) Put() {
	if p == nil {
		return
	}
	putParsedAndCid(p)
}

var parsedAndCidPool = &sync.Pool{
	New: func() any {
		return &ParsedAndCid{
			Data: ipldbindcode.Node(nil), // Initialize Data to nil
		}
	},
}

func getParsedAndCid() *ParsedAndCid {
	got := parsedAndCidPool.Get().(*ParsedAndCid)
	// ASSUMES that it was reset before being put into the pool.
	return got
}

func putParsedAndCid(p *ParsedAndCid) {
	if p == nil {
		return
	}
	p.Reset()
	parsedAndCidPool.Put(p)
}

var parsedAndCidSlicePool = &sync.Pool{
	New: func() any {
		return &ParsedAndCidSlice{}
	},
}

func getParsedAndCidSlice() *ParsedAndCidSlice {
	got := parsedAndCidSlicePool.Get().(*ParsedAndCidSlice)
	return got
}

func putParsedAndCidSlice(slice *ParsedAndCidSlice) {
	slice.Reset()
	parsedAndCidSlicePool.Put(slice)
}

// ParsedAndCidSlice.LinearGetByCid does a linear search for a CID in the ParsedAndCidSlice.
func (d ParsedAndCidSlice) LinearGetByCid(c cid.Cid) (*ParsedAndCid, bool) {
	for i := range d {
		if d[i].Cid.Equals(c) {
			return d[i], true
		}
	}
	return nil, false
}

// func (d DataAndCidSlice)SortByCid() {
func (d ParsedAndCidSlice) SortByCid() {
	sort.Slice(d, func(i, j int) bool {
		// return bytes.Compare(d[i].Cid.Bytes(), d[j].Cid.Bytes()) < 0
		return d[i].Cid.Cmp(&d[j].Cid) < 0
	})
}

// Binary search for a CID in the DataAndCidSlice. MUST be sorted by CID before calling this.
func (d ParsedAndCidSlice) ByCid(c cid.Cid) (*ParsedAndCid, bool) {
	// do binary search for the CID
	i := sort.Search(len(d), func(i int) bool {
		// return bytes.Compare(d[i].Cid.Bytes(), c.Bytes()) >= 0
		return d[i].Cid.Cmp(&c) >= 0
	})
	if i < len(d) && d[i].Cid.Equals(c) {
		return d[i], true
	}
	return nil, false
}

// Reset resets the ParsedAndCidSlice to an empty slice, recycling all ParsedAndCid elements.
func (d *ParsedAndCidSlice) Reset() {
	if d == nil {
		return
	}
	if len(*d) == 0 {
		return
	}
	for i := range *d {
		if (*d)[i] != nil {
			(*d)[i].Put() // recycle each ParsedAndCid
			(*d)[i] = nil
		}
	}
	*d = (*d)[:0]
}

func (d *ParsedAndCidSlice) Put() {
	putParsedAndCidSlice(d)
}

func (d *ParsedAndCidSlice) Push(dataAndCid *ParsedAndCid) {
	if d == nil {
		d = getParsedAndCidSlice()
	}
	*d = append(*d, dataAndCid)
}

func (d ParsedAndCidSlice) Epoch() func(func(*ipldbindcode.Epoch) bool) {
	return func(f func(*ipldbindcode.Epoch) bool) {
		for _, item := range d {
			if epoch, ok := item.Data.(*ipldbindcode.Epoch); ok {
				if !f(epoch) {
					break // stop iterating if the function returns false
				}
			}
		}
	}
}

func (d ParsedAndCidSlice) Subset() func(func(*ipldbindcode.Subset) bool) {
	return func(f func(*ipldbindcode.Subset) bool) {
		for _, item := range d {
			if subset, ok := item.Data.(*ipldbindcode.Subset); ok {
				if !f(subset) {
					break // stop iterating if the function returns false
				}
			}
		}
	}
}

func (d ParsedAndCidSlice) Block() func(func(*ipldbindcode.Block) bool) {
	return func(f func(*ipldbindcode.Block) bool) {
		for _, item := range d {
			if block, ok := item.Data.(*ipldbindcode.Block); ok {
				if !f(block) {
					break // stop iterating if the function returns false
				}
			}
		}
	}
}

func (d ParsedAndCidSlice) Rewards() func(func(*ipldbindcode.Rewards) bool) {
	return func(f func(*ipldbindcode.Rewards) bool) {
		for _, item := range d {
			if rewards, ok := item.Data.(*ipldbindcode.Rewards); ok {
				if !f(rewards) {
					break // stop iterating if the function returns false
				}
			}
		}
	}
}

func (d ParsedAndCidSlice) Entry() func(func(*ipldbindcode.Entry) bool) {
	return func(f func(*ipldbindcode.Entry) bool) {
		for _, item := range d {
			if entry, ok := item.Data.(*ipldbindcode.Entry); ok {
				if !f(entry) {
					break // stop iterating if the function returns false
				}
			}
		}
	}
}

func (d ParsedAndCidSlice) Transaction() func(func(*ipldbindcode.Transaction) bool) {
	return func(f func(*ipldbindcode.Transaction) bool) {
		for _, tx := range d.SortedTransactions() {
			if !f(tx) {
				break // stop iterating if the function returns false
			}
		}
	}
}

func (d ParsedAndCidSlice) DataFrame() func(func(*ipldbindcode.DataFrame) bool) {
	return func(f func(*ipldbindcode.DataFrame) bool) {
		for _, item := range d {
			if df, ok := item.Data.(*ipldbindcode.DataFrame); ok {
				if !f(df) {
					break // stop iterating if the function returns false
				}
			}
		}
	}
}

func (d ParsedAndCidSlice) Any() func(func(ipldbindcode.Node) bool) {
	return func(f func(ipldbindcode.Node) bool) {
		for _, item := range d {
			if !f(item.Data) {
				break // stop iterating if the function returns false
			}
		}
	}
}

func (d ParsedAndCidSlice) EpochByCid(c cid.Cid) (*ipldbindcode.Epoch, error) {
	epoch, ok := d.ByCid(c)
	if !ok {
		return nil, fmt.Errorf("epoch not found for CID %s", c.String())
	}
	if e, ok := epoch.Data.(*ipldbindcode.Epoch); ok {
		return e, nil
	}
	return nil, fmt.Errorf("data is not an Epoch for CID %s", c.String())
}

func (d ParsedAndCidSlice) SubsetByCid(c cid.Cid) (*ipldbindcode.Subset, error) {
	subset, ok := d.ByCid(c)
	if !ok {
		return nil, fmt.Errorf("subset not found for CID %s", c.String())
	}
	if s, ok := subset.Data.(*ipldbindcode.Subset); ok {
		return s, nil
	}
	return nil, fmt.Errorf("data is not a Subset for CID %s", c.String())
}

func (d ParsedAndCidSlice) BlockByCid(c cid.Cid) (*ipldbindcode.Block, error) {
	block, ok := d.ByCid(c)
	if !ok {
		return nil, fmt.Errorf("block not found for CID %s", c.String())
	}
	if b, ok := block.Data.(*ipldbindcode.Block); ok {
		return b, nil
	}
	return nil, fmt.Errorf("data is not a Block for CID %s", c.String())
}

func (d ParsedAndCidSlice) RewardsByCid(c cid.Cid) (*ipldbindcode.Rewards, error) {
	rewards, ok := d.ByCid(c)
	if !ok {
		return nil, fmt.Errorf("rewards not found for CID %s", c.String())
	}
	if r, ok := rewards.Data.(*ipldbindcode.Rewards); ok {
		return r, nil
	}
	return nil, fmt.Errorf("data is not a Rewards for CID %s", c.String())
}

func (d ParsedAndCidSlice) EntryByCid(c cid.Cid) (*ipldbindcode.Entry, error) {
	entry, ok := d.ByCid(c)
	if !ok {
		return nil, fmt.Errorf("entry not found for CID %s", c.String())
	}
	if e, ok := entry.Data.(*ipldbindcode.Entry); ok {
		return e, nil
	}
	return nil, fmt.Errorf("data is not an Entry for CID %s", c.String())
}

func (d ParsedAndCidSlice) TransactionByCid(c cid.Cid) (*ipldbindcode.Transaction, error) {
	tx, ok := d.ByCid(c)
	if !ok {
		return nil, fmt.Errorf("transaction not found for CID %s", c.String())
	}
	if t, ok := tx.Data.(*ipldbindcode.Transaction); ok {
		return t, nil
	}
	return nil, fmt.Errorf("data is not a Transaction for CID %s", c.String())
}

func (d ParsedAndCidSlice) DataFrameByCid(c cid.Cid) (*ipldbindcode.DataFrame, error) {
	df, ok := d.ByCid(c)
	if !ok {
		return nil, fmt.Errorf("data frame not found for CID %s", c.String())
	}
	if dataFrame, ok := df.Data.(*ipldbindcode.DataFrame); ok {
		return dataFrame, nil
	}
	return nil, fmt.Errorf("data is not a DataFrame for CID %s", c.String())
}

func (d ParsedAndCidSlice) AnyByCid(c cid.Cid) (ipldbindcode.Node, error) {
	any, ok := d.ByCid(c)
	if !ok {
		return nil, fmt.Errorf("node not found for CID %s", c.String())
	}
	if any.Data == nil {
		return nil, fmt.Errorf("data is nil for CID %s", c.String())
	}
	return any.Data, nil
}

func (d ParsedAndCidSlice) SortedTransactions() []*ipldbindcode.Transaction {
	var transactions []*ipldbindcode.Transaction
	for _, item := range d {
		if tx, ok := item.Data.(*ipldbindcode.Transaction); ok {
			transactions = append(transactions, tx)
		}
	}
	// sort by position, from the lowest to the highest.
	sort.Slice(transactions, func(i, j int) bool {
		posI, okI := transactions[i].GetPositionIndex()
		posJ, okJ := transactions[j].GetPositionIndex()
		if !okI || !okJ {
			return false // if either position is not set, we can't compare
		}
		return posI < posJ
	})
	return transactions
}

func (d ParsedAndCidSlice) CountTransactions() int {
	count := 0
	for _, item := range d {
		if _, ok := item.Data.(*ipldbindcode.Transaction); ok {
			count++
		}
	}
	return count
}
