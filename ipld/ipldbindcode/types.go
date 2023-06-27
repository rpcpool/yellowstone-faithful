package ipldbindcode

import "github.com/ipld/go-ipld-prime/datamodel"

type (
	List__Link []datamodel.Link
	Epoch      struct {
		Kind    int
		Epoch   int
		Subsets List__Link
	}
)

type Subset struct {
	Kind   int
	First  int
	Last   int
	Blocks List__Link
}
type (
	List__Shredding []Shredding
	Block           struct {
		Kind      int
		Slot      int
		Shredding List__Shredding
		Entries   List__Link
		Meta      SlotMeta
		Rewards   datamodel.Link
	}
)

type Rewards struct {
	Kind int
	Slot int
	Data DataFrame
}
type SlotMeta struct {
	Parent_slot int
	Blocktime   int
}
type Shredding struct {
	EntryEndIdx int
	ShredEndIdx int
}
type Entry struct {
	Kind         int
	NumHashes    int
	Hash         []uint8
	Transactions List__Link
}
type Transaction struct {
	Kind     int
	Data     DataFrame
	Metadata DataFrame
	Slot     int
	Index    **int
}
type DataFrame struct {
	Kind  int
	Hash  **int
	Index **int
	Total **int
	Data  []uint8
	Next  **List__Link
}

type (
	Hash   []uint8
	Buffer []uint8
)
