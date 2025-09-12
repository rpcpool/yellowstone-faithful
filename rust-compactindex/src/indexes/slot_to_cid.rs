use crate::reader::CompactIndexReader;
use crate::types::CompactIndexError;
use std::path::Path;

/// Index mapping slots to CIDs
pub struct SlotToCid {
    pub(crate) reader: CompactIndexReader,
}

impl SlotToCid {
    /// Index kind identifier
    pub const KIND: &'static [u8] = b"slot-to-cid";

    /// Open a slot-to-CID index
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
        
        Ok(SlotToCid { reader })
    }

    /// Look up a slot and return the CID bytes
    pub fn lookup(&self, slot: u64) -> Result<Vec<u8>, CompactIndexError> {
        let slot_bytes = slot.to_le_bytes();
        self.reader.lookup(&slot_bytes)
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

    /// Iterate over bucket indices
    pub fn bucket_indices(&self) -> impl Iterator<Item = u32> {
        self.reader.bucket_indices()
    }

    /// Get a bucket by index
    pub fn get_bucket(&self, index: u32) -> Result<crate::bucket::Bucket<std::io::Cursor<&[u8]>>, CompactIndexError> {
        self.reader.get_bucket(index)
    }

    /// Get the bucket that would contain the given key
    pub fn lookup_bucket(&self, key: &[u8]) -> Result<crate::bucket::Bucket<std::io::Cursor<&[u8]>>, CompactIndexError> {
        self.reader.lookup_bucket(key)
    }

    /// Enable or disable prefetching
    pub fn set_prefetch(&mut self, enabled: bool) {
        self.reader.set_prefetch(enabled);
    }
}

#[cfg(test)]
mod tests {
    #[allow(unused_imports)]
    use super::*;

    #[test]
    fn test_slot_encoding() {
        let slot: u64 = 432_100_000;
        let bytes = slot.to_le_bytes();
        let decoded = u64::from_le_bytes(bytes);
        assert_eq!(slot, decoded);
    }
}