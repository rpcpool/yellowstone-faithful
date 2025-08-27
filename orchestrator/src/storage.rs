use anyhow::{Context, Result};
use async_trait::async_trait;
use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::RwLock;
use tracing::{debug, info, warn};
use walkdir::WalkDir;

use crate::cache::{Cache, CacheOptions, DiskCache, generate_cache_key};
use crate::car_report::CarReportData;
use crate::config::{HttpStorageConfig, LocalStorageConfig, StorageConfig};
use serde::{Deserialize, Serialize};

/// Types of indexes used by Old Faithful
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize, Deserialize)]
pub enum IndexType {
    SlotToCid,
    SigToCid,
    CidToOffsetAndSize,
    SigExists,
    Gsfa,
}

/// Helper functions for cache key generation
fn generate_storage_cache_key(storage_id: &str, scan_type: &str) -> String {
    generate_cache_key(&["storage", storage_id, scan_type])
}

/// Generate cache key for CAR files scan - includes epoch range in key
fn generate_car_files_cache_key(storage_config: &StorageConfig) -> String {
    let (storage_id, epoch_range) = match storage_config {
        StorageConfig::Local(config) => {
            let range = config.epoch_range.as_ref()
                .map(|r| format!("_{}-{}", r.start, r.end))
                .unwrap_or_else(|| "_all".to_string());
            (format!("local:{}", config.path), range)
        },
        StorageConfig::Http(config) => {
            let range = config.epoch_range.as_ref()
                .map(|r| format!("_{}-{}", r.start, r.end))
                .unwrap_or_else(|| "_all".to_string());
            (format!("http:{}", config.url), range)
        },
    };
    generate_storage_cache_key(&format!("{}{}", storage_id, epoch_range), "car_files")
}

/// Generate cache key for index files scan - includes epoch range in key
fn generate_index_files_cache_key(storage_config: &StorageConfig) -> String {
    let (storage_id, epoch_range) = match storage_config {
        StorageConfig::Local(config) => {
            let range = config.epoch_range.as_ref()
                .map(|r| format!("_{}-{}", r.start, r.end))
                .unwrap_or_else(|| "_all".to_string());
            (format!("local:{}", config.path), range)
        },
        StorageConfig::Http(config) => {
            let range = config.epoch_range.as_ref()
                .map(|r| format!("_{}-{}", r.start, r.end))
                .unwrap_or_else(|| "_all".to_string());
            (format!("http:{}", config.url), range)
        },
    };
    generate_storage_cache_key(&format!("{}{}", storage_id, epoch_range), "index_files")
}

impl IndexType {
    /// Get the expected file suffix for this index type
    pub fn file_suffix(&self) -> &str {
        match self {
            IndexType::SlotToCid => "slot-to-cid.index",
            IndexType::SigToCid => "sig-to-cid.index", 
            IndexType::CidToOffsetAndSize => "cid-to-offset-and-size.index",
            IndexType::SigExists => "sig-exists.index",
            IndexType::Gsfa => "gsfa.index.tar.zstd", // GSFA is compressed
        }
    }
    
    /// Check if a filename matches this index type
    pub fn matches_filename(&self, filename: &str) -> bool {
        filename.ends_with(self.file_suffix())
    }
    
    /// Get all index types
    pub fn all() -> Vec<IndexType> {
        vec![
            IndexType::SlotToCid,
            IndexType::SigToCid,
            IndexType::CidToOffsetAndSize,
            IndexType::SigExists,
            IndexType::Gsfa,
        ]
    }
}

/// Represents a discovered CAR file with its epoch number
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CarFile {
    pub epoch: u64,
    pub path: String,
    pub size: Option<u64>,
    pub cid: Option<String>,
}

/// Represents a discovered index file
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IndexFile {
    pub epoch: u64,
    pub index_type: IndexType,
    pub path: String,
    pub size: Option<u64>,
}

/// Trait for storage backends that can discover and validate CAR files and indexes
#[async_trait]
pub trait StorageBackend: Send + Sync {
    /// Scan the storage for CAR files
    async fn scan_car_files(&self) -> Result<Vec<CarFile>>;
    
    /// Scan the storage for index files
    async fn scan_index_files(&self) -> Result<Vec<IndexFile>>;
    
    /// Check if a specific CAR file exists
    async fn car_file_exists(&self, epoch: u64) -> Result<bool>;
    
    /// Get storage identifier for logging
    fn identifier(&self) -> String;
}

/// Local filesystem storage backend
pub struct LocalStorage {
    path: PathBuf,
    epoch_range: Option<(u64, u64)>,
    cache: Option<Arc<DiskCache<Vec<CarFile>>>>,
    index_cache: Option<Arc<DiskCache<Vec<IndexFile>>>>,
    config: LocalStorageConfig,
}

impl LocalStorage {
    pub fn new(config: &LocalStorageConfig) -> Result<Self> {
        let path = PathBuf::from(&config.path);
        let epoch_range = config.epoch_range.as_ref().map(|r| (r.start, r.end));
        Ok(LocalStorage { 
            path, 
            epoch_range,
            cache: None,
            index_cache: None,
            config: config.clone(),
        })
    }
    
    /// Create LocalStorage with cache support
    pub fn with_cache(
        config: &LocalStorageConfig,
        cache: Option<Arc<DiskCache<Vec<CarFile>>>>,
        index_cache: Option<Arc<DiskCache<Vec<IndexFile>>>>,
    ) -> Result<Self> {
        let path = PathBuf::from(&config.path);
        let epoch_range = config.epoch_range.as_ref().map(|r| (r.start, r.end));
        Ok(LocalStorage { 
            path, 
            epoch_range,
            cache,
            index_cache,
            config: config.clone(),
        })
    }
    
    fn is_epoch_in_range(&self, epoch: u64) -> bool {
        match self.epoch_range {
            Some((start, end)) => epoch >= start && epoch <= end,
            None => true, // No range specified means all epochs
        }
    }
    
