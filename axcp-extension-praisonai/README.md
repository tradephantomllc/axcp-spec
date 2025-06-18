# AXCP ↔ PraisonAI Bridge (Skeleton)

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
