use thiserror::Error;

/// Magic bytes for compactindexsized format
pub const MAGIC: [u8; 8] = [b'c', b'o', b'm', b'p', b'i', b's', b'z', b'd'];

/// Current version of the index format
pub const VERSION: u8 = 1;

/// Maximum entries per bucket (hardcoded limit)
pub const MAX_ENTRIES_PER_BUCKET: u32 = 1 << 24; // 16M entries

/// Target average entries per bucket
pub const TARGET_ENTRIES_PER_BUCKET: u32 = 10000;

/// Size of the hash prefix used in entries (typically 3 bytes)
pub const HASH_SIZE: usize = 3;

/// Maximum value for uint24 (3 bytes)
pub const MAX_UINT24: u32 = (1 << 24) - 1;

/// Maximum value for uint48 (6 bytes)
pub const MAX_UINT48: u64 = (1 << 48) - 1;

/// An entry in the index
#[derive(Debug, Clone, PartialEq)]
pub struct Entry {
    /// Hash of the key (truncated)
    pub hash: u64,
    /// Value associated with the key
    pub value: Vec<u8>,
}

/// Metadata key-value pairs in the header
#[derive(Debug, Clone, Default)]
pub struct Metadata {
    pairs: Vec<(Vec<u8>, Vec<u8>)>,
}

impl Metadata {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn get(&self, key: &[u8]) -> Option<&[u8]> {
        self.pairs
            .iter()
            .find(|(k, _)| k == key)
            .map(|(_, v)| v.as_slice())
    }

    pub fn insert(&mut self, key: Vec<u8>, value: Vec<u8>) {
        self.pairs.push((key, value));
    }

    pub fn unmarshal_binary(&mut self, data: &[u8]) -> Result<(), CompactIndexError> {
        if data.is_empty() {
            return Ok(());
        }
        
        let mut offset = 0;
        
        // Read number of key-value pairs (1 byte)
        if offset >= data.len() {
            return Ok(());
        }
        let num_kvs = data[offset] as usize;
        offset += 1;
        
        // Read each key-value pair
        for _ in 0..num_kvs {
            // Read key length (1 byte)
            if offset >= data.len() {
                return Err(CompactIndexError::InvalidFormat(
                    "Invalid metadata format: unexpected end".into(),
                ));
            }
            let key_len = data[offset] as usize;
            offset += 1;

            // Read key
            if offset + key_len > data.len() {
                return Err(CompactIndexError::InvalidFormat(
                    "Invalid metadata key length".into(),
                ));
            }
            let key = data[offset..offset + key_len].to_vec();
            offset += key_len;

            // Read value length (1 byte)
            if offset >= data.len() {
                return Err(CompactIndexError::InvalidFormat(
                    "Invalid metadata value length position".into(),
                ));
            }
            let value_len = data[offset] as usize;
            offset += 1;

            // Read value
            if offset + value_len > data.len() {
                return Err(CompactIndexError::InvalidFormat(
                    "Invalid metadata value length".into(),
                ));
            }
            let value = data[offset..offset + value_len].to_vec();
            offset += value_len;

            self.pairs.push((key, value));
        }

        Ok(())
    }
}

/// Errors that can occur when working with compact indexes
#[derive(Error, Debug)]
pub enum CompactIndexError {
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Invalid magic bytes")]
    InvalidMagic,

    #[error("Invalid format: {0}")]
    InvalidFormat(String),

    #[error("Unsupported version: expected {expected}, got {got}")]
    UnsupportedVersion { expected: u8, got: u8 },

    #[error("Key not found")]
    NotFound,

    #[error("Bucket index out of bounds: {index} >= {max}")]
    BucketOutOfBounds { index: u32, max: u32 },

    #[error("Value size mismatch: expected {expected}, got {got}")]
    ValueSizeMismatch { expected: usize, got: usize },
}