    fn parse_epoch_from_car_path(path: &std::path::Path) -> Option<u64> {
        // Expected structure: {storagePath}/{epochNumber}/epoch-{epochNumber}.car
        // Get the filename
        let filename = path.file_name()?.to_str()?;
        
        // Check if it matches epoch-{number}.car pattern
        if let Some(name) = filename.strip_suffix(".car") {
            if let Some(epoch_str) = name.strip_prefix("epoch-") {
                if let Ok(epoch) = epoch_str.parse::<u64>() {
                    // Verify the parent directory also matches the epoch number
                    if let Some(parent) = path.parent() {
                        if let Some(parent_name) = parent.file_name() {
                            if let Ok(parent_epoch) = parent_name.to_str().unwrap_or("").parse::<u64>() {
                                if parent_epoch == epoch {
                                    return Some(epoch);
                                }
                            }
                        }
                    }
                    // If no parent directory or it doesn't match, still accept the file
                    return Some(epoch);
                }
            }
        }
        None
    }
    
    fn parse_epoch_from_index_path(path: &std::path::Path) -> Option<(u64, IndexType, Option<String>)> {
        // Expected structure: 
        // - Most indexes: {storagePath}/{epochNumber}/epoch-{epochNumber}-{cid}-mainnet-{indexNameInKebabCase}.index
        // - GSFA: {storagePath}/{epochNumber}/epoch-{epochNumber}-gsfa.index.tar.zstd
        
        // First, get the filename
        let filename = path.file_name()?.to_str()?;
        
        // Check which index type this is
        for index_type in IndexType::all() {
            if index_type.matches_filename(filename) {
                // Parse the filename for epoch and CID
                if let Some(start) = filename.find("epoch-") {
                    let after_epoch = &filename[start + 6..];
                    if let Some(first_dash) = after_epoch.find('-') {
                        if let Ok(epoch) = after_epoch[..first_dash].parse::<u64>() {
                            // Verify the parent directory matches the epoch number
                            if let Some(parent) = path.parent() {
                                if let Some(parent_name) = parent.file_name() {
                                    if let Ok(parent_epoch) = parent_name.to_str().unwrap_or("").parse::<u64>() {
                                        if parent_epoch != epoch {
                                            // Parent directory doesn't match epoch number
                                            continue;
                                        }
                                    }
                                }
                            }
                            
                            // Special case for GSFA - it doesn't include CID
                            if matches!(index_type, IndexType::Gsfa) {
                                return Some((epoch, index_type, None));
                            }
                            
                            // Extract CID if present (it comes before -mainnet-)
                            let mut cid = None;
                            if let Some(mainnet_pos) = filename.find("-mainnet-") {
                                // CID is between epoch number and -mainnet-
                                let between = &filename[start + 6 + first_dash + 1..mainnet_pos];
                                if !between.is_empty() {
                                    cid = Some(between.to_string());
                                }
                            }
                            return Some((epoch, index_type, cid));
                        }
                    }
                }
            }
        }
        None
    }
    
    async fn read_cid_file(epoch_dir: &std::path::Path, epoch: u64) -> Option<String> {
        // Look for epoch-{epoch}.cid file
        let cid_file = epoch_dir.join(format!("epoch-{}.cid", epoch));
        if cid_file.exists() {
            if let Ok(contents) = tokio::fs::read_to_string(&cid_file).await {
                let cid = contents.trim().to_string();
                if !cid.is_empty() {
                    debug!("Found CID for epoch {}: {}", epoch, cid);
                    return Some(cid);
                }
            }
        }
        None
    }
}

#[async_trait]
impl StorageBackend for LocalStorage {
    async fn scan_car_files(&self) -> Result<Vec<CarFile>> {
        // Try cache first if available
        if let Some(cache) = &self.cache {
            let cache_key = generate_car_files_cache_key(&StorageConfig::Local(self.config.clone()));
            
            match cache.get(&cache_key).await {
                Ok(Some(cached_files)) => {
                    debug!("Using cached CAR files scan results for local storage: {}", self.path.display());
                    return Ok(cached_files);
                }
                Ok(None) => {
                    debug!("Cache miss for CAR files scan: {}", self.path.display());
                }
                Err(e) => {
                    warn!("Cache error for CAR files scan, falling back to filesystem scan: {}", e);
                }
            }
        }
        
        let path = self.path.clone();
        let epoch_range = self.epoch_range;
        
        // Use spawn_blocking for filesystem operations to collect car files
        let mut car_files = tokio::task::spawn_blocking(move || -> Result<Vec<CarFile>> {
            let mut files = Vec::new();
            
            if !path.exists() {
                warn!("Local storage path does not exist: {}", path.display());
                return Ok(files);
            }
            
            let is_in_range = |epoch: u64| -> bool {
                match epoch_range {
                    Some((start, end)) => epoch >= start && epoch <= end,
                    None => true,
                }
            };
            
            for entry in WalkDir::new(&path)
                .follow_links(true)
                .into_iter()
                .filter_map(|e| e.ok())
            {
                let path = entry.path();
                if path.is_file() {
                    if let Some(filename) = path.file_name().and_then(|n| n.to_str()) {
                        if filename.ends_with(".car") {
                            if let Some(epoch) = Self::parse_epoch_from_car_path(path) {
                                // Check if epoch is within configured range
                                if is_in_range(epoch) {
                                    let metadata = entry.metadata()?;
                                    files.push(CarFile {
                                        epoch,
                                        path: path.to_string_lossy().to_string(),
                                        size: Some(metadata.len()),
                                        cid: None, // Will be populated later
                                    });
                                    debug!("Found CAR file: epoch {} at {}", epoch, path.display());
                                } else {
                                    debug!("Skipping epoch {} (outside configured range)", epoch);
                                }
                            } else {
                                debug!("Found CAR file with unparseable path: {}", path.display());
                            }
                        }
                    }
                }
            }
            
            files.sort_by_key(|f| f.epoch);
            Ok(files)
        })
        .await??;
        
        // Now read CID files for each found CAR file
        for car_file in &mut car_files {
            // Get the parent directory of the CAR file
            if let Some(parent_dir) = std::path::Path::new(&car_file.path).parent() {
                car_file.cid = Self::read_cid_file(parent_dir, car_file.epoch).await;
            }
        }
        
        // Store in cache if available
        if let Some(cache) = &self.cache {
            let cache_key = generate_car_files_cache_key(&StorageConfig::Local(self.config.clone()));
            if let Err(e) = cache.put(&cache_key, car_files.clone(), CacheOptions::default()).await {
                warn!("Failed to cache CAR files scan results: {}", e);
            } else {
                debug!("Cached CAR files scan results for local storage: {}", self.path.display());
            }
        }
        
        info!("Found {} CAR files in local storage: {}", car_files.len(), self.path.display());
        Ok(car_files)
    }
    
