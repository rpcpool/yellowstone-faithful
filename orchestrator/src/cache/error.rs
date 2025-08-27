//! Error types for the cache module.

use std::fmt;

/// Result type for cache operations
pub type CacheResult<T> = Result<T, CacheError>;

/// Errors that can occur during cache operations
#[derive(Debug)]
pub enum CacheError {
    /// IO error occurred during file operations
    Io(std::io::Error),
    
    /// Serialization/deserialization error
    Serialization(serde_json::Error),
    
    /// Cache entry not found
    NotFound(String),
    
    /// Cache entry has expired
    Expired(String),
    
    /// Cache is corrupted or invalid
    Corrupted(String),
    
    /// Lock acquisition failed
    LockFailed(String),
    
    /// Cache size limit exceeded
    SizeLimitExceeded {
        requested: u64,
        limit: u64,
    },
    
    /// Concurrent operation limit exceeded
    ConcurrencyLimitExceeded {
        active: usize,
        limit: usize,
    },
    
    /// Invalid cache key
    InvalidKey(String),
    
    /// Cache operation timed out
    Timeout(String),
    
    /// Configuration error
    Config(String),
    
    /// Generic error with message
    Other(String),
}

impl fmt::Display for CacheError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            CacheError::Io(err) => write!(f, "IO error: {}", err),
            CacheError::Serialization(err) => write!(f, "Serialization error: {}", err),
            CacheError::NotFound(key) => write!(f, "Cache entry not found: {}", key),
            CacheError::Expired(key) => write!(f, "Cache entry expired: {}", key),
            CacheError::Corrupted(msg) => write!(f, "Cache corrupted: {}", msg),
            CacheError::LockFailed(msg) => write!(f, "Lock acquisition failed: {}", msg),
            CacheError::SizeLimitExceeded { requested, limit } => {
                write!(f, "Cache size limit exceeded: requested {} bytes, limit {} bytes", requested, limit)
            }
            CacheError::ConcurrencyLimitExceeded { active, limit } => {
                write!(f, "Concurrency limit exceeded: {} active operations, limit {}", active, limit)
            }
            CacheError::InvalidKey(key) => write!(f, "Invalid cache key: {}", key),
            CacheError::Timeout(msg) => write!(f, "Cache operation timed out: {}", msg),
            CacheError::Config(msg) => write!(f, "Configuration error: {}", msg),
            CacheError::Other(msg) => write!(f, "Cache error: {}", msg),
        }
    }
}

impl std::error::Error for CacheError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            CacheError::Io(err) => Some(err),
            CacheError::Serialization(err) => Some(err),
            _ => None,
        }
    }
}

impl From<std::io::Error> for CacheError {
    fn from(err: std::io::Error) -> Self {
        CacheError::Io(err)
    }
}

impl From<serde_json::Error> for CacheError {
    fn from(err: serde_json::Error) -> Self {
        CacheError::Serialization(err)
    }
}

/// Convenience methods for creating specific errors
impl CacheError {
    /// Create a not found error
    pub fn not_found(key: &str) -> Self {
        CacheError::NotFound(key.to_string())
    }
    
    /// Create an expired error
    pub fn expired(key: &str) -> Self {
        CacheError::Expired(key.to_string())
    }
    
    /// Create a corrupted error
    pub fn corrupted(msg: &str) -> Self {
        CacheError::Corrupted(msg.to_string())
    }
    
    /// Create a lock failed error
    pub fn lock_failed(msg: &str) -> Self {
        CacheError::LockFailed(msg.to_string())
    }
    
    /// Create an invalid key error
    pub fn invalid_key(key: &str) -> Self {
        CacheError::InvalidKey(key.to_string())
    }
    
    /// Create a timeout error
    pub fn timeout(msg: &str) -> Self {
        CacheError::Timeout(msg.to_string())
    }
    
    /// Create a config error
    pub fn config(msg: &str) -> Self {
        CacheError::Config(msg.to_string())
    }
    
    /// Create a generic error
    pub fn other(msg: &str) -> Self {
        CacheError::Other(msg.to_string())
    }
}

/// Trait for converting errors to cache errors
pub trait IntoCacheError {
    fn into_cache_error(self, context: &str) -> CacheError;
}

impl IntoCacheError for std::io::Error {
    fn into_cache_error(self, context: &str) -> CacheError {
        match self.kind() {
            std::io::ErrorKind::NotFound => CacheError::NotFound(context.to_string()),
            std::io::ErrorKind::PermissionDenied => {
                CacheError::LockFailed(format!("Permission denied: {}", context))
            }
            std::io::ErrorKind::TimedOut => CacheError::Timeout(context.to_string()),
            _ => CacheError::Io(self),
        }
    }
}

impl IntoCacheError for serde_json::Error {
    fn into_cache_error(self, context: &str) -> CacheError {
        CacheError::Corrupted(format!("JSON error in {}: {}", context, self))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_error_display() {
        let err = CacheError::not_found("test_key");
        assert_eq!(err.to_string(), "Cache entry not found: test_key");
        
        let err = CacheError::SizeLimitExceeded {
            requested: 1000,
            limit: 500,
        };
        assert_eq!(err.to_string(), "Cache size limit exceeded: requested 1000 bytes, limit 500 bytes");
    }
    
    #[test]
    fn test_error_conversion() {
        let io_err = std::io::Error::new(std::io::ErrorKind::NotFound, "file not found");
        let cache_err = CacheError::from(io_err);
        matches!(cache_err, CacheError::Io(_));
    }
    
    #[test]
    fn test_into_cache_error() {
        let io_err = std::io::Error::new(std::io::ErrorKind::NotFound, "file not found");
        let cache_err = io_err.into_cache_error("test context");
        matches!(cache_err, CacheError::NotFound(_));
        
        let io_err = std::io::Error::new(std::io::ErrorKind::PermissionDenied, "permission denied");
        let cache_err = io_err.into_cache_error("test context");
        matches!(cache_err, CacheError::LockFailed(_));
    }
}