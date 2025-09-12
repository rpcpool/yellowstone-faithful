use crate::types::{MAX_UINT24, MAX_UINT48};
use byteorder::{ByteOrder, LittleEndian};

/// Convert a u32 to a 3-byte slice (uint24)
pub fn uint24_to_bytes(v: u32) -> [u8; 3] {
    assert!(v <= MAX_UINT24, "uint24_to_bytes: value out of range");
    let bytes = v.to_le_bytes();
    [bytes[0], bytes[1], bytes[2]]
}

/// Convert a 3-byte slice to u32 (uint24)
pub fn bytes_to_uint24(bytes: &[u8]) -> u32 {
    assert!(bytes.len() >= 3, "bytes_to_uint24: insufficient bytes");
    let mut buf = [0u8; 4];
    buf[..3].copy_from_slice(&bytes[..3]);
    u32::from_le_bytes(buf)
}

/// Convert a u64 to a 6-byte slice (uint48)
pub fn uint48_to_bytes(v: u64) -> [u8; 6] {
    assert!(v <= MAX_UINT48, "uint48_to_bytes: value out of range");
    let bytes = v.to_le_bytes();
    [bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5]]
}

/// Convert a 6-byte slice to u64 (uint48)
pub fn bytes_to_uint48(bytes: &[u8]) -> u64 {
    assert!(bytes.len() >= 6, "bytes_to_uint48: insufficient bytes");
    let mut buf = [0u8; 8];
    buf[..6].copy_from_slice(&bytes[..6]);
    u64::from_le_bytes(buf)
}

/// Decode an unsigned little-endian integer without bounds assertions.
/// Out-of-bounds bits are set to zero.
pub fn uint_le(buf: &[u8]) -> u64 {
    let mut full = [0u8; 8];
    let len = buf.len().min(8);
    full[..len].copy_from_slice(&buf[..len]);
    LittleEndian::read_u64(&full)
}

/// Encode an unsigned little-endian integer without bounds assertions.
pub fn put_uint_le(buf: &mut [u8], x: u64) {
    let full = x.to_le_bytes();
    let len = buf.len().min(8);
    buf[..len].copy_from_slice(&full[..len]);
}

/// Reversible uint64 permutation based on Murmur3 hash finalizer
pub fn hash_uint64(mut x: u64) -> u64 {
    x ^= x >> 33;
    x = x.wrapping_mul(0xff51afd7ed558ccd);
    x ^= x >> 33;
    x = x.wrapping_mul(0xc4ceb9fe1a85ec53);
    x ^= x >> 33;
    x
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_uint24_roundtrip() {
        let values = [0, 1, 255, 256, 65535, 65536, MAX_UINT24];
        for v in values {
            let bytes = uint24_to_bytes(v);
            let decoded = bytes_to_uint24(&bytes);
            assert_eq!(v, decoded);
        }
    }

    #[test]
    fn test_uint48_roundtrip() {
        let values = [0, 1, 255, 256, 65535, 65536, u32::MAX as u64, MAX_UINT48];
        for v in values {
            let bytes = uint48_to_bytes(v);
            let decoded = bytes_to_uint48(&bytes);
            assert_eq!(v, decoded);
        }
    }

    #[test]
    fn test_uint_le() {
        assert_eq!(uint_le(&[0x01]), 1);
        assert_eq!(uint_le(&[0x01, 0x02]), 0x0201);
        assert_eq!(uint_le(&[0x01, 0x02, 0x03, 0x04]), 0x04030201);
    }

    #[test]
    fn test_hash_uint64() {
        // Test that the hash function is deterministic
        let x = 0x123456789abcdef0;
        let h1 = hash_uint64(x);
        let h2 = hash_uint64(x);
        assert_eq!(h1, h2);
        
        // Test that it actually transforms the input
        assert_ne!(x, h1);
    }
}