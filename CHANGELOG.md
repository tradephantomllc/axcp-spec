# Changelog

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
