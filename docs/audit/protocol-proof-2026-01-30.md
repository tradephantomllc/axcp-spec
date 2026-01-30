# AXCP Protocol Proof - 2026-01-30

**Result**: PASS

## Commit SHA

```
612ed0aabeb0bc2502813a697ab33e3094eabac2
```

## Commands Executed

```bash
go run ./examples/go/authenticated_chat -server
go run ./examples/go/authenticated_chat
go run ./examples/go/authenticated_chat -replay
go test -v ./sdk/go/auth/...
go test -v ./sdk/go/negotiate/...
go test -v ./edge/gateway/internal/...
```

## Artifacts

```
artifacts/protocol-proof/20260130-220816/
├── server.log
├── client.log
└── replay_test.log
```

## Hard Evidence

### QUIC Transport
```
[QUIC]   ALPN:        axcp-auth-chat
[QUIC]   TLS Version: 0x0304
[QUIC]   Cipher Suite: 0x1301
```

### Protobuf Serialization
```
[PROTOBUF] proto.Unmarshal (received): bytes=260, sha256=4d0dd44e8807d21f272df79523fa5ddb5c01e7682c8cad9cb44284cd589be5fe
[PROTOBUF] proto.Marshal (response): bytes=263, sha256=b32e368e48012be3bff9dfbd85f51e3ae30489281b82b6adebcd0bbe4deba9d7
```

### DID + Ed25519 Authentication
```
Signature VALID
```

### Replay Protection
```
[REPLAY] REJECTED - Sequence 1 already seen (REPLAY ATTACK BLOCKED)
```

---

*M7.0 Release Gate: PASS*
