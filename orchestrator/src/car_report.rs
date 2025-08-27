use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::RwLock;
use tracing::{debug, info};

const CAR_REPORT_URL: &str = "https://raw.githubusercontent.com/rpcpool/yellowstone-faithful/gha-report/docs/CAR-REPORT.md";

/// Information about available epochs from the CAR report
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CarReportData {
    /// Set of epochs that have CAR files available
    pub available_epochs: HashSet<u64>,
    /// The highest epoch number seen
    pub max_epoch: u64,
    /// The lowest epoch number seen  
    pub min_epoch: u64,
    /// Epochs that have missing or incomplete data
    pub incomplete_epochs: HashSet<u64>,
}

/// Cache entry for the CAR report with timestamp
#[derive(Debug, Clone)]
struct CacheEntry {
    data: Arc<CarReportData>,
    cached_at: Instant,
}

/// Global cache for the CAR report with time-based expiration
static CAR_REPORT_CACHE: once_cell::sync::Lazy<Arc<RwLock<Option<CacheEntry>>>> = 
    once_cell::sync::Lazy::new(|| Arc::new(RwLock::new(None)));

/// Cache duration (24 hours)
const CACHE_DURATION: Duration = Duration::from_secs(24 * 60 * 60);

impl CarReportData {
    /// Fetch and parse the CAR report from GitHub (with 24-hour caching)
    pub async fn fetch() -> Result<Arc<Self>> {
        // Check cache first and validate expiration
        {
            let cache = CAR_REPORT_CACHE.read().await;
            if let Some(cache_entry) = cache.as_ref() {
                let age = cache_entry.cached_at.elapsed();
                if age < CACHE_DURATION {
                    debug!("Using cached CAR report (age: {:.1}h)", age.as_secs_f64() / 3600.0);
                    return Ok(Arc::clone(&cache_entry.data));
                } else {
                    debug!("CAR report cache expired (age: {:.1}h), will refresh", age.as_secs_f64() / 3600.0);
                }
            }
        }
        
        // Cache miss, fetch from GitHub
        info!("Fetching CAR report from GitHub...");
        
        let client = reqwest::Client::new();
        let response = client
            .get(CAR_REPORT_URL)
            .send()
            .await
            .context("Failed to fetch CAR report")?;
            
        let text = response
            .text()
            .await
            .context("Failed to read CAR report")?;
            
        let report_data = Arc::new(Self::parse(&text)?);
        
        // Store in cache with timestamp
        {
            let mut cache = CAR_REPORT_CACHE.write().await;
            *cache = Some(CacheEntry {
                data: Arc::clone(&report_data),
                cached_at: Instant::now(),
            });
        }
        
        Ok(report_data)
    }
    
    /// Parse the CAR report markdown to extract epoch information
    pub fn parse(report_text: &str) -> Result<Self> {
        let mut available_epochs = HashSet::new();
        let mut incomplete_epochs = HashSet::new();
        let mut max_epoch = 0u64;
        let mut min_epoch = u64::MAX;
        
        for line in report_text.lines() {
            // Skip header lines and empty lines
            if line.starts_with('|') && !line.contains("Epoch #") && !line.contains("---") {
                // Split by | and get the epoch number (first column after initial |)
                let parts: Vec<&str> = line.split('|').collect();
                if parts.len() >= 2 {
                    let epoch_str = parts[1].trim();
                    
                    // Skip "epoch is ongoing" entries
                    if epoch_str.contains("epoch") || epoch_str.contains("ongoing") {
                        continue;
                    }
                    
                    // Try to parse epoch number
                    if let Ok(epoch) = epoch_str.parse::<u64>() {
                        // Check if this epoch has a CAR file (column 2)
                        if parts.len() >= 3 {
                            let car_column = parts[2].trim();
                            
                            if car_column.contains(".car") && car_column.contains("epoch-") {
                                // This epoch has a CAR file
                                available_epochs.insert(epoch);
                                max_epoch = max_epoch.max(epoch);
                                min_epoch = min_epoch.min(epoch);
                                debug!("Found available epoch: {}", epoch);
                            } else if car_column == "❌" {
                                // This epoch is missing or incomplete
                                incomplete_epochs.insert(epoch);
                                debug!("Found incomplete epoch: {}", epoch);
                            }
                        }
                    }
                }
            }
        }
        
        if available_epochs.is_empty() {
            anyhow::bail!("No available epochs found in CAR report");
        }
        
        info!(
            "Parsed CAR report: {} available epochs (range {}-{}), {} incomplete",
            available_epochs.len(),
            min_epoch,
            max_epoch,
            incomplete_epochs.len()
        );
        
        Ok(CarReportData {
            available_epochs,
            max_epoch,
            min_epoch,
            incomplete_epochs,
        })
    }
    
    /// Check if an epoch is available
    pub fn is_epoch_available(&self, epoch: u64) -> bool {
        self.available_epochs.contains(&epoch)
    }
    
    /// Get a suggested epoch range based on available data
    pub fn suggest_epoch_range(&self) -> (u64, u64) {
        (self.min_epoch, self.max_epoch)
    }
    
    /// Get available epochs within a range
    pub fn get_epochs_in_range(&self, start: u64, end: u64) -> Vec<u64> {
        self.available_epochs
            .iter()
            .filter(|&&epoch| epoch >= start && epoch <= end)
            .copied()
            .collect()
    }
    
    /// Clear the CAR report cache (useful for testing or force refresh)
    pub async fn clear_cache() {
        let mut cache = CAR_REPORT_CACHE.write().await;
        *cache = None;
        debug!("CAR report cache cleared");
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_parse_car_report() {
        let sample_report = r#"
| Epoch #  | CAR  | CAR SHA256  | CAR filesize | tx meta check | poh check | Indices | Indices Size | Filecoin Deals | Slots
|---|---|---|---|---|---|---|---|---|---|
|829|epoch is|ongoing||||||||
| 828 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 827 | [epoch-827.car](https://files.old-faithful.net/827/epoch-827.car) | [580774a](https://files.old-faithful.net/827/epoch-827.sha256) | 740 GB | ✅ | ✅ | ✅ | 78 GB | ❌ | [827.slots.txt](https://files.old-faithful.net/827/827.slots.txt) |
| 826 | [epoch-826.car](https://files.old-faithful.net/826/epoch-826.car) | [cac9fb3](https://files.old-faithful.net/826/epoch-826.sha256) | 789 GB | ✅ | ✅ | ✅ | 82 GB | ❌ | [826.slots.txt](https://files.old-faithful.net/826/826.slots.txt) |
| 0 | [epoch-0.car](https://files.old-faithful.net/0/epoch-0.car) | [abc123](https://files.old-faithful.net/0/epoch-0.sha256) | 100 GB | ✅ | ✅ | ✅ | 10 GB | ✅ | [0.slots.txt](https://files.old-faithful.net/0/0.slots.txt) |
"#;
        
        let report = CarReportData::parse(sample_report).unwrap();
        
        assert!(report.is_epoch_available(827));
        assert!(report.is_epoch_available(826));
        assert!(report.is_epoch_available(0));
        assert!(!report.is_epoch_available(829));
        assert!(!report.is_epoch_available(828));
        
        assert!(report.incomplete_epochs.contains(&828));
        assert_eq!(report.min_epoch, 0);
        assert_eq!(report.max_epoch, 827);
    }
}