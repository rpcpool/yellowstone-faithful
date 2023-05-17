// Package compactindex is an immutable hashtable index format inspired by djb's constant database (cdb).
//
// # Design
//
// Compactindex is used to create secondary indexes over arbitrary flat files.
// Each index is a single, immutable flat file.
//
// Index files consist of a space-optimized and query-optimized key-value-like table.
//
// Instead of storing actual keys, the format stores FKS dynamic perfect hashes.
// And instead of storing values, the format contains offsets into some file.
//
// As a result, the database effectively only supports two operations, similarly to cdb.
// (Note that the actual Go interface is a bit more flexible).
//
//	func Create(kv map[[]byte]uint64) *Index
//	func (*Index) Lookup(key []byte) (value uint64, exist bool)
//
// # Buckets
//
// The set of items is split into buckets of approx 10000 records.
// The number of buckets is unlimited.
//
// The key-to-bucket assignment is determined by xxHash3 using uniform discrete hashing over the key space.
//
// The index file header also mentions the number of buckets and the file offset of each bucket.
//
// # Tables
//
// Each bucket contains a table of entries, indexed by a collision-free hash function.
//
// The hash function used in the entry table is xxHash.
// A 32-bit hash domain is prefixed to mine collision-free sets of hashes (FKS scheme).
// This hash domain is also recorded at the bucket header.
//
// Each bucket entry is a constant-size record consisting of a 3-byte hash and an offset to the value.
// The size of the offset integer is the minimal byte-aligned integer width that can represent the target file size.
//
// # Querying
//
// The query interface (DB) is backend-agnostic, supporting any storage medium that provides random reads.
// To name a few: Memory buffers, local files, arbitrary embedded buffers, HTTP range requests, plan9, etc...
//
// The DB struct itself performs zero memory allocations and therefore also doesn't cache.
// It is therefore recommended to provide a io.ReaderAt backed by a cache to improve performance.
//
// Given a key, the query strategy is simple:
//
//  1. Hash key to bucket using global hash function
//  2. Retrieve bucket offset from bucket header table
//  3. Hash key to entry using per-bucket hash function
//  4. Search for entry in bucket (binary search)
//
// The search strategy for locating entries in buckets can be adjusted to fit the latency/bandwidth profile of the underlying storage medium.
//
// For example, the fastest lookup strategy in memory is a binary search retrieving double cache lines at a time.
// When doing range requests against high-latency remote storage (e.g. S3 buckets),
// it is typically faster to retrieve and scan through large parts of a bucket (multiple kilobytes) at once.
//
// # Construction
//
// Constructing a compactindex requires upfront knowledge of the number of items and highest possible target offset (read: target file size).
//
// The process requires scratch space of around 16 bytes per entry. During generation, data is offloaded to disk for memory efficiency.
//
// The process works as follows:
//
//  1. Determine number of buckets and offset integer width
//     based on known input params (item count and target file size).
//  2. Linear pass over input data, populating temporary files that
//     contain the unsorted entries of each bucket.
//  3. For each bucket, brute force a perfect hash function that
//     defines a bijection between hash values and keys in the bucket.
//  4. For each bucket, sort by hash values.
//  5. Store to index.
//
// An alternative construction approach is available when the number of items or target file size is unknown.
// In this case, a set of keys is first serialized to a flat file.
package compactindex

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"sort"

	"github.com/cespare/xxhash/v2"
)

// Magic are the first eight bytes of an index.
var Magic = [8]byte{'r', 'd', 'c', 'e', 'c', 'i', 'd', 'x'}

// Header occurs once at the beginning of the index.
type Header struct {
	FileSize   uint64
	NumBuckets uint32
}

// headerSize is the size of the header at the beginning of the file.
const headerSize = 32

// Load checks the Magic sequence and loads the header fields.
func (h *Header) Load(buf *[headerSize]byte) error {
	// Use a magic byte sequence to bail fast when user passes a corrupted/unrelated stream.
	if *(*[8]byte)(buf[:8]) != Magic {
		return fmt.Errorf("not a radiance compactindex file")
	}
	*h = Header{
		FileSize:   binary.LittleEndian.Uint64(buf[8:16]),
		NumBuckets: binary.LittleEndian.Uint32(buf[16:20]),
	}
	// 12 bytes to spare for now. Might use it in the future.
	// Force to zero for now.
	for _, b := range buf[20:32] {
		if b != 0x00 {
			return fmt.Errorf("unsupported index version")
		}
	}
	return nil
}

func (h *Header) Store(buf *[headerSize]byte) {
	copy(buf[0:8], Magic[:])
	binary.LittleEndian.PutUint64(buf[8:16], h.FileSize)
	binary.LittleEndian.PutUint32(buf[16:20], h.NumBuckets)
	for i := 20; i < 32; i++ {
		buf[i] = 0
	}
}