    async fn scan_index_files(&self) -> Result<Vec<IndexFile>> {
        // Try cache first if available
        if let Some(cache) = &self.index_cache {
            let cache_key = generate_index_files_cache_key(&StorageConfig::Local(self.config.clone()));
            
            match cache.get(&cache_key).await {
                Ok(Some(cached_files)) => {
                    debug!("Using cached index files scan results for local storage: {}", self.path.display());
                    return Ok(cached_files);
                }
                Ok(None) => {
                    debug!("Cache miss for index files scan: {}", self.path.display());
                }
                Err(e) => {
                    warn!("Cache error for index files scan, falling back to filesystem scan: {}", e);
                }
            }
        }
        
        let path = self.path.clone();
        let epoch_range = self.epoch_range;
        
        let index_files = tokio::task::spawn_blocking(move || -> Result<Vec<IndexFile>> {
            let mut files = Vec::new();
            
            if !path.exists() {
                warn!("Local storage path does not exist: {}", path.display());
                return Ok(files);
            }
            
            let is_in_range = |epoch: u64| -> bool {
                match epoch_range {
                    Some((start, end)) => epoch >= start && epoch <= end,
                    None => true,
                }
            };
            
            for entry in WalkDir::new(&path)
                .follow_links(true)
                .into_iter()
                .filter_map(|e| e.ok())
            {
                let path = entry.path();
                // Check for index files (including compressed GSFA)
                let is_index = path.is_file() && (
                    path.to_string_lossy().ends_with(".index") ||
                    path.to_string_lossy().ends_with(".index.tar.zstd")
                );
                // Also check for old-style .indexdir directories
                let is_indexdir = path.is_dir() && path.to_string_lossy().ends_with(".indexdir");
                
                if is_index || is_indexdir {
                    // Use the new path-based parser
                    if let Some((epoch, index_type, cid)) = Self::parse_epoch_from_index_path(path) {
                        if is_in_range(epoch) {
                            let metadata = entry.metadata()?;
                            if let Some(ref cid) = cid {
                                debug!("Found index file: epoch {} type {:?} CID {} at {}", epoch, index_type, cid, path.display());
                            } else {
                                debug!("Found index file: epoch {} type {:?} at {}", epoch, index_type, path.display());
                            }
                            files.push(IndexFile {
                                epoch,
                                index_type,
                                path: path.to_string_lossy().to_string(),
                                size: Some(metadata.len()),
                            });
                        } else {
                            debug!("Skipping index for epoch {} (outside configured range)", epoch);
                        }
                    }
                }
            }
            
            files.sort_by(|a, b| {
                a.epoch.cmp(&b.epoch)
                    .then(a.index_type.cmp(&b.index_type))
            });
            Ok(files)
        })
        .await??;
        
        // Store in cache if available
        if let Some(cache) = &self.index_cache {
            let cache_key = generate_index_files_cache_key(&StorageConfig::Local(self.config.clone()));
            if let Err(e) = cache.put(&cache_key, index_files.clone(), CacheOptions::default()).await {
                warn!("Failed to cache index files scan results: {}", e);
            } else {
                debug!("Cached index files scan results for local storage: {}", self.path.display());
            }
        }
        
        info!("Found {} index files in local storage: {}", index_files.len(), self.path.display());
        Ok(index_files)
    }
    
    async fn car_file_exists(&self, epoch: u64) -> Result<bool> {
        let files = self.scan_car_files().await?;
        Ok(files.iter().any(|f| f.epoch == epoch))
    }
    
    fn identifier(&self) -> String {
        format!("local:{}", self.path.display())
    }
}

/// Cache entry for HTTP storage scan results
#[derive(Debug, Clone)]
struct HttpScanCache {
    car_files: Vec<CarFile>,
    index_files: Vec<IndexFile>,
    cached_at: Instant,
    index_scan_performed: bool,
}

/// HTTP storage backend with scan result caching
pub struct HttpStorage {
    url: String,
    client: reqwest::Client,
    #[allow(dead_code)]
    timeout_secs: u64,
    epoch_range: Option<(u64, u64)>,
    scan_cache: Arc<RwLock<Option<HttpScanCache>>>,
    cache: Option<Arc<DiskCache<Vec<CarFile>>>>,
    index_cache: Option<Arc<DiskCache<Vec<IndexFile>>>>,
    config: HttpStorageConfig,
}

/// Cache duration for HTTP scan results (24 hours)
const HTTP_SCAN_CACHE_DURATION: Duration = Duration::from_secs(24 * 60 * 60);

impl HttpStorage {
    pub fn new(config: &HttpStorageConfig) -> Result<Self> {
        // Parse timeout from string (e.g., "30s" -> 30)
        let timeout_secs = Self::parse_timeout(&config.timeout)?;
        
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(timeout_secs))
            .build()?;
        
        let epoch_range = config.epoch_range.as_ref().map(|r| (r.start, r.end));
            
        Ok(HttpStorage {
            url: config.url.clone(),
            client,
            timeout_secs,
            epoch_range,
            scan_cache: Arc::new(RwLock::new(None)),
            cache: None,
            index_cache: None,
            config: config.clone(),
        })
    }
    
    /// Create HttpStorage with cache support
    pub fn with_cache(
        config: &HttpStorageConfig,
        cache: Option<Arc<DiskCache<Vec<CarFile>>>>,
        index_cache: Option<Arc<DiskCache<Vec<IndexFile>>>>,
    ) -> Result<Self> {
        // Parse timeout from string (e.g., "30s" -> 30)
        let timeout_secs = Self::parse_timeout(&config.timeout)?;
        
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(timeout_secs))
            .build()?;
        
        let epoch_range = config.epoch_range.as_ref().map(|r| (r.start, r.end));
            
        Ok(HttpStorage {
            url: config.url.clone(),
            client,
            timeout_secs,
            epoch_range,
            scan_cache: Arc::new(RwLock::new(None)),
            cache,
            index_cache,
            config: config.clone(),
        })
    }
    
    fn is_epoch_in_range(&self, epoch: u64) -> bool {
        match self.epoch_range {
            Some((start, end)) => epoch >= start && epoch <= end,
            None => true,
        }
    }
    
    fn parse_timeout(timeout_str: &str) -> Result<u64> {
        let timeout_str = timeout_str.trim();
        if let Some(secs_str) = timeout_str.strip_suffix("s") {
            secs_str.parse::<u64>()
                .context("Invalid timeout value")
        } else {
            // Default to treating as seconds if no suffix
            timeout_str.parse::<u64>()
                .context("Invalid timeout value")
        }
    }
    
    async fn list_files(&self) -> Result<Vec<String>> {
        // HTTP storage doesn't support file listing
        // Files are discovered by probing known epochs
        Ok(Vec::new())
    }
}

