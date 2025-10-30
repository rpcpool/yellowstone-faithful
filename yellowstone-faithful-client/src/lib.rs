pub mod config;
pub mod error;
pub mod grpc;
pub mod models;

// Re-export commonly used types
pub use {
    config::GrpcConfig,
    error::{FaithfulError, Result},
    grpc::{connect_with_config, GrpcBuilder, GrpcClient, InterceptorXToken},
    models::*,
};

// Version information
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
pub const NAME: &str = env!("CARGO_PKG_NAME");

