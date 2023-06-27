package index_test

import (
	"fmt"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/gsfa/store/index"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
	"github.com/stretchr/testify/require"
)

func TestEncodeKeyPosition(t *testing.T) {
	key := []byte("abcdefg")
	offset := 4326
	size := 64
	encoded := index.EncodeKeyPosition(index.KeyPositionPair{key, types.Block{Offset: types.Position(offset), Size: types.Size(size)}})
	require.Equal(t,
		encoded,
		[]byte{
			0xe6, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00, 0x07, 0x61, 0x62, 0x63, 0x64, 0x65,
			0x66, 0x67,
		},
	)
}

func TestRecordListIterator(t *testing.T) {
	// Create records
	var keys []string
	for i := 0; i < 20; i++ {
		keys = append(keys, fmt.Sprintf("key-%02d", i))
	}

	var expected []index.Record
	for i, key := range keys {
		expected = append(expected, index.Record{
			KeyPositionPair: index.KeyPositionPair{
				Key:   []byte(key),
				Block: types.Block{Offset: types.Position(i), Size: types.Size(i)},
			},
			Pos: i * 19,
		})
	}

	// Encode them into records list
	var data []byte
	for _, record := range expected {
		encoded := index.EncodeKeyPosition(record.KeyPositionPair)
		data = append(data, encoded...)
	}

	// The record list have the bits that were used to determine the bucket as prefix
	prefixedData := append([]byte{0, 0, 0, 0}, data...)
	// Verify that it can be correctly iterated over those encoded records
	records := index.NewRecordList(prefixedData)
	recordsIter := records.Iter()
	for _, record := range expected {
		require.False(t, recordsIter.Done())
		require.Equal(t, record, recordsIter.Next())
	}

	// Verify that we can compute next position successfully.
	// First key
	r := records.GetRecord([]byte(keys[1]))
	npos := r.NextPos()
	nr := records.GetRecord([]byte(keys[2]))
	require.Equal(t, npos, nr.Pos)
}

func TestRecordListFindKeyPosition(t *testing.T) {
	// Create data
	keys := []string{"a", "ac", "b", "d", "de", "dn", "nky", "xrlfg"}
	// Encode them into records list
	var data []byte
	for i, key := range keys {
		encoded := index.EncodeKeyPosition(index.KeyPositionPair{[]byte(key), types.Block{Offset: types.Position(i), Size: types.Size(i)}})
		data = append(data, encoded...)
	}
	// The record list have the bits that were used to determine the bucket as prefix
	prefixedData := append([]byte{0, 0, 0, 0}, data...)
	records := index.NewRecordList(prefixedData)

	// First key
	pos, prevRecord, hasPrev := records.FindKeyPosition([]byte("ABCD"))
	require.Equal(t, pos, 0)
	require.False(t, hasPrev)

	// Between two keys with same prefix, but first one being shorter
	pos, prevRecord, _ = records.FindKeyPosition([]byte("ab"))
	require.Equal(t, pos, 14)
	require.Equal(t, prevRecord.Key, []byte("a"))

	// Between to keys with both having a different prefix
	pos, prevRecord, hasPrev = records.FindKeyPosition([]byte("c"))
	require.True(t, hasPrev)
	require.Equal(t, pos, 43)
	require.Equal(t, prevRecord.Key, []byte("b"))

	// Between two keys with both having a different prefix and the input key having a
	// different length
	pos, prevRecord, _ = records.FindKeyPosition([]byte("cabefg"))

	require.Equal(t, pos, 43)
	require.Equal(t, prevRecord.Key, []byte("b"))

	// Between two keys with both having a different prefix (with one character in common),
	// all keys having the same length
	pos, prevRecord, _ = records.FindKeyPosition([]byte("dg"))
	require.Equal(t, pos, 72)
	require.Equal(t, prevRecord.Key, []byte("de"))

	// Between two keys with both having a different prefix, no charachter in in common and
	// different length (shorter than the input key)
	pos, prevRecord, _ = records.FindKeyPosition([]byte("hello"))
	require.Equal(t, pos, 87)
	require.Equal(t, prevRecord.Key, []byte("dn"))

	// Between two keys with both having a different prefix, no charachter in in common and
	// different length (longer than the input key)
	pos, prevRecord, _ = records.FindKeyPosition([]byte("pz"))
	require.Equal(t, pos, 103)
	require.Equal(t, prevRecord.Key, []byte("nky"))

	// Last key
	pos, prevRecord, _ = records.FindKeyPosition([]byte("z"))
	require.Equal(t, pos, 121)
	require.Equal(t, prevRecord.Key, []byte("xrlfg"))
}

