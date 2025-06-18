//! AXCP ↔ PraisonAI bridge – Rust helpers
//!
//! This crate is intentionally minimal. It provides two helper functions:
//! - `wrap`   : encode an `AxcpEnvelope` protobuf message into bytes
//! - `unwrap` : decode bytes back into an `AxcpEnvelope`
//!
//! The protobuf definition (`AxcpEnvelope`) is compiled at build-time using `prost-build` in
//! `build.rs`. The generated code is included here via the `include!` macro.

pub mod pb {
    include!(concat!(env!("OUT_DIR"), "/axcp.rs"));
}

use prost::Message;
use pb::AxcpEnvelope;

/// Encode an [`AxcpEnvelope`] into a `Vec<u8>`.
pub fn wrap(env: AxcpEnvelope) -> Vec<u8> {
    let mut buf = Vec::with_capacity(env.encoded_len());
    env.encode(&mut buf).expect("encode AxcpEnvelope");
    buf
}

/// Decode bytes into an [`AxcpEnvelope`].
///
/// # Panics
/// Panics if the buffer does not contain a valid encoded `AxcpEnvelope`.
pub fn unwrap(buf: &[u8]) -> AxcpEnvelope {
    AxcpEnvelope::decode(buf).expect("decode AxcpEnvelope")
}
