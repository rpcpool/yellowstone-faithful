pub use tonic::{service::Interceptor, transport::ClientTlsConfig};
use {
    crate::{
        config::GrpcConfig,
        error::{FaithfulError, Result as FaithfulResult},
        grpc::generated::{
            old_faithful_client::OldFaithfulClient as GeneratedClient, BlockRequest,
            BlockTimeRequest, GetRequest, GetResponse, StreamBlocksRequest,
            StreamTransactionsRequest, TransactionRequest, VersionRequest,
        },
        models::{
            Block, BlockTime, StreamBlocksFilter, StreamTransactionsFilter, TransactionWithContext,
            VersionInfo,
        },
    },
    futures::Stream,
    std::{pin::Pin, time::Duration},
    tonic::{
        metadata::{errors::InvalidMetadataValue, AsciiMetadataValue, MetadataValue},
        service::interceptor::InterceptedService,
        transport::channel::{Channel, Endpoint},
        Request, Status, Streaming,
    },
    tracing::{debug, info},
};

#[derive(Debug, Clone)]
pub struct InterceptorXToken {
    pub x_token: Option<AsciiMetadataValue>,
    pub x_request_snapshot: bool,
}

impl Interceptor for InterceptorXToken {
    fn call(&mut self, mut request: Request<()>) -> Result<Request<()>, Status> {
        if let Some(x_token) = self.x_token.clone() {
            request.metadata_mut().insert("x-token", x_token);
        }
        if self.x_request_snapshot {
            request
                .metadata_mut()
                .insert("x-request-snapshot", MetadataValue::from_static("true"));
        }
        Ok(request)
    }
}

#[derive(Debug, thiserror::Error)]
pub enum GrpcClientError {
    #[error("gRPC status: {0}")]
    TonicStatus(#[from] Status),
    #[error("gRPC transport error: {0}")]
    TransportError(String),
}

pub type GrpcClientResult<T> = Result<T, GrpcClientError>;

pub struct GrpcClient<F> {
    pub client: GeneratedClient<InterceptedService<Channel, F>>,
}

impl GrpcClient<()> {
    pub fn build_from_shared(endpoint: impl Into<bytes::Bytes>) -> GrpcBuilderResult<GrpcBuilder> {
        Ok(GrpcBuilder::new(Endpoint::from_shared(endpoint)?))
    }

    pub fn build_from_static(endpoint: &'static str) -> GrpcBuilder {
        GrpcBuilder::new(Endpoint::from_static(endpoint))
    }
}

impl<F: Interceptor + 'static> GrpcClient<F> {
    pub const fn new(client: GeneratedClient<InterceptedService<Channel, F>>) -> Self {
        Self { client }
    }

    /// Get version information
    pub async fn get_version(&mut self) -> GrpcClientResult<VersionInfo> {
        debug!("Requesting version");
        let request = Request::new(VersionRequest {});
        let response = self.client.get_version(request).await?.into_inner();
        Ok(VersionInfo {
            version: response.version,
        })
    }

    /// Get a block by slot number
    pub async fn get_block(&mut self, slot: u64) -> GrpcClientResult<Block> {
        debug!("Requesting block at slot {}", slot);
        let request = Request::new(BlockRequest { slot });
        let response = self.client.get_block(request).await?.into_inner();
        Block::from_grpc(response).map_err(|e| GrpcClientError::TransportError(e.to_string()))
    }

    /// Get block time for a slot
    pub async fn get_block_time(&mut self, slot: u64) -> GrpcClientResult<BlockTime> {
        debug!("Requesting block time for slot {}", slot);
        let request = Request::new(BlockTimeRequest { slot });
        let response = self.client.get_block_time(request).await?.into_inner();
        Ok(BlockTime {
            slot,
            block_time: response.block_time,
        })
    }

    /// Get a transaction by signature
    pub async fn get_transaction(
        &mut self,
        signature: &[u8],
    ) -> GrpcClientResult<TransactionWithContext> {
        debug!("Requesting transaction with signature: {:?}", signature);
        let request = Request::new(TransactionRequest {
            signature: signature.to_vec(),
        });
        let response = self.client.get_transaction(request).await?.into_inner();
        TransactionWithContext::from_grpc(response)
            .map_err(|e| GrpcClientError::TransportError(e.to_string()))
    }

