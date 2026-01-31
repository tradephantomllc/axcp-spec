# AXCP Restructuring: Final Decisions Document

**Version:** 1.0
**Date:** 2026-01-17
**Status:** APPROVED - Ready for Roadmap Creation
**For:** PM to create operational roadmap with issue-by-issue tasks

---

## Executive Decision Summary

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| 1 | **Licensing** | Apache 2.0 (immediate) | Maximum adoption, zero friction |
| 2 | **Sync in Core** | Option A (zero sync) | Clean boundaries, clear monetization |
| 3 | **Spec Update** | Unify Profile 0+1 | "Secure Baseline" as Core profile |
| 4 | **Tier Naming** | Core / Advanced / Enterprise | Enterprise-friendly, clear |

---

## Workflow Decisions

### Repository Strategy

| Decision | Choice |
|----------|--------|
| **GitHub visibility** | Make repo PRIVATE during restructuring |
| **Branch strategy** | Create `restructure/open-core` branch for all work |
| **Merge strategy** | All changes via PR to restructure branch, then merge to main when complete |
| **CI requirement** | All tests must pass before any PR merge |

### Development Approach

```
main (frozen) ─────────────────────────────────────────────► (merge when ready)
                    │                                              ▲
                    │ branch                                       │ PR
                    ▼                                              │
restructure/open-core ──► work ──► PR ──► review ──► merge ───────┘
```

---

## Final Architecture

### Tier Structure (Confirmed)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         AXCP FINAL TIER STRUCTURE                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  AXCP CORE (Apache 2.0 - FREE)                                              │
│  ══════════════════════════════                                             │
│  Profile: Secure Baseline (0+1 unified)                                     │
│                                                                              │
│  ✅ QUIC + TLS 1.3 + Protobuf                                               │
│  ✅ DID mutual authentication (ECDH)                                        │
│  ✅ Ed25519 message signatures                                              │
│  ✅ Signature verification                                                  │
│  ✅ Replay attack protection                                                │
│  ✅ Profile negotiation                                                     │
│  ✅ Capability negotiation & tool discovery                                 │
│  ✅ Basic telemetry (no DP)                                                 │
│  ✅ Gateway: MCP ↔ AXCP ↔ A2A ↔ ACP bridging                               │
│  ✅ Error handling & retry envelopes                                        │
│  ❌ NO Context-Sync                                                         │
│  ❌ NO Differential Privacy                                                 │
│  ❌ NO mTLS client certificates                                             │
│                                                                              │
│  ─────────────────────────────────────────────────────────────────────────  │
│                                                                              │
│  AXCP ADVANCED (Commercial - $)                                             │
│  ══════════════════════════════                                             │
│  Profile 2 features                                                         │
│                                                                              │
│  Everything in Core, plus:                                                  │
│  ✅ Context-Sync with delta patches (CRDT)                                  │
│  ✅ Versioned context graph                                                 │
│  ✅ Subscription & invalidation                                             │
│  ✅ Store-and-forward sync                                                  │
│  ✅ mTLS client certificates                                                │
│  ✅ SGX/SEV enclave support (optional)                                      │
│  ✅ Differential Privacy (optional)                                         │
│  ✅ Advanced rate limiting                                                  │
│  ✅ Structured logging                                                      │
│  ✅ Email support                                                           │
│                                                                              │
│  ─────────────────────────────────────────────────────────────────────────  │
│                                                                              │
│  AXCP ENTERPRISE (Commercial - $$)                                          │
│  ═════════════════════════════════                                          │
│  Profile 3 features                                                         │
│                                                                              │
│  Everything in Advanced, plus:                                              │
│  ✅ Differential Privacy (MANDATORY)                                        │
│  ✅ Configurable ε/δ privacy budgets                                        │
│  ✅ PII filtering & redaction                                               │
│  ✅ Advanced metadata anonymization                                         │
│  ✅ Audit trails (Merkle tree verified)                                     │
│  ✅ Compliance reporting (GDPR, HIPAA, SOC2)                                │
│  ✅ TRI-AI Orchestration System                                             │
│  ✅ Multi-tenant isolation                                                  │
│  ✅ Dedicated support + SLA                                                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Repository Structure (Final)

