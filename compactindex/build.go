//go:build unix

package compactindex

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// Builder creates new compactindex files.
type Builder struct {
	Header
	buckets []tempBucket
	dir     string
}

// NewBuilder creates a new index builder.
//
// If dir is an empty string, a random temporary directory is used.
//
// numItems refers to the number of items in the index.
//
// targetFileSize is the size of the file that index entries point to.
// Can be set to zero if unknown, which results in a less efficient (larger) index.
func NewBuilder(dir string, numItems uint, targetFileSize uint64) (*Builder, error) {
	if dir == "" {
		var err error
		dir, err = os.MkdirTemp("", "compactindex-")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
	}
	if targetFileSize == 0 {
		targetFileSize = math.MaxUint64
	}

	numBuckets := (numItems + targetEntriesPerBucket - 1) / targetEntriesPerBucket
	buckets := make([]tempBucket, numBuckets)
	for i := range buckets {
		name := filepath.Join(dir, fmt.Sprintf("keys-%d", i))
		f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			return nil, err
		}
		buckets[i].file = f
		buckets[i].writer = bufio.NewWriter(f)
	}

	return &Builder{
		Header: Header{
			FileSize:   targetFileSize,
			NumBuckets: uint32(numBuckets),
		},
		buckets: buckets,
		dir:     dir,
	}, nil
}

// Insert writes a key-value mapping to the index.
//
// Index generation will fail if the same key is inserted twice.
// The writer must not pass a value greater than targetFileSize.
func (b *Builder) Insert(key []byte, value uint64) error {
	return b.buckets[b.Header.BucketHash(key)].writeTuple(key, value)
}

// Seal writes the final index to the provided file.
// This process is CPU-intensive, use context to abort prematurely.
//
// The file should be opened with access mode os.O_RDWR.
// Passing a non-empty file will result in a corrupted index.
func (b *Builder) Seal(ctx context.Context, f *os.File) (err error) {
	// TODO support in-place writing.

	// Write header.
	var headerBuf [headerSize]byte
	b.Header.Store(&headerBuf)
	_, err = f.Write(headerBuf[:])
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	// Create hole to leave space for bucket header table.
	bucketTableLen := int64(b.NumBuckets) * bucketHdrLen
	err = fallocate(f, headerSize, bucketTableLen)
	if err != nil {
		return fmt.Errorf("failed to fallocate() bucket table: %w", err)
	}
	// Seal each bucket.
	for i := range b.buckets {
		if err := b.sealBucket(ctx, i, f); err != nil {
			return err
		}
	}
	return nil
}

// sealBucket will mine a bucket hashtable, write entries to a file, a
func (b *Builder) sealBucket(ctx context.Context, i int, f *os.File) error {
	// Produce perfect hash table for bucket.
	bucket := &b.buckets[i]
	if err := bucket.flush(); err != nil {
		return err
	}
	const mineAttempts uint32 = 1000
	entries, domain, err := bucket.mine(ctx, mineAttempts)
	if err != nil {
		return fmt.Errorf("failed to mine bucket %d: %w", i, err)
	}
	// Find current file length.
	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to EOF: %w", err)
	}
	if offset < 0 {
		panic("os.File.Seek() < 0")
	}
	// Write header to file.
	desc := BucketDescriptor{
		BucketHeader: BucketHeader{
			HashDomain: domain,
			NumEntries: uint32(bucket.records),
			HashLen:    3, // TODO remove hardcoded constant
			FileOffset: uint64(offset),
		},
		Stride:      3 + intWidth(b.FileSize), // TODO remove hardcoded constant
		OffsetWidth: intWidth(b.FileSize),
	}
	// Write entries to file.
	wr := bufio.NewWriter(f)
	entryBuf := make([]byte, desc.HashLen+intWidth(b.FileSize)) // TODO remove hardcoded constant
	for _, entry := range entries {
		desc.marshalEntry(entryBuf, entry)
		if _, err := wr.Write(entryBuf[:]); err != nil {
			return fmt.Errorf("failed to write record to index: %w", err)
		}
	}
	if err := wr.Flush(); err != nil {
		return fmt.Errorf("failed to flush bucket to index: %w", err)
	}
	// Write header to file.
	if err := desc.BucketHeader.writeTo(f, uint(i)); err != nil {
		return fmt.Errorf("failed to write bucket header %d: %w", i, err)
	}
	return nil
}

func (b *Builder) Close() error {
	return os.RemoveAll(b.dir)
}

// tempBucket represents the "temporary bucket" file,
// a disk buffer containing a vector of key-value-tuples.
type tempBucket struct {
	records uint
	file    *os.File
	writer  *bufio.Writer
}

// writeTuple performs a buffered write of a KV-tuple.
func (b *tempBucket) writeTuple(key []byte, value uint64) (err error) {
	b.records++
	var static [10]byte
	binary.LittleEndian.PutUint16(static[0:2], uint16(len(key)))
	binary.LittleEndian.PutUint64(static[2:10], value)
	if _, err = b.writer.Write(static[:]); err != nil {
		return err
	}
	_, err = b.writer.Write(key)
	return
}

// flush empties the in-memory write buffer to the file.
func (b *tempBucket) flush() error {
	if err := b.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	b.writer = nil
	return nil
}

// mine repeatedly hashes the set of entries with different nonces.
//
// Returns a sorted list of hashtable entries upon finding a set of hashes without collisions.
// If a number of attempts was made without success, returns ErrCollision instead.
func (b *tempBucket) mine(ctx context.Context, attempts uint32) (entries []Entry, domain uint32, err error) {
	entries = make([]Entry, b.records)
	bitmap := make([]byte, 1<<21)

	rd := bufio.NewReader(b.file)
	for domain = uint32(0); domain < attempts; domain++ {
		if err = ctx.Err(); err != nil {
			return
		}
		// Reset bitmap
		for i := range bitmap {
			bitmap[i] = 0
		}
		// Reset reader
		if _, err = b.file.Seek(0, io.SeekStart); err != nil {
			return
		}
		rd.Reset(b.file)

		if hashErr := hashBucket(rd, entries, bitmap, domain); errors.Is(hashErr, ErrCollision) {
			continue
		} else if hashErr != nil {
			return nil, 0, hashErr
		}

		return // ok
	}

	return nil, domain, ErrCollision
}

// hashBucket reads and hashes entries from a temporary bucket file.
//
// Uses a 2^24 wide bitmap to detect collisions.
func hashBucket(rd *bufio.Reader, entries []Entry, bitmap []byte, nonce uint32) error {
	// TODO Don't hardcode this, choose hash depth dynamically
	mask := uint64(0xffffff)

	// Scan provided reader for entries and hash along the way.
	for i := range entries {
		// Read next key from file (as defined by writeTuple)
		var static [10]byte
		if _, err := io.ReadFull(rd, static[:]); err != nil {
			return err
		}
		keyLen := binary.LittleEndian.Uint16(static[0:2])
		value := binary.LittleEndian.Uint64(static[2:10])
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(rd, key); err != nil {
			return err
		}

		// Hash to entry
		hash := EntryHash64(nonce, key) & mask

		// Check for collision in bitmap
		bi, bj := hash/8, hash%8
		chunk := bitmap[bi]
		if (chunk>>bj)&1 == 1 {
			return ErrCollision
		}
		bitmap[bi] = chunk | (1 << bj)

		// Export entry
		entries[i] = Entry{
			Hash:  hash,
			Value: value,
		}
	}

	// Sort entries.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Hash < entries[j].Hash
	})

	return nil
}

var ErrCollision = errors.New("hash collision")
