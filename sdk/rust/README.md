# AXCP Rust Telemetry Adapter

[![Rust](https://github.com/tradephantomllc/axcp-spec/actions/workflows/rust-ci.yml/badge.svg)](https://github.com/tradephantomllc/axcp-spec/actions/workflows/rust-ci.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This crate is kept in the public AXCP Core repository as an experimental Rust HTTP/JSON telemetry adapter. It is **not** an implementation of the AXCP protocol: it has no QUIC transport, Protobuf envelopes, DID/Ed25519 signatures, recipient binding or replay protection. It is not published to crates.io.

## Current Scope

- Async HTTP client surface with `tokio` and `reqwest`
- Telemetry data collection and batching helpers
- Configurable HTTP timeouts and explicit caller-controlled retry after failure
- Optional dependencies behind feature flags; WebSocket/configuration flags do not implement complete adapters

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
        base_url: "https://telemetry.example.com".to_string(),
        api_key: Some("your-api-key".to_string()),
        ..Default::default()
    };

    let client = Client::new(config)?;
    TelemetryClient::new(client).record_metric("cpu.usage", 75.5).await?;

    Ok(())
}
```

## Verification

```bash
cargo fmt --all --check
cargo clippy --locked --all-targets --all-features -- -D warnings
cargo test --locked --all-features
```

## Delivery semantics

The adapter POSTs JSON to `/api/v1/telemetry` on a separately supplied HTTP service. The Core QUIC/MQTT gateway does not expose that endpoint. `api_key` is sent as a Bearer token and requires an `https://` base URL; `Client::new` refuses a plain `http://` URL with an API key unless `allow_insecure_http: true` is set explicitly (local development only). Redirects are not followed, error response bodies are read up to 4 KiB, and `Debug` output redacts the API key. `enable_telemetry=false` returns an explicit configuration error. A positive HTTP timeout is required.

`telemetry::TelemetryClient::record_metric` sends one point immediately. `Client::telemetry()` returns the buffered `client::TelemetryClient`: call `record(point)`, then `flush()` for a partial final batch. Reaching its positive batch size flushes automatically without re-entering the buffer mutex. Flushes are serialized; errors or cancellation preserve the original batch for explicit retry. A remote success followed by a lost response can produce duplicates. The buffer is in memory and is not flushed by dropping a client.

The batch size is also the pending-point capacity. During an outage, a full buffer rejects new points with `Error::TelemetryBufferFull { capacity }` before accepting them; call `flush()` after recovery. An HTTP error from a threshold-triggered `record()` means that point is already retained, so do not enqueue the same point again blindly. Capacity is measured in points, and callers control point/string sizes.
