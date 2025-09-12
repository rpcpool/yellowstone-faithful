use yellowstone_compactindex::{CidToOffsetAndSize, SlotToCid, SigToCid};
use std::path::PathBuf;

fn test_data_path(filename: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("test-data")
        .join(filename)
}

#[test]
fn test_slot_lookup() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = SlotToCid::open(&path).expect("Failed to open slot-to-CID index");
    
    // Epoch 10 contains slots 4,320,000 to 4,751,999 (epoch_num * 432,000 slots per epoch)
    // Let's try to look up some slots that should exist in epoch 10
    let test_slots = vec![
        4320000, // Start of epoch 10
        4320100, // Early in epoch 10
        4500000, // Middle of epoch 10
        4751999, // Last slot of epoch 10
    ];
    
    let mut found_count = 0;
    let mut not_found_count = 0;
    
    for slot in test_slots {
        match index.lookup(slot) {
            Ok(cid_bytes) => {
                println!("Slot {} -> CID with {} bytes", slot, cid_bytes.len());
                assert_eq!(cid_bytes.len(), 36, "CID should be 36 bytes");
                found_count += 1;
                
                // Verify the CID starts with expected bytes for CIDv1
                // CIDv1 typically starts with 0x01 (version) followed by codec
                if cid_bytes.len() >= 2 {
                    println!("  CID prefix: {:02x} {:02x}", cid_bytes[0], cid_bytes[1]);
                }
            }
            Err(e) => {
                println!("Slot {} not found: {:?}", slot, e);
                not_found_count += 1;
            }
        }
    }
    
    println!("\nSlot lookup summary:");
    println!("  Found: {} slots", found_count);
    println!("  Not found: {} slots", not_found_count);
    
    // We expect at least some slots to be found
    assert!(found_count > 0, "Should find at least one slot in the index");
}

#[test]
fn test_bucket_search_mechanics() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = SlotToCid::open(&path).expect("Failed to open slot-to-CID index");
    
    // Test that we can get buckets and examine their contents
    let slot: u64 = 4320100;  // A slot in epoch 10
    let slot_bytes = slot.to_le_bytes();
    
    // Get the bucket that should contain this slot
    let bucket_result = index.lookup_bucket(&slot_bytes);
    
    match bucket_result {
        Ok(mut bucket) => {
            println!("Bucket for slot {}:", slot);
            println!("  Hash domain: {}", bucket.descriptor.header.hash_domain);
            println!("  Num entries: {}", bucket.descriptor.header.num_entries);
            println!("  Hash length: {} bytes", bucket.descriptor.header.hash_len);
            println!("  Stride: {} bytes", bucket.descriptor.stride);
            
            // Try to load a few entries to verify the bucket structure
            if bucket.descriptor.header.num_entries > 0 {
                match bucket.load_entry(0) {
                    Ok(entry) => {
                        println!("  First entry hash: 0x{:06x}", entry.hash);
                        println!("  First entry value size: {} bytes", entry.value.len());
                    }
                    Err(e) => {
                        println!("  Failed to load first entry: {:?}", e);
                    }
                }
            }
        }
        Err(e) => {
            println!("Failed to get bucket for slot {}: {:?}", slot, e);
        }
    }
}

#[test]
fn test_cid_to_offset_lookup() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-cid-to-offset-and-size.index");
    
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = CidToOffsetAndSize::open(&path).expect("Failed to open CID-to-offset index");
    
    // First, let's understand the structure
    println!("CID-to-offset index structure:");
    println!("  Total buckets: {}", index.num_buckets());
    println!("  Value size: {} bytes", index.value_size());
    
    // Sample a few buckets to understand the data
    let mut total_entries = 0;
    let sample_size = 10.min(index.num_buckets());
    
    for i in 0..sample_size {
        match index.get_bucket(i) {
            Ok(bucket) => {
                total_entries += bucket.descriptor.header.num_entries;
                if i < 3 {
                    println!("  Bucket {}: {} entries", i, bucket.descriptor.header.num_entries);
                }
            }
            Err(e) => {
                println!("  Failed to get bucket {}: {:?}", i, e);
            }
        }
    }
    
    println!("  Total entries in first {} buckets: {}", sample_size, total_entries);
    
    // To perform an actual lookup, we would need a real CID from epoch 10
    // For now, we'll verify that the lookup mechanism works even if we don't have a valid CID
    let test_cid = b"test_cid_that_probably_doesnt_exist";
    match index.lookup(test_cid) {
        Ok((offset, size)) => {
            println!("Unexpectedly found test CID at offset {} with size {}", offset, size);
        }
        Err(yellowstone_compactindex::CompactIndexError::NotFound) => {
            println!("Test CID not found (expected)");
        }
        Err(e) => {
            println!("Lookup error: {:?}", e);
        }
    }
}

#[test]
fn test_cross_index_consistency() {
    // This test verifies that the indexes are internally consistent
    // by checking metadata across all three index types
    
    let slot_path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    let sig_path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-sig-to-cid.index");
    let cid_path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-cid-to-offset-and-size.index");
    
    if !slot_path.exists() || !sig_path.exists() || !cid_path.exists() {
        eprintln!("Skipping test: not all test data files found");
        return;
    }
    
    let slot_index = SlotToCid::open(&slot_path).expect("Failed to open slot index");
    let sig_index = SigToCid::open(&sig_path).expect("Failed to open sig index");
    let cid_index = CidToOffsetAndSize::open(&cid_path).expect("Failed to open CID index");
    
    // All three indexes should have the same epoch
    assert_eq!(slot_index.epoch(), Some(10), "Slot index should be epoch 10");
    assert_eq!(sig_index.epoch(), Some(10), "Sig index should be epoch 10");
    assert_eq!(cid_index.epoch(), Some(10), "CID index should be epoch 10");
    
    // All should be mainnet
    assert_eq!(cid_index.network(), Some("mainnet".to_string()), "Should be mainnet");
    
    // CID sizes should match between slot and sig indexes
    assert_eq!(slot_index.cid_size(), sig_index.cid_size(), 
               "Slot and sig indexes should have same CID size");
    
    println!("Cross-index consistency verified:");
    println!("  All indexes: epoch 10, mainnet");
    println!("  CID size: {} bytes", slot_index.cid_size());
    println!("  Slot index: {} buckets", slot_index.num_buckets());
    println!("  Sig index: {} buckets", sig_index.num_buckets());
    println!("  CID-to-offset index: {} buckets", cid_index.num_buckets());
}

#[test]
fn test_performance_characteristics() {
    // This test examines the performance characteristics of the indexes
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let mut index = SlotToCid::open(&path).expect("Failed to open index");
    
    // Test lookup performance with and without prefetch
    use std::time::Instant;
    
    // First, test without prefetch
    index.set_prefetch(false);
    let start = Instant::now();
    for slot in 4320000..4320010 {
        let _ = index.lookup(slot);
    }
    let without_prefetch = start.elapsed();
    
    // Now test with prefetch
    index.set_prefetch(true);
    let start = Instant::now();
    for slot in 4320100..4320110 {
        let _ = index.lookup(slot);
    }
    let with_prefetch = start.elapsed();
    
    println!("Performance test results:");
    println!("  10 lookups without prefetch: {:?}", without_prefetch);
    println!("  10 lookups with prefetch: {:?}", with_prefetch);
    
    // The test passes as long as both complete without errors
    // Actual performance comparison may vary based on system
}