#[async_trait]
impl StorageBackend for HttpStorage {
    async fn scan_car_files(&self) -> Result<Vec<CarFile>> {
        // Try disk cache first if available - cache key includes epoch range
        if let Some(cache) = &self.cache {
            let cache_key = generate_car_files_cache_key(&StorageConfig::Http(self.config.clone()));
            
            match cache.get(&cache_key).await {
                Ok(Some(cached_files)) => {
                    debug!("Using cached CAR files scan results for HTTP storage: {} (range-specific cache)", self.url);
                    return Ok(cached_files);
                }
                Ok(None) => {
                    debug!("Cache miss for CAR files scan: {}", self.url);
                }
                Err(e) => {
                    warn!("Cache error for CAR files scan, falling back to HTTP probe: {}", e);
                }
            }
        }
        
        // Check in-memory cache second
        {
            let cache = self.scan_cache.read().await;
            if let Some(cache_entry) = cache.as_ref() {
                let age = cache_entry.cached_at.elapsed();
                if age < HTTP_SCAN_CACHE_DURATION {
                    debug!("Using in-memory cached HTTP storage scan results for {} (age: {:.1}h)", 
                           self.url, age.as_secs_f64() / 3600.0);
                    return Ok(cache_entry.car_files.clone());
                } else {
                    debug!("HTTP storage in-memory cache expired for {} (age: {:.1}h), will refresh", 
                           self.url, age.as_secs_f64() / 3600.0);
                }
            }
        }
        
        let mut files = Vec::new();
        
        // HTTP storage requires probing for known epochs
        info!("Probing HTTP storage {} for CAR files (cache miss/expired)", self.url);
        
        // Fetch the CAR report to get list of available epochs
        match CarReportData::fetch().await {
            Ok(car_report) => {
                info!("CAR report loaded: {} epochs available", car_report.available_epochs.len());
                
                // Determine the range to probe based on configuration and CAR report
                let (start, end) = match self.epoch_range {
                    Some((s, e)) => {
                        info!("Using configured epoch range: {}-{}", s, e);
                        (s, e)
                    }
                    None => {
                        let suggested = car_report.suggest_epoch_range();
                        info!("No epoch range configured, using CAR report range: {}-{}", suggested.0, suggested.1);
                        suggested
                    }
                };
                
                // Get epochs that are both in the configured range AND available in the CAR report
                let epochs_to_probe = car_report.get_epochs_in_range(start, end);
                info!("Probing {} epochs that are known to exist", epochs_to_probe.len());
                
                // Probe epochs concurrently for better performance
                use futures::stream::{self, StreamExt};
                const CONCURRENT_REQUESTS: usize = 10;
                
                let base_url = self.url.trim_end_matches('/');
                let client = self.client.clone();
                
                let results: Vec<_> = stream::iter(epochs_to_probe)
                    .map(|epoch| {
                        let client = client.clone();
                        let base_url = base_url.to_string();
                        async move {
                            // New structure: {url}/{epoch}/epoch-{epoch}.car
                            let car_url = format!("{}/{}/epoch-{}.car", base_url, epoch, epoch);
                            let response = client.head(&car_url).send().await;
                            
                            if let Ok(resp) = response {
                                if resp.status().is_success() {
                                    let size = resp.headers()
                                        .get("content-length")
                                        .and_then(|v| v.to_str().ok())
                                        .and_then(|s| s.parse::<u64>().ok());
                                        
                                    debug!("Found CAR file for epoch {} via probe", epoch);
                                    // Try to fetch CID file for this epoch
                                    let cid_url = format!("{}/{}/epoch-{}.cid", base_url, epoch, epoch);
                                    let cid = match client.get(&cid_url).send().await {
                                        Ok(resp) if resp.status().is_success() => {
                                            match resp.text().await {
                                                Ok(text) => {
                                                    let cid = text.trim().to_string();
                                                    if !cid.is_empty() {
                                                        debug!("Found CID for epoch {}: {}", epoch, cid);
                                                        Some(cid)
                                                    } else {
                                                        None
                                                    }
                                                }
                                                Err(_) => None,
                                            }
                                        }
                                        _ => None,
                                    };
                                    
                                    return Some(CarFile {
                                        epoch,
                                        path: car_url,
                                        size,
                                        cid,
                                    });
                                } else {
                                    debug!("Epoch {} is in CAR report but not found at this storage location", epoch);
                                }
                            }
                            None
                        }
                    })
                    .buffer_unordered(CONCURRENT_REQUESTS)
                    .collect()
                    .await;
                
                // Add all found files
                for result in results {
                    if let Some(car_file) = result {
                        files.push(car_file);
                    }
                }
            }
            Err(e) => {
                warn!("Failed to fetch CAR report: {}. Falling back to range-based probing", e);
                
                // Fallback to simple range-based probing if CAR report fails
                let (start, end) = match self.epoch_range {
                    Some((s, e)) => (s, e),
                    None => {
                        warn!("No epoch range specified for HTTP storage, defaulting to 0-10");
                        (0, 10)
                    }
                };
                
                info!("Probing epochs {} to {} for CAR files", start, end);
                for epoch in start..=end {
                    // New structure: {url}/{epoch}/epoch-{epoch}.car
                    let car_url = format!("{}/{}/epoch-{}.car", 
                        self.url.trim_end_matches('/'), epoch, epoch);
                    let response = self.client.head(&car_url).send().await;
                    
                    if let Ok(resp) = response {
                        if resp.status().is_success() {
                            let size = resp.headers()
                                .get("content-length")
                                .and_then(|v| v.to_str().ok())
                                .and_then(|s| s.parse::<u64>().ok());
                                
                            // Try to fetch CID file for this epoch
                            let cid_url = format!("{}/{}/epoch-{}.cid", 
                                self.url.trim_end_matches('/'), epoch, epoch);
                            let cid = match self.client.get(&cid_url).send().await {
                                Ok(resp) if resp.status().is_success() => {
                                    match resp.text().await {
                                        Ok(text) => {
                                            let cid = text.trim().to_string();
                                            if !cid.is_empty() {
                                                debug!("Found CID for epoch {}: {}", epoch, cid);
                                                Some(cid)
                                            } else {
                                                None
                                            }
                                        }
                                        Err(_) => None,
                                    }
                                }
                                _ => None,
                            };
                            
                            files.push(CarFile {
                                epoch,
                                path: car_url,
                                size,
                                cid,
                            });
                            debug!("Found CAR file for epoch {} via probe", epoch);
                        }
                    }
                }
            }
        }
        
        // Cache the scan results in memory
        {
            let mut cache = self.scan_cache.write().await;
            if let Some(cache_entry) = cache.as_mut() {
                // Update existing cache entry
                cache_entry.car_files = files.clone();
                cache_entry.cached_at = Instant::now();
            } else {
                // Create new cache entry (index files will be empty initially)
                *cache = Some(HttpScanCache {
                    car_files: files.clone(),
                    index_files: Vec::new(),
                    cached_at: Instant::now(),
                    index_scan_performed: false,
                });
            }
        }
        
        // Store in disk cache if available
        if let Some(cache) = &self.cache {
            let cache_key = generate_car_files_cache_key(&StorageConfig::Http(self.config.clone()));
            if let Err(e) = cache.put(&cache_key, files.clone(), CacheOptions::default()).await {
                warn!("Failed to cache CAR files scan results: {}", e);
            } else {
                debug!("Cached CAR files scan results for HTTP storage: {}", self.url);
            }
        }
        
        info!("Found {} CAR files in HTTP storage: {} (results cached)", files.len(), self.url);
        files.sort_by_key(|f| f.epoch);
        Ok(files)
    }
    
