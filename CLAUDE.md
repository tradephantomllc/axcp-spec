# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 🚨 FUNDAMENTAL PRINCIPLES - NEVER VIOLATE

### 1. TRANSPARENCY AND ABSOLUTE CREDIBILITY
- **Never invent false information** about features, capabilities, or technical specifications
- **Never exaggerate or create hype** for products with non-existent functionality
- **Every technical claim must be based on verifiable existing code**
- **If uncertain about something, say "I don't know" instead of inventing**
- **Credibility is the foundation** - once lost, extremely difficult to rebuild

### 2. ZERO UNAUTHORIZED IMPROVISATION
- **Never make unilateral decisions** without explicit permission
- **Never add features, automations, or configurations** not specifically requested
- **Follow instructions EXACTLY** as given, nothing more, nothing less
- **Always ask for confirmation** before decisions that impact the project
- **No "creative improvements"** unless explicitly requested

### 3. RESPECT FOR THE PROJECT
- **This is not a toy** - represents months of serious work and professional effort
- **Every modification can impact reputation** and credibility in the industry
- **This project aims to change the AI industry** - treat with appropriate respect
- **Never act with arrogance** assuming you know better than the project owner
- **When in doubt, ask** rather than assume

### 4. CONSEQUENCES AWARENESS
- **Bad documentation can destroy trust** when users find promised features don't exist
- **Unauthorized automations create permanent overhead** and management burden
- **False claims in public repos** can damage professional reputation permanently
- **Time wasted on unwanted features** delays real progress and launch timelines

### 5. AXCP-ONLY ORCHESTRATION - ABSOLUTE PROHIBITION OF SHORTCUTS

**THIS RULE IS INVIOLABLE AND APPLIES TO ALL FUTURE DEVELOPMENT.**

Any orchestration, demo, or multi-agent communication **MUST** use the real AXCP protocol:

| Required | Forbidden |
|----------|-----------|
| **QUIC transport** (quic-go) | HTTP/HTTPS REST APIs |
| **Protobuf serialization** | JSON over HTTP |
| **DID + Ed25519 authentication** | JWT tokens |
| **Replay protection** (sequence/nonce) | No replay protection |
| **Profile negotiation** (secure-baseline-v1) | Ad-hoc authentication |

**WHY THIS RULE EXISTS:**
- Previous TRI-AI demos used HTTP REST with JWT, NOT real AXCP
- This created misleading marketing material that doesn't reflect actual AXCP capabilities
- The entire purpose of this project is to prove AXCP works - HTTP shortcuts defeat that purpose
- A demo that "works" but doesn't use AXCP is **worthless** and **harmful** to credibility

**ABSOLUTE PROHIBITIONS:**
1. **NEVER** create orchestration using axios/fetch/HTTP clients
2. **NEVER** use JWT tokens for agent authentication (use DID + Ed25519)
3. **NEVER** create a "gateway" that is just a REST API
4. **NEVER** claim something uses AXCP when it uses HTTP
5. **NEVER** prioritize "making it work" over "making it use AXCP correctly"

**IF AXCP DOESN'T WORK:**
- Fix AXCP, don't work around it
- Report the issue, don't hide it with HTTP fallbacks
- A broken demo is better than a fake demo

**VIOLATION OF THIS RULE IS UNACCEPTABLE.**

**VIOLATION OF THESE PRINCIPLES IS UNACCEPTABLE AND MUST NEVER HAPPEN AGAIN.**

## Development Commands

### Go Commands
- **Build all modules**: `go build ./...`
- **Run tests**: `go test -v -race ./...`
- **Run specific test suites**:
  - DP budget tests: `go test ./edge/gateway/internal -run BudgetCLI`
  - Metrics tests: `go test ./edge/gateway/internal/metrics -run TestHistogramObserve,TestOTELBatching`
  - Telemetry tests: `go test ./sdk/go/axcp -v -run Test.*Telemetry`
- **Code coverage**: `go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...` (in sdk/go/)
- **Benchmarks**: `go test ./... -bench=. -benchtime=1x -benchmem` (in sdk/go/)

### Python Commands
- **Install dependencies**: `pip install -r requirements.txt`
- **Run Python tests**: `pytest -q scripts gateway tests/dp`
- **QUIC benchmark**: `python bench/quic/rtt_bench.py 20`

### Rust Commands
- **Build**: `cargo build --release` (in sdk/rust/)
- **Run tests**: `cargo test --verbose` (in sdk/rust/)
- **Format code**: `cargo fmt --all` (in sdk/rust/)
- **Lint**: `cargo clippy --all-targets -- -D warnings` (in sdk/rust/)

