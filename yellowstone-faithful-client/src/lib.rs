pub mod config;
pub mod error;
pub mod grpc;
pub mod models;

// Re-export commonly used types
pub use config::GrpcConfig;
pub use error::{FaithfulError, Result};
pub use grpc::{connect_with_config, GrpcBuilder, GrpcClient, InterceptorXToken};
pub use models::*;

// Version information
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
pub const NAME: &str = env!("CARGO_PKG_NAME");

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_version() {
        assert!(!VERSION.is_empty());
        assert_eq!(NAME, "yellowstone-faithful-client");
    }
}
