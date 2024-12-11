package ipldbindcode

import (
	"bytes"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

type _array []any

// Get(i) returns the i-th element of the array, and bool indicating whether the element exists.
func (a _array) Get(i int) (any, bool) {
	if i >= len(a) {
		return nil, false
	}
	return a[i], true
}

func decodeCborLinkListFromAny(maybeList any) (List__Link, error) {
	if maybeList == nil {
		return nil, nil
	}
	var list List__Link = nil
	if subsetsArray, ok := maybeList.([]interface{}); ok {
		for _, subset := range subsetsArray {
			// each subset is represented raw as cbor.Tag
			rawTag, ok := subset.(cbor.Tag)
			if !ok {
				return nil, fmt.Errorf("expected cbor.Tag, got %T", subset)
			}
			// the tag number is the cbor tag number
			if rawTag.Number != 42 {
				return nil, fmt.Errorf("expected cbor tag number 42, got %d", rawTag.Number)
			}
			rawBytes, ok := rawTag.Content.([]byte)
			if !ok {
				return nil, fmt.Errorf("expected cbor tag content to be []byte, got %T", rawTag.Content)
			}
			// the tag content is the _cid.Cid, after the first byte
			_, _cid, err := cid.CidFromBytes(rawBytes[1:])
			if err != nil {
				return nil, fmt.Errorf("failed to cast cbor tag content to cid.Cid: %w", err)
			}
			list = append(list, cidlink.Link{Cid: _cid})
		}
	} else {
		return nil, fmt.Errorf("expected subsets to be []interface{}, got %T", maybeList)
	}
	return list, nil
}

// implement the BinaryUnmarshaler interface for EpochFast
func (x *Epoch) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		x.Kind = int(kind.(uint64))
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(4) {
		return fmt.Errorf("expected KindEpoch, got %d", x.Kind)
	}
	// second is the epoch uint64
	if epoch, ok := arr.Get(1); ok {
		x.Epoch = int(epoch.(uint64))
	} else {
		return fmt.Errorf("expected epoch to be present")
	}
	// third is the subsets []cid.Cid, represented as []any
	if subsets, ok := arr.Get(2); ok {
		var err error
		x.Subsets, err = decodeCborLinkListFromAny(subsets)
		if err != nil {
			return fmt.Errorf("failed to decode subsets: %w", err)
		}
	} else {
		return fmt.Errorf("expected subsets to be present")
	}

	return nil
}

func (x *Subset) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		x.Kind = int(kind.(uint64))
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(3) {
		return fmt.Errorf("expected KindSubset, got %d", x.Kind)
	}
	// second is the first uint64
	if first, ok := arr.Get(1); ok {
		x.First = int(first.(uint64))
	} else {
		return fmt.Errorf("expected first to be present")
	}
	// third is the last uint64
	if last, ok := arr.Get(2); ok {
		x.Last = int(last.(uint64))
	} else {
		return fmt.Errorf("expected last to be present")
	}
	// fourth is the blocks []cid.Cid, represented as []any
	if blocks, ok := arr.Get(3); ok {
		var err error
		x.Blocks, err = decodeCborLinkListFromAny(blocks)
		if err != nil {
			return fmt.Errorf("failed to decode blocks: %w", err)
		}
	} else {
		return fmt.Errorf("expected blocks to be present")
	}

	return nil
}