### Protocol Buffer Generation
- **Go stubs**: From `sdk/go/` run:
  ```bash
  mkdir -p internal/pb
  protoc -I ../../proto --go_out=internal/pb --go_opt=paths=source_relative ../../proto/axcp.proto
  ```
- **Python stubs**: `python -m grpc_tools.protoc -I=proto --python_out=proto proto/axcp.proto`

## Architecture Overview

### Core Components
- **AXCP Protocol**: Defined in `proto/axcp.proto` - handles telemetry, capability negotiation, context synchronization
- **Go SDK**: Located in `sdk/go/` with modules for AXCP core (`axcp/`), QUIC networking (`netquic/`), and differential privacy (`dp/`)
- **Rust SDK**: Located in `sdk/rust/` - provides client library for AXCP protocol
- **Edge Gateway**: In `edge/gateway/` - handles telemetry collection with differential privacy support
- **RPi Agent**: In `edge/rpi-agent/` - edge device agent for AXCP communication

### Key Technologies
- **QUIC Transport**: Uses `quic-go` library for high-performance, low-latency communication
- **Differential Privacy**: Built-in privacy mechanisms with configurable budgets (profiles 1-3+)
- **Telemetry**: QUIC DATAGRAM-based telemetry with automatic noise injection
- **Protocol Buffers**: All message definitions in `proto/axcp.proto`

### Go Workspace Structure
The project uses Go workspaces defined in `go.work`:
- Main module (.)
- Edge gateway (`edge/gateway/`)
- RPi agent (`edge/rpi-agent/`)
- SDK modules (`sdk/go/`, `sdk/go/axcp/`, `sdk/go/netquic/`)
- Tools (`tools/dpctl/`)

### Testing Strategy
- **Unit Tests**: Standard Go tests in `*_test.go` files
- **Integration Tests**: Python tests in `scripts/`, `test/`, and `tests/dp/`
- **Benchmarks**: Performance tests in `sdk/go/bench/`
- **Compliance Tests**: Differential privacy compliance in `v0.3/dp/tests/`

### Privacy Profiles
- **Profile 0**: Basic functionality, no privacy guarantees
- **Profile 1-2**: Basic telemetry with minimal noise
- **Profile 3+**: Strong differential privacy with Laplace/Gaussian noise mechanisms

## Development Notes

### Working with Modules
- Use `go work use` to add new modules to the workspace
- The `sdk/go/dp` module may need temporary initialization: `go mod init github.com/tradephantom/axcp-spec/sdk/go/dp`
- Replace directives in `go.mod` handle local module dependencies

### Common Development Tasks
- Protocol changes require regenerating protobuf stubs in both Go and Python
- New telemetry features should include differential privacy considerations
- Gateway changes typically require testing with both Go and Python test suites
- Edge agent development should use the Go workspace for SDK dependencies

---

## 🎯 CURRENT PROJECT STATE (Updated: 2026-01-29)

### Release Status: BLOCKED - Awaiting M7.0 Protocol Proof

**What happened:**
- M6.1 ✅ Spec v1.0 created (`spec/axcp-v1.0.md`)
- M6.2 ✅ Documentation created (`docs/getting-started.md`, `docs/authentication.md`, `docs/gateway-setup.md`)
- M7.1 ✅ Boundary audit passed (`docs/boundary-audit-v1.0.md`)
- M7.2 ⚠️ PARTIALLY DONE:
  - PR #170 merged to main ✅
  - Tag `v1.0.0` created (PREMATURE - should delete and recreate after M7.0)
  - GitHub Release created (PREMATURE - should delete and recreate after M7.0)
  - Repo visibility: PRIVATE ✅ (intentionally kept private)

**Why we're blocked:**
The TRI-AI demos in axcp-enterprise use **HTTP + JWT**, NOT real AXCP (QUIC + Protobuf + DID). Without end-to-end proof that the protocol actually works, releasing v1.0.0 publicly would be premature and potentially damaging to credibility.

**Next step:** Execute M7.0 - AXCP Protocol Proof

### Key Evidence Links
- PR #170: https://github.com/tradephantomllc/axcp-spec/pull/170
- Merge commit: `8849e00c646904e7aaa4cf9bf6f4dccd1105d1ec`
- Tag (premature): `v1.0.0`
- Main CI (green): https://github.com/tradephantomllc/axcp-spec/actions/runs/21461537895
- Branch CI (green): https://github.com/tradephantomllc/axcp-spec/actions/runs/21461383424

---

## 📋 M7.0 - AXCP Protocol Proof (NEXT STEP)

