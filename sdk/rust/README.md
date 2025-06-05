# AXCP Rust SDK

A Rust implementation of the AXCP client SDK.

## Features

- Async/await support
- Telemetry data collection and batching
- Configurable timeouts and retries
- Comprehensive error handling
- Built-in metrics and tracing

## Getting Started

Add the following to your `Cargo.toml`:

```toml
[dependencies]
axcp-rs = { path = "path/to/axcp-spec/sdk/rust" }
```

## Usage

```rust
use axcp_rs::prelude::*;
use std::error::Error;

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    // Initialize the SDK
    axcp_rs::init()?;

    // Create a client with default configuration
    let config = ClientConfig {
        base_url: "http://localhost:8080".to_string(),
        api_key: Some("your-api-key".to_string()),
        ..Default::default()
    };

    let client = Client::new(config)?;

    // Send telemetry data
    let telemetry = client.telemetry();
    telemetry.record_metric("cpu.usage", 75.5).await?;

    // Or use the builder for more complex metrics
    let data = TelemetryBuilder::new("memory.used", 1024.0)
        .with_tag("host", "server-1")
        .with_tag("region", "us-west-2")
        .build();

    client.telemetry().record(data).await?;

    Ok(())
}
```

## Configuration

The `ClientConfig` struct supports the following options:

- `base_url`: The base URL of the AXCP server (default: `http://localhost:8080`)
- `api_key`: Optional API key for authentication
- `timeout_secs`: Request timeout in seconds (default: 30)
- `enable_telemetry`: Whether to enable telemetry collection (default: `true`)

## Testing

Run the tests with:

```bash
cargo test
```

## License

Apache 2.0
