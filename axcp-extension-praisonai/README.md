# AXCP ↔ PraisonAI Bridge (alpha)

> **Status:** *experimental skeleton* – interface definitions & CI scaffold only.
> Production logic will land in `v0.2.x`.

---

## Why this bridge?

PraisonAI introduces a high-performance inference runtime focused on **privacy-preserving, on-device AI**.
AXCP (Adaptive eXchange Context Protocol) already ships:

* QUIC + Protobuf transport for ultra-low latency
* Context-sync and differential privacy layers
* DIDComm v2 secure identity / capability negotiation

The bridge glues the two worlds:

| Direction | Purpose |
|-----------|---------|
| **AXCP → PraisonAI** | Forward *AxcpEnvelope* commands to a local PraisonAI runtime (inference, embeddings, etc.) |
| **PraisonAI → AXCP** | Stream telemetry & inference results back into the AXCP mesh |

---

## Repository layout

```text
axcp-extension-praisonai/
│
├── proto/axcp.proto          # AXCP definitions (synced via submodule)
│
├── rust/bridge/              # Minimal Rust crate wrapping AxcpEnvelope
│   ├── Cargo.toml
│   ├── build.rs              # prost-build codegen
│   └── src/lib.rs
│
├── py/praisonbridge/         # Equivalent Python wrapper
│   ├── __init__.py
│   └── bridge.py
│
├── examples/
│   └── praison_echo.py       # Tiny round-trip demo
│
├── tests/                    # Cross-language round-trip tests
│   ├── test_roundtrip.rs
│   └── test_roundtrip.py
│
└── .github/workflows/ci.yml  # Lint + tests (Rust / Python)
```

---

## Quick-start

### Rust

```bash
cd rust/bridge
cargo test  # passes unit + round-trip tests
```

### Python (from repo root)

```bash
python -m pip install -r requirements-dev.txt
pytest       # runs Python unit + round-trip tests
```

Need to tweak the protobuf? Regenerate Python stubs with:

```bash
python -m grpc_tools.protoc -I proto \
  --python_out=py/praisonbridge \
  proto/axcp.proto
```

---

## CI

| Stage         | Tooling                        |
| ------------- | ------------------------------ |
| **Lint (Py)** | `ruff` (PEP-8 + best-practice) |
| **Test (Py)** | `pytest`                       |
| **Lint (Rs)** | `clippy --deny warnings`       |
| **Test (Rs)** | `cargo test`                   |

Runs on Ubuntu & macOS via GitHub Actions.

---

## Roadmap `v0.2`

* 🎯 **gRPC-QUIC** adapters – avoid extra TCP hops
* 🔒 SGX enclave stubs for confidential on-device inference
* 🔄 Telemetry batching with OTLP
* 📦 Publish `praisonbridge` to PyPI & crate to crates.io

Contributions welcome — please sign the CLA and open a feature request!

---

© 2025 TradePhantom LLC • Licensed under **BSL 1.1** (converts to Apache-2.0 on 2029-01-01)


This repository contains a **skeleton implementation** of an extension bridge to send or receive `AxcpEnvelope` protobuf messages over QUIC channels between the **Adaptive eXchange Context Protocol (AXCP)** ecosystem and **PraisonAI** components.

> ⚠️  No production logic is included yet – only minimal wrappers, tests and CI scaffold (Issue #35).

## Layout

```
proto/                # Protobuf definitions
  axcp.proto          # Placeholder – copy or submodule from axcp-spec
rust/bridge/          # Minimal Rust crate wrapping AxcpEnvelope
  Cargo.toml
  build.rs            # Generates Rust types with prost-build
  src/
    lib.rs            # wrap/unwrap helpers
  tests/
    test_roundtrip.rs # Rust round-trip unit test
py/                   # Python wrapper package
  praibridge/
    __init__.py
    bridge.py         # wrap/unwrap helpers
examples/
  praison_echo.py     # Simple echo demo
.tests/
  test_roundtrip.py   # Python round-trip test
  test_rust_cli.rs    # Extra Rust test (cargo)
.github/workflows/ci.yml  # CI: lint + test (Python / Rust)
```

## Quick start

```bash
# Rust
cd rust/bridge
cargo test                # runs Rust unit tests

# Python (from repo root)
python -m pip install -r requirements-dev.txt  # optional
pytest                    # runs Python tests
```

Generating Python protobuf stubs (optional):

```bash
python -m grpc_tools.protoc -I proto --python_out=. proto/axcp.proto
```

## CI

The GitHub Actions workflow runs on Ubuntu and macOS:

* Lint Python with `ruff`
* Run Python tests via `pytest`
* Lint Rust with `clippy -D warnings`
* Run Rust tests via `cargo test`

---

Initial bootstrap for Issue #35 – *feat(praison-bridge): bootstrap repo & CI scaffold*.
