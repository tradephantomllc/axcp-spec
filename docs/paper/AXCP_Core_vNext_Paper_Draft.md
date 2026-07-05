# AXCP Core vNext Paper Draft

**Working title:** AXCP Core: Message-Level Identity, Authentication, and Replay Protection for Autonomous Agent Communication
**Draft status:** Source draft for editorial review and LaTeX conversion
**Target venue/artifact:** Updated Zenodo preprint replacing or versioning the February 2026 AXCP v1.0 paper
**Prepared from repository state:** `tradephantomllc/axcp-spec` `main`, after PRs #254 and #255
**Date:** 2026-07-05
**Scope:** AXCP Core only, Apache 2.0 open-source repository

---

## Editorial Notes for Superbrain

This document is not the final paper. It is the technical source draft to be refined into the final manuscript and LaTeX.

The current Zenodo paper, published on 2026-02-04 as Version 1.0, describes AXCP as a secure communication protocol for autonomous AI agents and lists Go/Rust as the public implementation languages. The repository has since materially changed. The paper needs to reflect the current AXCP Core boundary and avoid claims that are no longer precise or that belong to commercial tiers.

Important alignment decisions:

1. AXCP Core is the public, Apache 2.0 protocol surface.
2. Advanced and Enterprise are commercial/private tiers and should be mentioned only at a high level.
3. Python and TypeScript SDKs are now part of the Core story and should no longer be described as "coming soon".
4. Rust should be described carefully as an experimental telemetry adapter, not as an equal canonical Core transport SDK.
5. The paper should not claim Differential Privacy is mandatory in Core or Enterprise. DP belongs to commercial controls and is configurable where implemented.
6. The paper should avoid unpublished Enterprise details such as CBE internals, SAM issuer mechanics, ARCANA internals, licensing enforcement design, or production gateway deployment details.
7. If performance numbers are included, they must be regenerated from the current repository and included as a dated benchmark table. Do not reuse old metrics without rerunning them.
8. The paper should make the distinction clear: TLS secures a channel; AXCP Secure Baseline binds sender identity, recipient identity, canonical payload bytes, timestamp, and replay sequence at the message layer.

Recommended paper version label: **v1.1** or **2026 revision**. The protocol spec remains **AXCP Core Specification v1.0 (Secure Baseline)** unless we intentionally publish a protocol-spec update.

---

## Version Synchronization Box

Use this as the source of truth for the final manuscript metadata:

| Field | Value |
|---|---|
| Paper version | AXCP Core Paper v1.1 / 2026 revision |
| Protocol specification | AXCP Core Specification v1.0, Secure Baseline |
| Repository | `tradephantomllc/axcp-spec` |
| Repository state | `main` after transcript/spec/authentication alignment |
| Publication target | Zenodo updated version under the existing concept DOI |
| Core boundary | Apache 2.0 public repository; no Advanced/Enterprise code |
| SDKs covered | Go, Python, TypeScript |
| Rust status | Experimental telemetry adapter |
| Performance claims | Omitted unless regenerated with current code, hardware, and commands |

Do not use "vNext" in the final paper title. It is only the working filename for this draft.

---

## Abstract

Autonomous AI agents increasingly coordinate across tools, services, gateways, and organizational boundaries. Existing agent integration protocols focus primarily on tool access, context exchange, or interoperability, while often delegating trust to the transport channel or to external identity infrastructure. AXCP, the Adaptive eXchange Context Protocol, addresses a narrower but critical problem: secure, verifiable communication between autonomous agents.

AXCP Core defines a Secure Baseline profile for message-level authentication, replay protection, and deterministic interoperability. Each AXCP envelope carries a Decentralized Identifier (DID), an Ed25519 detached signature, a timestamp, and replay material such as a monotonic sequence number. The signed authentication transcript binds the sender, recipient, canonical payload bytes, timestamp, and sequence-sensitive replay semantics so that messages remain attributable and tamper-evident beyond the lifetime of a transport connection.

This paper presents the current AXCP Core architecture, the Secure Baseline profile, the canonical signing model, replay protection requirements, cross-SDK interoperability guarantees, and the open-source implementation boundary. The current public repository provides Go, Python, and TypeScript Core SDKs with a shared Secure Baseline test vector that locks deterministic envelope encoding, DID authentication transcript bytes, Ed25519 signature bytes, and full encoded envelope bytes. Commercial features such as mTLS management, attestation, PII controls, compliance reporting, and advanced execution governance are outside the Core repository and are intentionally not part of this paper's implementation claims.