// Validate that the new key was properly added
func assertAddKey(t *testing.T, records index.RecordList, key []byte) {
	pos, _, _ := records.FindKeyPosition(key)
	newData := records.PutKeys([]index.KeyPositionPair{{key, types.Block{Offset: types.Position(773), Size: types.Size(48)}}}, pos, pos)
	// The record list have the bits that were used to determine the bucket as prefix
	prefixedNewData := append([]byte{0, 0, 0, 0}, newData...)
	newRecords := index.NewRecordList(prefixedNewData)
	insertedPos, insertedRecord, _ := newRecords.FindKeyPosition(key)
	require.Equal(t,
		insertedPos,
		pos+index.FileOffsetBytes+index.FileSizeBytes+index.KeySizeBytes+len(key),
	)
	require.Equal(t, insertedRecord.Key, key)
}

func TestRecordListAddKeyWithoutReplacing(t *testing.T) {
	// Create Data
	keys := []string{"a", "ac", "b", "d", "de", "dn", "nky", "xrlfg"}
	// Encode them into records list
	var data []byte
	for i, key := range keys {
		encoded := index.EncodeKeyPosition(index.KeyPositionPair{[]byte(key), types.Block{Offset: types.Position(i), Size: types.Size(i)}})
		data = append(data, encoded...)
	}
	// The record list have the bits that were used to determine the bucket as prefix
	prefixedData := append([]byte{0, 0, 0, 0}, data...)
	records := index.NewRecordList(prefixedData)

	// First key
	assertAddKey(t, records, []byte("ABCD"))

	// Between two keys with same prefix, but first one being shorter
	assertAddKey(t, records, []byte("ab"))

	// Between to keys with both having a different prefix
	assertAddKey(t, records, []byte("c"))

	// Between two keys with both having a different prefix and the input key having a
	// different length
	assertAddKey(t, records, []byte("cabefg"))

	// Between two keys with both having a different prefix (with one character in common),
	// all keys having the same length
	assertAddKey(t, records, []byte("dg"))

	// Between two keys with both having a different prefix, no charachter in in common and
	// different length (shorter than the input key)
	assertAddKey(t, records, []byte("hello"))

	// Between two keys with both having a different prefix, no charachter in in common and
	// different length (longer than the input key)
	assertAddKey(t, records, []byte("pz"))

	// Last key
	assertAddKey(t, records, []byte("z"))
}

// Validate that the previous key was properly replaced and the new key was added.
func assertAddKeyAndReplacePrev(t *testing.T, records index.RecordList, key []byte, newPrevKey []byte) {
	pos, prevRecord, hasPrev := records.FindKeyPosition(key)
	require.True(t, hasPrev)

	keys := []index.KeyPositionPair{{newPrevKey, prevRecord.Block}, {key, types.Block{Offset: types.Position(773), Size: types.Size(48)}}}
	newData := records.PutKeys(keys, prevRecord.Pos, pos)
	// The record list have the bits that were used to determine the bucket as prefix
	prefixedNewData := append([]byte{0, 0, 0, 0}, newData...)
	newRecords := index.NewRecordList(prefixedNewData)

	// Find the newly added prevKey
	insertedPrevKeyPos, insertedPrevRecord, hasPrev := newRecords.FindKeyPosition(newPrevKey)
	require.True(t, hasPrev)
	require.Equal(t, insertedPrevRecord.Pos, prevRecord.Pos)
	require.Equal(t, insertedPrevRecord.Key, newPrevKey)

	// Find the newly added key
	insertedPos, insertedRecord, hasPrev := newRecords.FindKeyPosition(key)
	require.True(t, hasPrev)
	require.Equal(t,
		insertedPos,
		// The prev key is longer, hence use its position instead of the original one
		insertedPrevKeyPos+index.FileOffsetBytes+index.FileSizeBytes+index.KeySizeBytes+len(key),
	)
	require.Equal(t, insertedRecord.Key, key)
}

