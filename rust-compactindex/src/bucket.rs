use crate::header::BucketHeader;
use crate::types::{CompactIndexError, Entry};
use crate::utils::uint_le;
use std::io::{Read, Seek, SeekFrom};

/// Bucket descriptor with stride information
#[derive(Debug, Clone)]
pub struct BucketDescriptor {
    pub header: BucketHeader,
    /// Size of one entry in bytes
    pub stride: u8,
    /// Width of value field in bytes
    pub value_width: u8,
}

impl BucketDescriptor {
    pub fn new(header: BucketHeader, value_size: u64) -> Self {
        let value_width = value_size as u8;
        let stride = header.hash_len + value_width;
        
        BucketDescriptor {
            header,
            stride,
            value_width,
        }
    }

    /// Unmarshal an entry from bytes
    pub fn unmarshal_entry(&self, buf: &[u8]) -> Entry {
        let hash_len = self.header.hash_len as usize;
        let hash = uint_le(&buf[..hash_len]);
        let value = buf[hash_len..hash_len + self.value_width as usize].to_vec();
        
        Entry { hash, value }
    }

    /// Marshal an entry to bytes
    pub fn marshal_entry(&self, entry: &Entry, buf: &mut [u8]) {
        assert!(buf.len() >= self.stride as usize, "Buffer too small");
        
        // Write hash (truncated to hash_len bytes)
        let hash_bytes = entry.hash.to_le_bytes();
        let hash_len = self.header.hash_len as usize;
        buf[..hash_len].copy_from_slice(&hash_bytes[..hash_len]);
        
        // Write value
        buf[hash_len..hash_len + self.value_width as usize]
            .copy_from_slice(&entry.value);
    }
}

/// Bucket provides access to entries within a bucket
pub struct Bucket<R> {
    pub descriptor: BucketDescriptor,
    pub reader: R,
    pub offset: u64,
}

impl<R: Read + Seek> Bucket<R> {
    pub fn new(descriptor: BucketDescriptor, reader: R, offset: u64) -> Self {
        Bucket {
            descriptor,
            reader,
            offset,
        }
    }

    /// Load all entries from the bucket
    pub fn load_all(&mut self) -> Result<Vec<Entry>, CompactIndexError> {
        let stride = self.descriptor.stride as usize;
        let num_entries = self.descriptor.header.num_entries as usize;
        let total_size = stride * num_entries;
        
        let mut buf = vec![0u8; total_size];
        self.reader.seek(SeekFrom::Start(self.offset))?;
        self.reader.read_exact(&mut buf)?;
        
        let mut entries = Vec::with_capacity(num_entries);
        for i in 0..num_entries {
            let start = i * stride;
            let entry_buf = &buf[start..start + stride];
            entries.push(self.descriptor.unmarshal_entry(entry_buf));
        }
        
        Ok(entries)
    }

    /// Load a single entry at index
    pub fn load_entry(&mut self, index: usize) -> Result<Entry, CompactIndexError> {
        if index >= self.descriptor.header.num_entries as usize {
            return Err(CompactIndexError::InvalidFormat(
                "Entry index out of bounds".into(),
            ));
        }
        
        let stride = self.descriptor.stride as usize;
        let offset = self.offset + (index * stride) as u64;
        
        let mut buf = vec![0u8; stride];
        self.reader.seek(SeekFrom::Start(offset))?;
        self.reader.read_exact(&mut buf)?;
        
        Ok(self.descriptor.unmarshal_entry(&buf))
    }

    /// Helper to load all entries in the bucket into memory
    fn load_all_entries(&mut self) -> Result<Vec<Entry>, CompactIndexError> {
        let num_entries = self.descriptor.header.num_entries as usize;
        let stride = self.descriptor.stride as usize;
        let total_size = num_entries * stride;
        let mut buf = vec![0u8; total_size];
        self.reader.seek(SeekFrom::Start(self.offset))?;
        self.reader.read_exact(&mut buf)?;
        let mut entries = Vec::with_capacity(num_entries);
        for i in 0..num_entries {
            let start = i * stride;
            let end = start + stride;
            let entry_buf = &buf[start..end];
            entries.push(self.descriptor.unmarshal_entry(entry_buf));
        }
        Ok(entries)
    }

    /// Lookup a key in the bucket using Eytzinger binary search
    pub fn lookup(&mut self, key: &[u8]) -> Result<Vec<u8>, CompactIndexError> {
        let target = self.descriptor.header.hash(key);
        let max = self.descriptor.header.num_entries as usize;
        let entries = self.load_all_entries()?;

        // Eytzinger binary search
        let mut index = 0;
        while index < max {
            let entry = &entries[index];

            if entry.hash == target {
                return Ok(entry.value.clone());
            }

            // Navigate the Eytzinger tree
            index = (index << 1) | 1;
            if entry.hash < target {
                index += 1;
            }

            // Check if we've gone out of bounds
            if index >= max {
                break;
            }
        }
        Err(CompactIndexError::NotFound)
    }
}

/// Search sorted entries using standard binary search (for in-memory data)
pub fn search_sorted_entries(entries: &[Entry], hash: u64) -> Option<&Entry> {
    entries.binary_search_by_key(&hash, |e| e.hash)
        .ok()
        .and_then(|i| entries.get(i))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_bucket_descriptor() {
        let header = BucketHeader {
            hash_domain: 42,
            num_entries: 10,
            hash_len: 3,
            file_offset: 1000,
        };
        
        let desc = BucketDescriptor::new(header, 8);
        assert_eq!(desc.stride, 11); // 3 (hash) + 8 (value)
        assert_eq!(desc.value_width, 8);
    }

    #[test]
    fn test_entry_marshal_unmarshal() {
        let header = BucketHeader {
            hash_domain: 0,
            num_entries: 1,
            hash_len: 3,
            file_offset: 0,
        };
        
        let desc = BucketDescriptor::new(header, 4);
        
        let entry = Entry {
            hash: 0x123456,
            value: vec![0xAB, 0xCD, 0xEF, 0x01],
        };
        
        let mut buf = vec![0u8; desc.stride as usize];
        desc.marshal_entry(&entry, &mut buf);
        
        let decoded = desc.unmarshal_entry(&buf);
        assert_eq!(decoded.hash, 0x123456);
        assert_eq!(decoded.value, vec![0xAB, 0xCD, 0xEF, 0x01]);
    }
}