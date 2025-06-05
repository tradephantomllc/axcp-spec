//! # AXCP Rust SDK
//! 
//! A Rust implementation of the AXCP client SDK.

#![warn(missing_docs)]
#![warn(rustdoc::missing_crate_level_docs)]
#![doc(html_logo_url = "https://example.com/logo.png")]

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

/// Current version of the SDK
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Initialize the SDK with default settings
pub fn init() -> Result<(), Box<dyn std::error::Error>> {
    // Set up default tracing subscriber
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env()
            .add_directive(tracing::Level::INFO.into()))
        .init();
    
    Ok(())
}