AXCP is best understood as a security layer for agentic communication rather than a replacement for tool protocols. MCP defines how models and applications expose tools and context; A2A-style systems address agent interoperability and discovery; AXCP Core focuses on ensuring that agent messages are authenticated, replay-resistant, and verifiable across implementation languages.

---

## 1. Introduction

Agentic systems are moving from single-process tool calls toward distributed workflows. A single task may involve a planning agent, a retrieval agent, a domain specialist, a gateway, a memory service, a telemetry collector, and one or more external tools. In this environment, communication itself becomes a security boundary.

Traditional web security often assumes that TLS is sufficient for transport confidentiality and integrity. TLS is necessary, but it does not by itself answer several questions that matter in agentic networks:

- Which agent authored this message?
- Was this message altered after it was created?
- Is this message being replayed from an earlier session?
- Can the message be verified after it leaves the original transport channel?
- Can independent implementations produce the same signed bytes?

These questions become more important when agent messages may trigger downstream tool calls, memory updates, delegated tasks, compliance logs, or cross-organization workflows. A secure channel alone protects traffic in transit; it does not create a durable, application-layer proof that a specific agent signed a specific envelope for a specific recipient at a specific time.

AXCP Core addresses this gap with a Secure Baseline profile built around:

- DID-based agent identity.
- Ed25519 detached signatures.
- Canonical authentication transcripts.
- Sequence and nonce replay protection.
- Security-first profile negotiation.
- Deterministic cross-SDK compatibility tests.

The design goal is deliberately narrow: AXCP Core is not a full agent framework, not an LLM runtime, not a tool marketplace, and not a commercial governance plane. It is the open protocol substrate for authenticated, replay-resistant message exchange between agents and gateways.

---

## 2. Positioning and Scope

### 2.1 What AXCP Core Is

AXCP Core is an open-source protocol and SDK set for secure agent communication. Its public repository contains:

- The AXCP Core Specification v1.0.
- The Protobuf schema used by Core envelopes.
- Go SDK components for authentication, profile negotiation, replay protection, transport helpers, and examples.
- Python SDK components for identity, signing, verification, replay protection, profile negotiation, QUIC stream framing, and telemetry datagram helpers.
- TypeScript SDK components for identity, signing, verification, replay protection, profile negotiation, stream framing, and Node TLS transport.
- A Rust telemetry adapter used for Core experiments, not a canonical full transport SDK.
- Cross-SDK Secure Baseline vectors and release-readiness checks.

### 2.2 What AXCP Core Is Not

AXCP Core intentionally excludes commercial and deployment-specific features. The following are out of scope for the Core repository and should not be presented as public Core features:

- Commercial license enforcement.
- Signed Agent Manifest governance.
- Capability-Bound Execution.
- ARCANA quantitative autonomy-risk calculus.
- mTLS certificate lifecycle management.
- Hardware attestation control planes.
- PII filtering and redaction pipelines.
- Compliance reporting workflows.
- Differential Privacy policy enforcement.
- Enterprise audit sink and control-center implementations.

These features may exist in commercial/private AXCP tiers, but this paper should treat them as separate product layers. The Core paper should not leak implementation details from those tiers.

AXCP Core authenticates messages. It does not decide whether the business action requested by an authenticated message is authorized. Authorization, approval workflow, policy enforcement, and execution governance belong to higher layers.

### 2.3 Relationship to MCP, A2A, and Tool Protocols

AXCP is complementary to existing agent protocols. MCP-like systems focus on connecting models or agents to tools and external context. A2A-style systems focus on agent interoperability, discovery, and task exchange. AXCP Core focuses on message security: identity, signature verification, replay resistance, and deterministic cross-language verification.

The simplest positioning statement is:

> MCP defines what an agent can call; AXCP verifies who sent the message and whether that message is authentic and fresh.

This framing avoids claiming that AXCP replaces the existing ecosystem. It instead positions AXCP as the security layer that agentic communication will increasingly need as workflows become more autonomous and operationally consequential.

---

## 3. Threat Model

AXCP Core targets the following threat classes:

### 3.1 Message Tampering

An attacker modifies an envelope in transit, in storage, or through a compromised intermediary. AXCP addresses this by signing a canonical authentication transcript over the payload and message bindings. Verification fails if the signed payload, sender, recipient, timestamp, or replay-sensitive fields are altered.

