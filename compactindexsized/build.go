package compactindexsized

// This is a fork of the original project at https://github.com/firedancer-io/radiance/tree/main/pkg/compactindex
// The following changes have been made:
// - The package has been renamed to `compactindexsized` to avoid conflicts with the original package
// - The values it indexes are N-byte values instead of 8-byte values. This allows to index CIDs (in particular sha256+CBOR CIDs), and other values, directly.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"syscall"

	"github.com/rpcpool/yellowstone-faithful/continuity"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
)

// Builder creates new compactindex files.
type Builder struct {
	Header     Header
	tmpDir     string
	headerSize int64
	closers    []io.Closer
	buckets    []tempBucket
}

// NewBuilderSized creates a new index builder.
//
// If dir is an empty string, a random temporary directory is used.
//
// numItems refers to the number of items in the index.
//
// valueSize is the size of each value in bytes. It must be > 0 and <= 256.
// All values must be of the same size.
func NewBuilderSized(
	tmpDir string,
	numItems uint, // NOTE: if `numItems` is not set correctly, either there will be collisions, or the buckets will be almost empty.
	valueSizeBytes uint,
) (*Builder, error) {
	if tmpDir == "" {
		var err error
		tmpDir, err = os.MkdirTemp("", "compactindex-")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
	}
	if valueSizeBytes == 0 {
		return nil, fmt.Errorf("valueSizeBytes must be > 0")
	}
	if valueSizeBytes > 255 {
		return nil, fmt.Errorf("valueSizeBytes must be <= 255")
	}
	if numItems == 0 {
		return nil, fmt.Errorf("numItems must be > 0")
	}

	numBuckets := (numItems + targetEntriesPerBucket - 1) / targetEntriesPerBucket
	buckets := make([]tempBucket, numBuckets)
	closers := make([]io.Closer, 0, numBuckets)
	for i := range buckets {
		name := filepath.Join(tmpDir, fmt.Sprintf("keys-%d", i))
		f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o666)
		if err != nil {
			for _, c := range closers {
				c.Close()
			}
			return nil, err
		}
		closers = append(closers, f)
		buckets[i].kv = newFileKV(f, valueSizeBytes)
		buckets[i].valueSize = uint(valueSizeBytes)
	}

	return &Builder{
		Header: Header{
			ValueSize:  uint64(valueSizeBytes),
			NumBuckets: uint32(numBuckets),
			Metadata:   &indexmeta.Meta{},
		},
		closers: closers,
		buckets: buckets,
		tmpDir:  tmpDir,
	}, nil
}

type kvRW interface {
	writeTuple(key []byte, value []byte) error
	readAll() ([]keyval, error)
}

var (
	_ kvRW = (*inMemoryKV)(nil)
	_ kvRW = (*fileKV)(nil)
)

type inMemoryKV struct {
	kv []keyval
}

func newInMemoryKV() *inMemoryKV {
	return &inMemoryKV{
		kv: make([]keyval, 0),
	}
}

func (m *inMemoryKV) writeTuple(key []byte, value []byte) error {
	m.kv = append(m.kv, newkv(key, value))
	return nil
}

func (m *inMemoryKV) readAll() ([]keyval, error) {
	return m.kv, nil
}

type fileKV struct {
	valueSize uint
	file      *os.File
	writer    *bufio.Writer
}

func newFileKV(file *os.File, valueSize uint) *fileKV {
	bufSize := 1024 * 8
	writer := bufio.NewWriterSize(file, bufSize)
	return &fileKV{
		valueSize: valueSize,
		file:      file,
		writer:    writer,
	}
}

func (b *fileKV) writeTuple(key []byte, value []byte) error {
	static := make([]byte, 2+b.valueSize)
	binary.LittleEndian.PutUint16(static[0:2], uint16(len(key)))
	copy(static[2:], value[:])
	if _, err := b.writer.Write(static[:]); err != nil {
		return err
	}
	_, err := b.writer.Write(key)
	return err
}

