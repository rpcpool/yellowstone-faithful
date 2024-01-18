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
package compactindexsized

// This is a fork of the original project at https://github.com/firedancer-io/radiance/tree/main/pkg/compactindex
// The following changes have been made:
// - The package has been renamed to `compactindexsized` to avoid conflicts with the original package
// - The values it indexes are N-byte values instead of 8-byte values. This allows to index CIDs (in particular sha256+CBOR CIDs), and other values, directly.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"sort"

	"github.com/cespare/xxhash/v2"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
)

// Magic are the first eight bytes of an index.
var Magic = [8]byte{'c', 'o', 'm', 'p', 'i', 's', 'z', 'd'}

const Version = uint8(1)

// Header occurs once at the beginning of the index.
type Header struct {
	ValueSize  uint64
	NumBuckets uint32
	Metadata   *indexmeta.Meta
}

// Load checks the Magic sequence and loads the header fields.
func (h *Header) Load(buf []byte) error {
	// Use a magic byte sequence to bail fast when user passes a corrupted/unrelated stream.
	if *(*[8]byte)(buf[:8]) != Magic {
		return fmt.Errorf("not a radiance compactindex file")
	}
	// read length of the rest of the header
	lenWithoutMagicAndLen := binary.LittleEndian.Uint32(buf[8:12])
	if lenWithoutMagicAndLen < 12 {
		return fmt.Errorf("invalid header length")
	}
	if lenWithoutMagicAndLen > uint32(len(buf)) {
		return fmt.Errorf("invalid header length")
	}
	// read the rest of the header
	*h = Header{
		ValueSize:  binary.LittleEndian.Uint64(buf[12:20]),
		NumBuckets: binary.LittleEndian.Uint32(buf[20:24]),
		Metadata:   new(indexmeta.Meta),
	}
	// Check version.
	if buf[24] != Version {
		return fmt.Errorf("unsupported index version: want %d, got %d", Version, buf[20])
	}
	// read key-value pairs
	if err := h.Metadata.UnmarshalBinary(buf[25:]); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	if h.ValueSize == 0 {
		return fmt.Errorf("value size not set")
	}
	if h.NumBuckets == 0 {
		return fmt.Errorf("number of buckets not set")
	}
	return nil
}

func (h *Header) Bytes() []byte {
	buf := new(bytes.Buffer)
	{
		// value size
		binary.Write(buf, binary.LittleEndian, h.ValueSize)
		// number of buckets
		binary.Write(buf, binary.LittleEndian, h.NumBuckets)
		// version
		buf.WriteByte(Version)
		// key-value pairs
		if h.Metadata == nil {
			h.Metadata = new(indexmeta.Meta)
		}
		kvb := h.Metadata.Bytes()
		buf.Write(kvb)
	}
	lenWithoutMagicAndLen := buf.Len()

	finalBuf := new(bytes.Buffer)
	finalBuf.Write(Magic[:])                                                   // magic
	binary.Write(finalBuf, binary.LittleEndian, uint32(lenWithoutMagicAndLen)) // length of the rest of the header
	finalBuf.Write(buf.Bytes())                                                // the rest of the header
	return finalBuf.Bytes()
}

// BucketHash returns the bucket index for the given key.
//
// Uses a truncated xxHash64 rotated until the result fits.
func (h *Header) BucketHash(key []byte) uint {
	u := xxhash.Sum64(key)
	n := uint64(h.NumBuckets)
	r := (-n) % n
	for u < r {
		u = hashUint64(u)
	}
	return uint(u % n)
}

// hashUint64 is a reversible uint64 permutation based on Google's
// Murmur3 hash finalizer (public domain)
func hashUint64(x uint64) uint64 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}

// BucketHeader occurs at the beginning of each bucket.
type BucketHeader struct {
	HashDomain uint32
	NumEntries uint32
	HashLen    uint8
	FileOffset uint64
	headerSize int64
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
	e.Value = make([]byte, b.OffsetWidth)
	copy(e.Value[:], buf[b.HashLen:b.HashLen+b.OffsetWidth])
	return
}

func (b *BucketDescriptor) marshalEntry(buf []byte, e Entry) {
	if len(buf) < int(b.Stride) {
		panic("serializeEntry: buf too small")
	}
	putUintLe(buf[0:b.HashLen], e.Hash)
	copy(buf[b.HashLen:b.HashLen+b.OffsetWidth], e.Value[:])
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
	Value []byte
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
func putUintLe(buf []byte, x uint64) {
	var full [8]byte
	binary.LittleEndian.PutUint64(full[:], x)
	copy(buf, full[:])
}
