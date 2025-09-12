use yellowstone_compactindex::{SlotToCid, SigToCid, CidToOffsetAndSize};
use std::path::PathBuf;
use std::env;
use reqwest::header::RANGE;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = env::args().collect();
    
    if args.len() < 4 {
        eprintln!("Usage: {} <lookup-index> <cid-to-offset-index> <car-url> [lookup-value]", args[0]);
        eprintln!();
        eprintln!("Examples:");
        eprintln!("  Fetch slot: {} slot-to-cid.index cid-to-offset.index https://example.com/epoch-10.car 4320000", args[0]);
        eprintln!("  Fetch sig:  {} sig-to-cid.index cid-to-offset.index https://example.com/epoch-10.car <base58-sig>", args[0]);
        eprintln!();
        eprintln!("If no lookup value is provided, reads from stdin (one per line)");
        std::process::exit(1);
    }
    
    let lookup_index_path = PathBuf::from(&args[1]);
    let cid_index_path = PathBuf::from(&args[2]);
    let car_url = &args[3];
    let lookup_value = args.get(4);
    
    // Load CID-to-offset index (always needed)
    eprintln!("Loading CID-to-offset index...");
    let cid_index = CidToOffsetAndSize::open(&cid_index_path)?;
    eprintln!("  Loaded: {} buckets", cid_index.num_buckets());
    
    // Determine lookup type and process
    if lookup_index_path.to_string_lossy().contains("slot") {
        // Slot-to-CID lookup
        let slot_index = SlotToCid::open(&lookup_index_path)?;
        eprintln!("  Loaded slot-to-CID index: {} buckets", slot_index.num_buckets());
        
        if let Some(slot_str) = lookup_value {
            // Single slot lookup
            let slot: u64 = slot_str.parse()?;
            process_slot(&slot_index, &cid_index, car_url, slot).await?;
        } else {
            // Read slots from stdin
            use std::io::{self, BufRead};
            let stdin = io::stdin();
            for line in stdin.lock().lines() {
                let line = line?;
                if let Ok(slot) = line.trim().parse::<u64>() {
                    match process_slot(&slot_index, &cid_index, car_url, slot).await {
                        Ok(_) => {},
                        Err(e) => eprintln!("Error processing slot {}: {}", slot, e),
                    }
                }
            }
        }
    } else if lookup_index_path.to_string_lossy().contains("sig") {
        // Signature-to-CID lookup
        let sig_index = SigToCid::open(&lookup_index_path)?;
        eprintln!("  Loaded sig-to-CID index: {} buckets", sig_index.num_buckets());
        
        if let Some(sig_str) = lookup_value {
            // Single signature lookup
            let sig_bytes = decode_base58(sig_str)?;
            process_signature(&sig_index, &cid_index, car_url, &sig_bytes).await?;
        } else {
            // Read signatures from stdin
            use std::io::{self, BufRead};
            let stdin = io::stdin();
            for line in stdin.lock().lines() {
                let line = line?;
                let sig_str = line.trim();
                if !sig_str.is_empty() {
                    match decode_base58(sig_str) {
                        Ok(sig_bytes) => {
                            match process_signature(&sig_index, &cid_index, car_url, &sig_bytes).await {
                                Ok(_) => {},
                                Err(e) => eprintln!("Error processing signature {}: {}", sig_str, e),
                            }
                        }
                        Err(e) => eprintln!("Invalid base58 signature {}: {}", sig_str, e),
                    }
                }
            }
        }
    } else {
        eprintln!("Error: Could not determine index type (should contain 'slot' or 'sig' in filename)");
        std::process::exit(1);
    }
    
    Ok(())
}

async fn process_slot(
    slot_index: &SlotToCid,
    cid_index: &CidToOffsetAndSize,
    car_url: &str,
    slot: u64,
) -> Result<(), Box<dyn std::error::Error>> {
    eprintln!("Processing slot {}...", slot);
    
    // Look up slot to get CID
    let cid_bytes = slot_index.lookup(slot)
        .map_err(|e| format!("Slot {} not found: {:?}", slot, e))?;
    
    eprintln!("  Found CID: {}", hex::encode(&cid_bytes[..8]));
    
    // Look up CID to get offset and size
    let (offset, size) = cid_index.lookup(&cid_bytes)
        .map_err(|e| format!("CID not found in offset index: {:?}", e))?;
    
    eprintln!("  CAR offset: {}, size: {}", offset, size);
    
    // Fetch the block data
    let block_data = fetch_car_block(car_url, offset, size).await?;
    
    // Output as base64
    use base64::Engine;
    let base64_data = base64::engine::general_purpose::STANDARD.encode(&block_data);
    println!("SLOT:{}", slot);
    println!("{}", base64_data);
    println!();
    
    Ok(())
}