### 3.2 Replay Attacks

An attacker captures a valid message and resends it later to trigger duplicated side effects. AXCP addresses this with sequence or nonce replay protection. Sequence numbers are tracked per peer with a sliding window and TTL. Nonce-based replay protection can be used where monotonic sequencing is not appropriate.

### 3.3 Identity Confusion

An attacker attempts to impersonate an agent by sending a syntactically valid envelope with another sender DID. AXCP requires the verifier to resolve the sender DID, extract an acceptable Ed25519 public key, and verify the detached signature against that key.

### 3.4 Downgrade Attacks

An attacker attempts to force communication into a weaker profile. AXCP Core requires security-first profile negotiation and forbids silent downgrade from `secure-baseline-v1` to deprecated `transport-only-v0`.

### 3.5 Cross-Implementation Drift

Different SDKs may encode the same semantic envelope differently, causing one implementation to sign bytes that another cannot verify. AXCP addresses this through shared deterministic Secure Baseline vectors across Go, Python, and TypeScript.

### 3.6 Explicit Non-Goals

AXCP Core does not claim to solve:

- Prompt injection.
- Sandboxing or code execution governance.
- Agent authorization policy.
- Human approval workflows.
- Key custody or enterprise PKI lifecycle.
- Runtime containment.
- Model behavior alignment.

Those problems require additional layers. AXCP Core provides the message-authentication substrate those layers can build on.

---

## 4. Protocol Overview

### 4.1 Envelope Structure

AXCP messages are represented as Protobuf envelopes. The Core envelope includes:

- Protocol version.
- Trace identifier.
- Security profile.
- One primary payload.
- Sender DID.
- Recipient DID.
- Timestamp.
- Sequence number.
- Detached signature.
- Optional attestation proof field reserved for higher tiers.

The current payload variants include context patches, capability messages, routing policy messages, profile negotiation messages, retry envelopes, telemetry datagrams, and structured errors. Some fields exist in the schema for compatibility or future tier integration, but their policy semantics may be outside Core.

### 4.2 Security Profiles

AXCP Core defines two profiles:

| Profile | Status | Authentication | Replay Protection | Production Use |
|---|---|---:|---:|---:|
| `secure-baseline-v1` | Stable | Required | Required | Yes |
| `transport-only-v0` | Deprecated | Not required | Not required | No |

The Secure Baseline profile is the production default. `transport-only-v0` exists only for compatibility and controlled testing. Implementations must reject deprecated downgrade unless explicitly configured for non-production use.

### 4.3 Profile Negotiation

Profile negotiation is security-first:

1. Validate protocol version.
2. Validate client-supported profiles.
3. Select the strongest mutually supported secure profile.
4. Reject unsupported or unknown profiles.
5. Reject deprecated fallback unless explicitly allowed.
6. Resolve the signature algorithm, preferring Ed25519.

This prevents a peer from silently falling back to an unsigned transport-only mode.

---

## 5. Identity and Authentication

### 5.1 DID-Based Identity

AXCP Core uses Decentralized Identifiers as the identity handle for agents and gateways. A DID must follow the standard structure:

```text
did:<method>:<id>
```

The current SDKs support `did:key` workflows for local identity generation and verification. Production systems may inject their own DID resolver implementation.

AXCP Core deliberately defines a resolver abstraction rather than a single global registry. This allows deployments to use DID:Web, private registries, internal trust stores, or other DID infrastructure without changing the envelope format.

### 5.2 Ed25519 Signatures

AXCP Core uses Ed25519 signatures for Secure Baseline authentication. A verifier must:

1. Validate the sender DID syntax.
2. Resolve the DID document.
3. Confirm that the DID document ID matches the requested DID.
4. Extract an acceptable Ed25519 verification key.
5. Verify the detached signature over the AXCP authentication transcript.

Allowed key types include:

- `Ed25519VerificationKey2020`
- `Ed25519VerificationKey2018`

### 5.3 Authentication Transcript

AXCP signs a canonical transcript with the prefix:

```text
AXCP-DID-AUTH-v1
```

The transcript binds:

- Sender DID.
- Recipient DID.
- Base64 canonical payload bytes.
- Timestamp in RFC3339 seconds.

The signed payload excludes detached authentication fields that must not recursively sign themselves:

- `sender_did`
- `recipient_did`
- `timestamp_ms`
- `signature`
- `attestation_proof`

