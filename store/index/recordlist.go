package index

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/rpcpool/yellowstone-faithful/store/types"
)

// BucketPrefixSize is how many bytes of bucket prefixes are stored.
const BucketPrefixSize int = 4

// FileOffsetBytes is the byte size of the file offset
const FileOffsetBytes int = 8

// FileSizeBytes is the byte size of the file size
const FileSizeBytes int = 4

// KeySizeBytes is key length slot, a one byte prefix
const KeySizeBytes int = 1

// KeyPositionPair contains a key, which is the unique prefix of the actual key, and the value
// which is a file offset.
type KeyPositionPair struct {
	Key []byte
	// The file offset, into the primary file, where the full key and its value
	// is actually stored.
	Block types.Block
}

// Record is a KeyPositionPair plus the actual position of the record in the record list
type Record struct {
	// The current position (in bytes) of the record within the [`RecordList`]
	Pos int
	KeyPositionPair
}

// RecordList is the main object that contains several [`Record`]s. Records can be stored and retrieved.
//
// The underlying data is a continuous range of bytes. The format is:
//
// ```text
//
//	|                  Once                  |      Repeated     |
//	|                                        |                   |
//	|                 4 bytes                | Variable size | … |
//	| Bit value used to determine the bucket |     Record    | … |
//
// ```
type RecordList []byte

// NewRecordList returns an iterable RecordList from the given byte array
func NewRecordList(data []byte) RecordList {
	return RecordList(data[BucketPrefixSize:])
}

// NewRecordList returns an iterable RecordList from the given byte array
func NewRecordListRaw(data []byte) RecordList {
	return RecordList(data)
}

// FindKeyPosition return the position where a key would be added.
//
// Returns the position together with the previous record.
func (rl RecordList) FindKeyPosition(key []byte) (pos int, prev Record, hasPrev bool) {
	rli := &RecordListIter{rl, 0}
	for !rli.Done() {
		record := rli.Next()
		// Location where the key gets inserted is found
		if bytes.Compare(record.Key, key) == 1 {
			pos = record.Pos
			return
		}
		hasPrev = true
		prev = record
	}
	pos = len(rl)
	return
}

// PutKeys puts keys at a certain position and returns the new data
//
// This method puts a continuous range of keys inside the data structure. The given range
// is where it is put. *This means that you can also overwrite existing keys.*
//
// This is needed if you insert a new key that fully contains an existing key. The existing
// key needs to replaced by one with a larger prefix, so that it is distinguishable from the
// new key.
func (rl RecordList) PutKeys(keys []KeyPositionPair, start int, end int) []byte {
	newKeys := make([]byte, 0,
		len(rl)-(end-start)+
			// Each key might have a different size, so just allocate an arbitrary size to
			// prevent more allocations. I picked 32 bytes as I don't expect hashes (hence
			// keys) to be bigger that that
			(len(keys))*(KeySizeBytes+FileOffsetBytes+FileSizeBytes+32))
	newKeys = append(newKeys, rl[:start]...)
	// Adding new keys to the beginning of the list.
	for i := range keys {
		newKeys = AddKeyPosition(newKeys, keys[i])
	}
	return append(newKeys, rl[end:]...)
}

// Get the primary storage file offset for that key.
//
// As the index is only storing prefixes and not the actual keys, the returned offset might
// match, it's not guaranteed. Once the key is retieved from the primary storage it needs to
// be checked if it actually matches.
func (rl RecordList) Get(key []byte) (types.Block, bool) {
	// Several prefixes can match a `key`, we are only interested in the last one that
	// matches, hence keep a match around until we can be sure it's the last one.
	rli := &RecordListIter{rl, 0}
	var blk types.Block
	var matched bool
	for !rli.Done() {
		record := rli.Next()
		// The stored prefix of the key needs to match the requested key.
		if bytes.HasPrefix(key, record.Key) {
			matched = true
			blk = record.Block
		} else if bytes.Compare(record.Key, key) == 1 {
			// No keys from here on can possibly match, hence stop iterating. If we had a prefix
			// match, return that, else return none
			break
		}
	}

	return blk, matched
}

