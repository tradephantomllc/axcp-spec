# AXCP Restructuring Plan: Open Core Model Transition

**Document Version:** 2.0
**Date:** 2026-01-17
**Author:** Product Management
**Status:** Draft for Review

---

## Executive Summary

This document outlines the strategic restructuring of AXCP from the current BUSL-1.1 licensing model to an **Open Core** model with Apache 2.0 for the base protocol and commercial licensing for advanced/enterprise features.

### Key Strategic Decision

**AXCP is NOT an alternative to MCP, A2A, or ACP** — it's an **interoperability layer** that connects all these ecosystems. This positioning requires the open source version to be **production-ready with full security**, otherwise it cannot serve as a credible bridge between protocols.

### Current Model Problems

1. **BUSL-1.1 Slows Adoption**: Many companies have policies against "quasi-open" licenses
2. **Narrow Enterprise Market**: With Security Levels 0-2 becoming free in 2029, only extreme compliance use cases (Level 3) remain monetizable
3. **Competitive Disadvantage**: MCP (MIT) and A2A have zero friction for adoption

### New Model: 3-Tier Structure

| Tier | License | Security Level | Target Users |
|------|---------|----------------|--------------|
| **AXCP Core** | Apache 2.0 | Profile 0+1 Unified (Production-Ready) | Everyone |
| **AXCP Advanced** | Commercial ($) | Profile 2 (Context-Sync, mTLS, optional DP) | Enterprises |
| **AXCP Enterprise** | Commercial ($$) | Profile 3 (Mandatory DP, PII, Compliance, TRI-AI) | Regulated Industries |

---

## Part 1: Strategic Rationale

### 1.1 Why Merge Profile-0 and Profile-1 into Core?

The original spec defined 4 profiles:
- Profile-0: Basic (QUIC + TLS only)
- Profile-1: Secure-Lite (DID auth + signatures)
- Profile-2: Secure + Sync
- Profile-3: Enterprise-Privacy

**Problem with Profile-0 alone:**
- No authentication = anyone can impersonate any agent
- No signatures = messages can be forged
- Not production-ready for remote/internet use
- Would require "custom" additions (PSK, API keys) that deviate from spec

**Solution: Unified Core with Profile-1 included:**
- Spec-compliant, no deviations
- Production-ready out of the box
- Full interoperability with MCP/A2A/ACP gateways
- DIDComm v2 is a W3C standard = credibility
- Ed25519 signatures = message integrity

### 1.2 AXCP as Interoperability Layer

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      AXCP: THE INTEROPERABILITY LAYER                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────┐      ┌─────────────────────────┐      ┌─────────┐            │
│   │   MCP   │◄────►│                         │◄────►│   A2A   │            │
│   │(Anthropic)     │      AXCP GATEWAY       │      │ (OpenAI)│            │
│   └─────────┘      │                         │      └─────────┘            │
│                    │  • Protocol translation │                              │
│   ┌─────────┐      │  • DID authentication   │      ┌─────────┐            │
│   │   ACP   │◄────►│  • Message signing      │◄────►│  REST   │            │
│   │         │      │  • Context bridging     │      │  APIs   │            │
│   └─────────┘      │  • Security enforcement │      └─────────┘            │
│                    └─────────────────────────┘                              │
│                              ▲         ▲                                    │
│                              │         │                                    │
│                    ┌─────────┴─────────┴─────────┐                          │
│                    │     AXCP NATIVE AGENTS      │                          │
│                    │  (Full protocol benefits)   │                          │
│                    └─────────────────────────────┘                          │
│                                                                              │
│  USE CASES:                                                                  │
│  ─────────────────────────────────────────────────────────────────────────  │
│  1. MCP ecosystem + A2A ecosystem communication via AXCP gateway            │
│  2. Add security layer to existing MCP/A2A deployments                      │
│  3. Build new AXCP-native systems with full interoperability                │
│  4. Enterprise orchestration across multiple AI providers                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.3 Why This Matters for the Gateway

With Profile-1 in Core:
- Gateway can **authenticate** incoming MCP/A2A connections
- Gateway can **sign** translated messages (provenance tracking)
- Gateway provides **audit trail** of protocol translations
- Gateway is **trustworthy** as a bridge component