    async fn scan_index_files(&self) -> Result<Vec<IndexFile>> {
        // Try disk cache first if available
        if let Some(cache) = &self.index_cache {
            let cache_key = generate_index_files_cache_key(&StorageConfig::Http(self.config.clone()));
            
            match cache.get(&cache_key).await {
                Ok(Some(cached_files)) => {
                    debug!("Using cached index files scan results for HTTP storage: {}", self.url);
                    return Ok(cached_files);
                }
                Ok(None) => {
                    debug!("Cache miss for index files scan: {}", self.url);
                }
                Err(e) => {
                    warn!("Cache error for index files scan, falling back to HTTP probe: {}", e);
                }
            }
        }
        
        // Check in-memory cache second - but only if index files have been scanned before
        {
            let cache = self.scan_cache.read().await;
            if let Some(cache_entry) = cache.as_ref() {
                // Only use cached index results if we've actually scanned for indexes before
                // (not just created an empty cache entry during CAR scanning)
                if !cache_entry.index_files.is_empty() || cache_entry.index_scan_performed {
                    let age = cache_entry.cached_at.elapsed();
                    if age < HTTP_SCAN_CACHE_DURATION {
                        debug!("Using in-memory cached HTTP storage index scan results for {} (age: {:.1}h)", 
                               self.url, age.as_secs_f64() / 3600.0);
                        return Ok(cache_entry.index_files.clone());
                    } else {
                        debug!("HTTP storage in-memory index cache expired for {} (age: {:.1}h), will refresh", 
                               self.url, age.as_secs_f64() / 3600.0);
                    }
                }
            }
        }
        
        let mut files = Vec::new();
        
        // HTTP storage requires probing for known epochs and index types
        // This is expensive, so we only probe epochs that we know have CAR files
        
        // First, get list of epochs with CAR files (this will use the cached results if available)
        let car_files = self.scan_car_files().await?;
        
        // Define the index types we want to probe for
        let index_types = [
            (IndexType::SlotToCid, "slot-to-cid"),
            (IndexType::SigToCid, "sig-to-cid"),
            (IndexType::CidToOffsetAndSize, "cid-to-offset-and-size"),
            (IndexType::SigExists, "sig-exists"),
            (IndexType::Gsfa, "gsfa"),
        ];
        
        info!("Probing for index files across {} epochs (this may take a moment)...", car_files.len());
        
        // For HTTP storage, we'll warn if there are too many epochs but still probe them
        // We'll use concurrent requests to speed things up significantly
        const MAX_EPOCHS_TO_PROBE: usize = 1000;
        
        let car_files_to_probe = if car_files.len() > MAX_EPOCHS_TO_PROBE {
            warn!("Large number of epochs ({}) to probe for indexes via HTTP.", car_files.len());
            warn!("Limiting to first {} epochs to avoid excessive delays.", MAX_EPOCHS_TO_PROBE);
            warn!("Consider using local storage or reducing the epoch_range in your config for better performance.");
            car_files.into_iter().take(MAX_EPOCHS_TO_PROBE).collect::<Vec<_>>()
        } else {
            car_files
        };
        
        // Use concurrent requests to probe indexes much faster
        use futures::stream::{self, StreamExt};
        const CONCURRENT_REQUESTS: usize = 20; // Process 20 concurrent requests at a time (conservative to avoid rate limits)
        
        let base_url = self.url.trim_end_matches('/').to_string();
        let client = self.client.clone();
        
        // Create probe futures for all index files
        let probe_futures: Vec<_> = car_files_to_probe
            .into_iter()
            .flat_map(|car_file| {
                let epoch = car_file.epoch;
                let cid = car_file.cid.clone();
                let base_url = base_url.clone();
                let client = client.clone();
                
                index_types.iter().map(move |(index_type, index_type_str)| {
                    let index_type = index_type.clone();
                    let index_type_str = *index_type_str;
                    let base_url = base_url.clone();
                    let client = client.clone();
                    let cid = cid.clone();
                    
                    async move {
                        // Build the index URL based on the index type
                        let index_url = if matches!(index_type, IndexType::Gsfa) {
                            format!("{}/{}/epoch-{}-gsfa.index.tar.zstd", base_url, epoch, epoch)
                        } else if let Some(ref cid) = cid {
                            format!("{}/{}/epoch-{}-{}-mainnet-{}.index",
                                base_url, epoch, epoch, cid, index_type_str)
                        } else {
                            format!("{}/{}/epoch-{}-{}.index",
                                base_url, epoch, epoch, index_type_str)
                        };
                        
                        debug!("Probing for index at: {}", index_url);
                        let response = client.head(&index_url).send().await;
                        
                        let mut found_index = None;
                        
                        if let Ok(resp) = response {
                            if resp.status().is_success() {
                                debug!("✓ Found index at: {}", index_url);
                                let size = resp.headers()
                                    .get("content-length")
                                    .and_then(|v| v.to_str().ok())
                                    .and_then(|s| s.parse::<u64>().ok());
                                
                                found_index = Some(IndexFile {
                                    epoch,
                                    index_type: index_type.clone(),
                                    path: index_url.clone(),
                                    size,
                                });
                            } else if cid.is_none() {
                                // Try alternative format without CID
                                debug!("✗ Index not found at: {} (status: {}), trying alt format", index_url, resp.status());
                                let alt_index_url = format!("{}/{}/{}-{}.index",
                                    base_url, epoch, index_type_str, epoch);
                                
                                if let Ok(alt_resp) = client.head(&alt_index_url).send().await {
                                    if alt_resp.status().is_success() {
                                        debug!("✓ Found index at alt location: {}", alt_index_url);
                                        let size = alt_resp.headers()
                                            .get("content-length")
                                            .and_then(|v| v.to_str().ok())
                                            .and_then(|s| s.parse::<u64>().ok());
                                        
                                        found_index = Some(IndexFile {
                                            epoch,
                                            index_type: index_type.clone(),
                                            path: alt_index_url,
                                            size,
                                        });
                                    }
                                }
                            }
                        } else {
                            debug!("✗ Failed to probe index at: {} (request failed)", index_url);
                        }
                        
                        found_index
                    }
                })
            })
            .collect();
        
        // Execute all probes concurrently with limited parallelism
        let results: Vec<Option<IndexFile>> = stream::iter(probe_futures)
            .buffer_unordered(CONCURRENT_REQUESTS)
            .collect()
            .await;
        
        // Collect all found indexes
        files = results.into_iter().flatten().collect();
        
        info!("Index probing complete. Found {} index files", files.len());
        
        // Cache the index scan results in memory
        {
            let mut cache = self.scan_cache.write().await;
            if let Some(cache_entry) = cache.as_mut() {
                // Update existing cache entry
                cache_entry.index_files = files.clone();
                cache_entry.cached_at = Instant::now();
                cache_entry.index_scan_performed = true;
            } else {
                // This shouldn't happen since scan_car_files should have created the cache
                *cache = Some(HttpScanCache {
                    car_files: Vec::new(), // Will be populated when scan_car_files is called
                    index_files: files.clone(),
                    cached_at: Instant::now(),
                    index_scan_performed: true,
                });
            }
        }
        
        // Store in disk cache if available
        if let Some(cache) = &self.index_cache {
            let cache_key = generate_index_files_cache_key(&StorageConfig::Http(self.config.clone()));
            if let Err(e) = cache.put(&cache_key, files.clone(), CacheOptions::default()).await {
                warn!("Failed to cache index files scan results: {}", e);
            } else {
                debug!("Cached index files scan results for HTTP storage: {}", self.url);
            }
        }
        
        info!("Found {} index files in HTTP storage: {} (results cached)", files.len(), self.url);
        files.sort_by(|a, b| {
            a.epoch.cmp(&b.epoch)
                .then(a.index_type.cmp(&b.index_type))
        });
        Ok(files)
    }
    
