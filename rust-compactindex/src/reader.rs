use crate::bucket::{Bucket, BucketDescriptor};
use crate::header::{BucketHeader, Header};
use crate::types::CompactIndexError;
use memmap2::Mmap;
use std::fs::File;
use std::io::Cursor;
use std::path::Path;

/// Reader for compact index files using memory mapping
pub struct CompactIndexReader {
    header: Header,
    header_size: usize,
    mmap: Mmap,
    prefetch: bool,
}

/// Reader for compact index from in-memory data
pub struct CompactIndexReaderSlice {
    header: Header,
    header_size: usize,
    data: Vec<u8>,
    prefetch: bool,
}

impl CompactIndexReader {
    /// Open an index file from disk using memory mapping
    pub fn open<P: AsRef<Path>>(path: P) -> Result<Self, CompactIndexError> {
        let file = File::open(path)?;
        let mmap = unsafe { Mmap::map(&file)? };
        
        let (header, header_size) = Header::load(&mmap)?;
        
        Ok(CompactIndexReader {
            header,
            header_size,
            mmap,
            prefetch: false,
        })
    }

    /// Open an index from a byte slice (useful for testing and in-memory data)
    pub fn from_slice(data: &[u8]) -> Result<CompactIndexReaderSlice, CompactIndexError> {
        let (header, header_size) = Header::load(data)?;
        
        Ok(CompactIndexReaderSlice {
            header,
            header_size,
            data: data.to_vec(),
            prefetch: false,
        })
    }

    /// Get the header
    pub fn header(&self) -> &Header {
        &self.header
    }

    /// Get value size in bytes
    pub fn value_size(&self) -> u64 {
        self.header.value_size
    }

    /// Get number of buckets
    pub fn num_buckets(&self) -> u32 {
        self.header.num_buckets
    }

    /// Get metadata value by key
    pub fn get_metadata(&self, key: &[u8]) -> Option<&[u8]> {
        self.header.metadata.get(key)
    }

    /// Enable or disable prefetching for bucket operations
    pub fn set_prefetch(&mut self, enabled: bool) {
        self.prefetch = enabled;
    }

    /// Calculate bucket offset in file
    fn bucket_offset(&self, index: u32) -> u64 {
        self.header_size as u64 + (index as u64) * BucketHeader::SIZE as u64
    }

    /// Get a bucket by index
    pub fn get_bucket(&self, index: u32) -> Result<Bucket<Cursor<&[u8]>>, CompactIndexError> {
        if index >= self.header.num_buckets {
            return Err(CompactIndexError::BucketOutOfBounds {
                index,
                max: self.header.num_buckets,
            });
        }

        // Read bucket header
        let bucket_header_offset = self.bucket_offset(index) as usize;
        if bucket_header_offset + BucketHeader::SIZE > self.mmap.len() {
            return Err(CompactIndexError::InvalidFormat(
                "Bucket header offset out of bounds".into(),
            ));
        }

        let mut header_buf = [0u8; BucketHeader::SIZE];
        header_buf.copy_from_slice(
            &self.mmap[bucket_header_offset..bucket_header_offset + BucketHeader::SIZE]
        );
        let bucket_header = BucketHeader::load(&header_buf);

        // Create bucket descriptor
        let descriptor = BucketDescriptor::new(bucket_header.clone(), self.header.value_size);

        // Calculate data range
        let data_start = bucket_header.file_offset as usize;
        let data_size = (bucket_header.num_entries as usize) * (descriptor.stride as usize);
        let data_end = data_start + data_size;

        if data_end > self.mmap.len() {
            return Err(CompactIndexError::InvalidFormat(
                "Bucket data out of bounds".into(),
            ));
        }

        // Create cursor for bucket data
        let data = &self.mmap[data_start..data_end];
        
        // If prefetch is enabled, we trigger OS page cache by reading a portion
        if self.prefetch {
            // Prefetch up to 3000 entries or all entries, whichever is smaller
            let entries_to_prefetch = std::cmp::min(3000, bucket_header.num_entries as usize);
            let prefetch_size = entries_to_prefetch * descriptor.stride as usize;
            let prefetch_end = std::cmp::min(data_start + prefetch_size, data_end);
            
            // Touch the memory to trigger OS prefetching
            if prefetch_end > data_start {
                let _ = &self.mmap[data_start..prefetch_end];
            }
        }
        
        let cursor = Cursor::new(data);
        Ok(Bucket::new(descriptor, cursor, 0))
    }