func (b *fileKV) readAll() ([]keyval, error) {
	// flush writer
	if err := b.writer.Flush(); err != nil {
		return nil, err
	}
	b.writer = nil
	_, err := b.file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	kv := make([]keyval, 0)
	reader := bufio.NewReader(b.file)
	static := make([]byte, 2+b.valueSize)
	for i := 0; ; i++ {
		_, err := io.ReadFull(reader, static[:])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		keyLen := binary.LittleEndian.Uint16(static[0:2])
		value := make([]byte, b.valueSize)
		copy(value[:], static[2:])
		key := make([]byte, keyLen)
		_, err = io.ReadFull(reader, key)
		if err != nil {
			return nil, err
		}
		kv = append(kv, newkv(key, value))
	}
	return kv, nil
}

// SetKind sets the kind of the index.
// If the kind is already set, it is overwritten.
func (b *Builder) SetKind(kind []byte) error {
	// check if kind is too long
	if len(kind) > indexmeta.MaxKeySize {
		return fmt.Errorf("kind is too long")
	}
	// check if kind is empty
	if len(kind) == 0 {
		return fmt.Errorf("kind is empty")
	}
	// check if kind is already set
	if b.Header.Metadata.Count(indexmeta.MetadataKey_Kind) > 0 {
		// remove kind
		b.Header.Metadata.Remove(indexmeta.MetadataKey_Kind)
	}
	// set kind
	b.Header.Metadata.Add(indexmeta.MetadataKey_Kind, kind)
	return nil
}

func (b *Builder) Metadata() *indexmeta.Meta {
	return b.Header.Metadata
}

func (b *Builder) getValueSize() int {
	return int(b.Header.ValueSize)
}

// Insert writes a key-value mapping to the index.
//
// Index generation will fail if the same key is inserted twice.
// The writer must not pass a value greater than targetFileSize.
func (b *Builder) Insert(key []byte, value []byte) error {
	return b.buckets[b.Header.BucketHash(key)].writeTuple(key, value)
}