async fn process_signature(
    sig_index: &SigToCid,
    cid_index: &CidToOffsetAndSize,
    car_url: &str,
    sig_bytes: &[u8],
) -> Result<(), Box<dyn std::error::Error>> {
    eprintln!("Processing signature...");
    
    // Look up signature to get CID
    let cid_bytes = sig_index.lookup(sig_bytes)
        .map_err(|e| format!("Signature not found: {:?}", e))?;
    
    eprintln!("  Found CID: {}", hex::encode(&cid_bytes[..8]));
    
    // Look up CID to get offset and size
    let (offset, size) = cid_index.lookup(&cid_bytes)
        .map_err(|e| format!("CID not found in offset index: {:?}", e))?;
    
    eprintln!("  CAR offset: {}, size: {}", offset, size);
    
    // Fetch the block data
    let block_data = fetch_car_block(car_url, offset, size).await?;
    
    // Output as base64
    use base64::Engine;
    let base64_data = base64::engine::general_purpose::STANDARD.encode(&block_data);
    use base58::ToBase58;
    println!("SIGNATURE:{}", sig_bytes.to_base58());
    println!("{}", base64_data);
    println!();
    
    Ok(())
}

async fn fetch_car_block(
    car_url: &str,
    offset: u64,
    size: u32,
) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    eprintln!("  Fetching {} bytes from offset {}...", size, offset);
    
    // Make HTTP range request
    let client = reqwest::Client::new();
    let end_byte = offset + size as u64 - 1;
    let range_header = format!("bytes={}-{}", offset, end_byte);
    
    let response = client
        .get(car_url)
        .header(RANGE, range_header)
        .send()
        .await?;
    
    if !response.status().is_success() {
        return Err(format!("HTTP error: {}", response.status()).into());
    }
    
    let car_block_bytes = response.bytes().await?;
    eprintln!("  Fetched {} bytes", car_block_bytes.len());
    
    // Parse CAR block format
    let block_data = parse_car_block(&car_block_bytes)?;
    eprintln!("  Extracted {} bytes of block data", block_data.len());
    
    Ok(block_data)
}

fn parse_car_block(car_bytes: &[u8]) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    // Read the varint length prefix
    let (section_len, remaining_after_varint) = unsigned_varint::decode::u64(car_bytes)
        .map_err(|e| format!("Failed to decode varint: {:?}", e))?;
    
    let varint_bytes_read = car_bytes.len() - remaining_after_varint.len();
    eprintln!("    CAR section length: {} (varint size: {})", section_len, varint_bytes_read);
    
    // The section_len includes the CID and the data
    // We need to skip the CID to get to the raw data
    
    // CID parsing (simplified - we'll just skip it)
    // CIDv1 format: version(1) + codec(1) + multihash
    // Multihash format: hash_func(1) + digest_size(1) + digest
    
    if remaining_after_varint.len() < section_len as usize {
        return Err(format!(
            "Incomplete CAR block: expected {} bytes, got {}", 
            section_len, remaining_after_varint.len()
        ).into());
    }
    
    // Parse CID to find its length
    let cid_len = parse_cid_length(remaining_after_varint)?;
    eprintln!("    CID length: {} bytes", cid_len);
    
    // Extract the data after the CID
    let data_start = cid_len;
    let data_end = section_len as usize;
    let block_data = remaining_after_varint[data_start..data_end].to_vec();
    
    Ok(block_data)
}

fn parse_cid_length(data: &[u8]) -> Result<usize, Box<dyn std::error::Error>> {
    if data.len() < 4 {
        return Err("Data too short for CID".into());
    }
    
    let mut offset = 0;
    
    // For CIDv1 (which yellowstone uses)
    if data[0] == 0x01 {
        offset += 1; // version
        offset += 1; // codec
        
        // Multihash
        offset += 1; // hash function
        let digest_size = data[offset] as usize;
        offset += 1; // digest size byte
        offset += digest_size; // actual digest
        
        Ok(offset)
    } else {
        // Might be CIDv0 or something else
        // For yellowstone data, we expect CIDv1
        Err("Unexpected CID format (not CIDv1)".into())
    }
}

fn decode_base58(input: &str) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    // Use bs58 for base58 decoding (Solana uses base58 for signatures)
    use base58::FromBase58;
    input.from_base58()
        .map_err(|e| format!("Invalid base58: {:?}", e).into())
}

// Base58 support is already imported where needed