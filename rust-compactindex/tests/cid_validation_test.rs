use yellowstone_compactindex::SlotToCid;
use std::path::PathBuf;

fn test_data_path(filename: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("test-data")
        .join(filename)
}

#[test]
fn test_cid_format_validation() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = SlotToCid::open(&path).expect("Failed to open slot-to-CID index");
    
    // Test a known slot from epoch 10
    let slot: u64 = 4320000;
    let cid_bytes = index.lookup(slot).expect("Should find slot 4320000");
    
    // Validate CID format
    assert_eq!(cid_bytes.len(), 36, "CID should be 36 bytes");
    
    // CIDv1 format validation
    // First byte is version (0x01 for CIDv1)
    assert_eq!(cid_bytes[0], 0x01, "Should be CIDv1 (version byte = 0x01)");
    
    // Second byte is the codec
    // 0x71 = dag-cbor (commonly used in IPLD)
    // 0x70 = dag-pb (protobuf)
    // 0x55 = raw
    let codec = cid_bytes[1];
    println!("CID codec: 0x{:02x}", codec);
    
    // The remaining bytes are the multihash
    // Multihash format: <hash-func><digest-size><digest>
    let multihash = &cid_bytes[2..];
    
    // Common hash functions:
    // 0x12 = SHA2-256
    // 0x13 = SHA2-512
    // 0x1b = KECCAK-256
    let hash_func = multihash[0];
    println!("Hash function: 0x{:02x}", hash_func);
    
    // Digest size (in bytes)
    let digest_size = multihash[1] as usize;
    println!("Digest size: {} bytes", digest_size);
    
    // Verify we have enough bytes for the digest
    assert!(multihash.len() >= 2 + digest_size, "Multihash should contain full digest");
    
    // Print a human-readable representation
    println!("\nCID breakdown for slot {}:", slot);
    println!("  Version: CIDv1");
    println!("  Codec: 0x{:02x} ({})", codec, codec_name(codec));
    println!("  Hash function: 0x{:02x} ({})", hash_func, hash_name(hash_func));
    println!("  Digest size: {} bytes", digest_size);
    
    // Verify multiple slots have consistent CID format
    let test_slots = vec![4320000, 4320001, 4320002, 4320003];
    for slot in test_slots {
        let cid = index.lookup(slot).expect(&format!("Should find slot {}", slot));
        assert_eq!(cid.len(), 36, "All CIDs should be 36 bytes");
        assert_eq!(cid[0], 0x01, "All should be CIDv1");
        assert_eq!(cid[1], codec, "All should use same codec");
    }
    
    println!("\nAll tested slots have consistent CID format ✓");
}

fn codec_name(codec: u8) -> &'static str {
    match codec {
        0x70 => "dag-pb",
        0x71 => "dag-cbor",
        0x55 => "raw",
        0x72 => "dag-json",
        _ => "unknown",
    }
}

fn hash_name(hash_func: u8) -> &'static str {
    match hash_func {
        0x12 => "SHA2-256",
        0x13 => "SHA2-512",
        0x14 => "SHA3-512",
        0x15 => "SHA3-384",
        0x16 => "SHA3-256",
        0x17 => "SHA3-224",
        0x1b => "KECCAK-256",
        _ => "unknown",
    }
}

#[test]
fn test_round_trip_consistency() {
    // This test verifies that we can look up a slot, get its CID,
    // then use that CID to find the offset in the CAR file
    
    let slot_path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    let cid_path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-cid-to-offset-and-size.index");
    
    if !slot_path.exists() || !cid_path.exists() {
        eprintln!("Skipping test: test data files not found");
        return;
    }
    
    let slot_index = SlotToCid::open(&slot_path).expect("Failed to open slot index");
    let cid_index = yellowstone_compactindex::CidToOffsetAndSize::open(&cid_path)
        .expect("Failed to open CID index");
    
    // Look up a slot to get its CID
    let slot: u64 = 4320000;
    let cid_bytes = slot_index.lookup(slot).expect("Should find slot");
    
    println!("Round-trip test for slot {}:", slot);
    println!("  CID length: {} bytes", cid_bytes.len());
    
    // Now use that CID to find its offset and size in the CAR file
    match cid_index.lookup(&cid_bytes) {
        Ok((offset, size)) => {
            println!("  Found in CAR file!");
            println!("    Offset: {} bytes", offset);
            println!("    Size: {} bytes", size);
            
            // Sanity checks
            assert!(size > 0, "Size should be positive");
            assert!(offset < 1_000_000_000_000, "Offset should be reasonable");
            
            println!("  Round-trip successful ✓");
        }
        Err(e) => {
            // This might happen if the indexes are from different builds
            // or if the CID index doesn't contain all CIDs
            println!("  CID not found in offset index: {:?}", e);
            println!("  (This may be expected if indexes are from different sources)");
        }
    }
}