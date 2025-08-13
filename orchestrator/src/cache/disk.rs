//! Disk-based cache implementation with atomic operations and concurrent safety.

use super::{
    Cache, CacheConfig, CacheEntry, CacheError, CacheMetadata, CacheOptions, CacheResult,
    CacheStats, create_cache_entry, is_expired,
};
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, SystemTime};
use tokio::fs;
use tokio::io::AsyncWriteExt;
use tokio::sync::{Mutex, RwLock, Semaphore};
use tracing::{debug, error, info, warn};

/// File-based lock implementation for atomic operations
#[derive(Debug)]
struct FileLock {
    lock_path: PathBuf,
}

impl FileLock {
    async fn new(cache_dir: &Path, key: &str) -> CacheResult<Self> {
        let lock_path = cache_dir.join("locks").join(format!("{}.lock", key));
        if let Some(parent) = lock_path.parent() {
            fs::create_dir_all(parent).await?;
        }
        Ok(FileLock { lock_path })
    }

    async fn acquire(&self, timeout: Duration) -> CacheResult<FileLockGuard> {
        let start = std::time::Instant::now();
        
        loop {
            match fs::OpenOptions::new()
                .create_new(true)
                .write(true)
                .open(&self.lock_path)
                .await
            {
                Ok(file) => {
                    return Ok(FileLockGuard {
                        _file: file,
                        lock_path: self.lock_path.clone(),
                    });
                }
                Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => {
                    if start.elapsed() > timeout {
                        return Err(CacheError::lock_failed(&format!(
                            "Failed to acquire lock after {:?}",
                            timeout
                        )));
                    }
                    tokio::time::sleep(Duration::from_millis(10)).await;
                }
                Err(e) => return Err(e.into()),
            }
        }
    }
}

/// Guard for file locks that automatically releases on drop
#[derive(Debug)]
struct FileLockGuard {
    _file: fs::File,
    lock_path: PathBuf,
}

impl Drop for FileLockGuard {
    fn drop(&mut self) {
        // Best effort cleanup - ignore errors since we're in drop
        if let Err(e) = std::fs::remove_file(&self.lock_path) {
            warn!("Failed to remove lock file {:?}: {}", self.lock_path, e);
        }
    }
}

/// Internal statistics tracking
#[derive(Debug)]
struct InternalStats {
    hits: AtomicU64,
    misses: AtomicU64,
    last_cleanup: Mutex<SystemTime>,
}

impl Default for InternalStats {
    fn default() -> Self {
        Self {
            hits: AtomicU64::new(0),
            misses: AtomicU64::new(0),
            last_cleanup: Mutex::new(SystemTime::now()),
        }
    }
}

/// Disk-based cache implementation
#[derive(Debug)]
pub struct DiskCache<T> {
    cache_dir: PathBuf,
    config: CacheConfig,
    stats: InternalStats,
    /// Semaphore to limit concurrent operations
    semaphore: Semaphore,
    /// In-memory index of cache entries for fast lookups
    index: RwLock<HashMap<String, CacheIndexEntry>>,
    _phantom: std::marker::PhantomData<T>,
}

/// Index entry for fast cache lookups
#[derive(Debug, Clone)]
struct CacheIndexEntry {
    file_path: PathBuf,
    created_at: SystemTime,
    expires_at: Option<SystemTime>,
    size_bytes: u64,
    checksum: Option<String>,
}

