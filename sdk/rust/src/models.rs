//! Data models for the experimental HTTP telemetry adapter.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fmt;

/// Represents a telemetry data point.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TelemetryData {
    /// The metric name.
    pub metric: String,
    /// The metric value.
    pub value: f64,
    /// Optional tags for the metric.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub tags: HashMap<String, String>,
    /// Timestamp in milliseconds since epoch.
    pub timestamp: Option<i64>,
}

/// Represents a batch of telemetry data points.
#[derive(Debug, Default, Serialize, Deserialize)]
pub struct TelemetryBatch {
    /// The telemetry data points in this batch.
    pub points: Vec<TelemetryData>,
}

/// Configuration for the telemetry HTTP client.
///
/// `Debug` output redacts `api_key`.
#[derive(Clone, Serialize, Deserialize)]
pub struct ClientConfig {
    /// The base URL of the telemetry HTTP endpoint.
    pub base_url: String,
    /// Bearer token sent with each request. Requires an `https://` base URL unless
    /// `allow_insecure_http` is set.
    pub api_key: Option<String>,
    /// The timeout for requests in seconds.
    #[serde(default = "default_timeout")]
    pub timeout_secs: u64,
    /// Whether to enable telemetry collection.
    #[serde(default = "default_true")]
    pub enable_telemetry: bool,
    /// Explicitly allow sending `api_key` over plain `http://` (local development only).
    #[serde(default)]
    pub allow_insecure_http: bool,
}

impl fmt::Debug for ClientConfig {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ClientConfig")
            .field("base_url", &self.base_url)
            .field("api_key", &self.api_key.as_ref().map(|_| "<redacted>"))
            .field("timeout_secs", &self.timeout_secs)
            .field("enable_telemetry", &self.enable_telemetry)
            .field("allow_insecure_http", &self.allow_insecure_http)
            .finish()
    }
}

fn default_timeout() -> u64 {
    30
}

fn default_true() -> bool {
    true
}

impl Default for ClientConfig {
    fn default() -> Self {
        Self {
            base_url: "http://localhost:8080".to_string(),
            api_key: None,
            timeout_secs: default_timeout(),
            enable_telemetry: true,
            allow_insecure_http: false,
        }
    }
}
