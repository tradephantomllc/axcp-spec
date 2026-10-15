//! # AXCP Rust telemetry adapter (experimental)
//!
//! An experimental HTTP/JSON client that POSTs telemetry batches to a separately
//! supplied HTTP endpoint (`/api/v1/telemetry`), optionally with a Bearer token.
//!
//! This crate is **not** an implementation of the AXCP protocol: it does not provide
//! QUIC transport, Protobuf envelopes, DID/Ed25519 signatures, recipient binding or
//! replay protection. Use the Go, TypeScript or Python SDKs for authenticated AXCP
//! messaging.

#![warn(missing_docs)]
#![warn(rustdoc::missing_crate_level_docs)]

pub mod client;
pub mod error;
pub mod models;
pub mod telemetry;

/// Re-exports commonly used types
pub mod prelude {
    pub use crate::client::Client;
    pub use crate::error::{Error, Result};
    pub use crate::models::*;
    pub use crate::telemetry::TelemetryClient;
}

use error::Result;

/// Current version of the crate
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Initialize a default `tracing` subscriber for the adapter
///
/// # Errors
///
/// Returns an error if the logger setup fails
pub fn init() -> Result<()> {
    // Set up default tracing subscriber
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::from_default_env()
                .add_directive(tracing::Level::INFO.into()),
        )
        .try_init()
        .map_err(|e| error::Error::Other(e.to_string()))
}
