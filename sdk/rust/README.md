# AXCP Rust SDK

[![Crates.io](https://img.shields.io/crates/v/axcp-rs.svg)](https://crates.io/crates/axcp-rs)
[![Documentation](https://docs.rs/axcp-rs/badge.svg)](https://docs.rs/axcp-rs)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Rust](https://github.com/tradephantom/axcp-spec/actions/workflows/rust-ci.yml/badge.svg)](https://github.com/tradephantom/axcp-spec/actions/workflows/rust-ci.yml)

Official Rust implementation of the Advanced eXchange Control Protocol (AXCP) client SDK.

## Features

- Async/await API with `tokio`
- Telemetry data collection and batching
- Configurable timeouts and retry policies
- Comprehensive error handling with `thiserror`
- Built-in metrics and tracing with `tracing`
- Multiple TLS backends (native-tls or rustls)
- WebSocket support for real-time communication

## Installation

Add this to your `Cargo.toml`:

```toml
[dependencies]
axcp-rs = "0.1.0-alpha.1"

# For async runtime (if not already in your project)
tokio = { version = "1.0", features = ["full"] }
```

### Feature Flags

- `default`: Uses `native-tls` for TLS
- `rustls`: Use `rustls` instead of `native-tls`
- `dev`: Include development dependencies for testing

```toml
[dependencies]
axcp-rs = { version = "0.1.0-alpha.1", default-features = false, features = ["rustls"] }
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
