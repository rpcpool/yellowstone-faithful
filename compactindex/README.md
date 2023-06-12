# a fast flat-file index for constant datasets

This package specifies a file format and Go implementation for indexing constant datasets.

*`compactindex` …*
- is an immutable file format;
- maps arbitrary keys into offsets in an external flat file;
- consumes a constant amount of space per entry
  - ~6-8 bytes, regardless of key size
  - 3 bytes per enty 
- `O(1)` complexity queries, with `2 + log2(10000)` lookups worst- & average-case (binary search);
- during construction, requires near-constant memory space and `O(n)` scratch space with regard to entries per file;
- during construction, features a constant >500k entry/s per-core write rate (2.5 GHz Intel laptop);
- works on any storage supporting random reads (regular files, HTTP range requests, on-chain, ...);
- is based on the "FKS method" which uses perfect (collision-free) hash functions in a two-level hashtable; [^1]
- is inspired by D. J. Bernstein's "constant database"; [^2]
- uses the xxHash64 non-cryptographic hash-function; [^3]

Refer to the Go documentation for the algorithms used and implementation details.

[![Go Reference](https://pkg.go.dev/badge/go.firedancer.io/radiance/pkg/compactindex.svg)](https://pkg.go.dev/go.firedancer.io/radiance/pkg/compactindex)

[^1]: Fredman, M. L., Komlós, J., & Szemerédi, E. (1984). Storing a Sparse Table with 0 (1) Worst Case Access Time. Journal of the ACM, 31(3), 538–544. https://doi.org/10.1145/828.1884
[^2]: cdb by D. J. Bernstein https://cr.yp.to/cdb.html
[^3]: Go implementation of xxHash by @cespare: https://github.com/cespare/xxhash/

## Interface

In programming terms:

```rs
fn lookup(key: &[byte]) -> Option<u64>
```

Given an arbitrary key, the index
- states whether the key exists in the index
- if it exists, maps the key to an integer (usually an offset into a file)

## Examples

Here are some example scenarios where `compactindex` is useful:

- When working with immutable data structures
  - Example: Indexing [IPLD CAR files][3] carrying Merkle-DAGs of content-addressable data
- When working with archived/constant data
  - Example: Indexing files in `.tar` archives
- When dealing with immutable remote storage such as S3-like object storage
  - Example: Storing the index and target file in S3, then using [HTTP range requests][4] to efficiently query data

[3]: https://ipld.io/specs/transport/car/
[4]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Range_requests

Here are some things compactindex cannot do:

- Cannot add more entries to an existing index
  - Reason 1: indexes are tightly packed, so there is no space to insert new entries (though `fallocate(2)` with `FALLOC_FL_INSERT_RANGE` would technically work)
  - Reason 2: the second-level hashtable uses a perfect hash function ensuring collision-free indexing of a subset of entries;
    inserting new entries might cause a collision requiring 
  - Reason 3: adding too many entries will eventually create an imbalance in the first-level hashtable;
    fixing this imbalance effectively requires re-constructing the file from scratch
- Cannot iterate over keys
  - Reason: compactindex stores hashes, not the entries themselves.
    This saves space but also allows for efficient random reads used during binary search

## File Format (v0)

**Encoding**

The file format contains binary packed structures with byte alignment.

Integers are encoded as little endian.

**File Header**

The file beings with a 32 byte file header.

```rust
#[repr(packed)]
struct FileHeader {
    magic: [u8; 8],       // 0x00
    max_value: u64,       // 0x08
    num_buckets: u32,     // 0x10
    padding_14: [u8; 12], // 0x14
}
```

- `magic` is set to the UTF-8 string `"rdcecidx"`.
  The reader should reject files that don't start with this string.
- `num_buckets` is set to the number of hashtable buckets.
- `max_value` indicates the integer width of index values.
- `padding_14` must be zero. (reserved for future use)

**Bucket Header Table**

The file header is followed by a vector of bucket headers.
The number of is set by `num_buckets` in the file header.

Each bucket header is 16 bytes long.

```rust
#[repr(packed)]
struct BucketHeader {
    hash_domain: u32, // 0x00
    num_entries: u32, // 0x04
    hash_len: u8,     // 0x08
    padding_09: u8,   // 0x09
    file_offset: u48, // 0x10
}
```

- `hash_domain` is a "salt" to the per-bucket hash function.
- `num_entries` is set to the number of records in the bucket.
- `hash_len` is the size of the per-record hash in bytes and currently hardcoded to `3`.
- `padding_09` must be zero.
- `file_offset` is an offset from the beginning of the file header to the start of the bucket entries.

**Bucket Entry Table**

Each bucket has a vector of entries with length `num_entries`.
This structure makes up the vast majority of the index.

```rust
#[repr(packed)]
struct Entry {
    hash: u??,
    value: u??,
}
```

The size of entry is static within a bucket. It is determined by its components:
- The size of `hash` in bytes equals `hash_len`
- The size of `value` in bytes equals the byte aligned integer width that is minimally required to represent `max_value`
