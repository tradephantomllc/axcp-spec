//! Experimental HTTP/JSON telemetry client.
//!
//! This is not an AXCP protocol client: requests are plain HTTP(S) + JSON with an
//! optional Bearer token, without AXCP envelopes, signatures or replay protection.

use crate::error::{Error, Result};
use crate::models::{ClientConfig, TelemetryBatch, TelemetryData};
use reqwest::Client as HttpClient;
use std::sync::Arc;
use tokio::sync::Mutex;

/// Maximum number of bytes of an error response body included in [`Error::Server`].
pub const MAX_ERROR_BODY_BYTES: usize = 4096;

/// HTTP client for a telemetry ingestion endpoint.
///
/// `Debug` output never contains the configured API key.
#[derive(Debug, Clone)]
pub struct Client {
    inner: Arc<ClientInner>,
}

#[derive(Debug)]
struct ClientInner {
    http_client: HttpClient,
    config: ClientConfig,
}

impl Client {
    /// Creates a new client with the given configuration.
    pub fn new(config: ClientConfig) -> Result<Self> {
        if config.timeout_secs == 0 {
            return Err(Error::Config(
                "HTTP timeout must be greater than zero".into(),
            ));
        }
        let base_url = url::Url::parse(&config.base_url)
            .map_err(|err| Error::Config(format!("invalid telemetry base URL: {err}")))?;
        if !matches!(base_url.scheme(), "http" | "https") || base_url.host_str().is_none() {
            return Err(Error::Config(
                "telemetry base URL must use HTTP or HTTPS with a host".into(),
            ));
        }
        if config.api_key.is_some() && base_url.scheme() == "http" && !config.allow_insecure_http {
            return Err(Error::Config(
                "refusing to send an API key over plain http://; use https:// or set allow_insecure_http"
                    .into(),
            ));
        }
        let http_client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(config.timeout_secs))
            // Never forward the Bearer token to a redirect target.
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .map_err(Error::from)?;

        Ok(Self {
            inner: Arc::new(ClientInner {
                http_client,
                config,
            }),
        })
    }

    /// Sends a batch of telemetry data to the server.
    pub async fn send_telemetry(&self, batch: TelemetryBatch) -> Result<()> {
        let url = format!(
            "{}/api/v1/telemetry",
            self.inner.config.base_url.trim_end_matches('/')
        );

        if !self.inner.config.enable_telemetry {
            return Err(Error::Config(
                "telemetry is disabled in ClientConfig".into(),
            ));
        }
        let mut request = self.inner.http_client.post(&url).json(&batch);
        if let Some(api_key) = &self.inner.config.api_key {
            request = request.bearer_auth(api_key);
        }
        let response = request.send().await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = read_capped_body(response, MAX_ERROR_BODY_BYTES).await;
            return Err(Error::Server(format!("Server returned {status}: {body}")));
        }

        Ok(())
    }

    /// Creates a telemetry client for sending metrics.
    pub fn telemetry(&self) -> TelemetryClient {
        TelemetryClient::new(self.clone())
    }
}

/// Reads at most `limit` bytes of a response body (lossy UTF-8), never the whole body.
async fn read_capped_body(mut response: reqwest::Response, limit: usize) -> String {
    let mut body = Vec::with_capacity(limit.min(1024));
    while body.len() < limit {
        match response.chunk().await {
            Ok(Some(chunk)) => {
                let take = chunk.len().min(limit - body.len());
                body.extend_from_slice(&chunk[..take]);
            }
            Ok(None) | Err(_) => break,
        }
    }
    String::from_utf8_lossy(&body).into_owned()
}

/// A client specifically for sending telemetry data.
#[derive(Debug, Clone)]
pub struct TelemetryClient {
    client: Client,
    buffer: std::sync::Arc<Mutex<Vec<TelemetryData>>>,
    batch_size: usize,
}

impl TelemetryClient {
    /// Creates a new telemetry client.
    pub fn new(client: Client) -> Self {
        Self {
            client,
            buffer: std::sync::Arc::new(Mutex::new(Vec::with_capacity(100))),
            batch_size: 100,
        }
    }

    /// Sets the batch size for sending telemetry data.
    pub fn with_batch_size(mut self, batch_size: usize) -> Self {
        self.batch_size = batch_size;
        self
    }

    /// Records a single telemetry data point.
    pub async fn record(&self, data: TelemetryData) -> Result<()> {
        if self.batch_size == 0 {
            return Err(Error::Config(
                "telemetry batch size must be greater than zero".into(),
            ));
        }
        let should_flush = {
            let mut buffer = self.buffer.lock().await;
            if buffer.len() >= self.batch_size {
                return Err(Error::TelemetryBufferFull {
                    capacity: self.batch_size,
                });
            }
            buffer.push(data);
            buffer.len() >= self.batch_size
        };
        if should_flush {
            self.flush().await?;
        }

        Ok(())
    }