Without Profile-1:
- Gateway would be a security hole
- No way to verify message integrity
- No identity verification
- Useless for production interoperability

---

## Part 2: New Architecture

### 2.1 Unified Core (Profile 0+1)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    AXCP CORE (Apache 2.0 - FREE)                            │
│                    ─────────────────────────────                            │
│                    Unified Profile 0+1: Production-Ready                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  TRANSPORT LAYER                      SECURITY LAYER                        │
│  ───────────────                      ──────────────                        │
│  • QUIC (RFC 9000)                    • DID mutual authentication (ECDH)   │
│  • Protobuf serialization             • Ed25519 message signatures          │
│  • TLS 1.3 mandatory                  • Signature verification              │
│  • QUIC DATAGRAM support              • Replay attack protection            │
│  • Stream multiplexing                • Profile negotiation                 │
│                                                                              │
│  PROTOCOL FEATURES                    INTEROPERABILITY                      │
│  ─────────────────                    ────────────────                      │
│  • Capability negotiation             • MCP ↔ AXCP gateway                  │
│  • Tool discovery & invocation        • A2A ↔ AXCP gateway                  │
│  • Error handling & codes             • ACP ↔ AXCP gateway                  │
│  • Telemetry collection               • REST API bridging                   │
│  • Basic routing policies             • Protocol translation                │
│  • Retry envelopes (store-forward)    • Identity mapping                    │
│                                                                              │
│  SDK SUPPORT                          DEPLOYMENT                            │
│  ───────────                          ──────────                            │
│  • Go SDK (reference)                 • Docker images                       │
│  • Rust SDK                           • Kubernetes Helm charts              │
│  • Python bindings                    • Cloud-ready                         │
│                                                                              │
│  SUITABLE FOR                         NOT INCLUDED                          │
│  ────────────                         ────────────                          │
│  ✅ Production deployments            • Context-Sync deltas                 │
│  ✅ Public internet communication     • mTLS client certificates            │
│  ✅ Multi-agent orchestration         • Differential privacy                │
│  ✅ MCP/A2A/ACP bridging              • SGX/SEV enclave support             │
│  ✅ Startup to enterprise scale       • PII filtering                       │
│  ✅ Single & multi-tenant             • Advanced audit trails               │
│                                       • Compliance reporting                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Advanced Tier (Profile 2)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    AXCP ADVANCED (Commercial - $)                           │
│                    ──────────────────────────────                           │
│                    Everything in Core, plus Profile 2                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  CONTEXT SYNCHRONIZATION              ENHANCED SECURITY                     │
│  ───────────────────────              ─────────────────                     │
│  • Delta patch format (CRDT)          • mTLS client certificates            │
│  • Versioned context graph            • Certificate chain validation        │
│  • Subscription & invalidation        • Enhanced access control             │
│  • Store-and-forward sync             • Rate limiting middleware            │
│  • Conflict resolution (LWW)          • Structured logging                  │
│                                                                              │
│  OPTIONAL FEATURES                    SUPPORT                               │
│  ─────────────────                    ───────                               │
│  • Differential privacy (optional)    • Priority email support              │
│  • SGX/SEV enclave support            • Integration assistance              │
│  • Attestation proof verification     • SLA available                       │
│                                                                              │
│  SUITABLE FOR                                                                │
│  ────────────                                                                │
│  ✅ Stateful multi-agent systems                                            │
│  ✅ Enterprise deployments requiring mTLS                                   │
│  ✅ Systems needing synchronized shared state                               │
│  ✅ Optional privacy-preserving telemetry                                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.3 Enterprise Tier (Profile 3)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    AXCP ENTERPRISE (Commercial - $$)                        │
│                    ─────────────────────────────────                        │
│                    Everything in Advanced, plus Profile 3                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  MANDATORY PRIVACY                    COMPLIANCE                            │
│  ─────────────────                    ──────────                            │
│  • Differential privacy (mandatory)   • GDPR compliance tools               │
│  • Configurable ε/δ budgets           • HIPAA compliance tools              │
│  • Laplace & Gaussian mechanisms      • SOC2 reporting                      │
│  • Privacy budget enforcement         • Audit trail generation              │
│                                                                              │
│  DATA PROTECTION                      TRI-AI ORCHESTRATION                  │
│  ───────────────                      ────────────────────                  │
│  • PII filtering & redaction          • Gemini CLI Agent (Research AI)     │
│  • Advanced metadata anonymization    • Codex CLI Agent (Code AI)          │
│  • Field-level encryption             • Claude Code integration             │
│  • Data minimization                  • Multi-agent coordination            │
│                                                                              │
│  AUDIT & MONITORING                   SUPPORT                               │
│  ──────────────────                   ───────                               │
│  • Merkle tree verified logs          • Dedicated support engineer          │
│  • Tamper-proof audit trails          • 24/7 SLA                            │
│  • Compliance dashboards              • On-premise deployment option        │
│  • Real-time monitoring               • Custom development available        │
│                                                                              │
│  SUITABLE FOR                                                                │
│  ────────────                                                                │
│  ✅ Financial services                                                       │
│  ✅ Healthcare / HIPAA                                                       │
│  ✅ Government / FedRAMP                                                     │
│  ✅ Any regulated industry                                                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.4 Feature Comparison Matrix

