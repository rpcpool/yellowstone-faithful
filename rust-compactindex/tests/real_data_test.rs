use yellowstone_compactindex::{CidToOffsetAndSize, SigToCid, SlotToCid};
use std::path::PathBuf;

fn test_data_path(filename: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("test-data")
        .join(filename)
}

#[test]
fn test_read_real_cid_to_offset_index() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-cid-to-offset-and-size.index");
    
    // Skip test if file doesn't exist
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = CidToOffsetAndSize::open(&path).expect("Failed to open CID-to-offset index");
    
    // Check basic properties
    assert_eq!(index.value_size(), 9, "CID-to-offset should have 9-byte values (6 offset + 3 size)");
    assert!(index.num_buckets() > 0, "Index should have buckets");
    
    // Check epoch metadata
    let epoch = index.epoch();
    assert_eq!(epoch, Some(10), "Should be epoch 10");
    
    // Check network metadata
    let network = index.network();
    assert_eq!(network, Some("mainnet".to_string()), "Should be mainnet");
    
    println!("CID-to-offset index loaded successfully:");
    println!("  - Buckets: {}", index.num_buckets());
    println!("  - Value size: {} bytes", index.value_size());
    println!("  - Epoch: {:?}", epoch);
    println!("  - Network: {:?}", network);
}

#[test]
fn test_read_real_sig_to_cid_index() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-sig-to-cid.index");
    
    // Skip test if file doesn't exist
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = SigToCid::open(&path).expect("Failed to open sig-to-CID index");
    
    // Check basic properties
    assert!(index.cid_size() > 0, "CID size should be positive");
    assert!(index.num_buckets() > 0, "Index should have buckets");
    
    // Check epoch metadata
    let epoch = index.epoch();
    assert_eq!(epoch, Some(10), "Should be epoch 10");
    
    println!("Sig-to-CID index loaded successfully:");
    println!("  - Buckets: {}", index.num_buckets());
    println!("  - CID size: {} bytes", index.cid_size());
    println!("  - Epoch: {:?}", epoch);
}

#[test]
fn test_read_real_slot_to_cid_index() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    
    // Skip test if file doesn't exist
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = SlotToCid::open(&path).expect("Failed to open slot-to-CID index");
    
    // Check basic properties
    assert!(index.cid_size() > 0, "CID size should be positive");
    assert!(index.num_buckets() > 0, "Index should have buckets");
    
    // Check epoch metadata
    let epoch = index.epoch();
    assert_eq!(epoch, Some(10), "Should be epoch 10");
    
    println!("Slot-to-CID index loaded successfully:");
    println!("  - Buckets: {}", index.num_buckets());
    println!("  - CID size: {} bytes", index.cid_size());
    println!("  - Epoch: {:?}", epoch);
}

#[test]
fn test_bucket_iteration_on_real_data() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    
    // Skip test if file doesn't exist
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = SlotToCid::open(&path).expect("Failed to open slot-to-CID index");
    
    // Test that we can iterate over a few buckets
    let mut bucket_count = 0;
    let mut total_entries = 0;
    
    for bucket_idx in index.bucket_indices().take(10) {
        let bucket = index.get_bucket(bucket_idx).expect("Failed to get bucket");
        bucket_count += 1;
        total_entries += bucket.descriptor.header.num_entries;
    }
    
    assert_eq!(bucket_count, 10, "Should have iterated over 10 buckets");
    assert!(total_entries > 0, "Should have found some entries in buckets");
    
    println!("Bucket iteration successful:");
    println!("  - Checked {} buckets", bucket_count);
    println!("  - Total entries in first 10 buckets: {}", total_entries);
}

#[test]
fn test_prefetch_on_real_data() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-cid-to-offset-and-size.index");
    
    // Skip test if file doesn't exist
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let mut index = CidToOffsetAndSize::open(&path).expect("Failed to open index");
    
    // Enable prefetching
    index.set_prefetch(true);
    
    // Get a bucket with prefetch enabled
    let bucket = index.get_bucket(0);
    assert!(bucket.is_ok(), "Should be able to get bucket with prefetch enabled");
    
    // Disable prefetch
    index.set_prefetch(false);
    
    // Should still work
    let bucket = index.get_bucket(1);
    assert!(bucket.is_ok(), "Should be able to get bucket with prefetch disabled");
    
    println!("Prefetch functionality verified on real data");
}