```
AXCP/
│
├── axcp-spec/                          # Apache 2.0 (CORE)
│   │
│   ├── LICENSE                         # Apache 2.0 license
│   ├── README.md                       # Interoperability-first messaging
│   ├── CHANGELOG.md
│   ├── CONTRIBUTING.md                 # Updated for Apache 2.0
│   ├── CLA.md                          # Review/update for Apache 2.0
│   │
│   ├── spec/
│   │   ├── axcp-v1.0.md               # Unified spec (Secure Baseline)
│   │   └── appendix_c_interop.md
│   │
│   ├── proto/
│   │   ├── axcp.proto                 # Core protocol (remove DP refs if any)
│   │   └── axcp_pb2.py
│   │
│   ├── sdk/
│   │   └── go/
│   │       ├── axcp/                  # Envelope, capability, telemetry
│   │       ├── netquic/               # QUIC transport
│   │       ├── auth/                  # NEW: DID + Ed25519 + replay
│   │       │   ├── did.go             # DID verification
│   │       │   ├── signer.go          # Ed25519 signing (from enterprise)
│   │       │   ├── verifier.go        # Signature verification
│   │       │   └── replay.go          # Replay protection
│   │       └── negotiate/             # NEW: Profile negotiation
│   │           └── profile.go
│   │
│   ├── gateway/                       # Core gateway (no DP)
│   │   ├── cmd/
│   │   └── internal/
│   │       ├── bridge/                # MCP/A2A/ACP bridging
│   │       ├── routing/
│   │       └── telemetry/             # Basic telemetry (no DP)
│   │
│   ├── examples/
│   │   └── go/
│   │       ├── simple_agent/
│   │       ├── mcp_bridge/
│   │       └── authenticated_chat/    # NEW: Shows DID auth
│   │
│   └── docs/
│       ├── getting-started.md
│       ├── authentication.md          # NEW: DID + Ed25519 guide
│       ├── interoperability.md
│       └── gateway-setup.md
│
│
└── axcp-enterprise/                    # COMMERCIAL (Advanced + Enterprise)
    │
    ├── LICENSE.commercial
    ├── README.md
    │
    ├── advanced/                       # Profile 2
    │   ├── context-sync/
    │   │   ├── patch.go
    │   │   ├── crdt.go
    │   │   ├── subscription.go
    │   │   └── store_forward.go
    │   ├── mtls/
    │   ├── dp/                        # Moved from axcp-spec
    │   │   ├── noise.go
    │   │   ├── budget.go
    │   │   └── config.go
    │   ├── enclave/
    │   └── middleware/
    │
    ├── enterprise/                     # Profile 3
    │   ├── secure/                    # Existing (keep)
    │   ├── dp-mandatory/
    │   ├── audit/
    │   └── compliance/
    │
    └── tri-ai/                        # Reorganized
        ├── gemini-cli-agent/
        ├── codex-cli-agent/
        └── claude-code-agent/
```

---

## Architectural Guardrails (PM Requirements)

These rules MUST be enforced throughout development:

### Rule 1: Dependency Direction
```
Core ──────────────────────────────────────────────►
  │
  │  Core NEVER imports from Advanced/Enterprise
  │
  ▼
Advanced ─────────────────────────────────────────►
  │
  │  Advanced imports Core, NEVER Enterprise
  │
  ▼
Enterprise ───────────────────────────────────────►
     Enterprise imports Core and Advanced
```

### Rule 2: No Cross-Module Internal Packages
- `internal/` packages stay within their module
- If code needs to be shared → make it a public package
- No import hacks or workarounds

### Rule 3: Independent CI
- Core CI must pass completely standalone
- Advanced CI runs Core tests + Advanced tests
- Enterprise CI runs all tests
- Each tier is independently buildable

### Rule 4: Clean Module Boundaries
```go
// CORRECT: Core module
module github.com/tradephantom/axcp-spec

// CORRECT: Advanced module (depends on Core)
module github.com/tradephantom/axcp-enterprise/advanced
require github.com/tradephantom/axcp-spec v1.0.0

// CORRECT: Enterprise module (depends on both)
module github.com/tradephantom/axcp-enterprise/enterprise
require github.com/tradephantom/axcp-spec v1.0.0
require github.com/tradephantom/axcp-enterprise/advanced v1.0.0
```

---

## File Migration Plan (Final)

### Phase A: Move FROM axcp-spec TO axcp-enterprise