| Feature | Core (Free) | Advanced ($) | Enterprise ($$) |
|---------|-------------|--------------|-----------------|
| **Transport** |
| QUIC + Protobuf | ✅ | ✅ | ✅ |
| TLS 1.3 | ✅ | ✅ | ✅ |
| QUIC Datagrams | ✅ | ✅ | ✅ |
| **Authentication** |
| DID mutual auth (ECDH) | ✅ | ✅ | ✅ |
| Ed25519 signatures | ✅ | ✅ | ✅ |
| Replay protection | ✅ | ✅ | ✅ |
| mTLS client certs | ❌ | ✅ | ✅ |
| **Protocol** |
| Capability negotiation | ✅ | ✅ | ✅ |
| Profile negotiation | ✅ | ✅ | ✅ |
| Tool discovery | ✅ | ✅ | ✅ |
| Context-Sync deltas | ❌ | ✅ | ✅ |
| **Privacy** |
| Basic telemetry | ✅ | ✅ | ✅ |
| Differential privacy | ❌ | Optional | Mandatory |
| PII filtering | ❌ | ❌ | ✅ |
| Metadata anonymization | ❌ | ❌ | ✅ |
| **Infrastructure** |
| SGX/SEV enclaves | ❌ | Optional | ✅ |
| Rate limiting | Basic | ✅ | ✅ |
| Structured logging | Basic | ✅ | ✅ |
| Audit trails | ❌ | ❌ | ✅ |
| **Interoperability** |
| MCP gateway | ✅ | ✅ | ✅ |
| A2A gateway | ✅ | ✅ | ✅ |
| ACP gateway | ✅ | ✅ | ✅ |
| REST API bridge | ✅ | ✅ | ✅ |
| **Orchestration** |
| Basic routing | ✅ | ✅ | ✅ |
| WASM policies | ❌ | ✅ | ✅ |
| TRI-AI system | ❌ | ❌ | ✅ |
| **Support** |
| Community (GitHub) | ✅ | ✅ | ✅ |
| Email support | ❌ | ✅ | ✅ |
| Priority support | ❌ | ❌ | ✅ |
| Dedicated engineer | ❌ | ❌ | ✅ |

---

## Part 3: Repository Structure

### 3.1 Target Structure

