# AXCP Core – Technical Specification (Draft v0.3)

> NOTE: This document is an *early draft* and is expected to evolve rapidly ahead of the v0.4 release.

## Problem Statement
Existing agent-to-agent protocols focus on message delivery but ignore efficient *context exchange*. AXCP introduces a **delta-based, transport-agnostic** coordination layer that minimises bandwidth, ensures privacy and enables deterministic replay across heterogeneous runtimes (cloud, edge, enclave).

## Design Goals (MVP)
1. **Transport Independence** – QUIC is the reference transport but the spec must map cleanly onto TCP/TLS and WebTransport.
2. **Delta Synchronisation** – Contexts are represented as *CRDT* documents and exchanged as binary deltas.
3. **Privacy & Compliance** – Built-in encryption profiles, differential-privacy telemetry and optional SGX attestation.
4. **Capability Discovery** – Agents advertise capabilities (supported profiles, extensions) via a signed envelope.
5. **Simplicity First** – Keep the core surface small; advanced features shipped as extensions.

## Envelope Schema (protobuf)
```protobuf
syntax = "proto3";
package axcp.v0;

// Every envelope carries exactly one payload and optional delta.
message AxcpEnvelope {
  uint32  version           = 1;  // wire format version (0x0003)
  string  trace_id          = 2;  // distributed trace correlation
  bytes   capability_set    = 3;  // CBOR-encoded capability map
  oneof payload {
    bytes  delta_patch      = 10; // RGA CRDT delta
    bytes  full_context     = 11; // entire context snapshot
  }
  bytes   signature         = 20; // Ed25519 signature of all previous fields
}
```
*See `proto/` directory for the authoritative IDL generated from this draft.*

## Security Profiles
| Profile | Transport  | Encryption | Attestation | Intended use |
|---------|------------|------------|-------------|--------------|
| 0       | QUIC/TLS 1.3 | TLS-AES-128-GCM | none | dev / local mesh |
| 1       | QUIC/TLS 1.3 | TLS-CHACHA20-POLY1305 | SGX attestation optional | PoC deployments |
| 2       | QUIC/TLS 1.3 | TLS-AES-256-GCM | Mandatory SGX attestation | Regulated env (HIPAA/GDPR) |

## Extension Points
* **Telemetry Datagram** – QUIC DATAGRAM frames carrying anonymised metrics.
* **Adaptive Encryption** – Negotiation of cipher-suites based on bandwidth & power budget.
* **Remote OTEL Streaming** – In-band forwarding of OTLP spans over AXCP channels.

---
For a top-level overview of the architecture see `docs/architecture.md`.
