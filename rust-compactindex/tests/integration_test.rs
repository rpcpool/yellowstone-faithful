use yellowstone_compactindex::{CompactIndexReader, CompactIndexError};

/// Create a minimal valid index for testing
fn create_test_index_data() -> Vec<u8> {
    use std::collections::HashMap;
    
    let mut data = Vec::new();
    
    // Magic bytes: 'compiszd'
    data.extend_from_slice(b"compiszd");
    
    // Header length (minimal: 8 bytes value_size + 4 bytes num_buckets + 1 byte version = 13)
    data.extend_from_slice(&13u32.to_le_bytes());
    
    // Value size (4 bytes per value)
    data.extend_from_slice(&4u64.to_le_bytes());
    
    // Number of buckets (2 buckets for this test)
    data.extend_from_slice(&2u32.to_le_bytes());
    
    // Version (1)
    data.push(1);
    
    // Bucket header table (2 buckets * 16 bytes each = 32 bytes)
    let bucket_data_start = data.len() + 32; // After both bucket headers
    
    // Bucket 0 header
    data.extend_from_slice(&0u32.to_le_bytes()); // hash domain
    data.extend_from_slice(&2u32.to_le_bytes()); // num entries
    data.push(3); // hash length
    data.push(0); // padding
    // File offset (48-bit)
    let offset0 = bucket_data_start as u64;
    data.extend_from_slice(&offset0.to_le_bytes()[..6]);
    
    // Bucket 1 header  
    data.extend_from_slice(&1u32.to_le_bytes()); // hash domain
    data.extend_from_slice(&1u32.to_le_bytes()); // num entries
    data.push(3); // hash length
    data.push(0); // padding
    // File offset (48-bit)
    let offset1 = (bucket_data_start + 14) as u64; // 2 entries * 7 bytes each
    data.extend_from_slice(&offset1.to_le_bytes()[..6]);
    
    // Bucket 0 data (2 entries, stride = 3 + 4 = 7 bytes each)
    // Entry 1: hash=0x111111, value=[1,2,3,4]
    data.extend_from_slice(&[0x11, 0x11, 0x11]);
    data.extend_from_slice(&[1, 2, 3, 4]);
    
    // Entry 2: hash=0x222222, value=[5,6,7,8]
    data.extend_from_slice(&[0x22, 0x22, 0x22]);
    data.extend_from_slice(&[5, 6, 7, 8]);
    
    // Bucket 1 data (1 entry)
    // Entry 1: hash=0x333333, value=[9,10,11,12]
    data.extend_from_slice(&[0x33, 0x33, 0x33]);
    data.extend_from_slice(&[9, 10, 11, 12]);
    
    data
}

#[test]
fn test_read_multi_bucket_index() {
    let data = create_test_index_data();
    let reader = CompactIndexReader::from_slice(&data).unwrap();
    
    // Test basic properties
    assert_eq!(reader.value_size(), 4);
    assert_eq!(reader.num_buckets(), 2);
    
    // Test bucket iteration
    let bucket_indices: Vec<u32> = reader.bucket_indices().collect();
    assert_eq!(bucket_indices, vec![0, 1]);
    
    // Test getting bucket 0
    let bucket0 = reader.get_bucket(0).unwrap();
    assert_eq!(bucket0.descriptor.header.num_entries, 2);
    assert_eq!(bucket0.descriptor.header.hash_domain, 0);
    
    // Test getting bucket 1
    let bucket1 = reader.get_bucket(1).unwrap();
    assert_eq!(bucket1.descriptor.header.num_entries, 1);
    assert_eq!(bucket1.descriptor.header.hash_domain, 1);
    
    // Test out of bounds bucket
    let result = reader.get_bucket(2);
    assert!(matches!(result, Err(CompactIndexError::BucketOutOfBounds { .. })));
}

#[test]
fn test_prefetch_functionality() {
    let data = create_test_index_data();
    let mut reader = CompactIndexReader::from_slice(&data).unwrap();
    
    // Test that prefetch can be enabled
    reader.set_prefetch(true);
    
    // Getting a bucket with prefetch enabled should work
    let bucket = reader.get_bucket(0);
    assert!(bucket.is_ok());
    
    // Disable prefetch
    reader.set_prefetch(false);
    
    // Should still work without prefetch
    let bucket = reader.get_bucket(1);
    assert!(bucket.is_ok());
}

#[test]
fn test_header_validation() {
    // Test with invalid magic
    let mut bad_data = vec![0u8; 20];
    bad_data[8..12].copy_from_slice(&13u32.to_le_bytes());
    let result = CompactIndexReader::from_slice(&bad_data);
    assert!(matches!(result, Err(CompactIndexError::InvalidMagic)));
    
    // Test with valid magic but invalid version
    let mut bad_version_data = Vec::new();
    bad_version_data.extend_from_slice(b"compiszd");
    bad_version_data.extend_from_slice(&13u32.to_le_bytes());
    bad_version_data.extend_from_slice(&4u64.to_le_bytes());
    bad_version_data.extend_from_slice(&1u32.to_le_bytes());
    bad_version_data.push(99); // Invalid version
    
    let result = CompactIndexReader::from_slice(&bad_version_data);
    assert!(matches!(result, Err(CompactIndexError::UnsupportedVersion { .. })));
}