use yellowstone_compactindex::{SlotToCid, CidToOffsetAndSize};
use std::path::PathBuf;
use std::env;
use std::time::Instant;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = env::args().collect();
    
    if args.len() < 3 {
        eprintln!("Usage: {} <slot-to-cid-index> <cid-to-offset-index> [start_slot] [end_slot]", args[0]);
        eprintln!();
        eprintln!("Examples:");
        eprintln!("  {} slot-to-cid.index cid-to-offset.index", args[0]);
        eprintln!("  {} slot-to-cid.index cid-to-offset.index 4320000 4320100", args[0]);
        std::process::exit(1);
    }
    
    let slot_index_path = PathBuf::from(&args[1]);
    let cid_index_path = PathBuf::from(&args[2]);
    
    // Optional slot range
    let start_slot = args.get(3).and_then(|s| s.parse::<u64>().ok());
    let end_slot = args.get(4).and_then(|s| s.parse::<u64>().ok());
    
    // Load indexes
    eprintln!("Loading indexes...");
    let start_time = Instant::now();
    
    let slot_index = SlotToCid::open(&slot_index_path)?;
    eprintln!("  Slot-to-CID index loaded: {} buckets", slot_index.num_buckets());
    
    let cid_index = CidToOffsetAndSize::open(&cid_index_path)?;
    eprintln!("  CID-to-offset index loaded: {} buckets", cid_index.num_buckets());
    
    eprintln!("  Load time: {:?}", start_time.elapsed());
    
    // Get epoch information
    let epoch = slot_index.epoch().unwrap_or(0);
    eprintln!("  Epoch: {}", epoch);
    
    // Determine slot range
    let (start, end) = if let (Some(s), Some(e)) = (start_slot, end_slot) {
        (s, e)
    } else {
        // Default to full epoch range
        // Each epoch is 432,000 slots
        let epoch_start = epoch * 432_000;
        let epoch_end = epoch_start + 432_000 - 1;
        (epoch_start, epoch_end)
    };
    
    eprintln!("\nProcessing slots {} to {} (inclusive)", start, end);
    eprintln!("Format: SLOT,CID_HEX,OFFSET,SIZE");
    eprintln!();
    
    // Process slots
    let mut found = 0;
    let mut not_found = 0;
    let mut cid_not_found = 0;
    let total = end - start + 1;
    let mut last_progress = Instant::now();
    
    for slot in start..=end {
        // Show progress every second
        if last_progress.elapsed().as_secs() >= 1 {
            eprintln!("Progress: {}/{} slots processed ({:.1}%)", 
                     slot - start, total, 
                     ((slot - start) as f64 / total as f64) * 100.0);
            last_progress = Instant::now();
        }
        
        // Look up slot to get CID
        match slot_index.lookup(slot) {
            Ok(cid_bytes) => {
                // Convert CID to hex for display
                let cid_hex = hex::encode(&cid_bytes);
                
                // Look up CID to get offset and size
                match cid_index.lookup(&cid_bytes) {
                    Ok((offset, size)) => {
                        println!("{},{},{},{}", slot, cid_hex, offset, size);
                        found += 1;
                    }
                    Err(_) => {
                        // CID exists in slot index but not in offset index
                        println!("{},{},NOT_FOUND,NOT_FOUND", slot, cid_hex);
                        cid_not_found += 1;
                    }
                }
            }
            Err(_) => {
                // Slot not found in index (might be a skipped slot)
                // Only print if explicitly requested via range
                if start_slot.is_some() && end_slot.is_some() {
                    println!("{},NOT_FOUND,NOT_FOUND,NOT_FOUND", slot);
                }
                not_found += 1;
            }
        }
    }
    
    eprintln!("\nComplete!");
    eprintln!("Summary:");
    eprintln!("  Total slots processed: {}", total);
    eprintln!("  Slots found: {}", found);
    eprintln!("  Slots not found: {}", not_found);
    if cid_not_found > 0 {
        eprintln!("  CIDs not found in offset index: {}", cid_not_found);
    }
    eprintln!("  Success rate: {:.2}%", (found as f64 / total as f64) * 100.0);
    
    Ok(())
}