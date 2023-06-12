package compactindex

import (
	"errors"
	"fmt"
	"io"
)

// DB is a compactindex handle.
type DB struct {
	Header
	Stream io.ReaderAt
}

// Open returns a handle to access a compactindex.
//
// The provided stream must start with the Magic byte sequence.
// Tip: Use io.NewSectionReader to create aligned substreams when dealing with a file that contains multiple indexes.
func Open(stream io.ReaderAt) (*DB, error) {
	// Read the static 32-byte header.
	// Ignore errors if the read fails after filling the buffer (e.g. EOF).
	var fileHeader [headerSize]byte
	n, readErr := stream.ReadAt(fileHeader[:], 0)
	if n < len(fileHeader) {
		// ReadAt must return non-nil error here.
		return nil, readErr
	}
	db := new(DB)
	if err := db.Header.Load(&fileHeader); err != nil {
		return nil, err
	}
	db.Stream = stream
	return db, nil
}

// Lookup queries for a key in the index and returns the value (offset), if any.
//
// Returns ErrNotFound if the key is unknown.
func (db *DB) Lookup(key []byte) (uint64, error) {
	bucket, err := db.LookupBucket(key)
	if err != nil {
		return 0, err
	}
	return bucket.Lookup(key)
}

// LookupBucket returns a handle to the bucket that might contain the given key.
func (db *DB) LookupBucket(key []byte) (*Bucket, error) {
	return db.GetBucket(db.Header.BucketHash(key))
}

// GetBucket returns a handle to the bucket at the given index.
func (db *DB) GetBucket(i uint) (*Bucket, error) {
	if i >= uint(db.Header.NumBuckets) {
		return nil, fmt.Errorf("out of bounds bucket index: %d >= %d", i, db.Header.NumBuckets)
	}

	// Fill bucket handle.
	bucket := &Bucket{
		BucketDescriptor: BucketDescriptor{
			Stride:      db.entryStride(),
			OffsetWidth: intWidth(db.FileSize),
		},
	}
	// Read bucket header.
	readErr := bucket.BucketHeader.readFrom(db.Stream, i)
	if readErr != nil {
		return nil, readErr
	}
	bucket.Entries = io.NewSectionReader(db.Stream, int64(bucket.FileOffset), int64(bucket.NumEntries)*int64(bucket.Stride))
	return bucket, nil
}

func (db *DB) entryStride() uint8 {
	hashSize := 3 // TODO remove hardcoded constant
	offsetSize := intWidth(db.FileSize)
	return uint8(hashSize) + offsetSize
}

func bucketOffset(i uint) int64 {
	return headerSize + int64(i)*bucketHdrLen
}

func (b *BucketHeader) readFrom(rd io.ReaderAt, i uint) error {
	var buf [bucketHdrLen]byte
	n, err := rd.ReadAt(buf[:], bucketOffset(i))
	if n < len(buf) {
		return err
	}
	b.Load(&buf)
	return nil
}

func (b *BucketHeader) writeTo(wr io.WriterAt, i uint) error {
	var buf [bucketHdrLen]byte
	b.Store(&buf)
	_, err := wr.WriteAt(buf[:], bucketOffset(i))
	return err
}

// Bucket is a database handle pointing to a subset of the index.
type Bucket struct {
	BucketDescriptor
	Entries *io.SectionReader
}

// maxEntriesPerBucket is the hardcoded maximum permitted number of entries per bucket.
const maxEntriesPerBucket = 1 << 24 // (16 * stride) MiB

// targetEntriesPerBucket is the average number of records in each hashtable bucket we aim for.
const targetEntriesPerBucket = 10000

// Load retrieves all entries in the hashtable.
func (b *Bucket) Load(batchSize int) ([]Entry, error) {
	if batchSize <= 0 {
		batchSize = 512 // default to reasonable batch size
	}
	// TODO bounds check
	if b.NumEntries > maxEntriesPerBucket {
		return nil, fmt.Errorf("refusing to load bucket with %d entries", b.NumEntries)
	}
	entries := make([]Entry, 0, b.NumEntries)

	stride := int(b.Stride)
	buf := make([]byte, batchSize*stride)
	off := int64(0)
	for {
		// Read another chunk.
		n, err := b.Entries.ReadAt(buf, off)
		// Decode all entries in it.
		sub := buf[:n]
		for len(sub) >= stride {
			entries = append(entries, b.unmarshalEntry(sub))
			sub = sub[stride:]
			off += int64(stride)
		}
		// Handle error.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		} else if err != nil {
			return nil, err
		}
	}

	return entries, nil
}

// TODO: This binary search algo is not optimized for high-latency remotes yet.

// Lookup queries for a key using binary search.
func (b *Bucket) Lookup(key []byte) (uint64, error) {
	return b.binarySearch(b.Hash(key))
}

func (b *Bucket) binarySearch(target uint64) (uint64, error) {
	low := 0
	high := int(b.NumEntries)
	for low <= high {
		median := (low + high) / 2
		entry, err := b.loadEntry(median)
		if err != nil {
			return 0, err
		}
		if entry.Hash == target {
			return entry.Value, nil
		} else if entry.Hash < target {
			low = median + 1
		} else {
			high = median - 1
		}
	}
	return 0, ErrNotFound
}

func (b *Bucket) loadEntry(i int) (Entry, error) {
	off := int64(i) * int64(b.Stride)
	buf := make([]byte, b.Stride)
	n, err := b.Entries.ReadAt(buf, off)
	if n != len(buf) {
		return Entry{}, err
	}
	return b.unmarshalEntry(buf), nil
}

// ErrNotFound marks a missing entry.
var ErrNotFound = errors.New("not found")
