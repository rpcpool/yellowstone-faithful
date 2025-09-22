use yellowstone_compactindex::SlotToCid;
use std::path::PathBuf;

fn test_data_path(filename: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("test-data")
        .join(filename)
}

#[test]
fn debug_slot_index_contents() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = SlotToCid::open(&path).expect("Failed to open slot-to-CID index");
    
    println!("Debugging slot index:");
    println!("  Total buckets: {}", index.num_buckets());
    
    // Let's examine the first few buckets to understand what's stored
    for bucket_idx in 0..3.min(index.num_buckets()) {
        match index.get_bucket(bucket_idx) {
            Ok(mut bucket) => {
                println!("\nBucket {}:", bucket_idx);
                println!("  Entries: {}", bucket.descriptor.header.num_entries);
                println!("  Hash domain: 0x{:08x}", bucket.descriptor.header.hash_domain);
                
                // Load the first few entries to see what they contain
                let entries_to_check = 3.min(bucket.descriptor.header.num_entries as usize);
                for i in 0..entries_to_check {
                    match bucket.load_entry(i) {
                        Ok(entry) => {
                            // The hash is the hashed slot number
                            println!("  Entry {}: hash=0x{:06x}, value_len={}", 
                                i, entry.hash, entry.value.len());
                            
                            // Try to understand what slots might be here
                            // We can't reverse the hash, but we can try some known slots
                        }
                        Err(e) => {
                            println!("  Failed to load entry {}: {:?}", i, e);
                        }
                    }
                }
            }
            Err(e) => {
                println!("Failed to get bucket {}: {:?}", bucket_idx, e);
            }
        }
    }
    
    // Now let's try slots from different epochs to understand the range
    println!("\nTesting various slot ranges:");
    
    // Solana epochs are 432000 slots each
    // Epoch 0: slots 0-431999
    // Epoch 1: slots 432000-863999
    // Epoch 2: slots 864000-1295999
    // ...
    // Epoch 10: slots 4320000-4751999
    
    let test_ranges = vec![
        (432000, 432010, "Epoch 1 start"),
        (4320000, 4320010, "Epoch 10 start"),
        (4751990, 4752000, "Epoch 10 end"),
        (0, 10, "Epoch 0 start"),
    ];
    
    for (start, end, description) in test_ranges {
        println!("\n{} (slots {}-{}):", description, start, end-1);
        let mut found = 0;
        for slot in start..end {
            match index.lookup(slot) {
                Ok(cid) => {
                    println!("  Slot {} found! CID length: {}", slot, cid.len());
                    found += 1;
                }
                Err(_) => {
                    // Not found, continue
                }
            }
        }
        if found == 0 {
            println!("  No slots found in this range");
        }
    }
}

#[test]
fn examine_hash_calculation() {
    let path = test_data_path("epoch-10-bafyreievp75op5qsrwdr3yk4hrdktkxl6ns5xysjxebaa6c5s77s2etfxa-mainnet-slot-to-cid.index");
    
    if !path.exists() {
        eprintln!("Skipping test: test data file not found at {:?}", path);
        return;
    }
    
    let index = SlotToCid::open(&path).expect("Failed to open slot-to-CID index");
    
    // Let's trace through the hash calculation for a specific slot
    let slot: u64 = 4320000; // Start of epoch 10
    let slot_bytes = slot.to_le_bytes();
    
    println!("Hash calculation for slot {}:", slot);
    println!("  Slot as bytes: {:?}", slot_bytes);
    
    // Get the bucket this slot would map to
    match index.lookup_bucket(&slot_bytes) {
        Ok(mut bucket) => {
            println!("  Maps to bucket with:");
            println!("    Hash domain: 0x{:08x}", bucket.descriptor.header.hash_domain);
            println!("    Entries: {}", bucket.descriptor.header.num_entries);
            
            // Calculate the hash that would be used for this slot
            let hash = bucket.descriptor.header.hash(&slot_bytes);
            println!("  Calculated hash: 0x{:06x}", hash);
            
            // Try to find this hash in the bucket
            println!("  Searching for this hash in bucket...");
            
            // Load all entries (expensive but for debugging)
            match bucket.load_all() {
                Ok(entries) => {
                    let mut found = false;
                    for (i, entry) in entries.iter().enumerate() {
                        if entry.hash == hash {
                            println!("    Found at position {}!", i);
                            println!("    CID length: {} bytes", entry.value.len());
                            found = true;
                            break;
                        }
                    }
                    if !found {
                        println!("    Hash not found in bucket");
                        
                        // Show the range of hashes in the bucket
                        if !entries.is_empty() {
                            let min_hash = entries.iter().map(|e| e.hash).min().unwrap();
                            let max_hash = entries.iter().map(|e| e.hash).max().unwrap();
                            println!("    Bucket hash range: 0x{:06x} - 0x{:06x}", min_hash, max_hash);
                        }
                    }
                }
                Err(e) => {
                    println!("    Failed to load entries: {:?}", e);
                }
            }
        }
        Err(e) => {
            println!("  Failed to get bucket: {:?}", e);
        }
    }
}