# AXCP M8.0 Protocol Proof Report

**Result**: **PASS**
**Commit**: `8275a501b3b627f7c9273ebd4bbfbf65c811eece`
**Generated**: `2026-07-05T21:08:13Z`

## Errata (2026-10-15)

- `go_targeted_tests.log` and `go_core_packages.log` report cached results; `go_root_module.log` ran no tests.
- The authenticated-chat run did not pass through the reference gateway, and the replay run uses a map inside the example program, not the SDK replay protector.
- The commit identifier is recorded for provenance only and is not guaranteed to be retrievable from the current `main` branch.
- See the [M8 proof pack errata](../../../docs/protocol-proof/M8_PROOF_PACK.md#errata-2026-10-15).

This artifact directory contains the raw command logs used by
`docs/protocol-proof/M8_PROOF_PACK.md`.

## Evidence Logs

- `environment.log`
- `authenticated_chat_server.log`
- `authenticated_chat_client.log`
- `authenticated_chat_replay.log`
- `go_targeted_tests.log`
- `go_core_packages.log`
- `go_root_module.log`
- `python_venv_install.log`
- `sdk_release_readiness.log`
- `python_tests.log`
- `typescript_tests.log`
- `typescript_typecheck.log`
- `typescript_pack_dry_run.log`

## Summary

| Surface | Result |
|---------|--------|
| QUIC authenticated chat E2E | PASS |
| Protobuf request/response hashes | PASS |
| DID + Ed25519 client/server signatures | PASS |
| Replay rejection | PASS |
| Go tests | PASS |
| Python SDK tests and release readiness | PASS |
| TypeScript SDK tests, typecheck, package dry-run | PASS |