```
AXCP/
├── axcp-spec/                      # Apache 2.0 (Core = Profile 0+1)
│   ├── LICENSE                     # Apache 2.0
│   ├── README.md                   # Project overview
│   ├── CHANGELOG.md                # Version history
│   │
│   ├── spec/                       # Protocol specification
│   │   ├── axcp-v1.0.md            # Unified spec (0+1 merged)
│   │   └── appendix_c_interop.md   # Interoperability profiles
│   │
│   ├── proto/                      # Protobuf definitions
│   │   ├── axcp.proto              # Core protocol messages
│   │   └── axcp_pb2.py             # Python generated
│   │
│   ├── sdk/
│   │   ├── go/                     # Go SDK (reference implementation)
│   │   │   ├── axcp/               # Core types (envelope, capability)
│   │   │   ├── netquic/            # QUIC transport
│   │   │   ├── auth/               # DID + Ed25519 (NEW)
│   │   │   │   ├── did.go          # DID verification
│   │   │   │   ├── signer.go       # Ed25519 signatures
│   │   │   │   ├── verifier.go     # Signature verification
│   │   │   │   └── replay.go       # Replay protection
│   │   │   └── negotiate/          # Profile negotiation (NEW)
│   │   │
│   │   ├── rust/                   # Rust SDK
│   │   └── python/                 # Python bindings
│   │
│   ├── gateway/                    # AXCP Gateway (Core features only)
│   │   ├── cmd/                    # CLI entry points
│   │   ├── internal/               # Gateway internals
│   │   │   ├── bridge/             # MCP/A2A/ACP bridging
│   │   │   ├── routing/            # Basic routing
│   │   │   └── telemetry/          # Non-DP telemetry
│   │   └── configs/                # Configuration files
│   │
│   ├── examples/                   # Usage examples
│   │   ├── go/
│   │   │   ├── simple_agent/       # Basic AXCP agent
│   │   │   ├── mcp_bridge/         # MCP ↔ AXCP example
│   │   │   └── multi_agent/        # Multi-agent orchestration
│   │   └── python/
│   │
│   └── docs/                       # Documentation
│       ├── getting-started.md
│       ├── authentication.md
│       ├── interoperability.md
│       └── gateway-setup.md
│
└── axcp-enterprise/                # Commercial (Advanced + Enterprise)
    ├── LICENSE.commercial          # Commercial license terms
    ├── README.md
    │
    ├── advanced/                   # Tier: Advanced (Profile 2)
    │   ├── context-sync/           # Delta synchronization
    │   │   ├── patch.go            # Delta patch application
    │   │   ├── crdt.go             # CRDT merge logic
    │   │   ├── subscription.go     # Context subscriptions
    │   │   └── store_forward.go    # Offline sync
    │   │
    │   ├── mtls/                   # mTLS middleware
    │   │   ├── verifier.go         # Client cert verification
    │   │   └── middleware.go       # HTTP/QUIC middleware
    │   │
    │   ├── dp/                     # Optional DP (moved from axcp-spec)
    │   │   ├── noise.go            # Laplace/Gaussian noise
    │   │   ├── budget.go           # Privacy budget
    │   │   └── config.go           # DP configuration
    │   │
    │   ├── enclave/                # SGX/SEV support
    │   │   ├── attestation.go      # Attestation verification
    │   │   └── sgx_stub.go         # SGX integration
    │   │
    │   └── middleware/             # Advanced middleware
    │       ├── rate_limit.go       # Rate limiting
    │       └── logging.go          # Structured logging
    │
    ├── enterprise/                 # Tier: Enterprise (Profile 3)
    │   ├── secure/                 # Secure telemetry (existing)
    │   │   ├── telemetry/
    │   │   │   ├── handler.go
    │   │   │   ├── pii_filter.go
    │   │   │   └── orchestration.go
    │   │   └── config/
    │   │       └── pii_schema.yaml
    │   │
    │   ├── dp-mandatory/           # Mandatory DP enforcement
    │   │   ├── enforcer.go         # DP policy enforcement
    │   │   └── validator.go        # DP compliance validation
    │   │
    │   ├── audit/                  # Audit system
    │   │   ├── merkle.go           # Merkle tree logs
    │   │   ├── trail.go            # Audit trail
    │   │   └── export.go           # Audit export
    │   │
    │   └── compliance/             # Compliance reporting
    │       ├── gdpr.go             # GDPR tools
    │       ├── hipaa.go            # HIPAA tools
    │       └── report.go           # Report generation
    │
    ├── tri-ai/                     # TRI-AI Orchestration (Enterprise only)
    │   ├── gemini-cli-agent/       # Research Intelligence
    │   ├── codex-cli-agent/        # Code Intelligence
    │   └── claude-code-agent/      # Claude orchestrator
    │
    └── charts/                     # Kubernetes deployment
        └── enterprise-node/
```