| Source | Destination | Notes |
|--------|-------------|-------|
| `edge/gateway/internal/dp_noise.go` | `advanced/dp/noise.go` | DP module |
| `edge/gateway/internal/dp_budget.go` | `advanced/dp/budget.go` | DP module |
| `config/dp_budget.yaml` | `advanced/dp/config/` | DP config |
| `sdk/go/dp/` | `advanced/dp/sdk/` | DP SDK |
| `tests/dp/` | `advanced/tests/dp/` | DP tests |
| `v0.3/dp/` | `advanced/v0.3/dp/` | DP v0.3 work |

### Phase B: Move FROM axcp-enterprise TO axcp-spec

| Source | Destination | Notes |
|--------|-------------|-------|
| `enterprise/secure/telemetry/signer.go` | `sdk/go/auth/signer.go` | Ed25519 (Core needs it) |

### Phase C: Reorganize WITHIN axcp-enterprise

| Source | Destination | Notes |
|--------|-------------|-------|
| `gemini-cli-agent/` | `tri-ai/gemini-cli-agent/` | Reorganize |
| `codex-cli-agent/` | `tri-ai/codex-cli-agent/` | Reorganize |
| `claude-code-agent/` | `tri-ai/claude-code-agent/` | Reorganize |

### Phase D: Create NEW in axcp-spec

| File | Purpose |
|------|---------|
| `sdk/go/auth/did.go` | DID verification |
| `sdk/go/auth/verifier.go` | Signature verification |
| `sdk/go/auth/replay.go` | Replay protection |
| `sdk/go/negotiate/profile.go` | Profile negotiation |

---

## Legal Checklist (Pre-Implementation)

Before writing any code, confirm:

| Item | Status | Action |
|------|--------|--------|
| CLA.md adequate for Apache 2.0 | ⬜ TODO | Review with legal if needed |
| Trademark policy for "AXCP" | ⬜ TODO | Create simple policy document |
| All Core code is Apache 2.0 compatible | ⬜ TODO | Audit dependencies |
| No enterprise code accidentally in Core | ⬜ TODO | Verify after migration |
| LICENSE file updated | ⬜ TODO | Replace BUSL with Apache 2.0 |
| Copyright headers updated | ⬜ TODO | Update in all Core files |

---

## Definition of Done (Per Phase)

Each phase is complete when:

1. ✅ All code changes committed to `restructure/open-core` branch
2. ✅ All tests pass (unit + integration)
3. ✅ CI pipeline green
4. ✅ No import errors or dependency issues
5. ✅ Documentation updated (if applicable)
6. ✅ PR reviewed and approved
7. ✅ No `internal/` cross-module imports
8. ✅ Core builds and tests independently

---

## Spec Update: Profile Naming

### Current (in spec)
```
Profile-0: Basic
Profile-1: Secure-Lite
Profile-2: Secure + Sync
Profile-3: Enterprise-Privacy
```

### New (to update in spec)
```
Core Profile: Secure Baseline (formerly 0+1)
  - Includes: transport, auth, signatures, replay protection
  - Note: "Profile-0 transport-only" is deprecated for production use

Advanced Profile: Sync + Hardening (formerly Profile-2)
  - Includes: Context-Sync, mTLS, optional DP, enclaves

Enterprise Profile: Privacy + Compliance (formerly Profile-3)
  - Includes: Mandatory DP, PII filtering, audit, compliance
```

---

## Product Messaging (Final)

### One-liner
> "AXCP Core is the production-ready secure baseline for AI agent interoperability."

### Expanded (for README/website)
> "AXCP Core provides secure transport, DID authentication, message signing, and protocol bridges for MCP, A2A, and ACP ecosystems. Advanced adds state synchronization and optional privacy controls. Enterprise adds compliance-grade privacy enforcement, auditability, and TRI-AI orchestration."

### Tagline
> "Connect every AI agent. Securely."

---

