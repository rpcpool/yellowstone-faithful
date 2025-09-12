use yellowstone_compactindex::{SlotToCid, CidToOffsetAndSize, SigToCid};
use std::path::PathBuf;
use std::env;
use std::collections::HashMap;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = env::args().collect();
    
    if args.len() < 2 {
        eprintln!("Usage: {} <index-file> [command]", args[0]);
        eprintln!();
        eprintln!("Commands:");
        eprintln!("  info     - Show index information (default)");
        eprintln!("  buckets  - Analyze bucket distribution");
        eprintln!("  sample   - Sample random entries");
        eprintln!();
        eprintln!("Examples:");
        eprintln!("  {} slot-to-cid.index", args[0]);
        eprintln!("  {} slot-to-cid.index buckets", args[0]);
        std::process::exit(1);
    }
    
    let index_path = PathBuf::from(&args[1]);
    let command = args.get(2).map(|s| s.as_str()).unwrap_or("info");
    
    // Determine index type by checking metadata
    // Try CID-to-offset first as it has specific value size
    if let Ok(index) = CidToOffsetAndSize::open(&index_path) {
        if index.value_size() == 9 {
            println!("Index Type: CID-to-Offset-and-Size");
            handle_cid_index(index, command)?;
        } else {
            // Not a CID-to-offset index, try others
            if let Ok(index) = SlotToCid::open(&index_path) {
                println!("Index Type: Slot-to-CID");
                handle_slot_index(index, command)?;
            } else if let Ok(index) = SigToCid::open(&index_path) {
                println!("Index Type: Signature-to-CID");
                handle_sig_index(index, command)?;
            } else {
                eprintln!("Error: Could not determine index type");
                std::process::exit(1);
            }
        }
    } else if let Ok(index) = SlotToCid::open(&index_path) {
        println!("Index Type: Slot-to-CID");
        handle_slot_index(index, command)?;
    } else if let Ok(index) = SigToCid::open(&index_path) {
        println!("Index Type: Signature-to-CID");
        handle_sig_index(index, command)?;
    } else {
        eprintln!("Error: Could not open index file as any known type");
        std::process::exit(1);
    }
    
    Ok(())
}

fn handle_slot_index(index: SlotToCid, command: &str) -> Result<(), Box<dyn std::error::Error>> {
    match command {
        "info" => {
            println!("Epoch: {:?}", index.epoch());
            println!("Number of buckets: {}", index.num_buckets());
            println!("CID size: {} bytes", index.cid_size());
            
            // Calculate expected slot range
            if let Some(epoch) = index.epoch() {
                let start_slot = epoch * 432_000;
                let end_slot = start_slot + 431_999;
                println!("Expected slot range: {} - {}", start_slot, end_slot);
            }
        }
        "buckets" => {
            analyze_buckets(&index)?;
        }
        "sample" => {
            sample_slots(&index)?;
        }
        _ => {
            eprintln!("Unknown command: {}", command);
        }
    }
    Ok(())
}

fn handle_sig_index(index: SigToCid, command: &str) -> Result<(), Box<dyn std::error::Error>> {
    match command {
        "info" => {
            println!("Epoch: {:?}", index.epoch());
            println!("Number of buckets: {}", index.num_buckets());
            println!("CID size: {} bytes", index.cid_size());
            println!("Signature size: {} bytes", SigToCid::SIGNATURE_SIZE);
        }
        "buckets" => {
            println!("\nBucket distribution:");
            let mut total_entries = 0u64;
            let mut min_entries = u32::MAX;
            let mut max_entries = 0u32;
            
            for i in 0..index.num_buckets() {
                if let Ok(bucket) = index.get_bucket(i) {
                    let entries = bucket.descriptor.header.num_entries;
                    total_entries += entries as u64;
                    min_entries = min_entries.min(entries);
                    max_entries = max_entries.max(entries);
                }
            }
            
            println!("  Total entries: {}", total_entries);
            println!("  Min entries per bucket: {}", min_entries);
            println!("  Max entries per bucket: {}", max_entries);
            println!("  Avg entries per bucket: {:.2}", 
                     total_entries as f64 / index.num_buckets() as f64);
        }
        _ => {
            eprintln!("Unknown command: {}", command);
        }
    }
    Ok(())
}

