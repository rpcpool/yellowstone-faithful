package compactindex36

// This is a fork of the original project at https://github.com/firedancer-io/radiance/tree/main/pkg/compactindex
// The following changes have been made:
// - The package has been renamed to `compactindex36` to avoid conflicts with the original package
// - The values it indexes are 36-bit values instead of 8-bit values. This allows to index CIDs (in particular sha256+CBOR CIDs) directly.

import (
	"errors"
	"fmt"
	"io"
)

// DB is a compactindex handle.
type DB struct {
	Header
	Stream   io.ReaderAt
	prefetch bool
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

func (db *DB) Prefetch(yes bool) {
	db.prefetch = yes
}

// Lookup queries for a key in the index and returns the value (offset), if any.
//
// Returns ErrNotFound if the key is unknown.
func (db *DB) Lookup(key []byte) ([36]byte, error) {
	bucket, err := db.LookupBucket(key)
	if err != nil {
		return Empty, err
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
			OffsetWidth: valueLength(),
		},
	}
	// Read bucket header.
	readErr := bucket.BucketHeader.readFrom(db.Stream, i)
	if readErr != nil {
		return nil, readErr
	}
	bucket.Entries = io.NewSectionReader(db.Stream, int64(bucket.FileOffset), int64(bucket.NumEntries)*int64(bucket.Stride))
	if db.prefetch {
		// TODO: find good value for numEntriesToPrefetch
		numEntriesToPrefetch := minInt64(3_000, int64(bucket.NumEntries))
		prefetchSize := (36 + 3) * numEntriesToPrefetch
		buf := make([]byte, prefetchSize)
		_, err := bucket.Entries.ReadAt(buf, 0)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	return bucket, nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (db *DB) entryStride() uint8 {
	hashSize := 3 // TODO remove hardcoded constant
	offsetSize := valueLength()
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
func (b *Bucket) Lookup(key []byte) ([36]byte, error) {
	return b.binarySearch(b.Hash(key))
}

var Empty [36]byte

func (b *Bucket) binarySearch(target uint64) ([36]byte, error) {
	low := 0
	high := int(b.NumEntries)
	return searchEytzinger(low, high, target, b.loadEntry)
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

func searchEytzinger(min int, max int, x uint64, getter func(int) (Entry, error)) ([36]byte, error) {
	var index int
	for index < max {
		k, err := getter(index)
		if err != nil {
			return Empty, err
		}
		if k.Hash == x {
			return k.Value, nil
		}
		index = index<<1 | 1
		if k.Hash < x {
			index++
		}
	}
	return Empty, ErrNotFound
}