    async fn car_file_exists(&self, epoch: u64) -> Result<bool> {
        // New structure: {url}/{epoch}/epoch-{epoch}.car
        let car_url = format!("{}/{}/epoch-{}.car", 
            self.url.trim_end_matches('/'), epoch, epoch);
        let response = self.client.head(&car_url).send().await?;
        Ok(response.status().is_success())
    }
    
    fn identifier(&self) -> String {
        format!("http:{}", self.url)
    }
}

impl HttpStorage {
    /// Clear the HTTP storage scan cache (useful for testing or force refresh)
    pub async fn clear_cache(&self) {
        let mut cache = self.scan_cache.write().await;
        *cache = None;
        debug!("HTTP storage cache cleared for {}", self.url);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_parse_index_path_with_cid() {
        // Test parsing index path with CID
        // Path structure: {storagePath}/{epochNumber}/epoch-{epochNumber}-{cid}-mainnet-{indexNameInKebabCase}.index
        let path = std::path::Path::new("/storage/500/epoch-500-bafybeig123-mainnet-slot-to-cid.index");
        let result = LocalStorage::parse_epoch_from_index_path(path);
        assert!(result.is_some());
        let (epoch, index_type, cid) = result.unwrap();
        assert_eq!(epoch, 500);
        assert_eq!(index_type, IndexType::SlotToCid);
        assert_eq!(cid, Some("bafybeig123".to_string()));
        
        // Test parsing other index types
        let test_cases = vec![
            ("/storage/0/epoch-0-bafybeig456-mainnet-sig-to-cid.index", 0, IndexType::SigToCid, "bafybeig456"),
            ("/storage/100/epoch-100-bafybeig789-mainnet-cid-to-offset-and-size.index", 100, IndexType::CidToOffsetAndSize, "bafybeig789"),
            ("/storage/200/epoch-200-bafybeig000-mainnet-sig-exists.index", 200, IndexType::SigExists, "bafybeig000"),
            ("/storage/300/epoch-300-bafybeig111-mainnet-gsfa.index.tar.zstd", 300, IndexType::Gsfa, "bafybeig111"),
        ];
        
        for (path_str, expected_epoch, expected_type, expected_cid) in test_cases {
            let path = std::path::Path::new(path_str);
            let result = LocalStorage::parse_epoch_from_index_path(path);
            assert!(result.is_some(), "Failed to parse: {}", path_str);
            let (epoch, index_type, cid) = result.unwrap();
            assert_eq!(epoch, expected_epoch);
            assert_eq!(index_type, expected_type);
            assert_eq!(cid, Some(expected_cid.to_string()));
        }
    }
    
    #[test]
    fn test_parse_index_path_without_cid() {
        // Test parsing legacy format without CID
        let path = std::path::Path::new("/storage/500/epoch-500-slot-to-cid.index");
        let result = LocalStorage::parse_epoch_from_index_path(path);
        assert!(result.is_some());
        let (epoch, index_type, cid) = result.unwrap();
        assert_eq!(epoch, 500);
        assert_eq!(index_type, IndexType::SlotToCid);
        assert_eq!(cid, None);
        
        // Test GSFA without CID (special case)
        let path = std::path::Path::new("/storage/300/epoch-300-gsfa.index.tar.zstd");
        let result = LocalStorage::parse_epoch_from_index_path(path);
        assert!(result.is_some());
        let (epoch, index_type, cid) = result.unwrap();
        assert_eq!(epoch, 300);
        assert_eq!(index_type, IndexType::Gsfa);
        assert_eq!(cid, None);
    }
    
    #[test]
    fn test_parse_index_path_mismatch_epoch() {
        // Test that we reject paths where directory doesn't match epoch number
        let path = std::path::Path::new("/storage/400/epoch-500-bafybeig123-mainnet-slot-to-cid.index");
        let result = LocalStorage::parse_epoch_from_index_path(path);
        assert!(result.is_none(), "Should reject path where directory doesn't match epoch");
    }
    
    #[test]
    fn test_parse_car_path() {
        // Test parsing CAR file path
        let path = std::path::Path::new("/storage/500/epoch-500.car");
        let result = LocalStorage::parse_epoch_from_car_path(path);
        assert_eq!(result, Some(500));
        
        // Test with non-matching parent directory
        let path = std::path::Path::new("/storage/different/epoch-500.car");
        let result = LocalStorage::parse_epoch_from_car_path(path);
        assert_eq!(result, Some(500)); // Should still work
        
        // Test invalid format
        let path = std::path::Path::new("/storage/500/data.car");
        let result = LocalStorage::parse_epoch_from_car_path(path);
        assert_eq!(result, None);
    }
}

/// Session cache for storage manager scan results
#[derive(Debug, Clone)]
struct SessionCache {
    car_files: HashMap<String, Vec<CarFile>>,
    index_files: HashMap<String, Vec<IndexFile>>,
}

/// Storage manager that handles multiple storage backends with session-based caching
pub struct StorageManager {
    backends: Vec<Box<dyn StorageBackend>>,
    session_cache: Arc<RwLock<Option<SessionCache>>>,
    scan_in_progress: Arc<tokio::sync::Mutex<bool>>,
}

impl StorageManager {
    pub async fn from_config(configs: &[StorageConfig]) -> Result<Self> {
        let mut backends: Vec<Box<dyn StorageBackend>> = Vec::new();
        
        for config in configs {
            let backend: Box<dyn StorageBackend> = match config {
                StorageConfig::Local(local) => {
                    Box::new(LocalStorage::new(local)?)
                }
                StorageConfig::Http(http) => {
                    Box::new(HttpStorage::new(http)?)
                }
            };
            backends.push(backend);
        }
        
        Ok(StorageManager { 
            backends,
            session_cache: Arc::new(RwLock::new(None)),
            scan_in_progress: Arc::new(tokio::sync::Mutex::new(false)),
        })
    }
    
