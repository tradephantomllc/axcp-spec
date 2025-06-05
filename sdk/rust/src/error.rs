//! Error handling for the AXCP Rust SDK.

use std::fmt;
use thiserror::Error;

/// A type alias for `Result<T, Error>`.
pub type Result<T> = std::result::Result<T, Error>;

/// The error type for the AXCP SDK.
#[derive(Debug, Error)]
pub enum Error {
    /// Represents a configuration error.
    #[error("Configuration error: {0}")]
    Config(String),

    /// Represents an I/O error.
    #[error("I/O error: {source}")]
    Io {
        /// The source of the I/O error.
        #[from]
        source: std::io::Error,
    },

    /// Represents a network error.
    #[error("Network error: {0}")]
    Network(String),

    /// Represents a serialization/deserialization error.
    #[error("Serialization error: {0}")]
    Serialization(String),

    /// Represents a protocol error.
    #[error("Protocol error: {0}")]
    Protocol(String),

    /// Represents an authentication error.
    #[error("Authentication failed: {0}")]
    Auth(String),

    /// Represents a timeout error.
    #[error("Operation timed out")]
    Timeout,

    /// Represents an error from the server.
    #[error("Server error: {0}")]
    Server(String),


    /// Represents any other kind of error.
    #[error("Unexpected error: {0}")]
    Other(String),
}

impl From<reqwest::Error> for Error {
    fn from(err: reqwest::Error) -> Self {
        if err.is_timeout() {
            Error::Timeout
        } else if err.is_connect() {
            Error::Network(format!("Connection error: {}", err))
        } else if err.is_decode() {
            Error::Serialization(format!("Failed to decode response: {}", err))
        } else {
            Error::Network(err.to_string())
        }
    }
}

impl From<serde_json::Error> for Error {
    fn from(err: serde_json::Error) -> Self {
        Error::Serialization(err.to_string())
    }
}

impl From<url::ParseError> for Error {
    fn from(err: url::ParseError) -> Self {
        Error::Config(format!("Invalid URL: {}", err))
    }
}