    /// Stream blocks in a slot range
    pub async fn stream_blocks(
        &mut self,
        start_slot: u64,
        end_slot: Option<u64>,
        filter: Option<StreamBlocksFilter>,
    ) -> GrpcClientResult<Pin<Box<dyn Stream<Item = FaithfulResult<Block>> + Send>>> {
        info!(
            "Streaming blocks from slot {} to {:?}",
            start_slot, end_slot
        );

        let filter_proto = filter.map(|f| crate::grpc::generated::StreamBlocksFilter {
            account_include: f.account_include,
        });

        let request = Request::new(StreamBlocksRequest {
            start_slot,
            end_slot,
            filter: filter_proto,
        });

        let response = self.client.stream_blocks(request).await?;
        let stream = response.into_inner();

        Ok(Box::pin(Self::map_block_stream(stream)))
    }

    /// Stream transactions in a slot range
    pub async fn stream_transactions(
        &mut self,
        start_slot: u64,
        end_slot: Option<u64>,
        filter: Option<StreamTransactionsFilter>,
    ) -> GrpcClientResult<Pin<Box<dyn Stream<Item = FaithfulResult<TransactionWithContext>> + Send>>>
    {
        info!(
            "Streaming transactions from slot {} to {:?}",
            start_slot, end_slot
        );

        let filter_proto = filter.map(|f| crate::grpc::generated::StreamTransactionsFilter {
            vote: f.vote,
            failed: f.failed,
            account_include: f.account_include,
            account_exclude: f.account_exclude,
            account_required: f.account_required,
        });

        let request = Request::new(StreamTransactionsRequest {
            start_slot,
            end_slot,
            filter: filter_proto,
        });

        let response = self.client.stream_transactions(request).await?;
        let stream = response.into_inner();

        Ok(Box::pin(Self::map_transaction_stream(stream)))
    }

    /// Batch get method using the Get RPC
    pub async fn batch_get(
        &mut self,
        requests: Vec<GetRequest>,
    ) -> GrpcClientResult<Pin<Box<dyn Stream<Item = FaithfulResult<GetResponse>> + Send>>> {
        debug!("Sending batch get with {} requests", requests.len());

        let request_stream = futures::stream::iter(requests);
        let request = Request::new(request_stream);

        let response = self.client.get(request).await?;
        let stream = response.into_inner();

        Ok(Box::pin(Self::map_get_response_stream(stream)))
    }

    /// Helper to map block stream responses
    fn map_block_stream(
        mut stream: Streaming<crate::grpc::generated::BlockResponse>,
    ) -> impl Stream<Item = FaithfulResult<Block>> {
        async_stream::stream! {
            while let Some(result) = stream.message().await.transpose() {
                match result {
                    Ok(block_response) => {
                        match Block::from_grpc(block_response) {
                            Ok(block) => yield Ok(block),
                            Err(e) => yield Err(e),
                        }
                    }
                    Err(e) => yield Err(FaithfulError::from(e)),
                }
            }
        }
    }

    /// Helper to map transaction stream responses
    fn map_transaction_stream(
        mut stream: Streaming<crate::grpc::generated::TransactionResponse>,
    ) -> impl Stream<Item = FaithfulResult<TransactionWithContext>> {
        async_stream::stream! {
            while let Some(result) = stream.message().await.transpose() {
                match result {
                    Ok(tx_response) => {
                        match TransactionWithContext::from_grpc(tx_response) {
                            Ok(tx) => yield Ok(tx),
                            Err(e) => yield Err(e),
                        }
                    }
                    Err(e) => yield Err(FaithfulError::from(e)),
                }
            }
        }
    }

    /// Helper to map GetResponse stream
    fn map_get_response_stream(
        mut stream: Streaming<GetResponse>,
    ) -> impl Stream<Item = FaithfulResult<GetResponse>> {
        async_stream::stream! {
            while let Some(result) = stream.message().await.transpose() {
                match result {
                    Ok(response) => yield Ok(response),
                    Err(e) => yield Err(FaithfulError::from(e)),
                }
            }
        }
    }
}