    /// Create StorageManager with disk cache support
    pub async fn from_config_with_cache(
        configs: &[StorageConfig], 
        cache_config: &crate::config::CacheConfig,
    ) -> Result<Self> {
        let mut backends: Vec<Box<dyn StorageBackend>> = Vec::new();
        
        // Only initialize cache if enabled
        let (car_cache, index_cache) = if cache_config.enabled {
            let cache_dir = std::path::Path::new(&cache_config.directory);
            let ttl = cache_config.parse_ttl().context("Failed to parse cache TTL")?;
            
            let cache_config_obj = crate::cache::CacheConfig {
                default_ttl: Some(ttl),
                max_size_bytes: Some(10 * 1024 * 1024 * 1024), // 10 GB default
                enable_compression: true,
                max_concurrent_ops: 10,
            };
            
            let car_cache = Arc::new(
                DiskCache::<Vec<CarFile>>::new(
                    cache_dir.join("car_files"), 
                    cache_config_obj.clone()
                ).await.context("Failed to initialize CAR files cache")?
            );
            
            let index_cache = Arc::new(
                DiskCache::<Vec<IndexFile>>::new(
                    cache_dir.join("index_files"), 
                    cache_config_obj
                ).await.context("Failed to initialize index files cache")?
            );
            
            (Some(car_cache), Some(index_cache))
        } else {
            (None, None)
        };
        
        for config in configs {
            let backend: Box<dyn StorageBackend> = match config {
                StorageConfig::Local(local) => {
                    Box::new(LocalStorage::with_cache(local, car_cache.clone(), index_cache.clone())?)
                }
                StorageConfig::Http(http) => {
                    Box::new(HttpStorage::with_cache(http, car_cache.clone(), index_cache.clone())?)
                }
            };
            backends.push(backend);
        }
        
        Ok(StorageManager { 
            backends,
            session_cache: Arc::new(RwLock::new(None)),
            scan_in_progress: Arc::new(tokio::sync::Mutex::new(false)),
        })
    }
    
