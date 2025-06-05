//! Telemetry functionality for the AXCP SDK.

use crate::client::Client;
use crate::error::Result;
use crate::models::TelemetryData;
use std::collections::HashMap;
use std::time::{SystemTime, UNIX_EPOCH};

/// A builder for creating telemetry data points.
#[derive(Debug, Default)]
pub struct TelemetryBuilder {
    metric: String,
    value: f64,
    tags: HashMap<String, String>,
    timestamp: Option<i64>,
}

impl TelemetryBuilder {
    /// Creates a new telemetry builder with the given metric name and value.
    pub fn new(metric: impl Into<String>, value: f64) -> Self {
        Self {
            metric: metric.into(),
            value,
            ..Default::default()
        }
    }

    /// Adds a tag to the telemetry data.
    pub fn with_tag(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
        self.tags.insert(key.into(), value.into());
        self
    }

    /// Sets the timestamp of the telemetry data.
    pub fn with_timestamp(mut self, timestamp: i64) -> Self {
        self.timestamp = Some(timestamp);
        self
    }

    /// Builds the telemetry data point.
    pub fn build(self) -> TelemetryData {
        TelemetryData {
            metric: self.metric,
            value: self.value,
            tags: self.tags,
            timestamp: self.timestamp.or_else(|| {
                SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .ok()
                    .map(|d| d.as_millis() as i64)
            }),
        }
    }
}

/// A telemetry client for recording metrics.
#[derive(Debug, Clone)]
pub struct TelemetryClient {
    client: Client,
}

impl TelemetryClient {
    /// Creates a new telemetry client.
    pub fn new(client: Client) -> Self {
        Self { client }
    }

    /// Records a metric with the given name and value.
    pub async fn record_metric(
        &self,
        metric: impl Into<String>,
        value: f64,
    ) -> Result<()> {
        let data = TelemetryBuilder::new(metric, value).build();
        self.client.telemetry().record(data).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_telemetry_builder() {
        let data = TelemetryBuilder::new("test.metric", 42.0)
            .with_tag("tag1", "value1")
            .with_tag("tag2", "value2")
            .with_timestamp(1234567890)
            .build();

        assert_eq!(data.metric, "test.metric");
        assert_eq!(data.value, 42.0);
        assert_eq!(data.tags.get("tag1"), Some(&"value1".to_string()));
        assert_eq!(data.tags.get("tag2"), Some(&"value2".to_string()));
        assert_eq!(data.timestamp, Some(1234567890));
    }
}
