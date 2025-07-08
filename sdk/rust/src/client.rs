//! Client implementation for the AXCP protocol.

use crate::error::{Error, Result};
use crate::models::{ClientConfig, TelemetryBatch, TelemetryData};
use reqwest::Client as HttpClient;
use std::sync::Arc;
use tokio::sync::Mutex;

/// Client for interacting with the AXCP server.
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
        let http_client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(config.timeout_secs))
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
        let url = format!("{}/api/v1/telemetry", self.inner.config.base_url);
        
        let response = self.inner.http_client
            .post(&url)
            .json(&batch)
            .send()
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            return Err(Error::Server(format!(
                "Server returned {status}: {body}"
            )));
        }

        Ok(())
    }

    /// Creates a telemetry client for sending metrics.
    pub fn telemetry(&self) -> TelemetryClient {
        TelemetryClient::new(self.clone())
    }
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
        let mut buffer = self.buffer.lock().await;
        buffer.push(data);

        if buffer.len() >= self.batch_size {
            self.flush().await?;
        }

        Ok(())
    }

    /// Flushes any buffered telemetry data to the server.
    pub async fn flush(&self) -> Result<()> {
        let batch = {
            let mut buffer = self.buffer.lock().await;
            let batch = TelemetryBatch {
                points: buffer.drain(..).collect(),
            };
            batch
        };

        if !batch.points.is_empty() {
            self.client.send_telemetry(batch).await?;
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
}
