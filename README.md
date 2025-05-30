# axcp-spec

[![CI](https://github.com/tradephantom/axcp-spec/actions/workflows/ci.yml/badge.svg)](https://github.com/tradephantom/axcp-spec/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tradephantom/axcp-spec/sdk/go.svg)](https://pkg.go.dev/github.com/tradephantom/axcp-spec/sdk/go)

> **Quick Start**: see [`examples/go/simple_chat`](examples/go/simple_chat)

© 2025 TradePhantom LLC – BSL 1.1 / Apache-2.0 fallback

AXCP – Adaptive eXchange Context Protocol. An open specification for ultra-efficient, privacy-preserving AI agent orchestration.
It improves on existing agent communication protocols (like MCP, A2A, and ACP) by integrating:

- QUIC + Protobuf transport for high-performance, low-latency communication  
- Delta-synced context cache to reduce token overhead  
- DIDComm v2 for decentralized capability negotiation and secure identity  
- SGX enclaves and differential privacy for confidential and auditable execution  
- Smart routing between cloud and edge nodes  

## Contents

- AXCP v0.2-alpha specification (RFC-style)  
- Transport schema and Protobuf IDL  
- No-code PoC orchestration workflows (Make, n8n)  
- Benchmark simulations and performance tests  
- License: BSL 1.1 (converts to Apache 2.0 on 2029-01-01)

## 🔐 Branch Protection

The `main` branch is protected by a ruleset that enforces CI testing, disallows direct pushes or deletions, and requires pull requests for all merges.  
Protection enforcement is pending until repository visibility is changed to public or upgraded to GitHub Team.

> Developed by [TradePhantom LLC](https://tradephantom.com) (New Mexico, US)  
> AI-native infrastructure for autonomous agents.
