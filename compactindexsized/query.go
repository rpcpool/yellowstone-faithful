package compactindexsized

// This is a fork of the original project at https://github.com/firedancer-io/radiance/tree/main/pkg/compactindex
// The following changes have been made:
// - The package has been renamed to `compactindexsized` to avoid conflicts with the original package
// - The values it indexes are N-byte values instead of 8-byte values. This allows to index CIDs (in particular sha256+CBOR CIDs), and other values, directly.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/valyala/bytebufferpool"
	"golang.org/x/sys/unix"
)

// DB is a compactindex handle.
type DB struct {
	Header     *Header
	headerSize int64
	Stream     io.ReaderAt
}

var ErrInvalidMagic = errors.New("invalid magic")

// Open returns a handle to access a compactindex.
//
// The provided stream must start with the Magic byte sequence.
// Tip: Use io.NewSectionReader to create aligned substreams when dealing with a file that contains multiple indexes.
func Open(stream io.ReaderAt) (*DB, error) {
	type fileDescriptor interface {
		Fd() uintptr
		Name() string
	}
	if f, ok := stream.(fileDescriptor); ok {
		// fadvise random access pattern for the whole file
		err := unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_RANDOM)
		if err != nil {
			slog.Warn("fadvise(RANDOM) failed", "error", err)
		}
	}
	// Read the static 32-byte header.
	// Ignore errors if the read fails after filling the buffer (e.g. EOF).
	var magicAndSize [8 + 4]byte
	n, readErr := stream.ReadAt(magicAndSize[:], 0)
	if n < len(magicAndSize) {
		// ReadAt must return non-nil error here.
		return nil, readErr
	}
	// check magic
	if !bytes.Equal(magicAndSize[:8], Magic[:]) {
		return nil, ErrInvalidMagic
	}
	size := binary.LittleEndian.Uint32(magicAndSize[8:])
	fileHeaderBuf := make([]byte, 8+4+size)
	n, readErr = stream.ReadAt(fileHeaderBuf, 0)
	if n < len(fileHeaderBuf) {
		// ReadAt must return non-nil error here.
		return nil, readErr
	}
	db := new(DB)
	db.Header = new(Header)
	db.Header.Metadata = new(indexmeta.Meta)
	if err := db.Header.Load(fileHeaderBuf); err != nil {
		return nil, err
	}
	db.headerSize = int64(8 + 4 + size)
	db.Stream = stream

	if f, ok := stream.(fileDescriptor); ok {
		{
			slog.Info("Warming up drives for bucket offsets (compactindexsized)...", "file", f.Name())
			startedWarmup := time.Now()
			dummyBuf := make([]byte, 1)
			warmedBuckets := 0
			for bucketIndex := range db.Header.NumBuckets {
				_, err := db.Stream.ReadAt(dummyBuf, bucketOffset(db.headerSize, uint(bucketIndex)))
				if err != nil {
					return nil, fmt.Errorf("failed to warm up page cache for bucket %d: %w", bucketIndex, err)
				}
				warmedBuckets++
			}
			slog.Info(
				"Cache warmup complete",
				"buckets_warmed", warmedBuckets,
				"duration", time.Since(startedWarmup).String(),
			)
		}
	}
	return db, nil
}

// GetKind returns the kind of the index.
func (db *DB) GetKind() ([]byte, bool) {
	return db.Header.Metadata.Get(indexmeta.MetadataKey_Kind)
}

// KindIs returns whether the index is of the given kind.
func (db *DB) KindIs(kind []byte) bool {
	got, ok := db.Header.Metadata.Get(indexmeta.MetadataKey_Kind)
	return ok && bytes.Equal(got, kind)
}

func (db *DB) GetValueSize() uint64 {
	value := db.Header.ValueSize
	if value == 0 {
		panic("value size not set")
	}
	return value
}

