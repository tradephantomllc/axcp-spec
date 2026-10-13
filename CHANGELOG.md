# Changelog

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
- Helm chart `enterprise-node` for Kubernetes deployments.
- Rust SDK async client (crate `axcp-sdk`).
- Differential-privacy budget CLI helpers.


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
