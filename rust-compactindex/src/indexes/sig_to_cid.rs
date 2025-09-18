use crate::reader::CompactIndexReader;
use crate::types::CompactIndexError;
use std::path::Path;

/// Index mapping transaction signatures to CIDs
pub struct SigToCid {
    pub(crate) reader: CompactIndexReader,
}

impl SigToCid {
    /// Expected signature size (64 bytes for Solana)
    pub const SIGNATURE_SIZE: usize = 64;
    
    /// Index kind identifier
    pub const KIND: &'static [u8] = b"sig-to-cid";

    /// Open a signature-to-CID index
    pub fn open<P: AsRef<Path>>(path: P) -> Result<Self, CompactIndexError> {
        let reader = CompactIndexReader::open(path)?;
        
        // Verify index kind from metadata if present
        if let Some(kind) = reader.get_metadata(super::metadata_keys::KIND) {
            if kind != Self::KIND {
                return Err(CompactIndexError::InvalidFormat(
                    format!("Wrong index kind: expected {:?}, got {:?}", 
                        Self::KIND, kind),
                ));
            }
        }
        
        Ok(SigToCid { reader })
    }

    /// Look up a signature and return the CID bytes
    pub fn lookup(&self, signature: &[u8]) -> Result<Vec<u8>, CompactIndexError> {
        if signature.len() != Self::SIGNATURE_SIZE {
            return Err(CompactIndexError::InvalidFormat(
                format!("Invalid signature size: expected {}, got {}", 
                    Self::SIGNATURE_SIZE, signature.len()),
            ));
        }
        
        self.reader.lookup(signature)
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

    /// Get the value size (CID size)
    pub fn cid_size(&self) -> u64 {
        self.reader.value_size()
    }

    /// Get number of buckets
    pub fn num_buckets(&self) -> u32 {
        self.reader.num_buckets()
    }

    /// Get a bucket by index
    pub fn get_bucket(&self, index: u32) -> Result<crate::bucket::Bucket<std::io::Cursor<&[u8]>>, CompactIndexError> {
        self.reader.get_bucket(index)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_signature_size_validation() {
        // A valid 64-byte signature
        let valid_sig = vec![0u8; 64];
        assert_eq!(valid_sig.len(), SigToCid::SIGNATURE_SIZE);
        
        // Invalid sizes
        let invalid_sig = vec![0u8; 32];
        assert_ne!(invalid_sig.len(), SigToCid::SIGNATURE_SIZE);
    }
}