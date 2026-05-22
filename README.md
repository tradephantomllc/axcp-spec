# axcp-spec

[![CI](https://github.com/tradephantomllc/axcp-spec/actions/workflows/ci.yml/badge.svg)](https://github.com/tradephantomllc/axcp-spec/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tradephantomllc/axcp-spec/sdk/go.svg)](https://pkg.go.dev/github.com/tradephantomllc/axcp-spec/sdk/go)
![License](https://img.shields.io/badge/license-Apache_2.0-green)

## Demo

[![AXCP Demo](https://img.youtube.com/vi/8rw4d-xnWks/maxresdefault.jpg)](https://youtu.be/8rw4d-xnWks)

> **Quick Start**: see [`docs/getting-started.md`](docs/getting-started.md)

AXCP – Adaptive eXchange Context Protocol. An open specification for ultra-efficient AI agent orchestration.
It improves on existing agent communication protocols (like MCP, A2A, and ACP) by integrating:

- QUIC + Protobuf transport for high-performance, low-latency communication
- Delta-synced context cache to reduce token overhead
- DIDComm v2 for decentralized capability negotiation and secure identity
- Smart routing between cloud and edge nodes
- Telemetry datagrams for real-time monitoring

> **Privacy & Security**: Differential Privacy, SGX enclaves, mTLS, and PII filtering are available in [AXCP Advanced/Enterprise](#axcp-enterprise) tiers.

## Specification

The authoritative protocol specification is **[AXCP Core Specification v1.0](spec/axcp-v1.0.md)** ("Secure Baseline").

This specification defines:
- Security profiles (`secure-baseline-v1` for production)
- DID-based mutual authentication with Ed25519 signatures
- Replay protection using sequence numbers or nonces
- Profile negotiation algorithm

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | Installation, setup, and running the authenticated chat example |
| [Authentication](docs/authentication.md) | DID model, Ed25519 signing, transcript format, replay protection |
| [Gateway Setup](docs/gateway-setup.md) | Configuring the gateway for Secure Baseline enforcement |
| [Python SDK](sdk/python/README.md) | Core Python SDK for DID identity, signing, verification, replay, and profile negotiation |
| [TypeScript SDK](sdk/typescript/README.md) | Core TypeScript SDK for DID identity, signing, verification, replay, and profile negotiation |
| [TypeScript Transport](docs/typescript-transport.md) | TypeScript stream framing, TLS adapter, and QUIC adapter decision notes |

## What's New in v1.0

- **Secure Baseline Profile**: Production-ready `secure-baseline-v1` profile with mandatory DID authentication
- **DID Authentication**: Mutual authentication using Decentralized Identifiers and Ed25519 signatures
- **Replay Protection**: Built-in replay attack prevention with sliding window or nonce-based modes
- **Clear Tier Separation**: Core vs Advanced vs Enterprise feature boundaries explicitly defined

## Telemetry Transport

AXCP provides high-performance telemetry data collection:

### QUIC DATAGRAM Transport

Telemetry data is transmitted using QUIC's unreliable DATAGRAM frames, providing:

- Ultra-low latency (no head-of-line blocking)
- Zero connection setup overhead for frequent metrics
- Minimal impact on application traffic
- Automatic coalescing of telemetry points during network congestion

### Privacy Features

For privacy-preserving metrics collection with Differential Privacy, see **AXCP Advanced** and **AXCP Enterprise** tiers below.

## Contents

- [AXCP Core Specification v1.0](spec/axcp-v1.0.md) (RFC-style)
- Transport schema and Protobuf IDL (`proto/axcp.proto`)
- Go SDK with authentication and negotiation (`sdk/go/`)
- Python SDK Core with signing, verification, replay, and profile negotiation (`sdk/python/`)
- TypeScript SDK Core with signing, verification, replay, and profile negotiation (`sdk/typescript/`)
- Benchmark simulations and performance tests
- License: Apache 2.0 (open source)

## License

**AXCP Core is open source under the Apache License 2.0.**

You are free to use, modify, and distribute AXCP Core for any purpose, including commercial applications, without fees or special permissions.

See [LICENSE](./LICENSE) for the full license text.

**Trademark:** See [TRADEMARK.md](./TRADEMARK.md) for name and logo usage guidelines.

**Repository Boundary:** This repository (`axcp-spec`) contains only AXCP Core. The Advanced and Enterprise features (Differential Privacy, CRDT sync, mTLS management, PII filtering, compliance reporting) are delivered in separate private repositories and are not included here.

## AXCP Advanced & Enterprise

For organizations requiring advanced capabilities, TradePhantom LLC offers commercial tiers:

| Tier | Key Features |
|------|--------------|
| **AXCP Advanced** | mTLS with DID-bound certificates, Hardware attestation (SGX/SEV), Rate limiting middleware, Context-Sync (CRDT), Differential Privacy (optional) |
| **AXCP Enterprise** | Secure telemetry ingestion, Differential Privacy (mandatory), PII filtering & redaction, Compliance reporting (GDPR/HIPAA/SOC2), Ed25519 audit trails, Prometheus & OpenTelemetry metrics |

For licensing inquiries: [enterprise@getaxcp.com](mailto:enterprise@getaxcp.com)

See [docs/licensing/](./docs/licensing/) for detailed licensing information.

## 🔐 Branch Protection

The `main` branch is protected by a ruleset that enforces CI testing, disallows direct pushes or deletions, and requires pull requests for all merges.  
Protection enforcement is pending until repository visibility is changed to public or upgraded to GitHub Team.

> Developed by [TradePhantom LLC](https://tradephantom.com) (New Mexico, US)  
> AI-native infrastructure for autonomous agents.
