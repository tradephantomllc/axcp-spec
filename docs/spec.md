# AXCP Core – Technical Specification (Draft v0.3)

> NOTE: This is a living document that will evolve until AXCP reaches v1.0. Until then, breaking changes are expected between minor versions.

## Problem Statement

Modern AI agents need a low-latency, privacy-preserving coordination layer that works across heterogeneous networks (LAN/WAN/edge) without relying on HTTP polling or heavyweight RPC stacks. AXCP provides a transport-agnostic, delta-based messaging protocol designed for conversational context exchange and telemetry streaming between autonomous agents and gateways.

## Envelope schema

```protobuf
message AxcpEnvelope {
  uint32  version   = 1; // protocol version
  string  trace_id  = 2; // distributed trace correlation
  bytes   payload   = 3; // compressed payload (protobuf/json/etc.)
  uint64  ts_nanos  = 4; // sender timestamp, ns precision
  uint32  profile   = 5; // security profile (0-2)
  map<string,string> tags = 6; // free-form metadata
}
```

## Security Profiles

| Profile | Channel security | Authentication | Differential privacy |
|---------|------------------|-----------------|----------------------|
| **0**   | None             | None            | Off                 |
| **1**   | QUIC + TLS 1.3   | mTLS optional   | Laplace noise        |
| **2**   | QUIC + TLS 1.3   | mTLS required   | Gaussian noise       |

Profile 0 is intended for local testing and air-gapped environments. Profiles 1 and 2 introduce increasing privacy guarantees via differential-privacy noise injection and stronger identity verification.

## Message Framing

AXCP can be transported over any 0-RTT capable substrate. The reference stack uses QUIC DATAGRAM frames to avoid HOL ‑ blocking. When DATAGRAM is unavailable, AXCP falls back to unidirectional QUIC streams.

## State Synchronisation

AXCP adopts a CRDT-like delta model where only mutations are exchanged. Each envelope may bundle multiple mutations to amortise overhead under high-frequency workloads.

## Backpressure & Flow Control

Gateways MAY send `AxcpControl` messages to throttle agents that exceed the negotiated QPS or privacy budget. Agents SHOULD respect `Retry-After` hints to avoid disconnect penalties.

## Future Work (v0.4 – v0.5)

* Capability discovery API for negotiating optional extensions.
* Signed bundle exchange to verify off-path gateway execution.
* Remote OpenTelemetry streaming directly from edge nodes.
