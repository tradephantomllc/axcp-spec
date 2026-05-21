# AXCP Rust Telemetry Adapter

[![Rust](https://github.com/tradephantomllc/axcp-spec/actions/workflows/rust-ci.yml/badge.svg)](https://github.com/tradephantomllc/axcp-spec/actions/workflows/rust-ci.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This crate is kept in the public AXCP Core repository as an experimental Rust telemetry adapter. It is not currently published to crates.io and should not be treated as the canonical AXCP QUIC/protobuf transport implementation.

## Current Scope

- Async HTTP client surface with `tokio` and `reqwest`
- Telemetry data collection and batching helpers
- Configurable timeouts and retry policies
- Metrics and tracing hooks through `tracing`
- Optional WebSocket helper support for telemetry experiments

## Non-Goals

- No crates.io publication from this repository
- No claim of full protocol parity with the Go SDK
- No Advanced or Enterprise privacy/compliance implementation

## Local Use

Reference the crate by path from this repository:

```toml
[dependencies]
axcp-sdk = { path = "sdk/rust" }
```

Example:

```rust
use axcp::prelude::*;
use std::error::Error;

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    axcp::init()?;

    let config = ClientConfig {
        base_url: "http://localhost:8080".to_string(),
        api_key: Some("your-api-key".to_string()),
        ..Default::default()
    };

    let client = Client::new(config)?;
    client.telemetry().record_metric("cpu.usage", 75.5).await?;

    Ok(())
}
```

## Verification

```bash
cargo fmt --all --check
cargo clippy --all-targets -- -D warnings
cargo test
```