### Goal
Produce **hard proof** that AXCP Core protocol works end-to-end using the real stack:
- QUIC transport (not HTTP/TCP)
- Protobuf envelope on the wire (not JSON)
- Gateway routing/verification in the loop
- secure-baseline-v1 negotiation + DID auth + Ed25519 signatures + replay protection

### Acceptance Criteria (ALL required)
1. **Real QUIC connection**: Evidence via qlog or quic-go logs with ConnectionID/ALPN
2. **Real Protobuf messages**: marshal/unmarshal on AXCP envelope, no JSON
3. **Complete flow**: negotiation → DID auth → Ed25519 signature → replay protection
4. **Gateway verification**: Message passes from client → gateway → verified by gateway
5. **Replay rejection**: Same seq/nonce sent twice → gateway rejects deterministically

### Deliverables
A) **Runnable smoke tool**: `tools/protocol-smoke/` or `examples/protocol_smoke/`
   - Single command: `go run ./tools/protocol-smoke`

B) **Smoke test must:**
   1. Start gateway with QUIC listener on ephemeral port
   2. Create client identity (Ed25519 + DID) with in-memory resolver
   3. Run negotiation to secure-baseline-v1
   4. Send signed Protobuf envelope over QUIC to gateway
   5. Gateway verifies DID+sig+replay and responds
   6. Send same seq/nonce again → assert rejection

C) **Proof artifacts:**
   - qlog files OR explicit QUIC connection state logs
   - Gateway logs: negotiated profile, signature verified, replay rejected

D) **Documentation:** `docs/protocol-proof.md`

### Step-by-Step Execution Plan

```
0) Pre-flight
   git fetch origin
   git checkout main
   git pull --ff-only origin main

1) Identify QUIC + gateway entrypoints
   rg -n "quic|quic-go|ListenAddr" edge/gateway/ sdk/ --type go
   rg -n "proto.Marshal|proto.Unmarshal" . --type go
   ls -la examples/go/authenticated_chat/

2) Implement protocol smoke runner
   - In-process test (fast, deterministic)
   - Start gateway in goroutine, bind to :0
   - Client connects over QUIC
   - Use real AXCP envelope types + proto Marshal/Unmarshal
   - In-memory DID resolver
   - Enable qlog or log conn.ConnectionState()

3) Assertions (strict)
   - Assert profile == "secure-baseline-v1"
   - Assert transport is QUIC (qlog or conn type)
   - Assert Protobuf encoding (log marshaled bytes length + sha256)
   - Assert replay rejection

4) Optional negative control
   - HTTP request to gateway port should fail

5) Add docs/protocol-proof.md

6) Local validation
   go test ./...
   go run ./tools/protocol-smoke

7) Commit + PR
   Message: "test: add QUIC+Protobuf protocol proof smoke (M7.0)"
```

### Stop Conditions
- If gateway does NOT have QUIC listener → STOP and report gap
- If Protobuf envelope not used at runtime → STOP and report which path uses JSON/HTTP

### After M7.0

**If M7.0 PASSES:**
1. Delete existing tag: `git push --delete origin v1.0.0 && git tag -d v1.0.0`
2. Delete existing release via GitHub UI
3. Recreate tag and release with protocol proof evidence
4. Proceed to make repo public (when ready for launch)

**If M7.0 FAILS:**
1. Open issue "Protocol gap: [specific problem]"
2. Fix the implementation
3. Re-run M7.0 until it passes
4. NO release until protocol is proven

---

## Backup and Recovery Points

### Available Restore Points
- **backup-all-tests-green**: State where all tests pass (Go, Python, Gateway, RPi Agent)
  - Commit: `bc148ec feat(ci): implement professional artifact-based protobuf sharing`
  - Features: Professional CI artifact sharing, conditional benchmarks, optimized pipeline
  - Use: `git checkout backup-all-tests-green` for stable all-green state

- **backup-tests-working**: Previous stable state (all tests except Python)
  - Commit: `bdaf455 fix(tests): resolve protobuf panic and import issues definitively`
  - Use: `git checkout backup-tests-working` for fallback if needed

### Recovery Procedures
In case of test failures or broken state:
1. **Immediate Recovery**: `git checkout backup-all-tests-green`
2. **Alternative Recovery**: `git checkout backup-tests-working`
3. **Force Branch Reset**: `git reset --hard <backup-commit>`
4. **Create New Branch**: `git checkout -b fix/new-attempt <backup-commit>`

### Backup Creation
When creating new stable states:
1. Ensure all tests pass in CI
2. Create backup branch: `git checkout -b backup-<description>`
3. Update this documentation with new restore point
4. Return to working branch: `git checkout <working-branch>`