use anyhow::Result;
use serde::{Deserialize, Serialize};
use std::path::Path;
use std::time::Duration;
use tracing::warn;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Config {
    #[serde(default)]
    pub storage: Vec<StorageConfig>,
    #[serde(default)]
    pub epoch_configs: Option<EpochConfigsSection>,
    #[serde(default)]
    pub cache: CacheConfig,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct EpochConfigsSection {
    pub output_dir: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct CacheConfig {
    #[serde(default = "default_cache_enabled")]
    pub enabled: bool,
    #[serde(default = "default_cache_directory")]
    pub directory: String,
    #[serde(default = "default_cache_ttl")]
    pub ttl: String,
}

impl Default for CacheConfig {
    fn default() -> Self {
        Self {
            enabled: default_cache_enabled(),
            directory: default_cache_directory(),
            ttl: default_cache_ttl(),
        }
    }
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(tag = "type", rename_all = "lowercase")]
pub enum StorageConfig {
    Local(LocalStorageConfig),
    Http(HttpStorageConfig),
    // S3 can be added later when direct S3 support is implemented in Old Faithful
    // For now, S3 buckets can be accessed via HTTP endpoints
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct LocalStorageConfig {
    pub path: String,
    /// Optional epoch range to scan (inclusive)
    #[serde(default)]
    pub epoch_range: Option<EpochRange>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct HttpStorageConfig {
    pub url: String,
    #[serde(default = "default_timeout")]
    pub timeout: String,
    /// Optional epoch range to scan (inclusive)
    #[serde(default)]
    pub epoch_range: Option<EpochRange>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct EpochRange {
    pub start: u64,
    pub end: u64,
}

fn default_cache_enabled() -> bool {
    false
}

fn default_cache_directory() -> String {
    "./cache".to_string()
}

fn default_cache_ttl() -> String {
    "24h".to_string()
}

fn default_timeout() -> String {
    "30s".to_string()
}

impl Config {
    /// Load configuration from a TOML file
    pub fn from_file<P: AsRef<Path>>(path: P) -> Result<Self> {
        let contents = std::fs::read_to_string(path)?;
        let config: Config = toml::from_str(&contents)?;
        Ok(config)
    }

    /// Validate the configuration structure (synchronous validation)
    pub fn validate(&self) -> Result<()> {
        if self.storage.is_empty() {
            anyhow::bail!("No storage configurations provided");
        }

        for (i, storage) in self.storage.iter().enumerate() {
            match storage {
                StorageConfig::Local(local) => {
                    if local.path.is_empty() {
                        anyhow::bail!("Storage #{}: Local storage path cannot be empty", i);
                    }
                    // Check if path exists
                    let path = std::path::Path::new(&local.path);
                    if !path.exists() {
                        warn!("Storage #{}: Local path does not exist: {}", i, local.path);
                    }
                }
                StorageConfig::Http(http) => {
                    if http.url.is_empty() {
                        anyhow::bail!("Storage #{}: HTTP URL cannot be empty", i);
                    }
                    // Validate URL format
                    if !http.url.starts_with("http://") && !http.url.starts_with("https://") {
                        anyhow::bail!("Storage #{}: HTTP URL must start with http:// or https://", i);
                    }
                }
            }
        }

        Ok(())
    }
}

impl CacheConfig {
    /// Parse the TTL string into a Duration
    /// Supports formats like "1h", "30m", "3600s", "24h"
    pub fn parse_ttl(&self) -> Result<Duration> {
        parse_duration(&self.ttl)
    }
}

/// Parse a duration string like "1h", "30m", "3600s" into a Duration
fn parse_duration(s: &str) -> Result<Duration> {
    if s.is_empty() {
        anyhow::bail!("Duration string cannot be empty");
    }
    
    let s = s.trim();
    if s.is_empty() {
        anyhow::bail!("Duration string cannot be empty after trimming");
    }
    
    // Find where the number ends and unit begins
    let mut number_end = s.len();
    for (i, c) in s.char_indices() {
        if c.is_ascii_alphabetic() {
            number_end = i;
            break;
        }
    }
    
    let (number_part, unit) = if number_end == s.len() {
        // No unit found, assume seconds
        (s, "s")
    } else {
        (&s[..number_end], &s[number_end..])
    };
    
    let value: u64 = number_part.parse()
        .map_err(|_| anyhow::anyhow!("Invalid number in duration: {}", number_part))?;
    
    let duration = match unit.to_lowercase().as_str() {
        "s" | "sec" | "second" | "seconds" => Duration::from_secs(value),
        "m" | "min" | "minute" | "minutes" => Duration::from_secs(value * 60),
        "h" | "hr" | "hour" | "hours" => Duration::from_secs(value * 3600),
        "d" | "day" | "days" => Duration::from_secs(value * 86400),
        _ => anyhow::bail!("Unknown duration unit: {}. Supported units: s, m, h, d", unit),
    };
    
    Ok(duration)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_config() {
        let toml_str = r#"
[[storage]]
type = "local"
path = "/var/lib/myapp/storage"

[[storage]]
type = "http"
url = "https://storage.example.com"
timeout = "60s"
"#;

        let config: Config = toml::from_str(toml_str).unwrap();
        assert_eq!(config.storage.len(), 2);
        
        match &config.storage[0] {
            StorageConfig::Local(local) => {
                assert_eq!(local.path, "/var/lib/myapp/storage");
            }
            _ => panic!("Expected local storage"),
        }

        match &config.storage[1] {
            StorageConfig::Http(http) => {
                assert_eq!(http.url, "https://storage.example.com");
                assert_eq!(http.timeout, "60s");
            }
            _ => panic!("Expected HTTP storage"),
        }
    }

    #[test]
    fn test_validate_config() {
        let toml_str = r#"
[[storage]]
type = "local"
path = "/var/lib/myapp/storage"
"#;

        let config: Config = toml::from_str(toml_str).unwrap();
        assert!(config.validate().is_ok());

        // Test empty storage
        let empty_config = Config { 
            storage: vec![],
            epoch_configs: None,
            cache: CacheConfig::default(),
        };
        assert!(empty_config.validate().is_err());

        // Test invalid HTTP URL
        let invalid_http = r#"
[[storage]]
type = "http"
url = "not-a-url"
"#;
        let config: Config = toml::from_str(invalid_http).unwrap();
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_cache_config_defaults() {
        let toml_str = r#"
[[storage]]
type = "local"
path = "/test/path"
"#;
        
        let config: Config = toml::from_str(toml_str).unwrap();
        assert!(!config.cache.enabled); // Default should be false
        assert_eq!(config.cache.directory, "./cache");
        assert_eq!(config.cache.ttl, "24h");
    }

    #[test]
    fn test_cache_config_parsing() {
        let toml_str = r#"
[[storage]]
type = "local"
path = "/test/path"

[cache]
enabled = true
directory = "/custom/cache"
ttl = "2h"
"#;
        
        let config: Config = toml::from_str(toml_str).unwrap();
        assert!(config.cache.enabled);
        assert_eq!(config.cache.directory, "/custom/cache");
        assert_eq!(config.cache.ttl, "2h");
    }

    #[test]
    fn test_parse_duration() {
        // Test various duration formats
        assert_eq!(parse_duration("30s").unwrap().as_secs(), 30);
        assert_eq!(parse_duration("5m").unwrap().as_secs(), 300);
        assert_eq!(parse_duration("2h").unwrap().as_secs(), 7200);
        assert_eq!(parse_duration("1d").unwrap().as_secs(), 86400);
        
        // Test case insensitive
        assert_eq!(parse_duration("1H").unwrap().as_secs(), 3600);
        
        // Test with full words
        assert_eq!(parse_duration("1hour").unwrap().as_secs(), 3600);
        assert_eq!(parse_duration("30minutes").unwrap().as_secs(), 1800);
        
        // Test numeric only (assumes seconds)
        assert_eq!(parse_duration("3600").unwrap().as_secs(), 3600);
        
        // Test error cases
        assert!(parse_duration("").is_err());
        assert!(parse_duration("invalid").is_err());
        assert!(parse_duration("10x").is_err());
        assert!(parse_duration("abc").is_err());
    }

    #[test]
    fn test_cache_config_ttl_parsing() {
        let cache_config = CacheConfig {
            enabled: true,
            directory: "./test".to_string(),
            ttl: "6h".to_string(),
        };
        
        let duration = cache_config.parse_ttl().unwrap();
        assert_eq!(duration.as_secs(), 6 * 3600);
    }

    #[test]
    fn test_backward_compatibility() {
        // Test that old configs without [cache] section still work
        let toml_str = r#"
[[storage]]
type = "local"
path = "/test/path"

[epoch_configs]
output_dir = "./configs"
"#;
        
        let config: Config = toml::from_str(toml_str).unwrap();
        assert!(config.validate().is_ok());
        assert!(!config.cache.enabled); // Should default to disabled
        assert_eq!(config.cache.directory, "./cache");
        assert_eq!(config.cache.ttl, "24h");
    }
}