## Implementation Sequence (PM Recommended Order)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    IMPLEMENTATION SEQUENCE                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  STEP 0: Pre-Work (Before Any Code)                                         │
│  ──────────────────────────────────                                          │
│  □ Make GitHub repo private                                                 │
│  □ Create branch: restructure/open-core                                     │
│  □ Review/update CLA.md                                                     │
│  □ Create trademark policy (simple)                                         │
│  □ Plan new repo layout (directories only, no code yet)                    │
│                                                                              │
│  STEP 1: Stabilize Repo Layout + CI Boundaries                              │
│  ─────────────────────────────────────────────                               │
│  □ Create new directory structure (empty folders)                          │
│  □ Update go.mod / go.work files for new layout                           │
│  □ Update CI to understand new structure                                   │
│  □ Verify Core CI runs independently                                       │
│  □ PR + merge to restructure branch                                        │
│                                                                              │
│  STEP 2: Move Ed25519 to Core (Public Package)                              │
│  ─────────────────────────────────────────────                               │
│  □ Copy signer.go to sdk/go/auth/signer.go                                │
│  □ Make it public package (not internal)                                   │
│  □ Add tests                                                               │
│  □ Update imports in enterprise to use Core version                       │
│  □ PR + merge                                                              │
│                                                                              │
│  STEP 3: Implement Core Auth (DID + Replay + Negotiation)                   │
│  ────────────────────────────────────────────────────────                    │
│  □ Create sdk/go/auth/did.go                                              │
│  □ Create sdk/go/auth/verifier.go                                         │
│  □ Create sdk/go/auth/replay.go                                           │
│  □ Create sdk/go/negotiate/profile.go                                     │
│  □ Unit tests for each                                                     │
│  □ Integration test: full auth flow                                        │
│  □ PR + merge                                                              │
│                                                                              │
│  STEP 4: Move DP Out of Core                                                │
│  ───────────────────────────                                                 │
│  □ Move dp_*.go files to advanced/dp/                                     │
│  □ Move sdk/go/dp/ to advanced/                                           │
│  □ Update all imports                                                      │
│  □ Remove DP from Core gateway                                            │
│  □ Verify Core still builds/tests without DP                              │
│  □ PR + merge                                                              │
│                                                                              │
│  STEP 5: Update Gateway with Auth                                           │
│  ────────────────────────────────                                            │
│  □ Integrate DID auth into gateway                                        │
│  □ Add signature verification to incoming messages                        │
│  □ Update MCP/A2A bridge with auth                                        │
│  □ Gateway tests                                                           │
│  □ PR + merge                                                              │
│                                                                              │
│  STEP 6: Reorganize Enterprise Structure                                    │
│  ───────────────────────────────────                                         │
│  □ Create advanced/ directory structure                                   │
│  □ Move TRI-AI agents to tri-ai/                                          │
│  □ Update imports throughout enterprise                                   │
│  □ Verify Advanced builds with Core dependency                            │
│  □ Verify Enterprise builds with both dependencies                        │
│  □ PR + merge                                                              │
│                                                                              │
│  STEP 7: Documentation + Licensing                                          │
│  ─────────────────────────────────                                           │
│  □ Update LICENSE to Apache 2.0                                           │
│  □ Update copyright headers in all Core files                             │
│  □ Write new README (interoperability messaging)                          │
│  □ Write authentication.md guide                                          │
│  □ Update spec to v1.0 (Secure Baseline)                                  │
│  □ PR + merge                                                              │
│                                                                              │
│  STEP 8: Final Verification + Main Merge                                    │
│  ───────────────────────────────────────                                     │
│  □ Full test suite (Core + Advanced + Enterprise)                         │
│  □ Verify no enterprise code in Core                                      │
│  □ Verify dependency directions correct                                   │
│  □ Final PR: restructure/open-core → main                                 │
│  □ Make repo public again                                                 │
│  □ Tag release v1.0.0                                                     │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Success Criteria

The restructuring is complete when:

1. ✅ Core repo (axcp-spec) is Apache 2.0 licensed
2. ✅ Core contains Profile 0+1 features (auth, signatures, replay, negotiation)
3. ✅ Core does NOT contain DP, Context-Sync, or enterprise features
4. ✅ Core builds and tests 100% independently
5. ✅ Gateway bridges MCP/A2A/ACP with authentication
6. ✅ Advanced and Enterprise tiers are cleanly separated
7. ✅ All CI pipelines green
8. ✅ Documentation reflects new structure
9. ✅ README emphasizes interoperability positioning

---

## Request for PM

Please use this document to create the operational roadmap with:

1. **GitHub Issues** for each step (with acceptance criteria)
2. **Test requirements** for each issue
3. **Dependencies** between issues
4. **Estimated effort** per issue

We will execute tasks step-by-step as assigned, with PR reviews at each stage.

---

**Document Status:** FINAL - Ready for roadmap creation
**Next Action:** PM creates issue-by-issue roadmap
**Execution Team:** Will implement according to roadmap tasks