### 3.2 File Migration Plan

#### Files to KEEP in axcp-spec (Core):
```
✅ proto/axcp.proto                    # Core protocol
✅ sdk/go/axcp/                        # Envelope, capability
✅ sdk/go/netquic/                     # QUIC transport
✅ examples/go/simple_chat/            # Basic example
```

#### Files to MOVE from axcp-spec → axcp-enterprise:
```
→ edge/gateway/internal/dp_noise.go    → advanced/dp/noise.go
→ edge/gateway/internal/dp_budget.go   → advanced/dp/budget.go
→ config/dp_budget.yaml                → advanced/dp/config.yaml
→ sdk/go/dp/                           → advanced/dp/sdk/
→ tests/dp/                            → advanced/tests/dp/
→ v0.3/dp/                             → advanced/v0.3/dp/
```

#### Files to MOVE from axcp-enterprise → axcp-spec (Core):
```
→ enterprise/secure/telemetry/signer.go → sdk/go/auth/signer.go
  (Ed25519 implementation - needed for Core)
```

#### Files to REORGANIZE within axcp-enterprise:
```
→ gemini-cli-agent/    → tri-ai/gemini-cli-agent/
→ codex-cli-agent/     → tri-ai/codex-cli-agent/
→ claude-code-agent/   → tri-ai/claude-code-agent/
```

---

## Part 4: Implementation Plan

### Phase 1: Core Authentication (Week 1-2)

**Objective:** Implement DID + Ed25519 in Core SDK

| Task | Priority | Effort | Notes |
|------|----------|--------|-------|
| Move `signer.go` from enterprise to Core | P0 | 0.5 day | Already implemented |
| Create `sdk/go/auth/did.go` | P0 | 3 days | DID verification logic |
| Create `sdk/go/auth/verifier.go` | P0 | 1 day | Signature verification |
| Create `sdk/go/auth/replay.go` | P0 | 1 day | Nonce/timestamp replay protection |
| Implement profile negotiation | P0 | 2 days | ProfileNegotiate/ProfileAck exchange |
| Update `netquic/client.go` with auth | P0 | 1 day | Integrate auth into connection |
| Add tests for auth module | P1 | 1 day | Unit + integration tests |

**Deliverable:** Go SDK with full Profile-1 authentication

### Phase 2: Code Separation (Week 3)

**Objective:** Clean separation between Apache 2.0 and Commercial code

| Task | Priority | Effort | Notes |
|------|----------|--------|-------|
| Move DP modules to enterprise | P0 | 1 day | All dp_*.go files |
| Reorganize enterprise folder structure | P0 | 0.5 day | advanced/ + enterprise/ + tri-ai/ |
| Update all import paths | P0 | 1 day | Go modules, Python imports |
| Update go.mod/go.work files | P0 | 0.5 day | Module dependencies |
| Create enterprise SDK extension pattern | P1 | 1 day | How enterprise extends Core |
| Update CI/CD pipelines | P1 | 1 day | Separate builds |

**Deliverable:** Two cleanly separated repositories

### Phase 3: Gateway Update (Week 4)

**Objective:** Gateway works with Core features only

| Task | Priority | Effort | Notes |
|------|----------|--------|-------|
| Remove DP from Core gateway | P0 | 0.5 day | Move to advanced/ |
| Add DID auth to gateway | P0 | 1 day | Verify incoming connections |
| Add signature verification | P0 | 1 day | Verify message integrity |
| Update MCP/A2A bridge | P1 | 2 days | Use new auth system |
| Add gateway documentation | P1 | 1 day | Setup and config guide |

**Deliverable:** Production-ready Core gateway

### Phase 4: Advanced Tier (Week 5-6)

**Objective:** Complete Advanced tier implementation

| Task | Priority | Effort | Notes |
|------|----------|--------|-------|
| Implement Context-Sync | P0 | 5 days | Delta patches, CRDT, subscriptions |
| Add mTLS middleware | P0 | 2 days | Client certificate verification |
| Integrate optional DP | P1 | 1 day | Wire up moved DP modules |
| Add rate limiting | P1 | 1 day | Configurable limits |
| Add structured logging | P1 | 1 day | JSON logging with levels |
| Create advanced tier tests | P1 | 2 days | Integration tests |