// BucketHash returns the bucket index for the given key.
//
// Uses a truncated xxHash64 rotated until the result fits.
func (h *Header) BucketHash(key []byte) uint {
	xsum := xxhash.Sum64(key)
	mask := maxCls64(uint64(h.NumBuckets))
	for {
		index := xsum & mask
		if index < uint64(h.NumBuckets) {
			return uint(index)
		}
		xsum = bits.RotateLeft64(xsum, bits.LeadingZeros64(mask))
	}
}

// BucketHeader occurs at the beginning of each bucket.
type BucketHeader struct {
	HashDomain uint32
	NumEntries uint32
	HashLen    uint8
	FileOffset uint64
}

// bucketHdrLen is the size of the header preceding the hash table entries.
const bucketHdrLen = 16

func (b *BucketHeader) Store(buf *[bucketHdrLen]byte) {
	binary.LittleEndian.PutUint32(buf[0:4], b.HashDomain)
	binary.LittleEndian.PutUint32(buf[4:8], b.NumEntries)
	buf[8] = b.HashLen
	buf[9] = 0
	putUintLe(buf[10:16], b.FileOffset)
}

func (b *BucketHeader) Load(buf *[bucketHdrLen]byte) {
	b.HashDomain = binary.LittleEndian.Uint32(buf[0:4])
	b.NumEntries = binary.LittleEndian.Uint32(buf[4:8])
	b.HashLen = buf[8]
	b.FileOffset = uintLe(buf[10:16])
}

// Hash returns the per-bucket hash of a key.
func (b *BucketHeader) Hash(key []byte) uint64 {
	xsum := EntryHash64(b.HashDomain, key)
	// Mask sum by hash length.
	return xsum & (math.MaxUint64 >> (64 - b.HashLen*8))
}

type BucketDescriptor struct {
	BucketHeader
	Stride      uint8 // size of one entry in bucket
	OffsetWidth uint8 // with of offset field in bucket
}

func (b *BucketDescriptor) unmarshalEntry(buf []byte) (e Entry) {
	e.Hash = uintLe(buf[0:b.HashLen])
	e.Value = uintLe(buf[b.HashLen : b.HashLen+b.OffsetWidth])
	return
}

func (b *BucketDescriptor) marshalEntry(buf []byte, e Entry) {
	if len(buf) < int(b.Stride) {
		panic("serializeEntry: buf too small")
	}
	putUintLe(buf[0:b.HashLen], e.Hash)
	putUintLe(buf[b.HashLen:b.HashLen+b.OffsetWidth], e.Value)
}

// SearchSortedEntries performs an in-memory binary search for a given hash.
func SearchSortedEntries(entries []Entry, hash uint64) *Entry {
	i, found := sort.Find(len(entries), func(i int) int {
		other := entries[i].Hash
		// Note: This is safe because neither side exceeds 2^24.
		return int(hash) - int(other)
	})
	if !found {
		return nil
	}
	if i >= len(entries) || entries[i].Hash != hash {
		return nil
	}
	return &entries[i]
}

// EntryHash64 is a xxHash-based hash function using an arbitrary prefix.
func EntryHash64(prefix uint32, key []byte) uint64 {
	const blockSize = 32
	var prefixBlock [blockSize]byte
	binary.LittleEndian.PutUint32(prefixBlock[:4], prefix)

	var digest xxhash.Digest
	digest.Reset()
	digest.Write(prefixBlock[:])
	digest.Write(key)
	return digest.Sum64()
}

// Entry is a single element in a hash table.
type Entry struct {
	Hash  uint64
	Value uint64
}

// intWidth returns the number of bytes minimally required to represent the given integer.
func intWidth(n uint64) uint8 {
	msb := 64 - bits.LeadingZeros64(n)
	return uint8((msb + 7) / 8)
}

// maxCls64 returns the max integer that has the same amount of leading zeros as n.
func maxCls64(n uint64) uint64 {
	return math.MaxUint64 >> bits.LeadingZeros64(n)
}

// uintLe decodes an unsigned little-endian integer without bounds assertions.
// out-of-bounds bits are set to zero.
func uintLe(buf []byte) uint64 {
	var full [8]byte
	copy(full[:], buf)
	return binary.LittleEndian.Uint64(full[:])
}

// putUintLe encodes an unsigned little-endian integer without bounds assertions.
// Returns true if the integer fully fit in the provided buffer.
func putUintLe(buf []byte, x uint64) bool {
	var full [8]byte
	binary.LittleEndian.PutUint64(full[:], x)
	copy(buf, full[:])
	return int(intWidth(x)) <= len(buf)
}