impl<T> DiskCache<T>
where
    T: Clone + Send + Sync + for<'de> Deserialize<'de> + Serialize,
{
    /// Create a new disk cache instance
    pub async fn new(cache_dir: impl AsRef<Path>, config: CacheConfig) -> CacheResult<Self> {
        let cache_dir = cache_dir.as_ref().to_path_buf();
        
        // Create cache directory structure
        fs::create_dir_all(&cache_dir).await?;
        fs::create_dir_all(cache_dir.join("data")).await?;
        fs::create_dir_all(cache_dir.join("locks")).await?;
        
        let cache = Self {
            cache_dir,
            semaphore: Semaphore::new(config.max_concurrent_ops),
            config,
            stats: InternalStats::default(),
            index: RwLock::new(HashMap::new()),
            _phantom: std::marker::PhantomData,
        };
        
        // Build index from existing files
        cache.rebuild_index().await?;
        
        info!("Disk cache initialized at {:?}", cache.cache_dir);
        Ok(cache)
    }

    /// Get the file path for a cache key
    fn get_cache_file_path(&self, key: &str) -> PathBuf {
        self.cache_dir.join("data").join(format!("{}.json", key))
    }

    /// Validate that a cache key is safe to use as a filename
    fn validate_key(&self, key: &str) -> CacheResult<()> {
        if key.is_empty() {
            return Err(CacheError::invalid_key("Key cannot be empty"));
        }
        
        if key.len() > 255 {
            return Err(CacheError::invalid_key("Key too long"));
        }
        
        // Check for invalid characters (allow colons for namespacing)
        if key.contains(['/', '\\', '*', '?', '"', '<', '>', '|']) {
            return Err(CacheError::invalid_key("Key contains invalid characters"));
        }
        
        Ok(())
    }

    /// Read a cache entry from disk with atomic operation
    async fn read_entry(&self, key: &str) -> CacheResult<Option<CacheEntry<T>>> {
        let _permit = self.semaphore.acquire().await.map_err(|_| {
            CacheError::ConcurrencyLimitExceeded {
                active: self.config.max_concurrent_ops,
                limit: self.config.max_concurrent_ops,
            }
        })?;

        let file_path = self.get_cache_file_path(key);
        
        // Check if file exists
        if !file_path.exists() {
            self.stats.misses.fetch_add(1, Ordering::Relaxed);
            return Ok(None);
        }

        // Acquire lock for reading
        let lock = FileLock::new(&self.cache_dir, key).await?;
        let _guard = lock.acquire(Duration::from_secs(30)).await?;

        // Read and deserialize file
        match fs::read_to_string(&file_path).await {
            Ok(content) => {
                match serde_json::from_str::<CacheEntry<T>>(&content) {
                    Ok(entry) => {
                        // Check if entry has expired
                        if is_expired(&entry) {
                            warn!("Cache entry expired for key: {}", key);
                            // Remove expired entry
                            let _ = fs::remove_file(&file_path).await;
                            self.remove_from_index(key).await;
                            self.stats.misses.fetch_add(1, Ordering::Relaxed);
                            return Err(CacheError::expired(key));
                        }
                        
                        self.stats.hits.fetch_add(1, Ordering::Relaxed);
                        debug!("Cache hit for key: {}", key);
                        Ok(Some(entry))
                    }
                    Err(e) => {
                        error!("Failed to deserialize cache entry for key {}: {}", key, e);
                        // Remove corrupted entry
                        let _ = fs::remove_file(&file_path).await;
                        self.remove_from_index(key).await;
                        self.stats.misses.fetch_add(1, Ordering::Relaxed);
                        Err(CacheError::corrupted(&format!(
                            "Invalid JSON for key {}: {}",
                            key, e
                        )))
                    }
                }
            }
            Err(e) => {
                if e.kind() == std::io::ErrorKind::NotFound {
                    self.stats.misses.fetch_add(1, Ordering::Relaxed);
                    Ok(None)
                } else {
                    error!("Failed to read cache file for key {}: {}", key, e);
                    Err(e.into())
                }
            }
        }
    }

    /// Write a cache entry to disk with atomic operation (temp file + rename)
    async fn write_entry(&self, key: &str, entry: &CacheEntry<T>) -> CacheResult<()> {
        let _permit = self.semaphore.acquire().await.map_err(|_| {
            CacheError::ConcurrencyLimitExceeded {
                active: self.config.max_concurrent_ops,
                limit: self.config.max_concurrent_ops,
            }
        })?;

        let file_path = self.get_cache_file_path(key);
        let temp_path = file_path.with_extension("json.tmp");

        // Acquire lock for writing
        let lock = FileLock::new(&self.cache_dir, key).await?;
        let _guard = lock.acquire(Duration::from_secs(30)).await?;

        // Serialize the entry
        let content = serde_json::to_string_pretty(entry)?;
        
        // Check size limits if configured
        if let Some(max_size) = self.config.max_size_bytes {
            let current_size = self.calculate_cache_size().await?;
            let entry_size = content.len() as u64;
            
            if current_size + entry_size > max_size {
                return Err(CacheError::SizeLimitExceeded {
                    requested: entry_size,
                    limit: max_size - current_size,
                });
            }
        }

        // Write to temporary file first
        let mut temp_file = fs::File::create(&temp_path).await?;
        temp_file.write_all(content.as_bytes()).await?;
        temp_file.sync_all().await?;
        drop(temp_file);

        // Atomic rename
        fs::rename(&temp_path, &file_path).await?;
        
        // Update index
        self.update_index(key, &file_path, entry).await;
        
        debug!("Cache entry written for key: {}", key);
        Ok(())
    }

    /// Update the in-memory index
    async fn update_index(&self, key: &str, file_path: &Path, entry: &CacheEntry<T>) {
        let mut index = self.index.write().await;
        let metadata = fs::metadata(file_path).await.ok();
        let size_bytes = metadata.map(|m| m.len()).unwrap_or(0);
        
        index.insert(
            key.to_string(),
            CacheIndexEntry {
                file_path: file_path.to_path_buf(),
                created_at: entry.created_at,
                expires_at: entry.expires_at,
                size_bytes,
                checksum: entry.metadata.checksum.clone(),
            },
        );
    }

    /// Remove entry from index
    async fn remove_from_index(&self, key: &str) {
        let mut index = self.index.write().await;
        index.remove(key);
    }

    /// Rebuild index from existing files
    async fn rebuild_index(&self) -> CacheResult<()> {
        let data_dir = self.cache_dir.join("data");
        if !data_dir.exists() {
            return Ok(());
        }

        let mut entries = fs::read_dir(&data_dir).await?;
        let mut index = HashMap::new();
        
        while let Some(entry) = entries.next_entry().await? {
            let path = entry.path();
            if path.extension().and_then(|s| s.to_str()) == Some("json") {
                if let Some(stem) = path.file_stem().and_then(|s| s.to_str()) {
                    // Try to read the file to get metadata
                    if let Ok(content) = fs::read_to_string(&path).await {
                        if let Ok(cache_entry) = serde_json::from_str::<CacheEntry<T>>(&content) {
                            let metadata = fs::metadata(&path).await.ok();
                            let size_bytes = metadata.map(|m| m.len()).unwrap_or(0);
                            
                            index.insert(
                                stem.to_string(),
                                CacheIndexEntry {
                                    file_path: path,
                                    created_at: cache_entry.created_at,
                                    expires_at: cache_entry.expires_at,
                                    size_bytes,
                                    checksum: cache_entry.metadata.checksum,
                                },
                            );
                        }
                    }
                }
            }
        }
        
        let mut cache_index = self.index.write().await;
        *cache_index = index;
        
        info!("Rebuilt cache index with {} entries", cache_index.len());
        Ok(())
    }

    /// Calculate total cache size
    async fn calculate_cache_size(&self) -> CacheResult<u64> {
        let index = self.index.read().await;
        Ok(index.values().map(|entry| entry.size_bytes).sum())
    }

    /// Get pattern matching function
    fn matches_pattern(key: &str, pattern: &str) -> bool {
        // Simple glob-style pattern matching
        if pattern == "*" {
            return true;
        }
        
        // Handle patterns with wildcards
        if pattern.contains('*') {
            let parts: Vec<&str> = pattern.split('*').collect();
            
            if parts.is_empty() {
                return true;
            }
            
            // Check first part (if not empty, must be at start)
            if !parts[0].is_empty() && !key.starts_with(parts[0]) {
                return false;
            }
            
            // Check last part (if not empty, must be at end)
            if !parts.last().unwrap().is_empty() && !key.ends_with(parts.last().unwrap()) {
                return false;
            }
            
            // Check middle parts appear in order
            let mut search_start = if parts[0].is_empty() { 0 } else { parts[0].len() };
            
            for part in &parts[1..parts.len() - 1] {
                if !part.is_empty() {
                    if let Some(pos) = key[search_start..].find(part) {
                        search_start = search_start + pos + part.len();
                    } else {
                        return false;
                    }
                }
            }
            
            true
        } else {
            key == pattern
        }
    }
}