func (x *Block) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		x.Kind = int(kind.(uint64))
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(2) {
		return fmt.Errorf("expected KindBlock, got %d", x.Kind)
	}
	// second is the slot uint64
	if slot, ok := arr.Get(1); ok {
		x.Slot = int(slot.(uint64))
	} else {
		return fmt.Errorf("expected slot to be present")
	}
	// third is the shredding []Shredding, represented as []any
	if shredding, ok := arr.Get(2); ok {
		if shreddingArray, ok := shredding.([]interface{}); ok {
			for _, shredding := range shreddingArray {
				// each shredding is represented raw as cbor.Tag
				rawShredding, ok := shredding.([]interface{})
				if !ok {
					return fmt.Errorf("expected []interface{}, got %T", shredding)
				}
				rawShreddingArr := _array(rawShredding)
				var shr Shredding
				if entryEndIdx, ok := rawShreddingArr.Get(0); ok {
					shr.EntryEndIdx = int(entryEndIdx.(uint64))
				} else {
					return fmt.Errorf("expected entry_end_idx to be present")
				}

				if shredEndIdx, ok := rawShreddingArr.Get(1); ok {
					asUint64, ok := shredEndIdx.(uint64)
					if ok {
						shr.ShredEndIdx = int(asUint64)
					} else {
						asInt64, ok := shredEndIdx.(int64)
						if ok {
							shr.ShredEndIdx = int(asInt64)
						} else {
							return fmt.Errorf("expected shred_end_idx to be uint64 or int64, got %T", shredEndIdx)
						}
					}
				} else {
					return fmt.Errorf("expected shred_end_idx to be present")
				}

				x.Shredding = append(x.Shredding, shr)
			}
		} else {
			return fmt.Errorf("expected shredding to be []interface{}, got %T", shredding)
		}
	} else {
		return fmt.Errorf("expected shredding to be present")
	}
	// fourth is the entries []cid.Cid, represented as []any
	if entries, ok := arr.Get(3); ok {
		var err error
		x.Entries, err = decodeCborLinkListFromAny(entries)
		if err != nil {
			return fmt.Errorf("failed to decode entries: %w", err)
		}
	} else {
		return fmt.Errorf("expected entries to be present")
	}
	// fifth is the meta SlotMeta
	if meta, ok := arr.Get(4); ok {
		metaArr := _array(meta.([]interface{}))
		var m SlotMeta
		if parentSlot, ok := metaArr.Get(0); ok {
			m.Parent_slot = int(parentSlot.(uint64))
		} else {
			return fmt.Errorf("expected parent_slot to be present")
		}
		if blocktime, ok := metaArr.Get(1); ok {
			m.Blocktime = int(blocktime.(uint64))
		} else {
			return fmt.Errorf("expected blocktime to be present")
		}
		if blockHeight, ok := metaArr.Get(2); ok {
			if blockHeight != nil {
				_blockHeight := int(blockHeight.(uint64))
				_blockHeight_ptr := &_blockHeight
				m.Block_height = &_blockHeight_ptr
			}
		}
		x.Meta = m
	} else {
		return fmt.Errorf("expected meta to be present")
	}
	// sixth is the rewards cid.Cid, represented as cbor.Tag
	if rewards, ok := arr.Get(5); ok {
		rawTag, ok := rewards.(cbor.Tag)
		if !ok {
			return fmt.Errorf("expected cbor.Tag, got %T", rewards)
		}
		// the tag number is the cbor tag number
		if rawTag.Number != 42 {
			return fmt.Errorf("expected cbor tag number 42, got %d", rawTag.Number)
		}
		rawBytes, ok := rawTag.Content.([]byte)
		if !ok {
			return fmt.Errorf("expected cbor tag content to be []byte, got %T", rawTag.Content)
		}
		_, _cid, err := cid.CidFromBytes(rawBytes[1:])
		if err != nil {
			return fmt.Errorf("failed to cast cbor tag content to cid.Cid: %w", err)
		}
		x.Rewards = cidlink.Link{Cid: _cid}
	} else {
		return fmt.Errorf("expected rewards to be present")
	}

	return nil
}

func (x *Rewards) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		x.Kind = int(kind.(uint64))
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(5) {
		return fmt.Errorf("expected KindRewards, got %d", x.Kind)
	}
	// second is the slot uint64
	if slot, ok := arr.Get(1); ok {
		x.Slot = int(slot.(uint64))
	} else {
		return fmt.Errorf("expected slot to be present")
	}
	// third is the data DataFrame
	if data, ok := arr.Get(2); ok {
		dataArr := _array(data.([]interface{}))
		var d DataFrame
		if kind, ok := dataArr.Get(0); ok {
			d.Kind = int(kind.(uint64))
		} else {
			return fmt.Errorf("expected kind to be present")
		}
		if hash, ok := dataArr.Get(1); ok {
			if hash != nil {
				_hash := int(hash.(uint64))
				_hash_ptr := &_hash
				d.Hash = &_hash_ptr
			}
		}
		if index, ok := dataArr.Get(2); ok {
			if index != nil {
				_index := int(index.(uint64))
				_index_ptr := &_index
				d.Index = &_index_ptr
			}
		}
		if total, ok := dataArr.Get(3); ok {
			if total != nil {
				_total := int(total.(uint64))
				_total_ptr := &_total
				d.Total = &_total_ptr
			}
		}
		if data, ok := dataArr.Get(4); ok {
			rawBytes, ok := data.([]byte)
			if !ok {
				return fmt.Errorf("expected cbor tag content to be []byte, got %T", rawBytes)
			}
			// TODO: what is the start byte?
			d.Data = Buffer(rawBytes)
		} else {
			return fmt.Errorf("expected data to be present")
		}
		x.Data = d
	} else {
		return fmt.Errorf("expected data to be present")
	}

	return nil
}

func (x *Entry) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		x.Kind = int(kind.(uint64))
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(1) {
		return fmt.Errorf("expected KindEntry, got %d", x.Kind)
	}
	// second is the num_hashes uint64
	if numHashes, ok := arr.Get(1); ok {
		x.NumHashes = int(numHashes.(uint64))
	} else {
		return fmt.Errorf("expected num_hashes to be present")
	}
	// third is the hash Hash
	if hash, ok := arr.Get(2); ok {
		h := hash.([]byte)
		x.Hash = h
	} else {
		return fmt.Errorf("expected hash to be present")
	}
	// fourth is the transactions []cid.Cid, represented as []any
	if transactions, ok := arr.Get(3); ok {
		var err error
		x.Transactions, err = decodeCborLinkListFromAny(transactions)
		if err != nil {
			return fmt.Errorf("failed to decode transactions: %w", err)
		}
	} else {
		return fmt.Errorf("expected transactions to be present")
	}

	return nil
}

