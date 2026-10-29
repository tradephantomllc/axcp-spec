# Changelog

## Unreleased

- **Go SDK (breaking where noted):**
  - Strict DID syntax (512-byte limit), native Ed25519 `did:key` (`EncodeDIDKey`, `DecodeDIDKey`, `GenerateIdentity`, `DIDKeyResolver`); resolvers can no longer bind a `did:key` to a different key; small-order keys rejected.
  - Replay protection rewritten as a bitmap window that never forgets active peers; idle-peer TTL raised to the timestamp window; bounded peers (fail closed); enabled by default in `NewValidator`, `ValidateRequest` and the middlewares.
  - Validators check version, profile, single payload member, unknown fields, undefined enums, sequence and signature length on every path; `AllowedDIDs` implemented. **Breaking:** unused `ClientConfig`, `NetworkConfig`, `RequireClientCert` and `RateLimitPerPeer` removed; payload-less envelopes are rejected; recipient errors map to `AUTH_DID_INVALID`.
  - **Breaking:** `auth.Session` requires a recipient and `PayloadForSequence`; sequences start at Unix microseconds. `CachingDIDResolver.PreloadDID` returns an error.
  - `FromBytesStrict` and strict QUIC receive; bounded frame reads with a body timeout; listener connection/stream limits and TLS 1.3 minimum; internal error text no longer sent to peers.
- **Gateway:** connection and stream limits (`-max-connections`, `-max-streams`), handshake/idle timeouts, strict frame decoding, code-only error replies, startup validation of `-trusted-did` (a `did:key` must encode the supplied key), loopback-only metrics with HTTP timeouts, dead buffer/telemetry code removed, `go vet` clean.
- **Examples:** `secure_baseline` and `authenticated_chat` use real `did:key` identities and full SDK validation in both directions; the replay demo uses the SDK `ReplayProtector`.
- Specification v1.0 (2026-10-15 revision): byte-exact canonical signing payload, strict envelope parsing, DID syntax and length limits, `did:key` derivation and resolver precedence, small-order key rejection, replay window and peer-state lifetime rules, expanded required rejections and error-code mapping; nonce mode withdrawn and profile negotiation wire format declared informative. See the specification's [Revision History](spec/axcp-v1.0.md#revision-history).
- **TypeScript SDK:** received bytes are kept in an amortized constant-time queue that coalesces small chunks; `TlsServer` and `TcpServer` limit concurrent connections (`maxConnections`, default 512) and close connections that deliver no complete frame within `firstFrameTimeoutMs` (default 10 s); `TlsServer` applies a 10 s TLS handshake timeout (`handshakeTimeoutMs`).
- **Breaking (2026-07-05, recorded here):** the Secure Baseline transcript was revised to bind sender DID, recipient DID, the canonical signing payload (including `sequence`) and the timestamp at second precision. The label `AXCP-DID-AUTH-v1` is unchanged; peers implementing the original `v1.0.0` transcript cannot interoperate with current peers.

## Unreleased — Core readiness corrections (2026-10-13)

- Handle Python QUIC STOP_SENDING and RESET_STREAM directionally, preserve readable error replies, always release the client connection, and normalize timeouts on Python 3.10+.
- Pin Go/Python protobuf generation to the same compiler and include license/notice files in SDK packages.

- Gateway authentication defaults on; settings are validated before network side effects. Signed telemetry uses `0xA1` and the same verifier/replay state as streams; legacy `0xA0` is development-only.
- Bound Go stream allocations/writes, preserve framing endianness, and reject invalid signed negotiation responses.
- Reuse Python SDK canonical bridge signatures, including sequence; fail closed when signing cannot be completed.
- Wait for MQTT QoS 1 acknowledgement and preserve original message/route through bounded in-memory retries.
- Consolidate Go netquic and telemetry examples into the parent SDK module. Consumers must remove obsolete nested-module requirements.
- Preserve repeated scalar defaults in TypeScript canonical Protobuf encoding; add a shared interoperability vector.
- Correct Rust batching, cancellation/error retention and immediate metric sends; honor HTTP adapter configuration.
- Align supported toolchain/dependencies, Docker build context, compose TLS/trust configuration and documentation with implemented Core scope.

Historical entries below describe their release context. Current support and limitations are defined by the README and maintained guides.

## v1.0.0 – 2026-01-29

### AXCP Core v1.0.0 (Secure Baseline)

First stable release of AXCP Core under Apache 2.0 open source license.

### Added

- **Secure Baseline Profile** (`secure-baseline-v1`): Production-ready security profile with mandatory authentication
- **DID Authentication**: Mutual authentication using Decentralized Identifiers and Ed25519 signatures
- **Replay Protection**: Built-in replay attack prevention with sliding window or nonce-based modes
- **Profile Negotiation**: Automatic security profile negotiation between peers
- **Authenticated Chat Example**: Complete working example demonstrating bidirectional authenticated exchange

### Documentation

- [AXCP Core Specification v1.0](spec/axcp-v1.0.md) - Authoritative protocol specification
- [Getting Started Guide](docs/getting-started.md) - Installation and quickstart
- [Authentication Guide](docs/authentication.md) - DID model, Ed25519 signing, replay protection
- [Gateway Setup Guide](docs/gateway-setup.md) - Gateway configuration for Secure Baseline

### Boundary

Advanced and Enterprise features (Differential Privacy, CRDT sync, mTLS management, PII filtering, compliance reporting) are delivered in separate private repositories and are not included in AXCP Core.

---

## 0.3-edge-beta – 2025-06-20

### Added

- QUIC DATAGRAM support for low-latency telemetry (#34).
- Rust SDK async client (crate `axcp-sdk`).


### Changed

- Gateway retry buffer configurable via flags and env vars.
- Updated protocol spec to `v0.3-edge-beta`.


### Fixed

- Histogram Prometheus metrics race conditions.
- gRPC stream timeouts on high latency networks.


### Breaking

- Replaced alpha version designators with semantic `edge-beta` pre-release tag.
- Gateway build now requires Go ≥1.23.

---

## May 2025
- Created and activated branch protection ruleset for `main` (Issue #SEC-1)
