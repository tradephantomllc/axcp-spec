# AXCP M8.0 Protocol Proof Pack

**Date**: 2026-07-05
**Result**: **PASS**

---

## 1. Commit SHA Tested

```text
8275a501b3b627f7c9273ebd4bbfbf65c811eece
```

Branch used for proof generation: `codex/generate-m8-proof-pack`

The tested commit includes the post-paper alignment fixes through PR #259:

- canonical signing payload alignment across Go, Python, and TypeScript
- non-mutating Go auth preflight validation
- edge replay marking after successful Ed25519 verification
- authenticated chat E2E stream lifecycle repair

---

## 2. Artifact Directory

```text
artifacts/protocol-proof/20260705-210803/
├── M8_VALIDATION_CHECKLIST.md
├── PROTOCOL_PROOF_REPORT.md
├── authenticated_chat_client.log
├── authenticated_chat_replay.log
├── authenticated_chat_server.log
├── environment.log
├── go_core_packages.log
├── go_root_module.log
├── go_targeted_tests.log
├── python_tests.log
├── python_venv_install.log
├── sdk_release_readiness.log
├── typescript_pack_dry_run.log
├── typescript_tests.log
└── typescript_typecheck.log
```

---

## 3. Commands Executed

```bash
go test ./edge/gateway/internal ./sdk/go/auth ./sdk/go/axcp
go test ./edge/gateway/internal/... ./sdk/go/...
go test ./...

# Authenticated chat E2E
cd examples/go/authenticated_chat && go run . -server
cd examples/go/authenticated_chat && go run .

# Replay proof
go run ./examples/go/authenticated_chat -replay

# Python SDK/release proof
/opt/homebrew/bin/python3 -m venv /tmp/axcp-m8-python-venv-20260705-210803
/tmp/axcp-m8-python-venv-20260705-210803/bin/python -m pip install -r requirements.txt -e sdk/python aioquic pytest-cov
/tmp/axcp-m8-python-venv-20260705-210803/bin/python scripts/verify_sdk_release_readiness.py
PYTHONPATH=$PWD /tmp/axcp-m8-python-venv-20260705-210803/bin/python -m pytest -q scripts gateway sdk/python/tests

# TypeScript SDK proof
cd sdk/typescript && npm test
cd sdk/typescript && npm run typecheck
cd sdk/typescript && npm pack --dry-run
```

---

## 4. Hard Evidence

### 4a. Environment

Source: `environment.log`

```text
go=go version go1.25.6 darwin/arm64
python=Python 3.14.2
node=v25.9.0
npm=11.12.1
rustc=rustc 1.93.1 (01f6ddf75 2026-02-11)
cargo=cargo 1.93.1 (083ac5135 2025-12-15)
```

### 4b. QUIC Transport Proof

Source: `authenticated_chat_server.log`

```text
[QUIC] Server Connection Established
[QUIC]   ALPN:        axcp-auth-chat
[QUIC]   TLS Version: 0x0304
[QUIC]   Cipher Suite: 0x1301
```

Source: `authenticated_chat_client.log`

```text
[QUIC] Client Connection Established
[QUIC]   ALPN:        axcp-auth-chat
[QUIC]   TLS Version: 0x0304
[QUIC]   Cipher Suite: 0x1301
```

### 4c. Protobuf Wire Proof

Source: `authenticated_chat_client.log` and `authenticated_chat_server.log`

```text
[PROTOBUF] proto.Marshal (request): bytes=260, sha256=a82b6084d2388bc3bcffebb47a89196922024f71c4b6ce7445b30dbdaf566312
[PROTOBUF] proto.Unmarshal (received): bytes=260, sha256=a82b6084d2388bc3bcffebb47a89196922024f71c4b6ce7445b30dbdaf566312
```

The request hash matches client-to-server exactly. The response is also serialized and received as Protobuf:

```text
[PROTOBUF] proto.Marshal (response): bytes=263, sha256=885313b989e8b341ff6bf2d4491c553de12992278d883833099d6ea0df6f9d6b
[PROTOBUF] proto.Unmarshal (response): bytes=263, sha256=885313b989e8b341ff6bf2d4491c553de12992278d883833099d6ea0df6f9d6b
```

### 4d. DID + Ed25519 Authentication Proof

Source: `authenticated_chat_server.log`

```text
SenderDID:  did:key:axcp-auth-chat-client
RecipientDID: did:key:axcp-auth-chat-server
Sequence:   1
Signature:  64 bytes
Signature VALID
```

Source: `authenticated_chat_client.log`

```text
SenderDID:  did:key:axcp-auth-chat-server
Sequence:   1
Server Signature VALID
Bidirectional authenticated exchange completed!
```

### 4e. Replay Protection Proof

Source: `authenticated_chat_replay.log`

```text
[REPLAY] ACCEPTED - Sequence 1 is new, recording
[RESULT] First request: PASS
[REPLAY] REJECTED - Sequence 1 already seen (REPLAY ATTACK BLOCKED)
[RESULT] Replay attack: BLOCKED (as expected)
[REPLAY] ACCEPTED - Sequence 2 is new, recording
[RESULT] New sequence: PASS
```

### 4f. Go Core Proof

Source: `go_targeted_tests.log`

```text
ok  	github.com/tradephantomllc/axcp-spec/edge/gateway/internal
ok  	github.com/tradephantomllc/axcp-spec/sdk/go/auth
ok  	github.com/tradephantomllc/axcp-spec/sdk/go/axcp
exit_code=0
```

Source: `go_root_module.log`

```text
$ go test ./...
exit_code=0
```

### 4g. Python SDK Proof

Source: `sdk_release_readiness.log`

```text
release-readiness: ok
exit_code=0
```

Source: `python_tests.log`

```text
42 passed
exit_code=0
```

### 4h. TypeScript SDK Proof

Source: `typescript_tests.log`

```text
tests 31
pass 31
fail 0
exit_code=0
```

Source: `typescript_typecheck.log`

```text
tsc -p tsconfig.json --noEmit
exit_code=0
```

Source: `typescript_pack_dry_run.log`

```text
name: @tradephantom/axcp
version: 0.1.0
total files: 53
exit_code=0
```

---

## 5. Summary

| Requirement | Evidence | Status |
|-------------|----------|--------|
| QUIC transport | ALPN `axcp-auth-chat`, TLS 1.3 | **PASS** |
| Protobuf wire format | request/response byte counts and SHA256 hashes | **PASS** |
| secure-baseline-v1 | authenticated chat profile 1 | **PASS** |
| DID + Ed25519 | client and server signatures valid | **PASS** |
| Replay protection | replayed sequence rejected after valid first use | **PASS** |
| Go SDK/gateway | targeted, Core subset, and root tests | **PASS** |
| Python SDK | release-readiness and 42 tests | **PASS** |
| TypeScript SDK | 31 tests, typecheck, package dry-run | **PASS** |

---

## Conclusion

**M8.0 Protocol Proof: PASS**

AXCP Core is validated on the current paper-ready implementation with:

- QUIC transport
- Protobuf envelopes
- secure-baseline-v1 profile
- DID + Ed25519 message authentication
- replay protection with sequence tracking
- non-mutating SDK validation preflight
- post-signature replay marking in the edge gateway
- Go, Python, and TypeScript SDK proof coverage

M8 supersedes the historical M7 proof pack as the current protocol evidence baseline.