**Deliverable:** Fully functional Advanced tier

### Phase 5: Enterprise Polish (Week 7-8)

**Objective:** Ensure Enterprise tier is production-ready

| Task | Priority | Effort | Notes |
|------|----------|--------|-------|
| Audit existing enterprise code | P0 | 1 day | Review for quality |
| Add DP enforcement layer | P0 | 2 days | Mandatory DP for Profile-3 |
| Enhance PII filtering | P1 | 1 day | More field types |
| Add compliance reporting | P1 | 2 days | GDPR/HIPAA templates |
| Enhance audit trails | P1 | 2 days | Merkle tree verification |
| Add multi-tenant isolation | P0 | 2 days | Tenant-scoped data |

**Deliverable:** Production-ready Enterprise tier

### Phase 6: Documentation & Launch (Week 9-10)

**Objective:** Prepare for public launch

| Task | Priority | Effort | Notes |
|------|----------|--------|-------|
| Update LICENSE to Apache 2.0 | P0 | 0.5 day | Core repository |
| Write new README | P0 | 1 day | Emphasize interoperability |
| Create getting-started guide | P0 | 2 days | Quick start tutorial |
| Document authentication | P0 | 1 day | DID + Ed25519 usage |
| Document interoperability | P0 | 2 days | MCP/A2A/ACP bridging |
| Update spec to v1.0 | P1 | 1 day | Merge Profile 0+1 in spec |
| Create pricing page | P1 | Business | Advanced/Enterprise pricing |
| Prepare arXiv paper update | P2 | 3 days | Updated architecture |

**Deliverable:** Launch-ready documentation and materials

---

## Part 5: Technical Details

### 5.1 DID Authentication Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     DID MUTUAL AUTHENTICATION FLOW                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Agent A                                              Agent B               │
│  ────────                                             ────────              │
│                                                                              │
│  1. Generate DID keypair (Ed25519)                                          │
│     did:axcp:abc123...                                                      │
│                                                                              │
│  2. ─────────── QUIC Connection (TLS 1.3) ───────────►                      │
│                                                                              │
│  3. ◄────────── ProfileNegotiate{mask=0b11, min=1} ──                       │
│     ─────────── ProfileNegotiate{mask=0b11, min=1} ──►                      │
│                                                                              │
│  4. ◄────────── ProfileAck{agreed=1} ────────────────                       │
│     ─────────── ProfileAck{agreed=1} ────────────────►                      │
│                                                                              │
│  5. ─────────── CapabilityOffer{did=abc123, sig=...} ►                      │
│     ◄────────── CapabilityOffer{did=def456, sig=...} ─                      │
│                                                                              │
│  6. Both sides verify:                                                       │
│     • DID resolves to valid public key                                      │
│     • Signature is valid for the message                                    │
│     • Nonce is fresh (replay protection)                                    │
│                                                                              │
│  7. ◄═══════════ Authenticated Session ═══════════════►                     │
│                                                                              │
│  8. All subsequent messages include:                                         │
│     • trace_id for correlation                                              │
│     • signature (Ed25519)                                                   │
│     • nonce (replay protection)                                             │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Message Signing Format

```protobuf
message AxcpEnvelope {
  uint32 version       = 1;   // protocol version
  string trace_id      = 2;   // correlation ID
  uint32 profile       = 3;   // 0-1 for Core, 2 for Advanced, 3 for Enterprise

  oneof payload { ... }

  // Core (Profile 1) - included in open source
  bytes  signature     = 100; // Ed25519 signature over (version || trace_id || payload)
  uint64 nonce         = 102; // Replay protection (timestamp + random)
  string signer_did    = 103; // DID of signing agent

  // Advanced (Profile 2) - commercial
  bytes  attestation_proof = 101; // SGX/SEV quote

  // Enterprise (Profile 3) - commercial
  DpParams dp_params   = 104; // Differential privacy parameters
}
```

### 5.3 Import Patterns