    /// Lookup a key in the index
    pub fn lookup(&self, key: &[u8]) -> Result<Vec<u8>, CompactIndexError> {
        // Find which bucket contains this key
        let bucket_index = self.header.bucket_hash(key);
        
        // Get the bucket and search within it
        let mut bucket = self.get_bucket(bucket_index)?;
        bucket.lookup(key)
    }

    /// Get the bucket that might contain the given key
    pub fn lookup_bucket(&self, key: &[u8]) -> Result<Bucket<Cursor<&[u8]>>, CompactIndexError> {
        let bucket_index = self.header.bucket_hash(key);
        self.get_bucket(bucket_index)
    }

    /// Iterate over all bucket indices
    pub fn bucket_indices(&self) -> impl Iterator<Item = u32> {
        0..self.header.num_buckets
    }
}

impl CompactIndexReaderSlice {
    /// Get the header
    pub fn header(&self) -> &Header {
        &self.header
    }

    /// Get value size in bytes
    pub fn value_size(&self) -> u64 {
        self.header.value_size
    }

    /// Get number of buckets
    pub fn num_buckets(&self) -> u32 {
        self.header.num_buckets
    }

    /// Get metadata value by key
    pub fn get_metadata(&self, key: &[u8]) -> Option<&[u8]> {
        self.header.metadata.get(key)
    }

    /// Enable or disable prefetching for bucket operations
    pub fn set_prefetch(&mut self, enabled: bool) {
        self.prefetch = enabled;
    }

    /// Calculate bucket offset in file
    fn bucket_offset(&self, index: u32) -> u64 {
        self.header_size as u64 + (index as u64) * BucketHeader::SIZE as u64
    }

    /// Get a bucket by index
    pub fn get_bucket(&self, index: u32) -> Result<Bucket<Cursor<&[u8]>>, CompactIndexError> {
        if index >= self.header.num_buckets {
            return Err(CompactIndexError::BucketOutOfBounds {
                index,
                max: self.header.num_buckets,
            });
        }

        // Read bucket header
        let bucket_header_offset = self.bucket_offset(index) as usize;
        if bucket_header_offset + BucketHeader::SIZE > self.data.len() {
            return Err(CompactIndexError::InvalidFormat(
                "Bucket header offset out of bounds".into(),
            ));
        }

        let mut header_buf = [0u8; BucketHeader::SIZE];
        header_buf.copy_from_slice(
            &self.data[bucket_header_offset..bucket_header_offset + BucketHeader::SIZE]
        );
        let bucket_header = BucketHeader::load(&header_buf);

        // Create bucket descriptor
        let descriptor = BucketDescriptor::new(bucket_header.clone(), self.header.value_size);

        // Calculate data range
        let data_start = bucket_header.file_offset as usize;
        let data_size = (bucket_header.num_entries as usize) * (descriptor.stride as usize);
        let data_end = data_start + data_size;

        if data_end > self.data.len() {
            return Err(CompactIndexError::InvalidFormat(
                "Bucket data out of bounds".into(),
            ));
        }

        // Create cursor for bucket data
        let data = &self.data[data_start..data_end];
        
        // If prefetch is enabled, we trigger cache warming by touching memory
        if self.prefetch {
            // Prefetch up to 3000 entries or all entries, whichever is smaller
            let entries_to_prefetch = std::cmp::min(3000, bucket_header.num_entries as usize);
            let prefetch_size = entries_to_prefetch * descriptor.stride as usize;
            let prefetch_end = std::cmp::min(data_start + prefetch_size, data_end);
            
            // Touch the memory to warm cache
            if prefetch_end > data_start {
                let _ = &self.data[data_start..prefetch_end];
            }
        }
        
        let cursor = Cursor::new(data);
        Ok(Bucket::new(descriptor, cursor, 0))
    }