    /// Scan all storage backends for CAR files and return the results (with session caching)
    pub async fn scan_all_car_files(&self) -> Result<HashMap<String, Vec<CarFile>>> {
        // Check session cache first
        {
            let cache = self.session_cache.read().await;
            if let Some(session_cache) = cache.as_ref() {
                if !session_cache.car_files.is_empty() {
                    debug!("Using session-cached CAR files scan results");
                    return Ok(session_cache.car_files.clone());
                }
            }
        }
        
        // Acquire the scan lock to prevent concurrent scans
        let _scan_lock = self.scan_in_progress.lock().await;
        
        // Double-check cache after acquiring lock (another thread may have populated it)
        {
            let cache = self.session_cache.read().await;
            if let Some(session_cache) = cache.as_ref() {
                if !session_cache.car_files.is_empty() {
                    debug!("Using session-cached CAR files scan results (populated while waiting for lock)");
                    return Ok(session_cache.car_files.clone());
                }
            }
        }
        
        info!("Session cache miss - performing fresh scan of all storage backends for CAR files");
        let mut results = HashMap::new();
        
        for backend in &self.backends {
            let identifier = backend.identifier();
            match backend.scan_car_files().await {
                Ok(files) => {
                    info!("Storage {} has {} CAR files", identifier, files.len());
                    results.insert(identifier, files);
                }
                Err(e) => {
                    warn!("Failed to scan storage {}: {}", backend.identifier(), e);
                    results.insert(identifier, Vec::new());
                }
            }
        }
        
        // Cache the results for this session
        {
            let mut cache = self.session_cache.write().await;
            if let Some(session_cache) = cache.as_mut() {
                session_cache.car_files = results.clone();
            } else {
                *cache = Some(SessionCache {
                    car_files: results.clone(),
                    index_files: HashMap::new(),
                });
            }
        }
        
        Ok(results)
    }
    
    /// Scan all storage backends for index files and return the results (with session caching)
    pub async fn scan_all_index_files(&self) -> Result<HashMap<String, Vec<IndexFile>>> {
        // Check session cache first
        {
            let cache = self.session_cache.read().await;
            if let Some(session_cache) = cache.as_ref() {
                if !session_cache.index_files.is_empty() {
                    debug!("Using session-cached index files scan results");
                    return Ok(session_cache.index_files.clone());
                }
            }
        }
        
        // Acquire the scan lock to prevent concurrent scans
        let _scan_lock = self.scan_in_progress.lock().await;
        
        // Double-check cache after acquiring lock (another thread may have populated it)
        {
            let cache = self.session_cache.read().await;
            if let Some(session_cache) = cache.as_ref() {
                if !session_cache.index_files.is_empty() {
                    debug!("Using session-cached index files scan results (populated while waiting for lock)");
                    return Ok(session_cache.index_files.clone());
                }
            }
        }
        
        info!("Session cache miss - performing fresh scan of all storage backends for index files");
        let mut results = HashMap::new();
        
        for backend in &self.backends {
            let identifier = backend.identifier();
            match backend.scan_index_files().await {
                Ok(files) => {
                    info!("Storage {} has {} index files", identifier, files.len());
                    results.insert(identifier, files);
                }
                Err(e) => {
                    warn!("Failed to scan indexes in storage {}: {}", backend.identifier(), e);
                    results.insert(identifier, Vec::new());
                }
            }
        }
        
        // Cache the results for this session
        {
            let mut cache = self.session_cache.write().await;
            if let Some(session_cache) = cache.as_mut() {
                session_cache.index_files = results.clone();
            } else {
                *cache = Some(SessionCache {
                    car_files: HashMap::new(),
                    index_files: results.clone(),
                });
            }
        }
        
        Ok(results)
    }
    
    /// Scan all storage backends for both CAR and index files together
    /// This is optimized to check for indexes immediately after finding each CAR file
    pub async fn scan_all_files_optimized(&self) -> Result<(HashMap<String, Vec<CarFile>>, HashMap<String, Vec<IndexFile>>)> {
        // Check session cache first
        {
            let cache = self.session_cache.read().await;
            if let Some(session_cache) = cache.as_ref() {
                if !session_cache.car_files.is_empty() && !session_cache.index_files.is_empty() {
                    debug!("Using session-cached scan results for both CAR and index files");
                    return Ok((session_cache.car_files.clone(), session_cache.index_files.clone()));
                }
            }
        }
        
        // Acquire the scan lock to prevent concurrent scans
        let _scan_lock = self.scan_in_progress.lock().await;
        
        // Double-check cache after acquiring lock
        {
            let cache = self.session_cache.read().await;
            if let Some(session_cache) = cache.as_ref() {
                if !session_cache.car_files.is_empty() && !session_cache.index_files.is_empty() {
                    debug!("Using session-cached scan results (populated while waiting for lock)");
                    return Ok((session_cache.car_files.clone(), session_cache.index_files.clone()));
                }
            }
        }
        
        info!("Starting optimized scan of all storage backends (CAR + indexes together)");
        let mut car_results = HashMap::new();
        let mut index_results = HashMap::new();
        
        for backend in &self.backends {
            let identifier = backend.identifier();
            
            // First scan for CAR files
            let car_files = match backend.scan_car_files().await {
                Ok(files) => files,
                Err(e) => {
                    warn!("Failed to scan storage {}: {}", backend.identifier(), e);
                    Vec::new()
                }
            };
            
            // Immediately scan for index files after finding CAR files
            // This allows the system to start processing complete epochs sooner
            let index_files = match backend.scan_index_files().await {
                Ok(files) => files,
                Err(e) => {
                    warn!("Failed to scan indexes in storage {}: {}", backend.identifier(), e);
                    Vec::new()
                }
            };
            
            car_results.insert(identifier.clone(), car_files);
            index_results.insert(identifier, index_files);
        }
        
        // Cache the results for this session
        {
            let mut cache = self.session_cache.write().await;
            *cache = Some(SessionCache {
                car_files: car_results.clone(),
                index_files: index_results.clone(),
            });
        }
        
        Ok((car_results, index_results))
    }
    
    /// Get a consolidated list of all available epochs across all storage
    pub async fn get_available_epochs(&self) -> Result<Vec<u64>> {
        let all_files = self.scan_all_car_files().await?;
        
        let mut epochs = std::collections::HashSet::new();
        for files in all_files.values() {
            for file in files {
                epochs.insert(file.epoch);
            }
        }
        
        let mut epochs: Vec<u64> = epochs.into_iter().collect();
        epochs.sort();
        Ok(epochs)
    }
    
    /// Check which storage backends have a specific epoch
    pub async fn find_epoch_locations(&self, epoch: u64) -> Result<Vec<String>> {
        let mut locations = Vec::new();
        
        for backend in &self.backends {
            if backend.car_file_exists(epoch).await? {
                locations.push(backend.identifier());
            }
        }
        
        Ok(locations)
    }
    
    /// Clear the session cache (useful for forcing a fresh scan)
    pub async fn clear_session_cache(&self) {
        let mut cache = self.session_cache.write().await;
        *cache = None;
        debug!("Storage manager session cache cleared");
    }
}