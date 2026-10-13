# SDK Release Readiness

This document defines the pre-release contract for public AXCP SDK packages.

## Release Scope

The public package set currently covers:

- Go SDK: reference implementation in `sdk/go`
- Python SDK: Secure Baseline core plus QUIC transport in `sdk/python`
- TypeScript SDK: Secure Baseline core plus stream/TLS transport in `sdk/typescript`
- Rust SDK: experimental telemetry adapter, not a canonical transport SDK

## Cross-SDK Parity

Go, Python, and TypeScript must all pass the shared Secure Baseline vector in
`testdata/sdk/secure_baseline_vector.json`. The additional
`testdata/sdk/repeated_defaults_vector.json` covers repeated empty-string presence
and canonical wire/signature parity, including the TypeScript encoder.

The vector locks:

- deterministic Ed25519 seed and `did:key`
- context patch payload
- canonical signing payload bytes, including the replay `sequence`
- DID auth transcript bytes
- detached signature bytes
- full encoded envelope bytes

Any change to canonical serialization or transcript construction must update the
fixture intentionally and should be reviewed as a protocol compatibility change.

## Local Gates

Run these before any public package publication:

```bash
python scripts/verify_sdk_release_readiness.py
PYTHONPATH=$PWD python -m pytest -q scripts gateway sdk/python/tests
(cd sdk/typescript && npm ci && npm test && npm run typecheck && npm pack --dry-run)
go test -race ./... ./sdk/go/... ./edge/gateway/... ./edge/rpi-agent/... ./test/...
(cd sdk/rust && cargo fmt --all --check && cargo test --locked --all-features && cargo clippy --locked --all-targets --all-features -- -D warnings)
git diff --check
```

## Reproducible protobuf generation

CI and local regeneration use `protoc 33.2` for both languages and
`protoc-gen-go v1.36.11` for Go. Do not regenerate Python with an unpinned
`grpcio-tools` version: its bundled compiler can change the generated runtime
requirements and fail the committed-file check. Python packages require
`protobuf>=6.33.5`, which accepts the generated 6.33.2 code.

```bash
protoc -I=proto --python_out=proto proto/axcp.proto
protoc -I=proto --python_out=sdk/python/src/axcp/pb proto/axcp.proto
protoc -I=proto --go_out=sdk/go/internal/pb --go_opt=paths=source_relative proto/axcp.proto
```

## Publication Notes

- Do not publish packages directly from an unreviewed branch.
- Publish only from a signed/tagged release commit after CI is green.
- Keep Python and TypeScript versions aligned while both packages are pre-1.0.
- Keep QUIC for TypeScript as an optional adapter until the runtime dependency
  is stable enough for public SDK users.

CI includes external consumer checks for Go (`GOWORK=off`), a Python wheel and a
TypeScript tarball. The readiness script verifies that all four SDK directories
carry the repository's exact LICENSE/NOTICE copies and that Python/TypeScript
package metadata includes them. SDK compatibility jobs cover Python 3.10 with
the minimum declared aioquic/protobuf versions and Python 3.14.

## Installation and deployment gates

- Build an external consumer with `GOWORK=off` using only the parent SDK module, importing both `axcp` and `netquic`. Repeat against the actual published revision before claiming registry availability.
- Build/install Python wheels and the TypeScript npm tarball outside the checkout; a local import is not a package-installation test.
- Build the gateway Dockerfile from repository root and validate the compose configuration with explicit TLS/trust settings.
- Run dependency advisories against the resolved Go, npm, Python and Rust graphs. Keep Go's patched supported baseline, CI and Docker toolchain aligned.
- Run the documented authenticated-chat example and confirm valid exchange plus duplicate rejection. The HTTP Rust adapter and Pi collector are not gateway clients.

These gates do not replace deployment-specific load, recovery or durable-delivery validation. A local passing suite does not publish a release or rewrite existing Git history.