#[derive(Debug, thiserror::Error)]
pub enum GrpcBuilderError {
    #[error("Failed to parse x-token: {0}")]
    MetadataValueError(#[from] InvalidMetadataValue),
    #[error("gRPC transport error: {0}")]
    TonicError(#[from] tonic::transport::Error),
}

pub type GrpcBuilderResult<T> = Result<T, GrpcBuilderError>;

#[derive(Debug)]
pub struct GrpcBuilder {
    pub endpoint: Endpoint,
    pub x_token: Option<AsciiMetadataValue>,
    pub x_request_snapshot: bool,
}

impl GrpcBuilder {
    // Create new builder
    const fn new(endpoint: Endpoint) -> Self {
        Self {
            endpoint,
            x_token: None,
            x_request_snapshot: false,
        }
    }

    pub fn from_shared(endpoint: impl Into<bytes::Bytes>) -> GrpcBuilderResult<Self> {
        Ok(Self::new(Endpoint::from_shared(endpoint)?))
    }

    pub fn from_static(endpoint: &'static str) -> Self {
        Self::new(Endpoint::from_static(endpoint))
    }

    // Create client
    fn build(self, channel: Channel) -> GrpcBuilderResult<GrpcClient<InterceptorXToken>> {
        let interceptor = InterceptorXToken {
            x_token: self.x_token,
            x_request_snapshot: self.x_request_snapshot,
        };

        let client = GeneratedClient::with_interceptor(channel, interceptor);

        Ok(GrpcClient::new(client))
    }

    pub async fn connect(self) -> GrpcBuilderResult<GrpcClient<InterceptorXToken>> {
        let channel = self.endpoint.connect().await?;
        self.build(channel)
    }

    pub fn connect_lazy(self) -> GrpcBuilderResult<GrpcClient<InterceptorXToken>> {
        let channel = self.endpoint.connect_lazy();
        self.build(channel)
    }

    // Set x-token
    pub fn x_token<T>(self, x_token: Option<T>) -> GrpcBuilderResult<Self>
    where
        T: TryInto<AsciiMetadataValue, Error = InvalidMetadataValue>,
    {
        Ok(Self {
            x_token: x_token.map(|x_token| x_token.try_into()).transpose()?,
            ..self
        })
    }

    // Include `x-request-snapshot`
    pub fn set_x_request_snapshot(self, value: bool) -> Self {
        Self {
            x_request_snapshot: value,
            ..self
        }
    }

    // Endpoint options
    pub fn connect_timeout(self, dur: Duration) -> Self {
        Self {
            endpoint: self.endpoint.connect_timeout(dur),
            ..self
        }
    }

    pub fn buffer_size(self, sz: impl Into<Option<usize>>) -> Self {
        Self {
            endpoint: self.endpoint.buffer_size(sz),
            ..self
        }
    }

    pub fn http2_adaptive_window(self, enabled: bool) -> Self {
        Self {
            endpoint: self.endpoint.http2_adaptive_window(enabled),
            ..self
        }
    }

    pub fn http2_keep_alive_interval(self, interval: Duration) -> Self {
        Self {
            endpoint: self.endpoint.http2_keep_alive_interval(interval),
            ..self
        }
    }

    pub fn initial_connection_window_size(self, sz: impl Into<Option<u32>>) -> Self {
        Self {
            endpoint: self.endpoint.initial_connection_window_size(sz),
            ..self
        }
    }

    pub fn initial_stream_window_size(self, sz: impl Into<Option<u32>>) -> Self {
        Self {
            endpoint: self.endpoint.initial_stream_window_size(sz),
            ..self
        }
    }

    pub fn keep_alive_timeout(self, duration: Duration) -> Self {
        Self {
            endpoint: self.endpoint.keep_alive_timeout(duration),
            ..self
        }
    }

    pub fn keep_alive_while_idle(self, enabled: bool) -> Self {
        Self {
            endpoint: self.endpoint.keep_alive_while_idle(enabled),
            ..self
        }
    }

    pub fn tcp_keepalive(self, tcp_keepalive: Option<Duration>) -> Self {
        Self {
            endpoint: self.endpoint.tcp_keepalive(tcp_keepalive),
            ..self
        }
    }

    pub fn tcp_nodelay(self, enabled: bool) -> Self {
        Self {
            endpoint: self.endpoint.tcp_nodelay(enabled),
            ..self
        }
    }

    pub fn timeout(self, dur: Duration) -> Self {
        Self {
            endpoint: self.endpoint.timeout(dur),
            ..self
        }
    }

    pub fn tls_config(self, tls_config: ClientTlsConfig) -> GrpcBuilderResult<Self> {
        Ok(Self {
            endpoint: self.endpoint.tls_config(tls_config)?,
            ..self
        })
    }
}

// Helper function to create a client from GrpcConfig
pub async fn connect_with_config(
    config: GrpcConfig,
) -> FaithfulResult<GrpcClient<InterceptorXToken>> {
    let mut builder = GrpcClient::build_from_shared(config.endpoint.clone())
        .map_err(|e| FaithfulError::ConnectionError(e.to_string()))?;

    // Set x-token if provided
    builder = builder
        .x_token(config.token)
        .map_err(|e| FaithfulError::ConnectionError(e.to_string()))?;

    // Set timeouts
    builder = builder
        .connect_timeout(config.connect_timeout)
        .timeout(config.request_timeout);

    // Set TCP keepalive
    if let Some(keepalive) = config.tcp_keepalive {
        builder = builder.tcp_keepalive(Some(keepalive));
    }

    // Set HTTP2 keepalive
    if let Some(interval) = config.http2_keepalive_interval {
        builder = builder.http2_keep_alive_interval(interval);
    }

    if let Some(timeout) = config.http2_keepalive_timeout {
        builder = builder.keep_alive_timeout(timeout);
    }

    // Connect lazily (like yellowstone-grpc-client)
    builder
        .connect_lazy()
        .map_err(|e| FaithfulError::ConnectionError(e.to_string()))
}

#[cfg(test)]
mod tests {
    use super::GrpcClient;

    #[tokio::test]
    async fn test_channel_https_success() {
        let endpoint = "https://tirton-mainbf8-11ea.mainnet.rpcpool.com:443";
        let x_token = "test-token";

        let res = GrpcClient::build_from_shared(endpoint);
        assert!(res.is_ok());

        let res = res.unwrap().x_token(Some(x_token));
        assert!(res.is_ok());

        let res = res.unwrap().connect_lazy();
        assert!(res.is_ok());
    }

    #[tokio::test]
    async fn test_channel_http_success() {
        let endpoint = "http://127.0.0.1:10000";
        let x_token = "test-token";

        let res = GrpcClient::build_from_shared(endpoint);
        assert!(res.is_ok());

        let res = res.unwrap().x_token(Some(x_token));
        assert!(res.is_ok());

        let res = res.unwrap().connect_lazy();
        assert!(res.is_ok());
    }

    #[tokio::test]
    async fn test_channel_empty_token_some() {
        let endpoint = "http://127.0.0.1:10000";
        let x_token = "";

        let res = GrpcClient::build_from_shared(endpoint);
        assert!(res.is_ok());

        let res = res.unwrap().x_token(Some(x_token));
        assert!(res.is_ok());
    }

    #[tokio::test]
    async fn test_channel_invalid_token_none() {
        let endpoint = "http://127.0.0.1:10000";

        let res = GrpcClient::build_from_shared(endpoint);
        assert!(res.is_ok());

        let res = res.unwrap().x_token::<String>(None);
        assert!(res.is_ok());

        let res = res.unwrap().connect_lazy();
        assert!(res.is_ok());
    }

    #[tokio::test]
    async fn test_channel_invalid_uri() {
        let endpoint = "sites/files/images/picture.png";

        let res = GrpcClient::build_from_shared(endpoint);
        assert!(res.is_err(), "Expected error for invalid URI");
    }
}