**Core SDK usage:**
```go
import (
    "github.com/tradephantom/axcp-spec/sdk/go/axcp"
    "github.com/tradephantom/axcp-spec/sdk/go/netquic"
    "github.com/tradephantom/axcp-spec/sdk/go/auth"
)

// Create authenticated client
client, err := netquic.Dial(addr,
    netquic.WithDID(myDID, myPrivateKey),
    netquic.WithProfile(1),
)

// Messages are automatically signed
env := axcp.NewEnvelope(traceID, 1)
env.SetCapabilityOffer(offer)
client.Send(env)  // Signature added automatically
```

**Advanced tier extension:**
```go
import (
    "github.com/tradephantom/axcp-spec/sdk/go/axcp"
    "github.com/tradephantom/axcp-spec/sdk/go/netquic"
    "github.com/tradephantom/axcp-enterprise/advanced/sync"
    "github.com/tradephantom/axcp-enterprise/advanced/mtls"
)

// Create client with mTLS + Context-Sync
client, err := netquic.Dial(addr,
    netquic.WithDID(myDID, myPrivateKey),
    netquic.WithProfile(2),
    mtls.WithClientCert(cert, key),
)

// Use Context-Sync
ctx := sync.NewContext("conversation-123")
ctx.Subscribe(client)
ctx.ApplyPatch(patch)
```

**Enterprise tier extension:**
```go
import (
    "github.com/tradephantom/axcp-enterprise/enterprise/secure/telemetry"
    "github.com/tradephantom/axcp-enterprise/enterprise/dp"
)

// Create enterprise handler with mandatory DP
handler := telemetry.NewHandler(
    telemetry.WithPIIFilter(schema),
    telemetry.WithMandatoryDP(epsilon, delta),
    telemetry.WithAuditTrail(auditConfig),
)
```

---

## Part 6: Licensing

### 6.1 Core License (Apache 2.0)

```
Copyright 2025-2026 TradePhantom LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

### 6.2 Commercial License Summary

**AXCP Advanced:**
- Per-node pricing: $X/node/month
- Volume discounts available
- Source code access (no redistribution)
- Email support included
- Annual commitment discount

**AXCP Enterprise:**
- Custom pricing based on deployment
- Unlimited nodes option
- Full source code access
- Dedicated support engineer
- SLA guarantees
- On-premise deployment support
- Compliance certification assistance

---

## Part 7: Success Metrics

### 7.1 Adoption (Core)

| Metric | 3 Month | 6 Month | 12 Month |
|--------|---------|---------|----------|
| GitHub Stars | 1,000 | 3,000 | 10,000 |
| Weekly Downloads | 500 | 2,000 | 10,000 |
| Production Deployments | 50 | 200 | 1,000 |
| Contributors | 15 | 40 | 100 |
| MCP/A2A Integrations | 5 | 20 | 50 |

### 7.2 Revenue (Commercial)

| Metric | 6 Month | 12 Month |
|--------|---------|----------|
| Advanced Customers | 10 | 50 |
| Enterprise Customers | 2 | 10 |
| MRR | $10,000 | $50,000 |
| Trial → Paid Conversion | 15% | 20% |

---

## Part 8: Risk Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| DID implementation complexity | Medium | High | Use existing libraries (did-jwt, go-did) |
| Low enterprise conversion | Medium | High | Clear value prop, free trial, case studies |
| Competitors add similar features | High | Medium | First-mover advantage, community, superior interop |
| Performance overhead from signatures | Low | Medium | Ed25519 is fast (~70k sigs/sec), optional for internal |

---

## Appendix A: Timeline Summary

```
Week 1-2:  Core Authentication (DID + Ed25519)
Week 3:    Code Separation (move files, update imports)
Week 4:    Gateway Update (auth integration)
Week 5-6:  Advanced Tier (Context-Sync, mTLS)
Week 7-8:  Enterprise Polish (DP enforcement, compliance)
Week 9-10: Documentation & Launch Preparation

Total: 10 weeks to launch-ready state
```

---

## Approval

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Product Owner | | | |
| Technical Lead | | | |

---

**Document History:**
- v1.0 (2026-01-17): Initial 4-tier structure
- v2.0 (2026-01-17): Revised to 3-tier structure with unified Core (Profile 0+1)
