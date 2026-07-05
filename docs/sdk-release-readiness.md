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
`testdata/sdk/secure_baseline_vector.json`.

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
(cd sdk/go && go test ./...)
(cd edge/gateway && go test ./...)
(cd edge/rpi-agent && go test ./...)
(cd sdk/rust && cargo fmt --all --check && cargo test --quiet)
git diff --check
```

## Publication Notes

- Do not publish packages directly from an unreviewed branch.
- Publish only from a signed/tagged release commit after CI is green.
- Keep Python and TypeScript versions aligned while both packages are pre-1.0.
- Keep QUIC for TypeScript as an optional adapter until the runtime dependency
  is stable enough for public SDK users.
