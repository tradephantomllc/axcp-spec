# axcp-spec

> **Dual Licence Notice** – this repo ships  
> • **AXCP Core** (BUSL-1.1 → Apache 2.0 in 2029) – source-available  
> • **AXCP Enterprise** (Commercial) – under `enterprise/`  
> See `ENTERPRISE_NOTICE.md` for details.

[![CI](https://github.com/tradephantom/axcp-spec/actions/workflows/ci.yml/badge.svg)](https://github.com/tradephantom/axcp-spec/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tradephantom/axcp-spec/sdk/go.svg)](https://pkg.go.dev/github.com/tradephantom/axcp-spec/sdk/go)
[![License: BUSL-1.1](https://img.shields.io/badge/License-BUSL--1.1-blue.svg)](LICENSE)
[![Version](https://img.shields.io/github/v/tag/tradephantom/axcp-spec?label=version&color=green)](https://github.com/tradephantom/axcp-spec/releases)
[![Issues](https://img.shields.io/github/issues/tradephantom/axcp-spec)](https://github.com/tradephantom/axcp-spec/issues)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

> **Quick Start**: see [`examples/go/simple_chat`](examples/go/simple_chat)

 2025 TradePhantom LLC – BUSL-1.1 / Apache-2.0 fallback

AXCP – Adaptive eXchange Context Protocol. An open specification for ultra-efficient, privacy-preserving AI agent orchestration.
It improves on existing agent communication protocols (like MCP, A2A, and ACP) by integrating:

- QUIC + Protobuf transport for high-performance, low-latency communication  
- Delta-synced context cache to reduce token overhead  
- DIDComm v2 for decentralized capability negotiation and secure identity  
- SGX enclaves and differential privacy for confidential and auditable execution  
- Smart routing between cloud and edge nodes  
- Telemetry datagrams for real-time monitoring with built-in differential privacy

## 🤖 Tri-AI Demo: Multi-Agent Orchestration

AXCP enables seamless coordination between heterogeneous AI agents, as demonstrated in our Tri-AI integration:

![Tri-AI Preview](docs/img/tri-ai-preview.gif)

**Key Features Demonstrated:**
- **Multi-Model Coordination**: Claude, GPT-4, and Gemini working together on complex tasks
- **Real-Time Context Sync**: Delta-based updates ensure all agents stay synchronized
- **Privacy-Preserving Telemetry**: Performance metrics collected with differential privacy
- **Cross-Platform Integration**: Agents running on different platforms and languages

> [📖 Learn More](docs/quickstart.md) | [🏗️ Architecture](docs/architecture.md) | [⚡ Get Started](examples/go/simple_chat)

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
- License: BUSL-1.1 (converts to Apache 2.0 on 2029-01-01)

## 🔐 Branch Protection

The `main` branch is protected by a ruleset that enforces CI testing, disallows direct pushes or deletions, and requires pull requests for all merges.  
Protection enforcement is pending until repository visibility is changed to public or upgraded to GitHub Team.

> Developed by [TradePhantom LLC](https://tradephantom.com) (New Mexico, US)  
> AI-native infrastructure for autonomous agents.