// GetRecord returns the full record for a key in the recordList
func (rl RecordList) GetRecord(key []byte) *Record {
	// Several prefixes can match a `key`, we are only interested in the last one that
	// matches
	var r *Record
	rli := &RecordListIter{rl, 0}
	for !rli.Done() {
		record := rli.Next()
		// The stored prefix of the key needs to match the requested key.
		if bytes.HasPrefix(key, record.Key) {
			r = &record
		} else if bytes.Compare(record.Key, key) == 1 {
			// No keys from here on can possibly match, hence stop iterating. If we had a prefix
			// match, return that, else return nil
			break
		}
	}

	// Return the record with larger match with prefix.
	return r
}

// ReadRecord reads a record from a slice at the given position.
//
// The given position must point to the first byte where the record starts.
func (rl RecordList) ReadRecord(pos int) Record {
	sizeOffset := pos + FileOffsetBytes + FileSizeBytes
	size := rl[int(sizeOffset)]
	return Record{
		pos,
		KeyPositionPair{rl[sizeOffset+KeySizeBytes : sizeOffset+KeySizeBytes+int(size)], types.Block{
			Offset: types.Position(binary.LittleEndian.Uint64(rl[pos:])),
			Size:   types.Size(binary.LittleEndian.Uint32(rl[pos+FileOffsetBytes:])),
		}},
	}
}

// Len returns the byte length of the record list.
func (rl RecordList) Len() int {
	return len(rl)
}

// Empty eturns true if the record list is empty.
func (rl RecordList) Empty() bool {
	return len(rl) == 0
}

// Iter returns an iterator for a record list
func (rl RecordList) Iter() *RecordListIter {
	return &RecordListIter{rl, 0}
}

// RecordListIter provides an easy mechanism to iterate a record list
type RecordListIter struct {
	records RecordList
	// The data we are iterating over
	// The current position within the data
	pos int
}

// Done indicates whether there are more records to read
func (rli *RecordListIter) Done() bool {
	return rli.pos >= len(rli.records)
}

// Next returns the next record in the list
func (rli *RecordListIter) Next() Record {
	record := rli.records.ReadRecord(rli.pos)
	// Prepare the internal state for the next call
	rli.pos += FileOffsetBytes + FileSizeBytes + KeySizeBytes + len(record.Key)
	return record
}

// NextPos returns the position of the next record.
func (r *Record) NextPos() int {
	return r.Pos + FileOffsetBytes + FileSizeBytes + KeySizeBytes + len(r.Key)
}

// AddKeyPosition extends record data with an encoded key and a file offset.
//
// The format is:
//
// ```text
//
//	|         8 bytes        |      1 byte     | Variable size < 256 bytes |
//	| Pointer to actual data | Size of the key |            Key            |
//
// ```
func AddKeyPosition(data []byte, keyPos KeyPositionPair) []byte {
	size := byte(len(keyPos.Key))
	offsetBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(offsetBytes, uint64(keyPos.Block.Offset))
	sizeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBytes, uint32(keyPos.Block.Size))
	return append(append(append(append(data, offsetBytes...), sizeBytes...), size), keyPos.Key...)
}

// EncodeKeyPosition a key and and offset into a single record
func EncodeKeyPosition(keyPos KeyPositionPair) []byte {
	encoded := make([]byte, 0, FileOffsetBytes+FileSizeBytes+KeySizeBytes+len(keyPos.Key))
	return AddKeyPosition(encoded, keyPos)
}

// ReadBucketPrefix reads the bucket prefix and returns it.
func ReadBucketPrefix(reader io.Reader) (BucketIndex, error) {
	bucketPrefixBuffer := make([]byte, BucketPrefixSize)
	_, err := io.ReadFull(reader, bucketPrefixBuffer)
	if err != nil {
		return 0, err
	}
	return BucketIndex(binary.LittleEndian.Uint32(bucketPrefixBuffer)), nil
}