    /// Lookup a key in the index
    pub fn lookup(&self, key: &[u8]) -> Result<Vec<u8>, CompactIndexError> {
        // Find which bucket contains this key
        let bucket_index = self.header.bucket_hash(key);
        
        // Get the bucket and search within it
        let mut bucket = self.get_bucket(bucket_index)?;
        bucket.lookup(key)
    }

    /// Get the bucket that might contain the given key
    pub fn lookup_bucket(&self, key: &[u8]) -> Result<Bucket<Cursor<&[u8]>>, CompactIndexError> {
        let bucket_index = self.header.bucket_hash(key);
        self.get_bucket(bucket_index)
    }

    /// Iterate over all bucket indices
    pub fn bucket_indices(&self) -> impl Iterator<Item = u32> {
        0..self.header.num_buckets
    }
}

/// Entry stride calculation
#[allow(dead_code)]
fn entry_stride(value_size: u64) -> u8 {
    crate::types::HASH_SIZE as u8 + value_size as u8
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{MAGIC, VERSION};
    #[allow(unused_imports)]
    use byteorder::{ByteOrder, LittleEndian};

    fn create_test_index() -> Vec<u8> {
        let mut data = Vec::new();
        
        // Write header
        data.extend_from_slice(&MAGIC);
        data.extend_from_slice(&13u32.to_le_bytes()); // header length (8 + 4 + 1)
        data.extend_from_slice(&4u64.to_le_bytes()); // value size = 4
        data.extend_from_slice(&1u32.to_le_bytes()); // num buckets = 1
        data.push(VERSION);
        
        // Write bucket header table (1 bucket)
        let bucket_header_start = data.len();
        data.extend_from_slice(&0u32.to_le_bytes()); // hash domain
        data.extend_from_slice(&2u32.to_le_bytes()); // num entries = 2
        data.push(3); // hash len = 3
        data.push(0); // padding
        
        // File offset for bucket data (right after headers)
        let bucket_data_offset = bucket_header_start + BucketHeader::SIZE;
        let offset_bytes = crate::utils::uint48_to_bytes(bucket_data_offset as u64);
        data.extend_from_slice(&offset_bytes);
        
        // Write bucket data (2 entries, stride = 3 + 4 = 7)
        // Entry 1: hash=0x111111, value=[1,2,3,4]
        data.extend_from_slice(&[0x11, 0x11, 0x11]); // hash
        data.extend_from_slice(&[1, 2, 3, 4]); // value
        
        // Entry 2: hash=0x222222, value=[5,6,7,8]
        data.extend_from_slice(&[0x22, 0x22, 0x22]); // hash
        data.extend_from_slice(&[5, 6, 7, 8]); // value
        
        data
    }

    #[test]
    fn test_reader_basic() {
        let data = create_test_index();
        let reader = CompactIndexReader::from_slice(&data).unwrap();
        
        assert_eq!(reader.value_size(), 4);
        assert_eq!(reader.num_buckets(), 1);
    }

    #[test]
    fn test_bucket_iteration() {
        let data = create_test_index();
        let reader = CompactIndexReader::from_slice(&data).unwrap();
        
        // Test bucket indices iteration
        let indices: Vec<u32> = reader.bucket_indices().collect();
        assert_eq!(indices.len(), 1);
        assert_eq!(indices[0], 0);
        
        // Test that we can get each bucket
        for idx in reader.bucket_indices() {
            let bucket = reader.get_bucket(idx);
            assert!(bucket.is_ok());
        }
    }
}