// SealAndClose writes the final index to the provided file.
// This process is CPU-intensive, use context to abort prematurely.
//
// The file should be opened with access mode os.O_RDWR.
// Passing a non-empty file will result in a corrupted index.
func (b *Builder) SealAndClose(ctx context.Context, file *os.File) (err error) {
	// TODO support in-place writing.

	// Write header.
	headerBuf := b.Header.Bytes()
	headerSize := int64(len(headerBuf))
	numWroteHeader, err := file.Write(headerBuf[:])
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if numWroteHeader != len(headerBuf) {
		return fmt.Errorf("failed to write header: wrote %d bytes, expected %d", numWroteHeader, len(headerBuf))
	}
	b.headerSize = headerSize
	// Create hole to leave space for bucket header table.
	bucketTableLen := int64(b.Header.NumBuckets) * bucketHdrLen
	err = fallocate(file, headerSize, bucketTableLen)
	if errors.Is(err, syscall.EOPNOTSUPP) {
		// The underlying file system may not support fallocate
		err = fake_fallocate(file, headerSize, bucketTableLen)
		if err != nil {
			return fmt.Errorf("failed to fake fallocate() bucket table: %w", err)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to fallocate() bucket table: %w", err)
	}
	// Seal each bucket.
	for i := range b.buckets {
		if err := b.sealBucket(ctx, i, file); err != nil {
			return fmt.Errorf("failed to seal bucket %d: %w", i, err)
		}
	}
	return continuity.New().
		Thenf("sync", func() error {
			if err := file.Sync(); err != nil {
				return fmt.Errorf("failed to sync file: %w", err)
			}
			return nil
		}).
		Thenf("close", func() error {
			if err := b.close(); err != nil {
				return fmt.Errorf("failed to close index: %w", err)
			}
			return nil
		}).
		Err()
}

// sealBucket will mine a bucket hashtable, write entries to a file, a
func (b *Builder) sealBucket(ctx context.Context, i int, f *os.File) error {
	// Produce perfect hash table for bucket.
	bucket := &b.buckets[i]
	if err := bucket.flush(); err != nil {
		return fmt.Errorf("failed to flush bucket %d: %w", i, err)
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
			HashLen:    HashSize,
			FileOffset: uint64(offset),
		},
		Stride:      b.getEntryStride(),
		OffsetWidth: uint8(b.getValueSize()),
	}
	desc.BucketHeader.headerSize = b.headerSize
	// Write entries to file.
	wr := bufio.NewWriter(f)
	entryBuf := make([]byte, b.getEntryStride()) // TODO remove hardcoded constant
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

func (b *Builder) getEntryStride() uint8 {
	offsetSize := b.getValueSize()
	return uint8(HashSize) + uint8(offsetSize)
}

func (b *Builder) close() error {
	for _, c := range b.closers {
		c.Close()
	}
	return os.RemoveAll(b.tmpDir)
}

// tempBucket represents the "temporary bucket" file,
// a disk buffer containing a vector of key-value-tuples.
type tempBucket struct {
	records   uint
	valueSize uint
	kv        kvRW
}

type keyval struct {
	key   []byte
	value []byte
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	if len(b) == 0 {
		return []byte{}
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func newkv(k, v []byte) keyval {
	return keyval{
		key:   cloneBytes(k),
		value: cloneBytes(v),
	}
}

// writeTuple performs a buffered write of a KV-tuple.
func (b *tempBucket) writeTuple(key []byte, value []byte) (err error) {
	b.records++
	return b.kv.writeTuple(key, value)
}

// flush empties the in-memory write buffer to the file.
func (b *tempBucket) flush() error {
	return nil
}

// mine repeatedly hashes the set of entries with different nonces.
//
// Returns a sorted list of hashtable entries upon finding a set of hashes without collisions.
// If a number of attempts was made without success, returns ErrCollision instead.
func (b *tempBucket) mine(ctx context.Context, attempts uint32) (entries []Entry, domain uint32, err error) {
	entries = make([]Entry, b.records)
	bitmap := make([]byte, 1<<21)

	kv, err := b.kv.readAll()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read all entries: %w", err)
	}
	dedupKeepNewest(kv)

	for domain = uint32(0); domain < attempts; domain++ {
		if err = ctx.Err(); err != nil {
			return
		}
		// Reset bitmap
		for i := range bitmap {
			bitmap[i] = 0
		}

		if hashErr := hashBucket(b.valueSize, kv, entries, bitmap, domain); errors.Is(hashErr, ErrCollision) {
			continue
		} else if hashErr != nil {
			return nil, 0, hashErr
		}

		return // ok
	}

	return nil, domain, ErrCollision
}

func dedupKeepNewest(kv []keyval) {
	// reverse, so that newer entries are first
	slices.Reverse(kv)
	// dedup, keeping the first entry
	kv = slices.CompactFunc(kv, func(i keyval, j keyval) bool {
		return bytes.Equal(i.key, j.key)
	})
}

var ErrCollision = errors.New("hash collision")

// hashBucket reads and hashes entries from a temporary bucket file.
//
// Uses a 2^24 wide bitmap to detect collisions.
func hashBucket(
	valueSize uint,
	kv []keyval,
	entries []Entry,
	bitmap []byte,
	nonce uint32,
) error {
	// TODO Don't hardcode this, choose hash depth dynamically
	mask := uint64(0xffffff)

	// Scan provided reader for entries and hash along the way.
	for i := range entries {
		key := kv[i].key
		value := kv[i].value

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
	sortWithCompare(entries, func(i, j int) bool {
		return entries[i].Hash < entries[j].Hash
	})

	return nil
}

func sortWithCompare[T any](a []T, compare func(i, j int) bool) {
	sort.Slice(a, compare)
	sorted := make([]T, len(a))
	eytzinger(a, sorted, 0, 1)
	copy(a, sorted)
}

func eytzinger[T any](in, out []T, i, k int) int {
	if k <= len(in) {
		i = eytzinger(in, out, i, 2*k)
		out[k-1] = in[i]
		i++
		i = eytzinger(in, out, i, 2*k+1)
	}
	return i
}
