# Changelog

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
