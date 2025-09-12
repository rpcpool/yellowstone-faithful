use crate::reader::CompactIndexReader;
use crate::types::CompactIndexError;
use crate::utils::{bytes_to_uint24, bytes_to_uint48, uint24_to_bytes, uint48_to_bytes};
use std::path::Path;

/// Index mapping CIDs to offset and size in CAR files
pub struct CidToOffsetAndSize {
    pub(crate) reader: CompactIndexReader,
}

impl CidToOffsetAndSize {
    /// Expected value size for this index type (6 bytes offset + 3 bytes size)
    pub const VALUE_SIZE: u64 = 9;
    
    /// Index kind identifier
    pub const KIND: &'static [u8] = b"cid-to-offset-and-size";

    /// Open a CID-to-offset-and-size index
    pub fn open<P: AsRef<Path>>(path: P) -> Result<Self, CompactIndexError> {
        let reader = CompactIndexReader::open(path)?;
        
        // Verify this is the correct index type
        if reader.value_size() != Self::VALUE_SIZE {
            return Err(CompactIndexError::ValueSizeMismatch {
                expected: Self::VALUE_SIZE as usize,
                got: reader.value_size() as usize,
            });
        }
        
        // Optionally verify index kind from metadata
        if let Some(kind) = reader.get_metadata(super::metadata_keys::KIND) {
            if kind != Self::KIND {
                return Err(CompactIndexError::InvalidFormat(
                    format!("Wrong index kind: expected {:?}, got {:?}", 
                        Self::KIND, kind),
                ));
            }
        }
        
        Ok(CidToOffsetAndSize { reader })
    }

    /// Look up a CID and return (offset, size)
    pub fn lookup(&self, cid_bytes: &[u8]) -> Result<(u64, u32), CompactIndexError> {
        let value = self.reader.lookup(cid_bytes)?;
        
        if value.len() != Self::VALUE_SIZE as usize {
            return Err(CompactIndexError::InvalidFormat(
                format!("Invalid value size: {}", value.len()),
            ));
        }
        
        let offset = bytes_to_uint48(&value[..6]);
        let size = bytes_to_uint24(&value[6..9]);
        
        Ok((offset, size))
    }

    /// Get epoch number from metadata
    pub fn epoch(&self) -> Option<u64> {
        self.reader.get_metadata(super::metadata_keys::EPOCH)
            .and_then(|bytes| {
                if bytes.len() == 8 {
                    Some(u64::from_le_bytes([
                        bytes[0], bytes[1], bytes[2], bytes[3],
                        bytes[4], bytes[5], bytes[6], bytes[7],
                    ]))
                } else {
                    None
                }
            })
    }

    /// Get network from metadata
    pub fn network(&self) -> Option<String> {
        self.reader.get_metadata(super::metadata_keys::NETWORK)
            .and_then(|bytes| String::from_utf8(bytes.to_vec()).ok())
    }

    /// Get number of buckets
    pub fn num_buckets(&self) -> u32 {
        self.reader.num_buckets()
    }

    /// Get value size
    pub fn value_size(&self) -> u64 {
        self.reader.value_size()
    }

    /// Enable or disable prefetching
    pub fn set_prefetch(&mut self, enabled: bool) {
        self.reader.set_prefetch(enabled);
    }

    /// Get a bucket by index
    pub fn get_bucket(&self, index: u32) -> Result<crate::bucket::Bucket<std::io::Cursor<&[u8]>>, CompactIndexError> {
        self.reader.get_bucket(index)
    }
}

/// Helper to encode offset and size for storage
#[allow(dead_code)]
pub fn encode_offset_and_size(offset: u64, size: u32) -> [u8; 9] {
    let mut result = [0u8; 9];
    result[..6].copy_from_slice(&uint48_to_bytes(offset));
    result[6..9].copy_from_slice(&uint24_to_bytes(size));
    result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_encode_decode_offset_and_size() {
        let offset = 0x123456789ABC;
        let size = 0xDEF012;
        
        let encoded = encode_offset_and_size(offset, size);
        
        let decoded_offset = bytes_to_uint48(&encoded[..6]);
        let decoded_size = bytes_to_uint24(&encoded[6..9]);
        
        assert_eq!(decoded_offset, offset);
        assert_eq!(decoded_size, size);
    }
}