use crate::types::{CompactIndexError, Metadata, MAGIC, VERSION};
use crate::utils::{hash_uint64, uint_le};
use byteorder::{ByteOrder, LittleEndian};
use xxhash_rust::xxh64::xxh64;

/// Header occurs once at the beginning of the index
#[derive(Debug, Clone)]
pub struct Header {
    /// Size of values in bytes
    pub value_size: u64,
    /// Number of buckets in the index
    pub num_buckets: u32,
    /// Metadata key-value pairs
    pub metadata: Metadata,
}

impl Header {
    /// Load header from bytes, checking magic and version
    pub fn load(buf: &[u8]) -> Result<(Self, usize), CompactIndexError> {
        // Check magic bytes
        if buf.len() < 8 || buf[..8] != MAGIC {
            return Err(CompactIndexError::InvalidMagic);
        }

        // Read header length
        if buf.len() < 12 {
            return Err(CompactIndexError::InvalidFormat(
                "Buffer too small for header".into(),
            ));
        }
        let header_len = LittleEndian::read_u32(&buf[8..12]) as usize;
        
        if header_len < 12 {
            return Err(CompactIndexError::InvalidFormat(
                "Invalid header length".into(),
            ));
        }

        let total_header_size = 8 + 4 + header_len;
        if buf.len() < total_header_size {
            return Err(CompactIndexError::InvalidFormat(
                "Buffer too small for complete header".into(),
            ));
        }

        // Parse header fields
        let value_size = LittleEndian::read_u64(&buf[12..20]);
        let num_buckets = LittleEndian::read_u32(&buf[20..24]);
        
        // Check version
        if buf.len() < 25 {
            return Err(CompactIndexError::InvalidFormat(
                "No version byte".into(),
            ));
        }
        let version = buf[24];
        if version != VERSION {
            return Err(CompactIndexError::UnsupportedVersion {
                expected: VERSION,
                got: version,
            });
        }

        // Parse metadata
        let mut metadata = Metadata::new();
        if total_header_size > 25 {
            metadata.unmarshal_binary(&buf[25..total_header_size])?;
        }

        // Validate fields
        if value_size == 0 {
            return Err(CompactIndexError::InvalidFormat(
                "Value size not set".into(),
            ));
        }
        if num_buckets == 0 {
            return Err(CompactIndexError::InvalidFormat(
                "Number of buckets not set".into(),
            ));
        }

        Ok((
            Header {
                value_size,
                num_buckets,
                metadata,
            },
            total_header_size,
        ))
    }

    /// Calculate bucket index for a given key
    pub fn bucket_hash(&self, key: &[u8]) -> u32 {
        let h = xxh64(key, 0);
        
        // Based on Go implementation's logic
        let n = self.num_buckets as u64;
        let mut u = h % n;
        
        // Fast mod reduction
        if (h - u) / n < u {
            u = hash_uint64(u);
        }
        
        (u % n) as u32
    }
}

/// Bucket header information
#[derive(Debug, Clone)]
pub struct BucketHeader {
    /// Hash domain for this bucket
    pub hash_domain: u32,
    /// Number of entries in this bucket
    pub num_entries: u32,
    /// Length of hash in bytes (typically 3)
    pub hash_len: u8,
    /// File offset to bucket data
    pub file_offset: u64,
}

impl BucketHeader {
    pub const SIZE: usize = 16;

    /// Load bucket header from bytes
    pub fn load(buf: &[u8; Self::SIZE]) -> Self {
        BucketHeader {
            hash_domain: LittleEndian::read_u32(&buf[0..4]),
            num_entries: LittleEndian::read_u32(&buf[4..8]),
            hash_len: buf[8],
            file_offset: uint_le(&buf[10..16]),
        }
    }

    /// Calculate per-bucket hash of a key
    pub fn hash(&self, key: &[u8]) -> u64 {
        let xsum = entry_hash64(self.hash_domain, key);
        // Mask sum by hash length
        xsum & (u64::MAX >> (64 - self.hash_len * 8))
    }
}

/// xxHash-based hash function using an arbitrary prefix
pub fn entry_hash64(prefix: u32, key: &[u8]) -> u64 {
    const BLOCK_SIZE: usize = 32;
    let mut prefix_block = [0u8; BLOCK_SIZE];
    LittleEndian::write_u32(&mut prefix_block[..4], prefix);
    
    // Combine prefix and key for hashing
    let mut data = Vec::with_capacity(BLOCK_SIZE + key.len());
    data.extend_from_slice(&prefix_block);
    data.extend_from_slice(key);
    
    xxh64(&data, 0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_header_load() {
        // Create a minimal valid header
        let mut buf = Vec::new();
        buf.extend_from_slice(&MAGIC);
        buf.extend_from_slice(&13u32.to_le_bytes()); // header len (8 + 4 + 1 for value_size, num_buckets, version)
        buf.extend_from_slice(&8u64.to_le_bytes()); // value size
        buf.extend_from_slice(&100u32.to_le_bytes()); // num buckets
        buf.push(VERSION); // version
        
        let (header, size) = Header::load(&buf).unwrap();
        assert_eq!(header.value_size, 8);
        assert_eq!(header.num_buckets, 100);
        assert_eq!(size, 8 + 4 + 13); // magic + len + header content
    }

    #[test]
    fn test_invalid_magic() {
        let buf = vec![0; 20];
        assert!(matches!(
            Header::load(&buf),
            Err(CompactIndexError::InvalidMagic)
        ));
    }
}