    /// Flushes any buffered telemetry data to the server.
    pub async fn flush(&self) -> Result<()> {
        // Serialize flushes and retain the original points until acknowledgement.
        // An HTTP error or cancellation leaves the buffer available for retry.
        let mut buffer = self.buffer.lock().await;
        if !buffer.is_empty() {
            self.client
                .send_telemetry(TelemetryBatch {
                    points: buffer.clone(),
                })
                .await?;
            buffer.clear();
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use mockito::Server;

    #[tokio::test]
    async fn test_send_telemetry() {
        // Start a mock server
        let mut server = Server::new_async().await;

        // Create a mock for the telemetry endpoint
        let _m = server
            .mock("POST", "/api/v1/telemetry")
            .with_status(200)
            .create_async()
            .await;

        // Create client with mock server URL
        let config = ClientConfig {
            base_url: server.url(),
            ..Default::default()
        };

        let client = Client::new(config).unwrap();
        let batch = TelemetryBatch { points: vec![] };

        // Send telemetry and verify the result
        let result = client.send_telemetry(batch).await;
        assert!(result.is_ok());

        // Verify the mock was called
        _m.assert_async().await;
    }
    #[tokio::test]
    async fn threshold_flush_retains_failed_batch_and_retries_exact_data() {
        let mut server = Server::new_async().await;
        let client = Client::new(ClientConfig {
            base_url: server.url(),
            api_key: Some("test-key".into()),
            allow_insecure_http: true,
            ..Default::default()
        })
        .unwrap();
        let telemetry = client.telemetry().with_batch_size(2);
        let point = crate::telemetry::TelemetryBuilder::new("cpu", 42.0)
            .with_timestamp(123)
            .build();
        telemetry.record(point.clone()).await.unwrap();
        let failed = server
            .mock("POST", "/api/v1/telemetry")
            .with_status(503)
            .expect(1)
            .create_async()
            .await;
        let result = tokio::time::timeout(
            std::time::Duration::from_secs(2),
            telemetry.record(point.clone()),
        )
        .await
        .expect("batch threshold deadlocked");
        assert!(result.is_err());
        failed.assert_async().await;
        failed.remove_async().await;
        let expected = serde_json::to_value(TelemetryBatch {
            points: vec![point.clone(), point],
        })
        .unwrap();
        let success = server
            .mock("POST", "/api/v1/telemetry")
            .match_header("authorization", "Bearer test-key")
            .match_body(mockito::Matcher::Json(expected))
            .with_status(200)
            .expect(1)
            .create_async()
            .await;
        let (left, right) = tokio::join!(telemetry.flush(), telemetry.flush());
        left.unwrap();
        right.unwrap();
        success.assert_async().await;
        assert!(telemetry.buffer.lock().await.is_empty());
    }

    #[tokio::test]
    async fn cancelled_flush_preserves_buffer() {
        use tokio::io::AsyncReadExt;
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let client = Client::new(ClientConfig {
            base_url: format!("http://{}", listener.local_addr().unwrap()),
            ..Default::default()
        })
        .unwrap();
        let telemetry = client.telemetry();
        telemetry
            .record(crate::telemetry::TelemetryBuilder::new("pending", 1.0).build())
            .await
            .unwrap();
        let (seen_tx, seen_rx) = tokio::sync::oneshot::channel();
        let server = tokio::spawn(async move {
            let (mut stream, _) = listener.accept().await.unwrap();
            let mut bytes = [0u8; 4096];
            let count = stream.read(&mut bytes).await.unwrap();
            assert!(count > 0, "client closed before sending its request");
            seen_tx.send(()).unwrap();
            std::future::pending::<()>().await;
        });
        let cloned = telemetry.clone();
        let flush = tokio::spawn(async move { cloned.flush().await });
        tokio::time::timeout(std::time::Duration::from_secs(2), seen_rx)
            .await
            .unwrap()
            .unwrap();
        flush.abort();
        assert!(flush.await.unwrap_err().is_cancelled());
        assert_eq!(telemetry.buffer.lock().await.len(), 1);
        server.abort();
        let _ = server.await;
    }

    #[tokio::test]
    async fn immediate_metric_and_invalid_configuration() {
        let mut server = Server::new_async().await;
        let client = Client::new(ClientConfig {
            base_url: server.url(),
            ..Default::default()
        })
        .unwrap();
        let expected = server
            .mock("POST", "/api/v1/telemetry")
            .match_body(mockito::Matcher::PartialJson(
                serde_json::json!({"points":[{"metric":"single","value":2.0}]}),
            ))
            .with_status(200)
            .expect(1)
            .create_async()
            .await;
        crate::telemetry::TelemetryClient::new(client.clone())
            .record_metric("single", 2.0)
            .await
            .unwrap();
        expected.assert_async().await;
        let invalid = client.telemetry().with_batch_size(0);
        assert!(matches!(
            invalid
                .record(crate::telemetry::TelemetryBuilder::new("bad", 0.0).build())
                .await,
            Err(Error::Config(_))
        ));
        assert!(invalid.buffer.lock().await.is_empty());
        let disabled = Client::new(ClientConfig {
            enable_telemetry: false,
            ..Default::default()
        })
        .unwrap();
        assert!(matches!(
            disabled.send_telemetry(TelemetryBatch::default()).await,
            Err(Error::Config(_))
        ));
        assert!(Client::new(ClientConfig {
            timeout_secs: 0,
            ..Default::default()
        })
        .is_err());
    }
    #[test]
    fn bearer_token_requires_https_unless_explicitly_allowed() {
        let insecure = ClientConfig {
            base_url: "http://telemetry.example".into(),
            api_key: Some("secret".into()),
            ..Default::default()
        };
        assert!(matches!(
            Client::new(insecure.clone()),
            Err(Error::Config(_))
        ));
        assert!(Client::new(ClientConfig {
            allow_insecure_http: true,
            ..insecure.clone()
        })
        .is_ok());
        assert!(Client::new(ClientConfig {
            base_url: "https://telemetry.example".into(),
            ..insecure.clone()
        })
        .is_ok());
        assert!(Client::new(ClientConfig {
            api_key: None,
            ..insecure
        })
        .is_ok());
    }

    #[test]
    fn debug_output_redacts_api_key() {
        let config = ClientConfig {
            base_url: "https://telemetry.example".into(),
            api_key: Some("super-secret-token".into()),
            ..Default::default()
        };
        let client = Client::new(config.clone()).unwrap();
        for rendered in [format!("{config:?}"), format!("{client:?}")] {
            assert!(!rendered.contains("super-secret-token"), "{rendered}");
            assert!(rendered.contains("<redacted>"), "{rendered}");
        }
    }

    #[tokio::test]
    async fn error_bodies_are_capped() {
        let mut server = Server::new_async().await;
        let _m = server
            .mock("POST", "/api/v1/telemetry")
            .with_status(500)
            .with_body("x".repeat(MAX_ERROR_BODY_BYTES * 8))
            .create_async()
            .await;
        let client = Client::new(ClientConfig {
            base_url: server.url(),
            ..Default::default()
        })
        .unwrap();
        match client.send_telemetry(TelemetryBatch::default()).await {
            Err(Error::Server(message)) => {
                assert!(
                    message.len() <= MAX_ERROR_BODY_BYTES + 64,
                    "{}",
                    message.len()
                );
                assert!(message.contains("500"));
            }
            other => panic!("unexpected result: {other:?}"),
        }
    }

    #[tokio::test]
    async fn redirects_are_not_followed_with_credentials() {
        let mut server = Server::new_async().await;
        let _redirect = server
            .mock("POST", "/api/v1/telemetry")
            .with_status(307)
            .with_header("location", "https://elsewhere.example/collect")
            .create_async()
            .await;
        let client = Client::new(ClientConfig {
            base_url: server.url(),
            api_key: Some("token".into()),
            allow_insecure_http: true,
            ..Default::default()
        })
        .unwrap();
        assert!(matches!(
            client.send_telemetry(TelemetryBatch::default()).await,
            Err(Error::Server(_))
        ));
    }

    #[tokio::test]
    async fn sustained_outage_keeps_a_bounded_pending_batch() {
        let mut server = Server::new_async().await;
        let _failure = server
            .mock("POST", "/api/v1/telemetry")
            .with_status(503)
            .create_async()
            .await;
        let client = Client::new(ClientConfig {
            base_url: server.url(),
            ..Default::default()
        })
        .unwrap();
        let telemetry = client.telemetry().with_batch_size(2);
        let point = crate::telemetry::TelemetryBuilder::new("outage", 1.0).build();
        telemetry.record(point.clone()).await.unwrap();
        assert!(telemetry.record(point.clone()).await.is_err());
        for _ in 0..5 {
            assert!(matches!(
                telemetry.record(point.clone()).await,
                Err(Error::TelemetryBufferFull { capacity: 2 })
            ));
        }
        assert_eq!(
            telemetry.buffer.lock().await.len(),
            2,
            "outage must not grow the retained batch"
        );
    }
}
