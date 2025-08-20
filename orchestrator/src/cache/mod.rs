//! Cache module for storing and retrieving scan results from storage backends.
//!
//! This module provides a trait-based caching system that can store scan results
//! (CAR files and index files) to disk and retrieve them efficiently. The cache
//! is designed to survive process restarts and handle concurrent access safely.

use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use std::time::{Duration, SystemTime};

pub mod disk;
pub mod error;

pub use disk::DiskCache;
pub use error::{CacheError, CacheResult};

/// Represents a cached scan result with metadata
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheEntry<T> {
    /// The cached data
    pub data: T,
    /// When this entry was created
    pub created_at: SystemTime,
    /// When this entry expires (if applicable)
    pub expires_at: Option<SystemTime>,
    /// Version or etag of the cached data for validation
    pub version: Option<String>,
    /// Additional metadata about the cache entry
    pub metadata: CacheMetadata,
}

/// Metadata associated with a cache entry
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheMetadata {
    /// The storage backend this came from
    pub source: String,
    /// Size of the original data in bytes
    pub size_bytes: Option<u64>,
    /// Checksum of the original data
    pub checksum: Option<String>,
    /// Any additional tags for categorization
    pub tags: Vec<String>,
}

/// Configuration for cache behavior
#[derive(Debug, Clone)]
pub struct CacheConfig {
    /// Default TTL for cache entries
    pub default_ttl: Option<Duration>,
    /// Maximum size of the cache directory in bytes
    pub max_size_bytes: Option<u64>,
    /// Whether to compress cached data
    pub enable_compression: bool,
    /// Number of concurrent operations allowed
    pub max_concurrent_ops: usize,
}

impl Default for CacheConfig {
    fn default() -> Self {
        Self {
            default_ttl: Some(Duration::from_secs(24 * 60 * 60)), // 24 hours
            max_size_bytes: Some(10 * 1024 * 1024 * 1024), // 10 GB
            enable_compression: true,
            max_concurrent_ops: 10,
        }
    }
}

/// Statistics about cache usage
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheStats {
    /// Total number of cache hits
    pub hits: u64,
    /// Total number of cache misses
    pub misses: u64,
    /// Number of cache entries
    pub entry_count: u64,
    /// Total size of cached data in bytes
    pub total_size_bytes: u64,
    /// Number of expired entries
    pub expired_entries: u64,
    /// Last cleanup time
    pub last_cleanup: SystemTime,
}

impl Default for CacheStats {
    fn default() -> Self {
        Self {
            hits: 0,
            misses: 0,
            entry_count: 0,
            total_size_bytes: 0,
            expired_entries: 0,
            last_cleanup: SystemTime::now(),
        }
    }
}

/// Options for cache operations
#[derive(Debug, Clone, Default)]
pub struct CacheOptions {
    /// Override the default TTL for this entry
    pub ttl: Option<Duration>,
    /// Force refresh even if cached entry exists and is valid
    pub force_refresh: bool,
    /// Skip validation of cached entry
    pub skip_validation: bool,
    /// Additional tags to apply to this entry
    pub tags: Vec<String>,
}

/// Trait defining cache operations
#[async_trait]
pub trait Cache<T>: Send + Sync
where
    T: Clone + Send + Sync + for<'de> Deserialize<'de> + Serialize,
{
    /// Get an entry from the cache by key
    async fn get(&self, key: &str) -> CacheResult<Option<T>>;

    /// Get an entry with full metadata
    async fn get_with_metadata(&self, key: &str) -> CacheResult<Option<CacheEntry<T>>>;

    /// Store an entry in the cache
    async fn put(&self, key: &str, value: T, options: CacheOptions) -> CacheResult<()>;

    /// Store an entry with explicit metadata
    async fn put_with_metadata(&self, key: &str, entry: CacheEntry<T>) -> CacheResult<()>;

    /// Check if a key exists in the cache (without deserializing)
    async fn contains(&self, key: &str) -> CacheResult<bool>;

    /// Remove an entry from the cache
    async fn remove(&self, key: &str) -> CacheResult<bool>;

    /// Clear all entries from the cache
    async fn clear(&self) -> CacheResult<()>;

    /// Remove expired entries
    async fn cleanup_expired(&self) -> CacheResult<u64>;

    /// Get cache statistics
    async fn stats(&self) -> CacheResult<CacheStats>;

    /// Get all keys matching a pattern
    async fn keys(&self, pattern: Option<&str>) -> CacheResult<Vec<String>>;

    /// Remove all entries matching a pattern
    async fn remove_pattern(&self, pattern: &str) -> CacheResult<u64>;

    /// Validate cache integrity
    async fn validate(&self) -> CacheResult<Vec<String>>;
}

/// Helper function to generate cache keys
pub fn generate_cache_key(components: &[&str]) -> String {
    use sha2::{Digest, Sha256};
    
    let combined = components.join(":");
    let mut hasher = Sha256::new();
    hasher.update(combined.as_bytes());
    let hash = hasher.finalize();
    
    // Use first 16 characters of hex hash as key
    format!("{:x}", hash)[..16].to_string()
}

/// Helper function to check if a cache entry has expired
pub fn is_expired(entry: &CacheEntry<impl Clone>) -> bool {
    if let Some(expires_at) = entry.expires_at {
        SystemTime::now() > expires_at
    } else {
        false
    }
}

/// Helper function to create a cache entry with TTL
pub fn create_cache_entry<T>(
    data: T,
    ttl: Option<Duration>,
    metadata: CacheMetadata,
) -> CacheEntry<T> {
    let created_at = SystemTime::now();
    let expires_at = ttl.map(|ttl| created_at + ttl);
    
    CacheEntry {
        data,
        created_at,
        expires_at,
        version: None,
        metadata,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generate_cache_key() {
        let key1 = generate_cache_key(&["storage", "bucket", "file.car"]);
        let key2 = generate_cache_key(&["storage", "bucket", "file.car"]);
        let key3 = generate_cache_key(&["storage", "bucket", "other.car"]);
        
        assert_eq!(key1, key2);
        assert_ne!(key1, key3);
        assert_eq!(key1.len(), 16);
    }

    #[test]
    fn test_is_expired() {
        let metadata = CacheMetadata {
            source: "test".to_string(),
            size_bytes: None,
            checksum: None,
            tags: vec![],
        };
        
        // Not expired (no expiration)
        let entry1 = CacheEntry {
            data: "test".to_string(),
            created_at: SystemTime::now(),
            expires_at: None,
            version: None,
            metadata: metadata.clone(),
        };
        assert!(!is_expired(&entry1));
        
        // Not expired (expires in future)
        let entry2 = CacheEntry {
            data: "test".to_string(),
            created_at: SystemTime::now(),
            expires_at: Some(SystemTime::now() + Duration::from_secs(3600)),
            version: None,
            metadata: metadata.clone(),
        };
        assert!(!is_expired(&entry2));
        
        // Expired
        let entry3 = CacheEntry {
            data: "test".to_string(),
            created_at: SystemTime::now() - Duration::from_secs(7200),
            expires_at: Some(SystemTime::now() - Duration::from_secs(3600)),
            version: None,
            metadata,
        };
        assert!(is_expired(&entry3));
    }
}