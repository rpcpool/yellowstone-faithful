//! Rust implementation of compactindexsized format for yellowstone-faithful
//! 
//! This crate provides read access to compact indexes used by yellowstone-faithful
//! for efficient lookups of blockchain data.

pub mod bucket;
pub mod header;
pub mod indexes;
pub mod reader;
pub mod types;
pub mod utils;

pub use reader::CompactIndexReader;
pub use types::{CompactIndexError, Entry, Metadata};

// Re-export index-specific types
pub use indexes::{CidToOffsetAndSize, SlotToCid, SigToCid};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_library_exports() {
        // Ensure main types are accessible
        let _ = std::mem::size_of::<CompactIndexReader>();
        let _ = std::mem::size_of::<Entry>();
    }
}