The replay `sequence` remains covered by the signed payload because replay protection consumes it. Sequence tampering must invalidate the detached signature before replay state is touched.

This point is important for the updated paper. Earlier implementation drift excluded `sequence` in some SDKs. The current repository aligns Go, Python, and TypeScript so that replay-sensitive sequence changes modify the signed payload and fail verification.

---

## 6. Replay Protection

AXCP Core provides replay protection through sequence numbers or nonces.

### 6.1 Sequence Mode

In sequence mode, each incoming message includes a sequence number. The receiver maintains replay state per peer:

1. Expire replay entries older than the TTL.
2. Reject any sequence already seen within the TTL.
3. Reject sequences outside the configured sliding window.
4. Accept new sequences and update the highest-seen sequence when appropriate.

Sequence mode is appropriate for ordered or mostly ordered communication channels where each peer can maintain monotonic counters.

### 6.2 Nonce Mode

Nonce mode is appropriate where strict sequence ordering is not feasible. Each message uses a unique nonce. The receiver stores seen nonces for the TTL and rejects duplicates.

### 6.3 Verification Order

Implementations should verify the detached signature before mutating replay state. This prevents unauthenticated messages from consuming replay slots or polluting the replay cache.

---

## 7. Transport and Encoding

### 7.1 Transport Assumptions

AXCP is designed to run over efficient transport layers such as QUIC/TLS. However, the security model does not depend solely on the transport channel. Secure Baseline adds application-layer authenticity and replay protection above the transport.

The M7 proof path uses the authenticated-chat example ALPN `axcp-auth-chat`. The final paper should describe that value as a proof/example ALPN unless the Core specification explicitly standardizes a universal ALPN registry entry.

### 7.2 Protobuf Encoding

AXCP uses Protobuf envelopes for compact, typed message exchange. The current Core schema is shared across Go, Python, and TypeScript.

The updated paper should avoid unverified claims such as "3x smaller than JSON" unless a current benchmark is regenerated and included with methodology. It is safe to state that Protobuf provides typed binary encoding and can reduce overhead compared with verbose text encodings depending on payload shape.

### 7.3 QUIC DATAGRAM Telemetry

AXCP Core includes telemetry datagram support for low-latency metrics-sized payloads. The current public repository includes QUIC DATAGRAM support paths and tests in the Go netquic surface, plus Python datagram helpers.

This should be framed as telemetry transport support, not as a privacy or compliance feature. Differential Privacy and privacy budget policy belong to commercial tiers and should not be claimed as Core features.

### 7.4 TypeScript Transport Boundary

The TypeScript SDK provides transport-independent framing and Node TLS stream transport. Native QUIC is not bundled because the supported Node runtime path does not expose a stable `node:quic` module. The paper should state this boundary clearly to avoid overclaiming TypeScript QUIC support.

---

## 8. Cross-SDK Interoperability

AXCP Core now has an explicit interoperability contract across Go, Python, and TypeScript.

The shared Secure Baseline vector at:

```text
testdata/sdk/secure_baseline_vector.json
```

locks:

- Deterministic Ed25519 seed and `did:key`.
- Context patch payload.
- Canonical signing payload bytes.
- Replay `sequence` inclusion in signed bytes.
- DID authentication transcript bytes.
- Detached Ed25519 signature bytes.
- Full encoded envelope bytes.

Any change to canonical serialization or transcript construction is treated as a compatibility-sensitive protocol change.

### 8.1 Go SDK

The Go SDK is the reference Core implementation. It contains authentication, profile negotiation, replay protection, canonical envelope encoding, gateway-related tests, and transport helpers.

### 8.2 Python SDK

The Python SDK implements the Core security surface:

- Ed25519 identity generation.
- `did:key` derivation and parsing.
- Deterministic Protobuf envelope encoding.
- DID authentication transcript signing and verification.
- Timestamp validation.
- Sequence replay protection.
- Secure Baseline profile negotiation.
- Async QUIC stream transport with AXCP envelope framing.
- QUIC DATAGRAM helpers for telemetry-sized payloads.

### 8.3 TypeScript SDK

The TypeScript SDK implements the Core security surface:

- Ed25519 identity generation.
- `did:key` derivation.
- DID parsing and resolver interfaces.
- Deterministic AXCP Protobuf envelope encoding.
- DID authentication transcript signing and verification.
- Timestamp validation.
- Sequence and nonce replay protection.
- Secure Baseline profile negotiation.
- Stream framing and Node TLS transport.

