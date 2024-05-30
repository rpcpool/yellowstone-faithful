pub use {prost, tonic};

pub mod proto {
    tonic::include_proto!("old_faithful");
}
