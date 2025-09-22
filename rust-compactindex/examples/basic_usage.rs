use yellowstone_compactindex::{CompactIndexReader, SigToCid, SlotToCid, CidToOffsetAndSize};
use std::error::Error;

fn main() -> Result<(), Box<dyn Error>> {
    // Example 1: Reading a signature-to-CID index
    println!("Example 1: Signature to CID lookup");
    println!("-----------------------------------");
    
    // In practice, you would use a real index file path
    // let sig_index = SigToCid::open("path/to/sig-to-cid.index")?;
    
    // For demonstration, we'll show the API structure
    // let signature = hex::decode("...")?; // 64-byte signature
    // let cid = sig_index.lookup(&signature)?;
    // println!("CID for signature: {:?}", cid);
    
    println!("SigToCid index expects 64-byte signatures");
    println!();

    // Example 2: Reading a slot-to-CID index
    println!("Example 2: Slot to CID lookup");
    println!("------------------------------");
    
    // let slot_index = SlotToCid::open("path/to/slot-to-cid.index")?;
    // let slot: u64 = 432_100_000;
    // let cid = slot_index.lookup(slot)?;
    // println!("CID for slot {}: {:?}", slot, cid);
    
    println!("SlotToCid index maps slot numbers to CIDs");
    println!();

    // Example 3: Reading a CID-to-offset-and-size index
    println!("Example 3: CID to Offset and Size lookup");
    println!("-----------------------------------------");
    
    // let offset_index = CidToOffsetAndSize::open("path/to/cid-to-offset.index")?;
    // let cid = b"example_cid_bytes";
    // let (offset, size) = offset_index.lookup(cid)?;
    // println!("Offset: {}, Size: {} for CID", offset, size);
    
    println!("CidToOffsetAndSize returns (offset: u64, size: u32) tuples");
    println!();

    // Example 4: Direct reader usage with prefetching
    println!("Example 4: Direct reader with prefetching");
    println!("------------------------------------------");
    
    // let mut reader = CompactIndexReader::open("path/to/index.file")?;
    // 
    // // Enable prefetching for better performance on multiple lookups
    // reader.set_prefetch(true);
    // 
    // // Perform lookups
    // let key = b"lookup_key";
    // let value = reader.lookup(key)?;
    // println!("Found value: {:?}", value);
    
    println!("Prefetching improves performance for multiple lookups");
    println!();

    // Example 5: Iterating over buckets
    println!("Example 5: Bucket iteration");
    println!("----------------------------");
    
    // let reader = CompactIndexReader::open("path/to/index.file")?;
    // 
    // // Iterate over all bucket indices
    // for bucket_idx in reader.bucket_indices() {
    //     let bucket = reader.get_bucket(bucket_idx)?;
    //     println!("Bucket {}: {} entries", bucket_idx, bucket.descriptor.header.num_entries);
    // }
    
    println!("Bucket iteration is useful for debugging and analysis");
    println!();

    // Example 6: Accessing metadata
    println!("Example 6: Index metadata");
    println!("--------------------------");
    
    // let reader = CompactIndexReader::open("path/to/index.file")?;
    // 
    // // Check index type/kind
    // if let Some(kind) = reader.get_metadata(b"kind") {
    //     println!("Index kind: {}", String::from_utf8_lossy(kind));
    // }
    // 
    // // Check epoch if available
    // if let Some(epoch_bytes) = reader.get_metadata(b"epoch") {
    //     if epoch_bytes.len() == 8 {
    //         let epoch = u64::from_le_bytes(epoch_bytes.try_into().unwrap());
    //         println!("Epoch: {}", epoch);
    //     }
    // }
    
    println!("Metadata provides additional context about the index");

    Ok(())
}