### 8.4 Rust Boundary

Rust is currently present as an experimental telemetry adapter for AXCP Core experiments. It should not be described as a production-equivalent canonical transport SDK unless that scope is implemented and tested later.

---

## 9. Security Properties

AXCP Core provides the following properties when `secure-baseline-v1` is correctly implemented and configured:

### 9.1 Message Authenticity

A valid AXCP message is signed by the private key corresponding to the sender DID. Verification requires resolving the DID document and validating an Ed25519 signature.

### 9.2 Message Integrity

Changes to the canonical signed payload invalidate the signature. This includes changes to payload content and replay sequence.

### 9.3 Recipient Binding

The recipient DID is included in the authentication transcript. A signature created for one intended recipient cannot be transparently reused as a valid message for another recipient without detection.

### 9.4 Replay Resistance

Sequence or nonce state prevents re-accepting the same signed message within the configured replay window.

### 9.5 Auditability

Because signatures are at the message layer, an AXCP message can remain verifiable outside the original TLS session. This enables later inspection of message provenance. Core provides the cryptographic substrate; durable audit storage and compliance workflows belong to higher tiers.

### 9.6 Downgrade Resistance

Secure Baseline negotiation prevents silent fallback into unsigned transport-only mode.

---

## 10. Implementation and Verification

### 10.1 Repository Structure

The current public repository contains:

```text
spec/axcp-v1.0.md
proto/axcp.proto
sdk/go/
sdk/python/
sdk/typescript/
sdk/rust/
edge/gateway/
edge/rpi-agent/
testdata/sdk/secure_baseline_vector.json
docs/
```

### 10.2 Continuous Integration

The current CI checks include:

- Go tests, including race-enabled paths.
- Python tests and SDK release-readiness checks.
- TypeScript typecheck, tests, and package dry-run.
- Rust stable/beta/nightly checks.
- Rust formatting, clippy, docs, and unused dependency checks.
- Gateway telemetry checks.
- RPi agent checks.
- Example build checks.
- CLA/license status.

The final paper may state that CI covers these categories, but should avoid claiming a fixed total test count unless generated automatically during final release.

### 10.3 Local Release Gates

The release-readiness document defines local gates:

```bash
python scripts/verify_sdk_release_readiness.py
PYTHONPATH=$PWD python -m pytest -q scripts gateway sdk/python/tests
(cd sdk/typescript && npm ci && npm test && npm run typecheck && npm pack --dry-run)
(cd sdk/go && go test ./...)
(cd edge/gateway && go test ./...)
(cd edge/rpi-agent && go test ./...)
(cd sdk/rust && cargo fmt --all --check && cargo test --quiet)
git diff --check
```

These commands are useful as the final paper's reproducibility appendix.

---

## 11. Comparison With Existing Approaches

This section needs final citations from primary sources before publication. Use only official MCP, A2A, protocol documentation, or academic/security references.

### 11.1 Tool Protocols

Tool protocols standardize how models or agents call tools and exchange context. They are valuable for developer adoption and ecosystem interoperability. Their primary concern is not necessarily message-level attribution, verifiability, or replay protection between autonomous agents.

AXCP Core can wrap or complement such systems by adding message-level DID authentication and replay protection.

### 11.2 Transport Security Alone

TLS protects a connection. AXCP Secure Baseline protects the message. The difference matters when messages are stored, forwarded, audited, replayed, or routed through intermediaries.

### 11.3 OAuth and Centralized Identity

OAuth-based systems are familiar and operationally mature, but they generally depend on an identity provider and token lifecycle. AXCP's DID model allows agent identity to be represented directly by cryptographic keys and resolver-backed DID documents. This does not eliminate the need for governance; it separates the Core identity primitive from any single centralized provider.

### 11.4 gRPC/HTTP-Based Agent Communication

gRPC and HTTP-based protocols provide mature transport and tooling. AXCP's contribution is not merely a transport choice, but the application-layer security envelope: signed messages, deterministic transcript construction, and replay protection.

---

## 12. Limitations

AXCP Core has important limitations:

1. Core does not decide whether an agent is authorized to perform a business action.
2. Core does not prove that an agent's internal reasoning is safe.
3. Core does not sandbox code execution.
4. Core does not provide enterprise key custody, policy management, or compliance reporting.
5. Core does not solve prompt injection.
6. Core does not publish a universal DID registry.
7. Core interoperability depends on strict canonicalization; implementers must use SDK-provided helpers rather than hand-built transcripts.

