# AXCP Protocol Proof - 2026-07-05

**Milestone**: M8.0
**Result**: **PASS**

## Commit SHA

```text
8275a501b3b627f7c9273ebd4bbfbf65c811eece
```

## Commands Executed

```bash
go test ./edge/gateway/internal ./sdk/go/auth ./sdk/go/axcp
go test ./edge/gateway/internal/... ./sdk/go/...
go test ./...
go run ./examples/go/authenticated_chat -replay
PYTHONPATH=$PWD /tmp/axcp-m8-python-venv-20260705-210803/bin/python -m pytest -q scripts gateway sdk/python/tests
cd sdk/typescript && npm test
cd sdk/typescript && npm run typecheck
cd sdk/typescript && npm pack --dry-run
```

Authenticated chat E2E was executed with one server process and one client process under `examples/go/authenticated_chat`.

## Artifacts

```text
artifacts/protocol-proof/20260705-210803/
```

## Hard Evidence

### QUIC Transport

```text
[QUIC]   ALPN:        axcp-auth-chat
[QUIC]   TLS Version: 0x0304
[QUIC]   Cipher Suite: 0x1301
```

### Protobuf Serialization

```text
[PROTOBUF] proto.Marshal (request): bytes=260, sha256=a82b6084d2388bc3bcffebb47a89196922024f71c4b6ce7445b30dbdaf566312
[PROTOBUF] proto.Unmarshal (received): bytes=260, sha256=a82b6084d2388bc3bcffebb47a89196922024f71c4b6ce7445b30dbdaf566312
```

### DID + Ed25519 Authentication

```text
Signature VALID
Server Signature VALID
```

### Replay Protection

```text
[REPLAY] REJECTED - Sequence 1 already seen (REPLAY ATTACK BLOCKED)
```

### SDK Readiness

```text
release-readiness: ok
42 passed
tests 31
pass 31
```

---

*M8.0 Release Gate: PASS*
