# axcp-spec

[![CI](https://github.com/tradephantom/axcp-spec/actions/workflows/ci.yml/badge.svg)](https://github.com/tradephantom/axcp-spec/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tradephantom/axcp-spec/sdk/go.svg)](https://pkg.go.dev/github.com/tradephantom/axcp-spec/sdk/go)
![License](https://img.shields.io/badge/license-Apache_2.0-green)

### 🔥 Tri-AI Orchestration Demo  
<a href="https://youtu.be/j9OzbG62BNY" target="_blank">
  <img src="docs/img/tri-ai-preview.gif" alt="Tri-AI demo" width="640">
</a>

*Click to watch the full demo (YouTube, 3 min).*  

> **Quick Start**: see [`docs/quickstart.md`](docs/quickstart.md)

AXCP – Adaptive eXchange Context Protocol. An open specification for ultra-efficient, privacy-preserving AI agent orchestration.
It improves on existing agent communication protocols (like MCP, A2A, and ACP) by integrating:

- QUIC + Protobuf transport for high-performance, low-latency communication  
- Delta-synced context cache to reduce token overhead  
- DIDComm v2 for decentralized capability negotiation and secure identity  
- SGX enclaves and differential privacy for confidential and auditable execution  
- Smart routing between cloud and edge nodes  
- Telemetry datagrams for real-time monitoring with built-in differential privacy

## What's New in v0.3

- **Telemetry Datagrams**: Low-latency telemetry data collection with QUIC DATAGRAM extension
- **Differential Privacy**: Built-in support for privacy-preserving metrics collection with configurable privacy budgets
- **Edge Gateway**: Enhanced gateway with telemetry support for edge computing scenarios
- **Improved Testing**: Comprehensive test suite for differential privacy and telemetry features

## Telemetry and Differential Privacy

AXCP v0.3 introduces a novel approach to telemetry data collection that prioritizes both performance and privacy:

### QUIC DATAGRAM Transport

Telemetry data is transmitted using QUIC's unreliable DATAGRAM frames, providing:

- Ultra-low latency (no head-of-line blocking)
- Zero connection setup overhead for frequent metrics
- Minimal impact on application traffic
- Automatic coalescing of telemetry points during network congestion

### Privacy-Preserving Metrics

Built-in differential privacy mechanisms protect sensitive telemetry data:

- **Profile-Based Privacy**: Privacy guarantees increase with profile level
  - Profile 1-2: Basic telemetry with minimal noise
  - Profile 3+: Strong differential privacy guarantees
  
- **Configurable Noise Mechanisms**:
  - Laplace noise for discrete metrics (counters, percentages)
  - Gaussian noise for continuous metrics (timing, memory usage)
  
- **Adaptive Privacy Budget**: Each gateway maintains a privacy budget that adapts based on:
  - Query sensitivity
  - Data volume
  - Time-based budget replenishment

### Implementation Status

The current implementation provides a solid foundation while maintaining simplicity:

- Basic UDP benchmarks for initial round-trip validation
- Progressive enhancement toward full QUIC+SSL implementation
- Privacy mechanisms with configurable parameters

## Contents

- AXCP v0.3-edge-beta specification (RFC-style)
- Transport schema and Protobuf IDL  
- No-code PoC orchestration workflows (Make, n8n)  
- Benchmark simulations and performance tests  
- License: Apache 2.0 (open source)

## License

**AXCP Core is open source under the Apache License 2.0.**

You are free to use, modify, and distribute AXCP Core for any purpose, including commercial applications, without fees or special permissions.

See [LICENSE](./LICENSE) for the full license text.

**Trademark:** See [TRADEMARK.md](./TRADEMARK.md) for name and logo usage guidelines.

## AXCP Enterprise

For organizations requiring advanced capabilities, TradePhantom LLC offers commercial tiers:

| Tier | Key Features |
|------|--------------|
| **AXCP Advanced** | Context-Sync (CRDT), mTLS certificates, SGX/SEV enclaves, Differential Privacy (optional) |
| **AXCP Enterprise** | Differential Privacy (mandatory), PII filtering, Compliance reporting (GDPR/HIPAA/SOC2), Audit trails, TRI-AI Orchestration, Dedicated support |

For licensing inquiries: [licensing@tradephantom.com](mailto:licensing@tradephantom.com)

See [docs/licensing/](./docs/licensing/) for detailed licensing information.

## 🔐 Branch Protection

The `main` branch is protected by a ruleset that enforces CI testing, disallows direct pushes or deletions, and requires pull requests for all merges.  
Protection enforcement is pending until repository visibility is changed to public or upgraded to GitHub Team.

> Developed by [TradePhantom LLC](https://tradephantom.com) (New Mexico, US)  
> AI-native infrastructure for autonomous agents.
