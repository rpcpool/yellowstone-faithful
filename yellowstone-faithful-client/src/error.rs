use thiserror::Error;

/// Main error type for the Old Faithful client
#[derive(Debug, Error)]
pub enum FaithfulError {
    /// gRPC transport or connection error
    #[error("gRPC error: {0}")]
    GrpcError(#[from] tonic::Status),

    /// gRPC transport error
    #[error("gRPC transport error: {0}")]
    GrpcTransportError(#[from] tonic::transport::Error),

    /// Data not found error
    #[error("Data not found: {0}")]
    NotFound(String),

    /// Invalid parameter error
    #[error("Invalid parameter: {0}")]
    InvalidParameter(String),

    /// Internal error
    #[error("Internal error: {0}")]
    Internal(String),

    /// Timeout error
    #[error("Request timeout")]
    Timeout,

    /// Connection error
    #[error("Connection error: {0}")]
    ConnectionError(String),

    /// Protocol mismatch error
    #[error("Protocol mismatch: {0}. The endpoint may not be a valid gRPC server.")]
    ProtocolMismatch(String),

    /// Invalid response error
    #[error("Invalid response: {0}")]
    InvalidResponse(String),

    /// Base58 decoding error
    #[error("Base58 decode error: {0}")]
    Base58DecodeError(#[from] bs58::decode::Error),

    /// Base64 decoding error
    #[error("Base64 decode error: {0}")]
    Base64DecodeError(#[from] base64::DecodeError),

    /// Generic error
    #[error("Error: {0}")]
    Other(#[from] anyhow::Error),
}

/// Type alias for Results using FaithfulError
pub type Result<T> = std::result::Result<T, FaithfulError>;

impl From<crate::grpc::client::GrpcClientError> for FaithfulError {
    fn from(err: crate::grpc::client::GrpcClientError) -> Self {
        match err {
            crate::grpc::client::GrpcClientError::TonicStatus(status) => {
                FaithfulError::GrpcError(status)
            }
            crate::grpc::client::GrpcClientError::TransportError(msg) => {
                FaithfulError::ConnectionError(msg)
            }
        }
    }
}

impl From<crate::grpc::client::GrpcBuilderError> for FaithfulError {
    fn from(err: crate::grpc::client::GrpcBuilderError) -> Self {
        match err {
            crate::grpc::client::GrpcBuilderError::MetadataValueError(e) => {
                FaithfulError::InvalidParameter(e.to_string())
            }
            crate::grpc::client::GrpcBuilderError::TonicError(e) => {
                FaithfulError::ConnectionError(e.to_string())
            }
        }
    }
}

impl FaithfulError {
    /// Convert from gRPC GetResponseErrorCode
    pub fn from_grpc_error_code(code: i32, message: String) -> Self {
        match code {
            1 => FaithfulError::NotFound(message),
            _ => FaithfulError::Internal(message),
        }
    }
}
