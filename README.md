# AXCP Core

[![CI](https://github.com/tradephantomllc/axcp-spec/actions/workflows/ci.yml/badge.svg)](https://github.com/tradephantomllc/axcp-spec/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tradephantomllc/axcp-spec/sdk/go/axcp.svg)](https://pkg.go.dev/github.com/tradephantomllc/axcp-spec/sdk/go/axcp)
![License](https://img.shields.io/badge/license-Apache_2.0-green)

AXCP (Adaptive eXchange Context Protocol) provides open protocol definitions and building blocks for exchanging signed context, capability messages and telemetry between agents. Applications explicitly configure peers, identities, trust and message handling.

Start with the [local quickstart](docs/quickstart.md), then the [gateway guide](docs/gateway-setup.md). The normative security contract is [AXCP Core Specification v1.0](spec/axcp-v1.0.md); current message definitions are in [proto/axcp.proto](proto/axcp.proto).

## What Core provides

| Component | Implemented scope |
|---|---|
| Go SDK | Protobuf envelopes, DID resolver interface, Ed25519 signing primitives, replay checks, optional profile-handshake helper, QUIC streams and DATAGRAM transport |
| Python SDK | Identity, envelope signing/verification, replay and profile primitives, async QUIC transport |
| TypeScript SDK | Identity, envelope signing/verification, replay and profile primitives, stream framing and Node TLS adapter; no bundled native QUIC adapter |
| Reference gateway | QUIC/TLS listener, configured sender trust, Secure Baseline enforcement, MQTT forwarding and bounded in-memory retry |
| MCP conversion example | Narrow conversion of an MCP-shaped tool descriptor into a capability offer; context patch decoding demonstration |
| Rust adapter | Experimental HTTP telemetry client; separate from the Core QUIC gateway |
| Raspberry Pi example | Local system-statistics collector; it does not transmit to the gateway |

Secure Baseline (`profile=1`) requires application authentication, recipient binding, timestamp freshness and replay checks. The profile is configured on each endpoint, not negotiated on the wire: the reference gateway enables Secure Baseline by default through its `-secure-baseline` flag and requires a server DID, trusted sender keys and TLS configuration. The SDK handshake helpers are optional and are not used by the gateway. Both stream and authenticated telemetry datagrams use the same signature/replay verifier. `profile=0` retains TLS encryption but omits application authentication; it requires explicit development opt-in.

A context patch is a message representation. Core does not automatically synchronize agent memory, discover or route peers, execute tools, manage certificates, or orchestrate agents. The signed transcript is AXCP-specific; this repository does not implement DIDComm v2 or complete MCP/A2A/ACP adapters. DATAGRAM delivery is unreliable and has no automatic coalescing or end-to-end processing receipt. Benchmark results are workload-specific, not a general performance guarantee.

## Documentation

- [Getting started and SDK examples](docs/getting-started.md)
- [Authentication and replay](docs/authentication.md)
- [Gateway configuration, delivery semantics and limits](docs/gateway-setup.md)
- [Architecture and Core boundaries](docs/architecture.md)
- [Go SDK and module migration](sdk/go/README.md)
- [Python SDK](sdk/python/README.md), [TypeScript SDK](sdk/typescript/README.md), [TypeScript transport](docs/typescript-transport.md)
- [SDK release gates](docs/sdk-release-readiness.md)
- [Security policy](SECURITY.md), [contributing](CONTRIBUTING.md)

Superseded draft specifications and dated proof packs are historical material. They do not define the current supported installation or feature set. The specification's [Revision History](spec/axcp-v1.0.md#revision-history) records an incompatible transcript change after the initial 1.0.0 tag.

## Open source and commercial product

The current Core source is licensed under [Apache 2.0](LICENSE), including commercial use subject to its terms. See [NOTICE](NOTICE) and [trademark guidance](TRADEMARK.md).

Commercial automation, CRDT synchronization, privacy controls, managed mTLS/attestation, PII controls and enterprise governance are outside this repository. The commercial product unlocks capabilities according to its license; Advanced and Enterprise are historical repository/feature labels, not two public Core editions. No commercial implementation or runtime license gate is added to Core. Commercial scope and availability must be confirmed separately; this repository makes no certification or compliance guarantee.

[Licensing information](docs/licensing/README.md) · [Commercial inquiries](mailto:enterprise@getaxcp.com)

Changes to `main` go through pull requests and the repository's required checks. Local verification is distinct from a published package or approved release.

Developed by [TradePhantom LLC](https://tradephantom.com).
