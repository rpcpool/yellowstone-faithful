package ipldbindcode

import (
	"github.com/ipld/go-ipld-prime/datamodel"
)

type (
	List__Link []datamodel.Link
	Epoch      struct {
		Kind    int        `json:"kind" yaml:"kind"`
		Epoch   int        `json:"epoch" yaml:"epoch"`
		Subsets List__Link `json:"subsets" yaml:"subsets"`
	}
)

type Subset struct {
	Kind   int        `json:"kind" yaml:"kind"`
	First  int        `json:"first" yaml:"first"`
	Last   int        `json:"last" yaml:"last"`
	Blocks List__Link `json:"blocks" yaml:"blocks"`
}
type (
	List__Shredding []Shredding
	Block           struct {
		Kind      int             `json:"kind" yaml:"kind"`
		Slot      int             `json:"slot" yaml:"slot"`
		Shredding List__Shredding `json:"shredding" yaml:"shredding"`
		Entries   List__Link      `json:"entries" yaml:"entries"`
		Meta      SlotMeta        `json:"meta" yaml:"meta"`
		Rewards   datamodel.Link  `json:"rewards" yaml:"rewards"`
	}
)

type Rewards struct {
	Kind int       `json:"kind" yaml:"kind"`
	Slot int       `json:"slot" yaml:"slot"`
	Data DataFrame `json:"data" yaml:"data"`
}
type SlotMeta struct {
	Parent_slot  int   `json:"parent_slot" yaml:"parent_slot"`
	Blocktime    int   `json:"blocktime" yaml:"blocktime"`
	Block_height **int `json:"block_height" yaml:"block_height"`
}
type Shredding struct {
	EntryEndIdx int `json:"entry_end_idx" yaml:"entry_end_idx"`
	ShredEndIdx int `json:"shred_end_idx" yaml:"shred_end_idx"`
}
type Entry struct {
	Kind         int        `json:"kind" yaml:"kind"`
	NumHashes    int        `json:"num_hashes" yaml:"num_hashes"`
	Hash         Hash       `json:"hash" yaml:"hash"`
	Transactions List__Link `json:"transactions" yaml:"transactions"`
}
type Transaction struct {
	Kind     int       `json:"kind" yaml:"kind"`
	Data     DataFrame `json:"data" yaml:"data"`
	Metadata DataFrame `json:"metadata" yaml:"metadata"`
	Slot     int       `json:"slot" yaml:"slot"`
	Index    **int     `json:"index" yaml:"index"`
}
type DataFrame struct {
	Kind  int          `json:"kind" yaml:"kind"`
	Hash  **int        `json:"hash" yaml:"hash"`
	Index **int        `json:"index" yaml:"index"`
	Total **int        `json:"total" yaml:"total"`
	Data  Buffer       `json:"data" yaml:"data"`
	Next  **List__Link `json:"next" yaml:"next"`
}
