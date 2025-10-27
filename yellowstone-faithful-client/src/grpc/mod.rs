// This module contains gRPC client implementation

#[allow(clippy::all)]
#[allow(warnings)]
pub mod generated {
    // The generated code will be included here by build.rs
    include!(concat!(env!("OUT_DIR"), "/old_faithful.rs"));
}

pub mod client;

pub use client::{GrpcClient, GrpcBuilder, connect_with_config, InterceptorXToken};