// If a new key is added and it fully contains the previous key, them the previous key needs
// to be updated as well. This is what these tests are about.
func TestRecordListAddKeyAndReplacePrev(t *testing.T) {
	// Create Data
	keys := []string{"a", "ac", "b", "d", "de", "dn", "nky", "xrlfg"}
	// Encode them into records list
	var data []byte
	for i, key := range keys {
		encoded := index.EncodeKeyPosition(index.KeyPositionPair{[]byte(key), types.Block{Offset: types.Position(i), Size: types.Size(i)}})
		data = append(data, encoded...)
	}
	// The record list have the bits that were used to determine the bucket as prefix
	prefixedData := append([]byte{0, 0, 0, 0}, data...)
	records := index.NewRecordList(prefixedData)

	// Between two keys with same prefix, but first one being shorter
	assertAddKeyAndReplacePrev(t, records, []byte("ab"), []byte("aa"))

	// Between two keys with same prefix, but first one being shorter. Replacing the previous
	// key which is more than one character longer than the existong one.
	assertAddKeyAndReplacePrev(t, records, []byte("ab"), []byte("aaaa"))

	// Between to keys with both having a different prefix
	assertAddKeyAndReplacePrev(t, records, []byte("c"), []byte("bx"))

	// Between two keys with both having a different prefix and the input key having a
	// different length
	assertAddKeyAndReplacePrev(t, records, []byte("cabefg"), []byte("bbccdd"))

	// Between two keys with both having a different prefix (with one character in common),
	// extending the prev key with an additional character to be distinguishable from the new
	// key
	assertAddKeyAndReplacePrev(t, records, []byte("deq"), []byte("dej"))

	// Last key
	assertAddKeyAndReplacePrev(t, records, []byte("xrlfgu"), []byte("xrlfgs"))
}

func TestRecordListGetKey(t *testing.T) {
	// Create Data
	keys := []string{"a", "ac", "b", "de", "dn", "nky", "xrlfg"}
	// Encode them into records list
	var data []byte
	for i, key := range keys {
		encoded := index.EncodeKeyPosition(index.KeyPositionPair{[]byte(key), types.Block{Offset: types.Position(i), Size: types.Size(i)}})
		data = append(data, encoded...)
	}
	// The record list have the bits that were used to determine the bucket as prefix
	prefixedData := append([]byte{0, 0, 0, 0}, data...)
	records := index.NewRecordList(prefixedData)

	// First key
	blk, has := records.Get([]byte("a"))
	require.True(t, has)
	require.Equal(t, blk, types.Block{Offset: types.Position(0), Size: types.Size(0)})

	// Key with same prefix, but it's the second one
	blk, has = records.Get([]byte("ac"))
	require.True(t, has)
	require.Equal(t, blk, types.Block{Offset: types.Position(1), Size: types.Size(1)})

	// Key with same length as two other keys, sharing a prefix
	blk, has = records.Get([]byte("de"))
	require.True(t, has)
	require.Equal(t, blk, types.Block{Offset: types.Position(3), Size: types.Size(3)})

	// Key that is sharing a prefix, but is longer
	blk, has = records.Get([]byte("dngho"))
	require.True(t, has)
	require.Equal(t, blk, types.Block{Offset: types.Position(4), Size: types.Size(4)})

	// Key that is the last one
	blk, has = records.Get([]byte("xrlfg"))
	require.True(t, has)
	require.Equal(t, blk, types.Block{Offset: types.Position(6), Size: types.Size(6)})

	// Key that is shorter than the inserted ones cannot match
	_, has = records.Get([]byte("d"))
	require.False(t, has)

	// Key that is before all keys
	_, has = records.Get([]byte("ABCD"))
	require.False(t, has)

	// Key that is after all keys
	_, has = records.Get([]byte("zzzzz"))
	require.False(t, has)

	// Key that matches a prefix of some keys, but doesn't match fully
	_, has = records.Get([]byte("dg"))
	require.False(t, has)
}
