# rust-compactindex

Rust implementation of the compactindexsized format for yellowstone-faithful, providing high-performance access to Solana blockchain data through content-addressable storage.

## Overview

This crate provides tools for working with compact indexes that map Solana slots and transaction signatures to Content IDs (CIDs), and CIDs to their offsets and sizes in CAR files. It includes utilities for reading existing indexes, analyzing their structure, and fetching actual blockchain data from remote CAR files.

## Features

- **Compact Index Reading**: Memory-mapped, cache-efficient access to large index files
- **Multiple Index Types**: Support for slot-to-CID, signature-to-CID, and CID-to-offset mappings
- **Remote Data Fetching**: HTTP range requests to fetch specific blocks without downloading entire CAR files
- **CAR Format Parsing**: Extract raw Solana data from IPLD CAR blocks
- **Batch Processing**: Process multiple lookups efficiently via stdin
- **Index Analysis**: Tools to inspect and validate index structure and performance

## Command-Line Tools

### fetch_block - Remote Block Data Fetcher

Fetches Solana block and transaction data from remote CAR files using HTTP range requests.

#### Usage
```bash
fetch_block <lookup-index> <cid-to-offset-index> <car-url> [lookup-value]
```

#### Arguments
- `lookup-index`: Path to either a slot-to-cid or sig-to-cid index file
- `cid-to-offset-index`: Path to the cid-to-offset-and-size index file  
- `car-url`: URL of the CAR file (e.g., https://files.old-faithful.net/10/epoch-10.car)
- `lookup-value`: (Optional) The slot number or signature to fetch. If omitted, reads from stdin.

#### Examples

Fetch a single slot:
```bash
fetch_block \
  epoch-10-slot-to-cid.index \
  epoch-10-cid-to-offset.index \
  https://files.old-faithful.net/10/epoch-10.car \
  4320000
```

Batch processing (multiple slots):
```bash
echo -e "4320000\n4320001\n4320002" | fetch_block \
  epoch-10-slot-to-cid.index \
  epoch-10-cid-to-offset.index \
  https://files.old-faithful.net/10/epoch-10.car
```

Fetch by transaction signature:
```bash
fetch_block \
  epoch-10-sig-to-cid.index \
  epoch-10-cid-to-offset.index \
  https://files.old-faithful.net/10/epoch-10.car \
  [base58-signature]
```

#### Output Format
```
SLOT:4320000
[base64 encoded Solana block data]

SIGNATURE:[base58 signature]
[base64 encoded transaction data]
```

#### Performance
- Uses HTTP range requests to fetch only needed bytes (typically 3-10KB per block)
- No need to download entire CAR files (which can be hundreds of GB)
- Typical fetch time: 50-200ms per block depending on network latency

### dump_slots - Slot Mapping Extractor

Dumps slot-to-CID-to-offset mappings, showing where each slot's data is located in the CAR file.

#### Usage
```bash
dump_slots <slot-to-cid-index> <cid-to-offset-index> [start_slot] [end_slot]
```

#### Examples

Dump all slots in an epoch:
```bash
dump_slots epoch-10-slot-to-cid.index epoch-10-cid-to-offset.index
```

Dump a specific range:
```bash
dump_slots epoch-10-slot-to-cid.index epoch-10-cid-to-offset.index 4320000 4320100
```

#### Output Format
CSV format: `SLOT,CID_HEX,OFFSET,SIZE`

```
4320000,0171122017490cc7d025467d9dd0c7c1c27f53c8a8dbe089c03d52a75c2d19e59639e203,13243,3225
4320001,01711220a4019e38b12adf4ce767b638c5a6ab99aa010bc6099fe78778a63ac13b19e346,46349,3104
```

### analyze_index - Index Statistics Tool

Analyzes and reports statistics about compact index files.

#### Usage
```bash
analyze_index <index-file> [command]
```

#### Commands
- **info** (default): Show basic index information
- **buckets**: Analyze bucket distribution and load factor
- **sample**: Sample random entries from the index

#### Examples

Get basic info:
```bash
analyze_index epoch-10-slot-to-cid.index
```

Output:
```
Index Type: Slot-to-CID
Epoch: Some(10)
Number of buckets: 43
CID size: 36 bytes
Expected slot range: 4320000 - 4751999
```

Analyze bucket distribution:
```bash
analyze_index epoch-10-slot-to-cid.index buckets
```

## Library Usage

```rust
use yellowstone_compactindex::{SlotToCid, CidToOffsetAndSize};

// Open indexes
let slot_index = SlotToCid::open("slot-to-cid.index")?;
let cid_index = CidToOffsetAndSize::open("cid-to-offset.index")?;

// Lookup slot -> CID -> offset/size
let cid = slot_index.lookup(4320000)?;
let (offset, size) = cid_index.lookup(&cid)?;

println!("Slot 4320000 is at offset {} with size {} bytes", offset, size);
```

## Building

Release builds (recommended for performance):
```bash
cargo build --release --bin fetch_block
cargo build --release --bin dump_slots
cargo build --release --bin analyze_index
```

The binaries will be in `target/release/`.

## How It Works

### Data Pipeline
1. **Index Lookup**: Uses compact indexes to map slot/signature → CID → offset/size
2. **HTTP Range Request**: Fetches only the specific byte range from the CAR file
3. **CAR Format Parsing**: 
   - Reads the varint length prefix
   - Skips the CID bytes
   - Extracts the raw block data
4. **Output**: Encodes the raw data as base64 for safe text transmission

### Index Structure
The indexes use a bucket-based hash table with Eytzinger-ordered binary search within each bucket:
- Fast lookups with good cache locality
- Compact storage (minimal overhead)
- Support for billions of entries
- Memory-mapped access for large files

## Performance

- **Index Access**: O(log n) lookups using cache-efficient Eytzinger layout
- **Memory Usage**: Memory-mapped files with minimal RAM overhead
- **Network Efficiency**: Fetch only 3-10KB per block instead of entire 100GB+ CAR files
- **Processing Speed**: ~500 slots per second for bulk operations

## Integration Examples

Decode and save to file:
```bash
fetch_block ... 4320000 | tail -n1 | base64 -d > block-4320000.bin
```

Pipe to analysis tool:
```bash
fetch_block ... 4320000 | tail -n1 | base64 -d | analyze-block
```

Process multiple blocks:
```bash
for slot in $(seq 4320000 4320010); do
  fetch_block ... $slot | process-block
done
```

## Use Cases

1. **Remote Data Access**: Fetch specific Solana blocks without downloading entire epochs
2. **Data Extraction**: Use `dump_slots` to find specific blocks in CAR files
3. **Index Validation**: Use `analyze_index` to verify index integrity
4. **Performance Analysis**: Check bucket distribution for lookup performance
5. **Data Migration**: Export slot mappings for processing in other tools
6. **Debugging**: Trace how blocks are stored and accessed

## Requirements

- Rust 1.70 or later
- Network access for remote CAR files (when using fetch_block)
- Index files generated from yellowstone-faithful CAR files
- CAR files must support HTTP range requests (most CDNs and web servers do)

## Error Handling

The tools provide clear error messages for:
- Slots/signatures not found in indexes
- Network errors or timeouts
- Invalid CAR format data
- Incomplete or corrupted blocks

## License

AGPL-3.0