// Lookup queries for a key in the index and returns the value (offset), if any.
//
// Returns ErrNotFound if the key is unknown.
func (db *DB) Lookup(key []byte) ([]byte, error) {
	bucket, err := db.LookupBucket(key)
	if err != nil {
		return nil, err
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
			OffsetWidth: uint8(db.GetValueSize()),
		},
	}
	bucket.BucketHeader.headerSize = db.headerSize
	// Read bucket header.
	readErr := bucket.BucketHeader.readFrom(db.Stream, i)
	if readErr != nil {
		return nil, readErr
	}
	bucket.Entries = io.NewSectionReader(db.Stream, int64(bucket.FileOffset), int64(bucket.NumEntries)*int64(bucket.Stride))

	return bucket, nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

const HashSize = 3

func (db *DB) entryStride() uint8 {
	offsetSize := db.GetValueSize()
	return uint8(HashSize) + uint8(offsetSize)
}

func bucketOffset(headerSize int64, i uint) int64 {
	return headerSize + int64(i)*bucketHdrLen
}

func (b *BucketHeader) readFrom(rd io.ReaderAt, i uint) error {
	var buf [bucketHdrLen]byte
	n, err := rd.ReadAt(buf[:], bucketOffset(b.headerSize, i))
	if n < len(buf) {
		return err
	}
	b.Load(&buf)
	return nil
}

func (b *BucketHeader) writeTo(wr io.WriterAt, i uint) error {
	var buf [bucketHdrLen]byte
	b.Store(&buf)
	_, err := wr.WriteAt(buf[:], bucketOffset(b.headerSize, i))
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
func (b *Bucket) Lookup(key []byte) ([]byte, error) {
	// startedAt := time.Now()
	// Read all entries into memory in one read, then perform the search.
	// This avoids multiple small reads during the binary search.

	if b.NumEntries > maxEntriesPerBucket {
		return nil, fmt.Errorf("refusing to load bucket with %d entries for lookup", b.NumEntries)
	}

	numBytes := int64(b.NumEntries) * int64(b.Stride)
	if numBytes == 0 {
		return nil, ErrNotFound
	}

	entriesBuf := bytebufferpool.Get()
	defer bytebufferpool.Put(entriesBuf)
	entriesBuf.Reset()

	// startReadAllAt := time.Now()
	// n, err := entriesBuf.ReadFrom(b.Entries)
	entriesBuf.B = make([]byte, numBytes)
	n, err := io.ReadFull(b.Entries, entriesBuf.B[:numBytes])
	// slog.Info("Bucket.Lookup: ReadFrom took", "duration", time.Since(startReadAllAt), "bytesRead", n, "numEntries", b.NumEntries, "numBytes", numBytes, "keyHash", b.Hash(key), "totalDuration", time.Since(startedAt))

	// ReadAt must return a non-nil error if n < len(buf).
	// SectionReader will return io.EOF if it reads exactly to the end.
	// We fail if we read too few bytes, or if we got an error that wasn't io.EOF.
	if int64(n) < numBytes {
		return nil, fmt.Errorf("short read on bucket: read %d, expected %d: %w", n, numBytes, err)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("error reading bucket entries: %w", err)
	}

	// Define a getter closure that operates on the in-memory buffer.
	getter := func(i int) (Entry, error) {
		off := i * int(b.Stride)
		// Bounds check (should be guaranteed by searchEytzinger's `high` limit, but defense-in-depth)
		if off+int(b.Stride) > len(entriesBuf.B) {
			return Entry{}, fmt.Errorf("internal error: search index %d out of bounds for buffer length %d", i, len(entriesBuf.B))
		}
		entryData := entriesBuf.B[off : off+int(b.Stride)]
		return b.unmarshalEntry(entryData), nil
	}

	// Now perform the search in-memory.
	target := b.Hash(key)
	low := 0
	high := int(b.NumEntries)
	return searchEytzinger(low, high, target, getter)
}

// ErrNotFound marks a missing entry.
var ErrNotFound = errors.New("not found")

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func searchEytzinger(min int, max int, x uint64, getter func(int) (Entry, error)) ([]byte, error) {
	var index int
	for index < max {
		k, err := getter(index)
		if err != nil {
			return nil, err
		}
		if k.Hash == x {
			return k.Value, nil
		}
		index = index<<1 | 1
		if k.Hash < x {
			index++
		}
		if index < min {
			return nil, ErrNotFound
		}
	}
	return nil, ErrNotFound
}
