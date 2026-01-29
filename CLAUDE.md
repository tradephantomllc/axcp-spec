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