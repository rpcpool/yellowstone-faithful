use std::time::Duration;

/// Configuration for the gRPC client
#[derive(Debug, Clone)]
pub struct GrpcConfig {
    /// gRPC endpoint URL (e.g., "http://localhost:8889")
    pub endpoint: String,

    /// Optional authentication token (sent as x-token header)
    pub token: Option<String>,

    /// Connection timeout
    pub connect_timeout: Duration,

    /// Request timeout
    pub request_timeout: Duration,

    /// Enable TLS
    pub tls: bool,

    /// Maximum number of retries on connection failure
    pub max_retries: u32,

    /// Initial backoff duration for retries
    pub retry_backoff: Duration,

    /// TCP keepalive interval
    pub tcp_keepalive: Option<Duration>,

    /// HTTP2 keepalive interval
    pub http2_keepalive_interval: Option<Duration>,

    /// HTTP2 keepalive timeout
    pub http2_keepalive_timeout: Option<Duration>,
}

impl Default for GrpcConfig {
    fn default() -> Self {
        Self {
            endpoint: "http://localhost:8889".to_string(),
            token: None,
            connect_timeout: Duration::from_secs(10),
            request_timeout: Duration::from_secs(30),
            tls: false,
            max_retries: 3,
            retry_backoff: Duration::from_millis(100),
            tcp_keepalive: Some(Duration::from_secs(60)),
            http2_keepalive_interval: Some(Duration::from_secs(30)),
            http2_keepalive_timeout: Some(Duration::from_secs(10)),
        }
    }
}

impl GrpcConfig {
    /// Create a new GrpcConfig with the given endpoint
    pub fn new(endpoint: impl Into<String>) -> Self {
        Self {
            endpoint: endpoint.into(),
            ..Default::default()
        }
    }

    /// Set the authentication token
    pub fn with_token(mut self, token: impl Into<String>) -> Self {
        self.token = Some(token.into());
        self
    }

    /// Set the connection timeout
    pub fn with_connect_timeout(mut self, timeout: Duration) -> Self {
        self.connect_timeout = timeout;
        self
    }

    /// Set the request timeout
    pub fn with_request_timeout(mut self, timeout: Duration) -> Self {
        self.request_timeout = timeout;
        self
    }

    /// Enable TLS
    pub fn with_tls(mut self, tls: bool) -> Self {
        self.tls = tls;
        self
    }

    /// Set the maximum number of retries
    pub fn with_max_retries(mut self, max_retries: u32) -> Self {
        self.max_retries = max_retries;
        self
    }
}
