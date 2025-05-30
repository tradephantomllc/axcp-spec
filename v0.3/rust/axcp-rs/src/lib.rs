// SPDX-License-Identifier: Apache-2.0
// Placeholder – v0.3 structure only

//! # AXCP Rust Implementation
//! 
//! A high-performance, zero-copy implementation of the AXCP protocol in Rust.

#![warn(missing_docs)]
#![forbid(unsafe_code)]

pub mod client;
pub mod error;
pub mod quic;
pub mod types;

/// Re-exports for common usage
pub use client::AxcpClient;
pub use error::{AxcpError, Result};

/// Current protocol version
pub const PROTOCOL_VERSION: &str = "0.3.0";

/// Core AXCP client implementation
#[derive(Debug)]
pub struct AxcpClient {
    // TODO: Implement client state
}

impl AxcpClient {
    /// Creates a new AXCP client
    pub fn new() -> Self {
        Self {
            // Initialize client state
        }
    }

    /// Connects to an AXCP server
    pub async fn connect(&mut self, _endpoint: &str) -> Result<()> {
        // TODO: Implement connection logic
        Ok(())
    }
}

/// Error type for AXCP operations
#[derive(Debug, thiserror::Error)]
pub enum AxcpError {
    /// Connection error
    #[error("connection error: {0}")]
    Connection(#[from] std::io::Error),
    
    /// Protocol error
    #[error("protocol error: {0}")]
    Protocol(String),
    
    /// Invalid configuration
    #[error("invalid configuration: {0}")]
    Config(String),
}

/// Result type for AXCP operations
pub type Result<T> = std::result::Result<T, AxcpError>;