func (x *Transaction) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		x.Kind = int(kind.(uint64))
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(0) {
		return fmt.Errorf("expected KindTransaction, got %d", x.Kind)
	}
	// second is the data DataFrame
	if data, ok := arr.Get(1); ok {
		dataArr := _array(data.([]interface{}))
		var d DataFrame
		if kind, ok := dataArr.Get(0); ok {
			d.Kind = int(kind.(uint64))
		} else {
			return fmt.Errorf("expected kind to be present")
		}
		if hash, ok := dataArr.Get(1); ok {
			if hash != nil {
				_hash := int(hash.(uint64))
				_hash_ptr := &_hash
				d.Hash = &_hash_ptr
			}
		}
		if index, ok := dataArr.Get(2); ok {
			if index != nil {
				_index := int(index.(uint64))
				_index_ptr := &_index
				d.Index = &_index_ptr
			}
		}
		if total, ok := dataArr.Get(3); ok {
			if total != nil {
				_total := int(total.(uint64))
				_total_ptr := &_total
				d.Total = &_total_ptr
			}
		}
		if data, ok := dataArr.Get(4); ok {
			rawBytes, ok := data.([]byte)
			if !ok {
				return fmt.Errorf("expected cbor tag content to be []byte, got %T", rawBytes)
			}

			d.Data = Buffer(rawBytes)
		} else {
			return fmt.Errorf("expected data to be present")
		}
		x.Data = d
	} else {
		return fmt.Errorf("expected data to be present")
	}
	// third is the metadata DataFrame
	if metadata, ok := arr.Get(2); ok {
		metaArr := _array(metadata.([]interface{}))
		var m DataFrame
		if kind, ok := metaArr.Get(0); ok {
			m.Kind = int(kind.(uint64))
		} else {
			return fmt.Errorf("expected kind to be present")
		}
		if hash, ok := metaArr.Get(1); ok {
			if hash != nil {
				_hash := int(hash.(uint64))
				_hash_ptr := &_hash
				m.Hash = &_hash_ptr
			}
		}
		if index, ok := metaArr.Get(2); ok {
			if index != nil {
				_index := int(index.(uint64))
				_index_ptr := &_index
				m.Index = &_index_ptr
			}
		}
		if total, ok := metaArr.Get(3); ok {
			if total != nil {
				_total := int(total.(uint64))
				_total_ptr := &_total
				m.Total = &_total_ptr
			}
		}
		if data, ok := metaArr.Get(4); ok {
			rawBytes, ok := data.([]byte)
			if !ok {
				return fmt.Errorf("expected cbor tag content to be []byte, got %T", rawBytes)
			}
			m.Data = Buffer(rawBytes)
		} else {
			return fmt.Errorf("expected data to be present")
		}
		x.Metadata = m
	} else {
		return fmt.Errorf("expected metadata to be present")
	}
	// fourth is the slot uint64
	if slot, ok := arr.Get(3); ok {
		x.Slot = int(slot.(uint64))
	} else {
		return fmt.Errorf("expected slot to be present")
	}
	// fifth is the index uint64
	if index, ok := arr.Get(4); ok {
		if index != nil {
			_index := int(index.(uint64))
			_index_ptr := &_index
			x.Index = &_index_ptr
		}
	}

	return nil
}

func tryInterfaceToUint64OrInt64(i interface{}) (int, error) {
	if i == nil {
		return 0, nil
	}
	asUint64, ok := i.(uint64)
	if ok {
		return int(asUint64), nil
	}
	asInt64, ok := i.(int64)
	if ok {
		return int(asInt64), nil
	}
	return 0, fmt.Errorf("expected uint64 or int64, got %T", i)
}

func (x *DataFrame) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		x.Kind = int(kind.(uint64))
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(6) {
		return fmt.Errorf("expected KindDataFrame, got %d", x.Kind)
	}
	// second is the hash int
	if hash, ok := arr.Get(1); ok {
		if hash != nil {
			_hash, err := tryInterfaceToUint64OrInt64(hash)
			if err != nil {
				return fmt.Errorf("failed to cast hash to int: %w", err)
			}
			_hash_ptr := &_hash
			x.Hash = &_hash_ptr
		}
	}
	// third is the index int
	if index, ok := arr.Get(2); ok {
		if index != nil {
			_index := int(index.(uint64))
			_index_ptr := &_index
			x.Index = &_index_ptr
		}
	}
	// fourth is the total int
	if total, ok := arr.Get(3); ok {
		if total != nil {
			_total := int(total.(uint64))
			_total_ptr := &_total
			x.Total = &_total_ptr
		}
	}
	// fifth is the data Buffer
	if data, ok := arr.Get(4); ok {
		rawBytes, ok := data.([]byte)
		if !ok {
			return fmt.Errorf("expected cbor tag content to be []byte, got %T", rawBytes)
		}
		x.Data = Buffer(rawBytes)
	} else {
		return fmt.Errorf("expected data to be present")
	}
	// sixth is the next []cid.Cid, represented as []any
	if next, ok := arr.Get(5); ok {
		next, err := decodeCborLinkListFromAny(next)
		if err != nil {
			return fmt.Errorf("failed to decode next: %w", err)
		}
		next_ptr := &next
		x.Next = &next_ptr
	}

	return nil
}
