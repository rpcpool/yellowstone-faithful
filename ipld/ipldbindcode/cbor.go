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

// Set(i, v) sets the i-th element of the array to v.
func (a *_array) Set(i int, v any) {
	if i >= len(*a) {
		*a = append(*a, make([]any, i-len(*a)+1)...)
	}
	(*a)[i] = v
}

func newArray(l int, init ...any) _array {
	a := make(_array, l)
	if len(init) > 0 {
		if len(init) > l {
			panic("initial values exceed array length")
		}
		copy(a, init)
	}
	return a
}

func encodeCBOR(data any) ([]byte, error) {
	// Create encoding mode.
	opts := cbor.CanonicalEncOptions() // use preset options as a starting point
	// opts.IndefLength = cbor.IndefLengthAllowed
	// opts.NilContainers = cbor.NilContainerAsNull
	// opts.ShortestFloat = cbor.ShortestFloat16
	em, err := opts.EncMode() // create an immutable encoding mode
	if err != nil {
		return nil, fmt.Errorf("failed to create encoding mode: %w", err)
	}

	// API matches encoding/json.
	var buf bytes.Buffer
	enc := em.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

var (
	_ cbor.Unmarshaler = (*Epoch)(nil)
	_ cbor.Marshaler   = (*Epoch)(nil)
)

// implement the BinaryUnmarshaler interface for EpochFast
func (x *Epoch) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		kind, err := getUint64FromInterface(kind)
		if err != nil {
			return fmt.Errorf("failed to get kind: %w", err)
		}
		x.Kind = int(kind)
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(4) {
		return fmt.Errorf("expected KindEpoch, got %d", x.Kind)
	}
	// second is the epoch uint64
	if epoch, ok := arr.Get(1); ok {
		epoch, err := getUint64FromInterface(epoch)
		if err != nil {
			return fmt.Errorf("failed to get epoch: %w", err)
		}
		x.Epoch = int(epoch)
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

func (x *Epoch) MarshalCBOR() ([]byte, error) {
	arr := newArray(3)
	arr.Set(0, uint64(x.Kind))
	arr.Set(1, uint64(x.Epoch))
	subsets := make([]interface{}, len(x.Subsets))
	for i, subset := range x.Subsets {
		subsets[i] = cbor.Tag{Number: 42, Content: append([]byte{0}, subset.(cidlink.Link).Cid.Bytes()...)}
	}
	arr.Set(2, subsets)
	return encodeCBOR(arr)
}

var (
	_ cbor.Unmarshaler = (*Subset)(nil)
	_ cbor.Marshaler   = (*Subset)(nil)
)

func (x *Subset) MarshalCBOR() ([]byte, error) {
	arr := newArray(4)
	arr.Set(0, int64(x.Kind))
	arr.Set(1, int64(x.First))
	arr.Set(2, int64(x.Last))
	blocks := make([]interface{}, len(x.Blocks))
	for i, block := range x.Blocks {
		blocks[i] = cbor.Tag{Number: 42, Content: append([]byte{0}, block.(cidlink.Link).Cid.Bytes()...)}
	}
	// arr.Set(3, cbor.Tag{Number: 0x99, Content: blocks})
	{
		a, err := EncodeArrayWith16BitLen(blocks...)
		if err != nil {
			return nil, fmt.Errorf("EncodeArrayWith16BitLen error: %w", err)
		}
		arr.Set(3, cbor.RawMessage(a))
	}
	// arr.Set(3, blocks)
	return encodeCBOR(arr)
}

func EncodeArrayWith16BitLen(values ...any) ([]byte, error) {
	// 1) Encode each item individually to CBOR.
	var items []cbor.RawMessage
	for _, v := range values {
		b, err := cbor.Marshal(v) // uses default options
		if err != nil {
			return nil, fmt.Errorf("cbor.Marshal error: %w", err)
		}
		items = append(items, b)
	}

	// 2) Build the array header: 0x99 = two-byte array length
	//    Then write length as big-endian uint16.
	length := len(items) // number of array elements
	header := []byte{
		0x99,
		byte(length >> 8),
		byte(length),
	}

	// 3) Concatenate header + item bytes
	result := make([]byte, 0, len(header))
	result = append(result, header...)
	for _, it := range items {
		result = append(result, it...)
	}

	return result, nil
}

func (x *Subset) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		kind, err := getUint64FromInterface(kind)
		if err != nil {
			return fmt.Errorf("failed to get kind: %w", err)
		}
		x.Kind = int(kind)
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(3) {
		return fmt.Errorf("expected KindSubset, got %d", x.Kind)
	}
	// second is the first uint64
	if first, ok := arr.Get(1); ok {
		first, err := getUint64FromInterface(first)
		if err != nil {
			return fmt.Errorf("failed to get first: %w", err)
		}
		x.First = int(first)
	} else {
		return fmt.Errorf("expected first to be present")
	}
	// third is the last uint64
	if last, ok := arr.Get(2); ok {
		last, err := getUint64FromInterface(last)
		if err != nil {
			return fmt.Errorf("failed to get last: %w", err)
		}
		x.Last = int(last)
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

var (
	_ cbor.Unmarshaler = (*Block)(nil)
	_ cbor.Marshaler   = (*Block)(nil)
)

func (x *Block) MarshalCBOR() ([]byte, error) {
	arr := newArray(6)
	arr.Set(0, uint64(x.Kind))
	arr.Set(1, uint64(x.Slot))
	shredding := make([]interface{}, len(x.Shredding))
	for i, shr := range x.Shredding {
		shreddingArr := newArray(2)
		shreddingArr.Set(0, uint64(shr.EntryEndIdx))
		shreddingArr.Set(1, uint64(shr.ShredEndIdx))
		shredding[i] = shreddingArr
	}
	arr.Set(2, shredding)
	entries := make([]interface{}, len(x.Entries))
	for i, entry := range x.Entries {
		entries[i] = cbor.Tag{Number: 42, Content: append([]byte{0}, entry.(cidlink.Link).Cid.Bytes()...)}
	}
	arr.Set(3, entries)
	meta := newArray(3)
	meta.Set(0, uint64(x.Meta.Parent_slot))
	meta.Set(1, uint64(x.Meta.Blocktime))
	if x.Meta.Block_height != nil && *x.Meta.Block_height != nil {
		meta.Set(2, uint64(**x.Meta.Block_height))
	}
	arr.Set(4, meta)
	arr.Set(5, cbor.Tag{Number: 42, Content: append([]byte{0}, x.Rewards.(cidlink.Link).Cid.Bytes()...)})
	return encodeCBOR(arr)
}

func (x *Block) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		kind, err := getUint64FromInterface(kind)
		if err != nil {
			return fmt.Errorf("failed to get kind: %w", err)
		}
		x.Kind = int(kind)
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(2) {
		return fmt.Errorf("expected KindBlock, got %d", x.Kind)
	}
	// second is the slot uint64
	if slot, ok := arr.Get(1); ok {
		slot, err := getUint64FromInterface(slot)
		if err != nil {
			return fmt.Errorf("failed to get slot: %w", err)
		}
		x.Slot = int(slot)
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
					entryEndIdx, err := getUint64FromInterface(entryEndIdx)
					if err != nil {
						return fmt.Errorf("failed to get entry_end_idx: %w", err)
					}
					shr.EntryEndIdx = int(entryEndIdx)
				} else {
					return fmt.Errorf("expected entry_end_idx to be present")
				}

				if shredEndIdx, ok := rawShreddingArr.Get(1); ok {
					shredEndIdx, err := getUint64FromInterface(shredEndIdx)
					if err != nil {
						return fmt.Errorf("failed to get shred_end_idx: %w", err)
					}
					shr.ShredEndIdx = int(shredEndIdx)
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
			parentSlot, err := getUint64FromInterface(parentSlot)
			if err != nil {
				return fmt.Errorf("failed to get parent_slot: %w", err)
			}
			m.Parent_slot = int(parentSlot)
		} else {
			return fmt.Errorf("expected parent_slot to be present")
		}
		if blocktime, ok := metaArr.Get(1); ok {
			blocktime, err := getUint64FromInterface(blocktime)
			if err != nil {
				return fmt.Errorf("failed to get blocktime: %w", err)
			}
			m.Blocktime = int(blocktime)
		} else {
			return fmt.Errorf("expected blocktime to be present")
		}
		if blockHeight, ok := metaArr.Get(2); ok {
			if blockHeight != nil {
				blockHeight, err := getUint64FromInterface(blockHeight)
				if err != nil {
					return fmt.Errorf("failed to get block_height: %w", err)
				}
				_blockHeight := int(blockHeight)
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

func getUint64FromInterface(i interface{}) (uint64, error) {
	asUint64, ok := i.(uint64)
	if ok {
		return asUint64, nil
	}
	asInt64, ok := i.(int64)
	if ok {
		return uint64(asInt64), nil
	}
	return 0, fmt.Errorf("expected uint64 or int64, got %T", i)
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

var (
	_ cbor.Unmarshaler = (*Rewards)(nil)
	_ cbor.Marshaler   = (*Rewards)(nil)
)

func (x *Rewards) MarshalCBOR() ([]byte, error) {
	arr := newArray(3)
	arr.Set(0, uint64(x.Kind))
	arr.Set(1, uint64(x.Slot))
	data := newArray(5)
	data.Set(0, uint64(x.Data.Kind))
	if x.Data.Hash != nil && *x.Data.Hash != nil {
		data.Set(1, uint64(**x.Data.Hash))
	}
	if x.Data.Index != nil && *x.Data.Index != nil {
		data.Set(2, uint64(**x.Data.Index))
	}
	if x.Data.Total != nil && *x.Data.Total != nil {
		data.Set(3, uint64(**x.Data.Total))
	}
	data.Set(4, x.Data.Data)
	arr.Set(2, data)
	return encodeCBOR(arr)
}

func (x *Rewards) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		kind, err := getUint64FromInterface(kind)
		if err != nil {
			return fmt.Errorf("failed to get kind: %w", err)
		}
		x.Kind = int(kind)
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(5) {
		return fmt.Errorf("expected KindRewards, got %d", x.Kind)
	}
	// second is the slot uint64
	if slot, ok := arr.Get(1); ok {
		slot, err := getUint64FromInterface(slot)
		if err != nil {
			return fmt.Errorf("failed to get slot: %w", err)
		}
		x.Slot = int(slot)
	} else {
		return fmt.Errorf("expected slot to be present")
	}
	// third is the data DataFrame
	if data, ok := arr.Get(2); ok {
		dataArr := _array(data.([]interface{}))
		var d DataFrame
		if kind, ok := dataArr.Get(0); ok {
			kind, err := getUint64FromInterface(kind)
			if err != nil {
				return fmt.Errorf("failed to get kind: %w", err)
			}
			d.Kind = int(kind)
		} else {
			return fmt.Errorf("expected kind to be present")
		}
		if hash, ok := dataArr.Get(1); ok {
			if hash != nil {
				hash, err := getUint64FromInterface(hash)
				if err != nil {
					return fmt.Errorf("failed to get hash: %w", err)
				}
				_hash := int(hash)
				_hash_ptr := &_hash
				d.Hash = &_hash_ptr
			}
		}
		if index, ok := dataArr.Get(2); ok {
			if index != nil {
				index, err := getUint64FromInterface(index)
				if err != nil {
					return fmt.Errorf("failed to get index: %w", err)
				}
				_index := int(index)
				_index_ptr := &_index
				d.Index = &_index_ptr
			}
		}
		if total, ok := dataArr.Get(3); ok {
			if total != nil {
				total, err := getUint64FromInterface(total)
				if err != nil {
					return fmt.Errorf("failed to get total: %w", err)
				}
				_total := int(total)
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

var (
	_ cbor.Unmarshaler = (*Entry)(nil)
	_ cbor.Marshaler   = (*Entry)(nil)
)

func (x *Entry) MarshalCBOR() ([]byte, error) {
	arr := newArray(4)
	arr.Set(0, uint64(x.Kind))
	arr.Set(1, uint64(x.NumHashes))
	arr.Set(2, x.Hash)
	transactions := make([]interface{}, len(x.Transactions))
	for i, transaction := range x.Transactions {
		transactions[i] = cbor.Tag{Number: 42, Content: append([]byte{0}, transaction.(cidlink.Link).Cid.Bytes()...)}
	}
	arr.Set(3, transactions)
	return encodeCBOR(arr)
}

func (x *Entry) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		kind, err := getUint64FromInterface(kind)
		if err != nil {
			return fmt.Errorf("failed to get kind: %w", err)
		}
		x.Kind = int(kind)
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(1) {
		return fmt.Errorf("expected KindEntry, got %d", x.Kind)
	}
	// second is the num_hashes uint64
	if numHashes, ok := arr.Get(1); ok {
		numHashes, err := getUint64FromInterface(numHashes)
		if err != nil {
			return fmt.Errorf("failed to get num_hashes: %w", err)
		}
		x.NumHashes = int(numHashes)
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

var (
	_ cbor.Unmarshaler = (*Transaction)(nil)
	_ cbor.Marshaler   = (*Transaction)(nil)
)

func (x *Transaction) MarshalCBOR() ([]byte, error) {
	arr := newArray(5)
	arr.Set(0, uint64(x.Kind))
	data := newArray(5)
	data.Set(0, uint64(x.Data.Kind))
	if x.Data.Hash != nil && *x.Data.Hash != nil {
		data.Set(1, uint64(**x.Data.Hash))
	}
	if x.Data.Index != nil && *x.Data.Index != nil {
		data.Set(2, uint64(**x.Data.Index))
	}
	if x.Data.Total != nil && *x.Data.Total != nil {
		data.Set(3, uint64(**x.Data.Total))
	}
	data.Set(4, x.Data.Data)
	arr.Set(1, data)
	meta := newArray(5)
	meta.Set(0, uint64(x.Metadata.Kind))
	if x.Metadata.Hash != nil && *x.Metadata.Hash != nil {
		meta.Set(1, uint64(**x.Metadata.Hash))
	}
	if x.Metadata.Index != nil && *x.Metadata.Index != nil {
		meta.Set(2, uint64(**x.Metadata.Index))
	}
	if x.Metadata.Total != nil && *x.Metadata.Total != nil {
		meta.Set(3, uint64(**x.Metadata.Total))
	}
	meta.Set(4, x.Metadata.Data)
	arr.Set(2, meta)
	arr.Set(3, uint64(x.Slot))
	if x.Index != nil && *x.Index != nil {
		arr.Set(4, uint64(**x.Index))
	}
	return encodeCBOR(arr)
}

func (x *Transaction) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		kind, err := getUint64FromInterface(kind)
		if err != nil {
			return fmt.Errorf("failed to get kind: %w", err)
		}
		x.Kind = int(kind)
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
			kind, err := getUint64FromInterface(kind)
			if err != nil {
				return fmt.Errorf("failed to get kind: %w", err)
			}
			d.Kind = int(kind)
		} else {
			return fmt.Errorf("expected kind to be present")
		}
		if hash, ok := dataArr.Get(1); ok {
			if hash != nil {
				hash, err := getUint64FromInterface(hash)
				if err != nil {
					return fmt.Errorf("failed to get hash: %w", err)
				}
				_hash := int(hash)
				_hash_ptr := &_hash
				d.Hash = &_hash_ptr
			}
		}
		if index, ok := dataArr.Get(2); ok {
			if index != nil {
				index, err := getUint64FromInterface(index)
				if err != nil {
					return fmt.Errorf("failed to get index: %w", err)
				}
				_index := int(index)
				_index_ptr := &_index
				d.Index = &_index_ptr
			}
		}
		if total, ok := dataArr.Get(3); ok {
			if total != nil {
				total, err := getUint64FromInterface(total)
				if err != nil {
					return fmt.Errorf("failed to get total: %w", err)
				}
				_total := int(total)
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
			kind, err := getUint64FromInterface(kind)
			if err != nil {
				return fmt.Errorf("failed to get kind: %w", err)
			}
			m.Kind = int(kind)
		} else {
			return fmt.Errorf("expected kind to be present")
		}
		if hash, ok := metaArr.Get(1); ok {
			if hash != nil {
				hash, err := getUint64FromInterface(hash)
				if err != nil {
					return fmt.Errorf("failed to get hash: %w", err)
				}
				_hash := int(hash)
				_hash_ptr := &_hash
				m.Hash = &_hash_ptr
			}
		}
		if index, ok := metaArr.Get(2); ok {
			if index != nil {
				index, err := getUint64FromInterface(index)
				if err != nil {
					return fmt.Errorf("failed to get index: %w", err)
				}
				_index := int(index)
				_index_ptr := &_index
				m.Index = &_index_ptr
			}
		}
		if total, ok := metaArr.Get(3); ok {
			if total != nil {
				total, err := getUint64FromInterface(total)
				if err != nil {
					return fmt.Errorf("failed to get total: %w", err)
				}
				_total := int(total)
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
		slot, err := getUint64FromInterface(slot)
		if err != nil {
			return fmt.Errorf("failed to get slot: %w", err)
		}
		x.Slot = int(slot)
	} else {
		return fmt.Errorf("expected slot to be present")
	}
	// fifth is the index uint64
	if index, ok := arr.Get(4); ok {
		if index != nil {
			index, err := getUint64FromInterface(index)
			if err != nil {
				return fmt.Errorf("failed to get index: %w", err)
			}
			_index := int(index)
			_index_ptr := &_index
			x.Index = &_index_ptr
		}
	}

	return nil
}

var (
	_ cbor.Unmarshaler = (*DataFrame)(nil)
	_ cbor.Marshaler   = (*DataFrame)(nil)
)

func (x *DataFrame) MarshalCBOR() ([]byte, error) {
	arr := newArray(6)
	arr.Set(0, uint64(x.Kind))
	if x.Hash != nil && *x.Hash != nil {
		arr.Set(1, int(**x.Hash))
	}
	if x.Index != nil && *x.Index != nil {
		arr.Set(2, int(**x.Index))
	}
	if x.Total != nil && *x.Total != nil {
		arr.Set(3, int(**x.Total))
	}
	arr.Set(4, x.Data)
	if x.Next != nil && *x.Next != nil {
		next := make([]interface{}, len(**x.Next))
		for i, n := range **x.Next {
			next[i] = cbor.Tag{Number: 42, Content: append([]byte{0}, n.(cidlink.Link).Cid.Bytes()...)}
		}
		arr.Set(5, next)
	}
	return encodeCBOR(arr)
}

func (x *DataFrame) UnmarshalCBOR(data []byte) error {
	dec := cbor.NewDecoder(bytes.NewReader(data))
	var arr _array
	if err := dec.Decode(&arr); err != nil {
		return err
	}
	// first is the kind uint64
	if kind, ok := arr.Get(0); ok {
		kind, err := getUint64FromInterface(kind)
		if err != nil {
			return fmt.Errorf("failed to get kind: %w", err)
		}
		x.Kind = int(kind)
	} else {
		return fmt.Errorf("expected kind to be present")
	}
	if x.Kind != int(6) {
		return fmt.Errorf("expected KindDataFrame, got %d", x.Kind)
	}
	// second is the hash int
	if hash, ok := arr.Get(1); ok {
		if hash != nil {
			hash, err := getUint64FromInterface(hash)
			if err != nil {
				return fmt.Errorf("failed to cast hash to int: %w", err)
			}
			_hash := int(hash)
			_hash_ptr := &_hash
			x.Hash = &_hash_ptr
		}
	}
	// third is the index int
	if index, ok := arr.Get(2); ok {
		if index != nil {
			index, err := getUint64FromInterface(index)
			if err != nil {
				return fmt.Errorf("failed to cast index to int: %w", err)
			}
			_index := int(index)
			_index_ptr := &_index
			x.Index = &_index_ptr
		}
	}
	// fourth is the total int
	if total, ok := arr.Get(3); ok {
		if total != nil {
			total, err := getUint64FromInterface(total)
			if err != nil {
				return fmt.Errorf("failed to cast total to int: %w", err)
			}
			_total := int(total)
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