fn handle_cid_index(index: CidToOffsetAndSize, command: &str) -> Result<(), Box<dyn std::error::Error>> {
    match command {
        "info" => {
            println!("Epoch: {:?}", index.epoch());
            println!("Network: {:?}", index.network());
            println!("Number of buckets: {}", index.num_buckets());
            println!("Value size: {} bytes (6 offset + 3 size)", index.value_size());
        }
        "buckets" => {
            println!("\nBucket distribution:");
            let mut total_entries = 0u64;
            let mut size_distribution = HashMap::new();
            
            for i in 0..index.num_buckets().min(100) {
                if let Ok(bucket) = index.get_bucket(i) {
                    let entries = bucket.descriptor.header.num_entries;
                    total_entries += entries as u64;
                    *size_distribution.entry(entries / 100 * 100).or_insert(0) += 1;
                }
            }
            
            println!("  Sampled {} buckets", 100.min(index.num_buckets()));
            println!("  Total entries in sample: {}", total_entries);
            
            println!("\n  Distribution (entries per bucket):");
            let mut sizes: Vec<_> = size_distribution.keys().collect();
            sizes.sort();
            for size in sizes {
                let count = size_distribution[size];
                println!("    {}-{}: {} buckets", size, size + 99, count);
            }
        }
        _ => {
            eprintln!("Unknown command: {}", command);
        }
    }
    Ok(())
}

fn analyze_buckets(index: &SlotToCid) -> Result<(), Box<dyn std::error::Error>> {
    println!("\nAnalyzing bucket distribution...");
    
    let mut total_entries = 0u64;
    let mut min_entries = u32::MAX;
    let mut max_entries = 0u32;
    let mut bucket_sizes = Vec::new();
    
    for i in 0..index.num_buckets() {
        if let Ok(bucket) = index.get_bucket(i) {
            let entries = bucket.descriptor.header.num_entries;
            total_entries += entries as u64;
            min_entries = min_entries.min(entries);
            max_entries = max_entries.max(entries);
            bucket_sizes.push(entries);
            
            if i < 3 {
                println!("  Bucket {}: {} entries, hash domain: 0x{:08x}", 
                         i, entries, bucket.descriptor.header.hash_domain);
            }
        }
    }
    
    // Calculate statistics
    let avg_entries = total_entries as f64 / index.num_buckets() as f64;
    bucket_sizes.sort();
    let median_entries = bucket_sizes[bucket_sizes.len() / 2];
    
    println!("\nStatistics:");
    println!("  Total entries: {}", total_entries);
    println!("  Total buckets: {}", index.num_buckets());
    println!("  Min entries per bucket: {}", min_entries);
    println!("  Max entries per bucket: {}", max_entries);
    println!("  Avg entries per bucket: {:.2}", avg_entries);
    println!("  Median entries per bucket: {}", median_entries);
    
    // Calculate load factor
    let load_factor = (max_entries as f64 - min_entries as f64) / avg_entries;
    println!("  Load factor variance: {:.2}%", load_factor * 100.0);
    
    Ok(())
}

fn sample_slots(index: &SlotToCid) -> Result<(), Box<dyn std::error::Error>> {
    println!("\nSampling slots from index...");
    
    if let Some(epoch) = index.epoch() {
        let epoch_start = epoch * 432_000;
        
        // Sample some slots at different points in the epoch
        let sample_points = vec![
            (epoch_start, "Epoch start"),
            (epoch_start + 100_000, "~23% through epoch"),
            (epoch_start + 216_000, "Middle of epoch"),
            (epoch_start + 300_000, "~69% through epoch"),
            (epoch_start + 431_999, "Epoch end"),
        ];
        
        println!("\nSlot samples:");
        for (slot, description) in sample_points {
            match index.lookup(slot) {
                Ok(cid) => {
                    println!("  Slot {} ({}): Found", slot, description);
                    println!("    CID: {}", hex::encode(&cid[..8]));
                }
                Err(_) => {
                    println!("  Slot {} ({}): Not found", slot, description);
                }
            }
        }
    }
    
    Ok(())
}