These limitations are intentional. AXCP Core should remain small, auditable, and interoperable.

---

## 13. Future Work

Future work should be separated into public Core evolution and private/commercial product evolution.

### 13.1 Public Core Roadmap

Possible public Core roadmap items:

- Additional formal interoperability vectors.
- More conformance tests.
- Expanded examples for authenticated agent-to-agent exchange.
- Public benchmark methodology regenerated from current SDKs.
- Additional DID method examples.
- Improved transport adapters where runtime support is stable.

### 13.2 Commercial Tier Roadmap

Commercial tier capabilities should remain high-level in this paper:

- mTLS management.
- Hardware attestation.
- PII controls.
- Configurable privacy controls.
- Compliance reporting.
- Signed agent identity governance.
- Execution governance.
- Enterprise audit and control-plane features.

Do not include implementation mechanics in the public paper.

### 13.3 Research Roadmap

ARCANA and related autonomy-risk research can be mentioned only as future research if needed, without formulas or architecture details in this Core paper. The safer approach is to keep ARCANA out of this paper entirely and publish it as a separate research artifact once ready.

---

## 14. Conclusion

AXCP Core provides a focused security layer for autonomous agent communication. Its Secure Baseline profile binds agent identity, recipient identity, canonical payload bytes, timestamp, and replay-sensitive sequence material into a verifiable Ed25519 authentication transcript.

The updated repository state strengthens the original AXCP paper in three ways. First, AXCP Core now has a clearer public boundary: the Apache 2.0 repository contains the Core protocol and SDKs, while Advanced and Enterprise capabilities remain separate commercial tiers. Second, the SDK surface is broader and more practical: Go, Python, and TypeScript now share the Secure Baseline security model. Third, cross-SDK compatibility is tested with a deterministic vector that locks the exact bytes used for signing, transcript construction, signature verification, and envelope encoding.

AXCP should not be presented as a replacement for the agent ecosystem. It is better positioned as the message-security layer that can make agent-to-agent and agent-to-gateway communication verifiable, replay-resistant, and auditable as autonomy increases.

---

## Suggested LaTeX Structure

```text
\title{AXCP Core: Message-Level Identity, Authentication, and Replay Protection for Autonomous Agent Communication}
\author{Julio Elizondo Rodriguez \\ TradePhantom LLC}
\date{2026}

\begin{abstract}
...
\end{abstract}

\section{Introduction}
\section{Positioning and Scope}
\section{Threat Model}
\section{Protocol Overview}
\section{Identity and Authentication}
\section{Replay Protection}
\section{Transport and Encoding}
\section{Cross-SDK Interoperability}
\section{Security Properties}
\section{Implementation and Verification}
\section{Comparison With Existing Approaches}
\section{Limitations}
\section{Future Work}
\section{Conclusion}
\appendix
\section{Secure Baseline Test Vector}
\section{Release Verification Commands}
```

---

## Claims Requiring Final Verification Before Publication

Do not publish the final paper until these are checked:

1. Current CI badge is green on `main`.
2. Current repository has no open required-check PRs.
3. The shared Secure Baseline vector passes in Go, Python, and TypeScript.
4. Any performance table is regenerated from current code.
5. Any test count is generated from current CI or scripts, not copied from the February paper.
6. Zenodo metadata lists current implementation languages: Go, Python, TypeScript, and Rust with Rust properly qualified.
7. Commercial-tier feature wording matches the website and private product boundary.
8. No private Enterprise implementation details are included.
9. The DOI versioning strategy is chosen: new Zenodo version under the same concept DOI or separate record.
10. The final LaTeX bibliography uses primary sources only for MCP, A2A, DID, Ed25519, QUIC/TLS, and Protobuf claims.

---

## Source Anchors

Use these repository files as anchors during final editing:

- `spec/axcp-v1.0.md`
- `proto/axcp.proto`
- `docs/authentication.md`
- `docs/gateway-setup.md`
- `docs/sdk-release-readiness.md`
- `sdk/go/`
- `sdk/python/README.md`
- `sdk/typescript/README.md`
- `sdk/rust/README.md`
- `testdata/sdk/secure_baseline_vector.json`
- `README.md`

Use the existing Zenodo v1.0 record as prior-publication context:

- `https://zenodo.org/records/18475648`
