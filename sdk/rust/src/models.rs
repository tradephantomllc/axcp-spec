//! Data models for the AXCP protocol.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

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

/// Configuration for the AXCP client.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ClientConfig {
    /// The base URL of the AXCP server.
    pub base_url: String,
    /// The API key for authentication.
    pub api_key: Option<String>,
    /// The timeout for requests in seconds.
    #[serde(default = "default_timeout")]
    pub timeout_secs: u64,
    /// Whether to enable telemetry collection.
    #[serde(default = "default_true")]
    pub enable_telemetry: bool,
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
        }
    }
}