#[async_trait]
impl<T> Cache<T> for DiskCache<T>
where
    T: Clone + Send + Sync + for<'de> Deserialize<'de> + Serialize,
{
    async fn get(&self, key: &str) -> CacheResult<Option<T>> {
        self.validate_key(key)?;
        
        match self.read_entry(key).await? {
            Some(entry) => Ok(Some(entry.data)),
            None => Ok(None),
        }
    }

    async fn get_with_metadata(&self, key: &str) -> CacheResult<Option<CacheEntry<T>>> {
        self.validate_key(key)?;
        self.read_entry(key).await
    }

    async fn put(&self, key: &str, value: T, options: CacheOptions) -> CacheResult<()> {
        self.validate_key(key)?;
        
        let metadata = CacheMetadata {
            source: "disk_cache".to_string(),
            size_bytes: None,
            checksum: None,
            tags: options.tags,
        };
        
        let ttl = options.ttl.or(self.config.default_ttl);
        let entry = create_cache_entry(value, ttl, metadata);
        
        self.write_entry(key, &entry).await
    }

    async fn put_with_metadata(&self, key: &str, entry: CacheEntry<T>) -> CacheResult<()> {
        self.validate_key(key)?;
        self.write_entry(key, &entry).await
    }

    async fn contains(&self, key: &str) -> CacheResult<bool> {
        self.validate_key(key)?;
        
        let index = self.index.read().await;
        if let Some(index_entry) = index.get(key) {
            // Check if expired
            if let Some(expires_at) = index_entry.expires_at {
                if SystemTime::now() > expires_at {
                    return Ok(false);
                }
            }
            Ok(true)
        } else {
            Ok(false)
        }
    }

    async fn remove(&self, key: &str) -> CacheResult<bool> {
        self.validate_key(key)?;
        
        let file_path = self.get_cache_file_path(key);
        let lock = FileLock::new(&self.cache_dir, key).await?;
        let _guard = lock.acquire(Duration::from_secs(30)).await?;
        
        let exists = file_path.exists();
        if exists {
            fs::remove_file(&file_path).await?;
            self.remove_from_index(key).await;
            debug!("Removed cache entry for key: {}", key);
        }
        
        Ok(exists)
    }

    async fn clear(&self) -> CacheResult<()> {
        let data_dir = self.cache_dir.join("data");
        if data_dir.exists() {
            fs::remove_dir_all(&data_dir).await?;
            fs::create_dir_all(&data_dir).await?;
        }
        
        let mut index = self.index.write().await;
        index.clear();
        
        info!("Cache cleared");
        Ok(())
    }

    async fn cleanup_expired(&self) -> CacheResult<u64> {
        let now = SystemTime::now();
        let mut removed_count = 0;
        
        // Get list of expired keys from index
        let expired_keys: Vec<String> = {
            let index = self.index.read().await;
            index
                .iter()
                .filter_map(|(key, entry)| {
                    if let Some(expires_at) = entry.expires_at {
                        if now > expires_at {
                            Some(key.clone())
                        } else {
                            None
                        }
                    } else {
                        None
                    }
                })
                .collect()
        };
        
        // Remove expired entries
        for key in expired_keys {
            if self.remove(&key).await? {
                removed_count += 1;
            }
        }
        
        // Update last cleanup time
        *self.stats.last_cleanup.lock().await = now;
        
        if removed_count > 0 {
            info!("Cleaned up {} expired cache entries", removed_count);
        }
        
        Ok(removed_count)
    }

    async fn stats(&self) -> CacheResult<CacheStats> {
        let index = self.index.read().await;
        let entry_count = index.len() as u64;
        let total_size_bytes = index.values().map(|e| e.size_bytes).sum();
        
        // Count expired entries
        let now = SystemTime::now();
        let expired_entries = index
            .values()
            .filter(|entry| {
                entry
                    .expires_at
                    .map(|expires_at| now > expires_at)
                    .unwrap_or(false)
            })
            .count() as u64;
        
        Ok(CacheStats {
            hits: self.stats.hits.load(Ordering::Relaxed),
            misses: self.stats.misses.load(Ordering::Relaxed),
            entry_count,
            total_size_bytes,
            expired_entries,
            last_cleanup: *self.stats.last_cleanup.lock().await,
        })
    }

    async fn keys(&self, pattern: Option<&str>) -> CacheResult<Vec<String>> {
        let index = self.index.read().await;
        
        let keys: Vec<String> = if let Some(pattern) = pattern {
            index
                .keys()
                .filter(|key| Self::matches_pattern(key, pattern))
                .cloned()
                .collect()
        } else {
            index.keys().cloned().collect()
        };
        
        Ok(keys)
    }

    async fn remove_pattern(&self, pattern: &str) -> CacheResult<u64> {
        let keys_to_remove = self.keys(Some(pattern)).await?;
        let mut removed_count = 0;
        
        for key in keys_to_remove {
            if self.remove(&key).await? {
                removed_count += 1;
            }
        }
        
        Ok(removed_count)
    }

    async fn validate(&self) -> CacheResult<Vec<String>> {
        let mut issues = Vec::new();
        let index = self.index.read().await;
        
        for (key, index_entry) in index.iter() {
            // Check if file exists
            if !index_entry.file_path.exists() {
                issues.push(format!("Missing file for key: {}", key));
                continue;
            }
            
            // Try to read and parse the file
            match fs::read_to_string(&index_entry.file_path).await {
                Ok(content) => {
                    if let Err(e) = serde_json::from_str::<CacheEntry<T>>(&content) {
                        issues.push(format!("Corrupted entry for key {}: {}", key, e));
                    }
                }
                Err(e) => {
                    issues.push(format!("Cannot read file for key {}: {}", key, e));
                }
            }
        }
        
        Ok(issues)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[tokio::test]
    async fn test_basic_cache_operations() {
        let temp_dir = TempDir::new().unwrap();
        let config = CacheConfig::default();
        let cache: DiskCache<String> = DiskCache::new(temp_dir.path(), config).await.unwrap();

        // Test put and get
        let key = "test_key";
        let value = "test_value".to_string();
        
        cache
            .put(key, value.clone(), CacheOptions::default())
            .await
            .unwrap();
        
        let retrieved = cache.get(key).await.unwrap();
        assert_eq!(retrieved, Some(value));
        
        // Test contains
        assert!(cache.contains(key).await.unwrap());
        
        // Test remove
        assert!(cache.remove(key).await.unwrap());
        assert!(!cache.contains(key).await.unwrap());
    }

    #[tokio::test]
    async fn test_cache_expiration() {
        let temp_dir = TempDir::new().unwrap();
        let config = CacheConfig::default();
        let cache: DiskCache<String> = DiskCache::new(temp_dir.path(), config).await.unwrap();

        let key = "expire_key";
        let value = "expire_value".to_string();
        
        // Set with very short TTL
        let options = CacheOptions {
            ttl: Some(Duration::from_millis(50)),
            ..Default::default()
        };
        
        cache.put(key, value, options).await.unwrap();
        
        // Should exist initially
        assert!(cache.contains(key).await.unwrap());
        
        // Wait for expiration
        tokio::time::sleep(Duration::from_millis(100)).await;
        
        // Should be expired - contains should return false for expired entries
        assert!(!cache.contains(key).await.unwrap());
        
        // And get should fail with expired error
        let result = cache.get(key).await;
        assert!(result.is_err());
        
        // The error should be an expiration error
        if let Err(e) = result {
            matches!(e, crate::cache::CacheError::Expired(_));
        }
    }

    #[tokio::test]
    async fn test_pattern_matching() {
        assert!(DiskCache::<String>::matches_pattern("test_key", "*"));
        assert!(DiskCache::<String>::matches_pattern("test_key", "test*"));
        assert!(DiskCache::<String>::matches_pattern("test_key", "*key"));
        assert!(DiskCache::<String>::matches_pattern("test_key", "test_key"));
        assert!(!DiskCache::<String>::matches_pattern("test_key", "other*"));
        
        // Test complex patterns
        assert!(DiskCache::<String>::matches_pattern("system:config:main", "*:config:*"));
        assert!(DiskCache::<String>::matches_pattern("system:config:cache", "*:config:*"));
        assert!(!DiskCache::<String>::matches_pattern("user:profile:main", "*:config:*"));
        assert!(DiskCache::<String>::matches_pattern("user:1:profile", "user:*"));
        assert!(!DiskCache::<String>::matches_pattern("system:config:main", "user:*"));
    }
}