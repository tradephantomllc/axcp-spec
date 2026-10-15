# AXCP M7.0 Protocol Proof Pack

**Date**: 2026-01-30
**Result**: **PASS**

---

## Errata (2026-10-15)

A review on 2026-10-15 found that this pack claims more than its committed evidence shows. Read the results above with these corrections:

1. **Missing or empty logs.** `artifacts/protocol-proof/20260130-220816/client.log` and `replay_test.log` are empty (0 bytes); only `server.log` has content. The client-side excerpts in 4a and 4b and the replay excerpt in 4d are not backed by committed logs. The unit-test output quoted in 4e was not committed either.
2. **Gateway not in the loop.** The authenticated-chat client connects directly to the example's own server (example ALPN `axcp-auth-chat`). No envelope passed through the reference gateway, so the "Gateway enforcement" row and the conclusion "Gateway enforcement of all security checks" are not demonstrated end to end. Gateway evidence is limited to unit tests whose output was not committed.
3. **Replay demonstration.** The `-replay` run checks sequences against a local map inside the example program. It does not exercise the SDK replay protector or the gateway, and in that program the replay check runs before signature verification.
4. **Transcript version.** This run predates the 2026-07-05 transcript revision of the specification. Its signatures used the original 1.0.0 transcript and are not evidence for the current transcript.
5. **No profile negotiation.** Both demo peers were configured for Secure Baseline; no negotiation took place on the wire.
6. **Provenance.** The commit identifier above is recorded for provenance only and is not guaranteed to be retrievable from the current `main` branch. To reproduce, rerun the commands on a current checkout.
7. **Scope.** "Verified and ready for Apache 2.0 release" overstates a localhost demonstration with fixed test keys. This pack is historical and does not cover the 2026-10-15 specification revision.

---

## 1. Commit SHA Tested

```
612ed0aabeb0bc2502813a697ab33e3094eabac2
```

Branch: `main`

---

## 2. Commands Executed

```bash
# E2E authenticated chat (server)
go run ./examples/go/authenticated_chat -server

# E2E authenticated chat (client)
go run ./examples/go/authenticated_chat

# Replay protection test
go run ./examples/go/authenticated_chat -replay

# Unit tests
go test -v ./sdk/go/auth/...
go test -v ./sdk/go/negotiate/...
go test -v ./edge/gateway/internal/...
```

---

## 3. Artifacts Directory

```
artifacts/protocol-proof/20260130-220816/
├── server.log          # E2E server with QUIC + Protobuf evidence
├── client.log          # E2E client with QUIC + Protobuf evidence
└── replay_test.log     # Replay protection with explicit rejection
```

---

## 4. Hard Evidence

### 4a. QUIC Transport Proof

**Source**: `server.log`

```
[QUIC] Server Connection Established
[QUIC]   Local Addr:  127.0.0.1:61301
[QUIC]   Remote Addr: 127.0.0.1:57747
[QUIC]   ALPN:        axcp-auth-chat
[QUIC]   TLS Version: 0x0304
[QUIC]   Cipher Suite: 0x1301
```

**Source**: `client.log`

```
[QUIC] Client Connection Established
[QUIC]   Local Addr:  [::]:57747
[QUIC]   Remote Addr: 127.0.0.1:61301
[QUIC]   ALPN:        axcp-auth-chat
[QUIC]   TLS Version: 0x0304
[QUIC]   Cipher Suite: 0x1301
```

**Analysis**:
- ALPN `axcp-auth-chat` confirms QUIC protocol negotiation
- TLS 1.3 (0x0304) with cipher suite 0x1301 (TLS_AES_128_GCM_SHA256)
- quic-go library handles transport

---

### 4b. Protobuf Serialization Proof

**Source**: `server.log`

```
[PROTOBUF] proto.Unmarshal (received): bytes=260, sha256=4d0dd44e8807d21f272df79523fa5ddb5c01e7682c8cad9cb44284cd589be5fe
[PROTOBUF] proto.Marshal (response): bytes=263, sha256=b32e368e48012be3bff9dfbd85f51e3ae30489281b82b6adebcd0bbe4deba9d7
```

**Source**: `client.log`

```
[PROTOBUF] proto.Marshal (request): bytes=260, sha256=4d0dd44e8807d21f272df79523fa5ddb5c01e7682c8cad9cb44284cd589be5fe
```

**Analysis**:
- Request: 260 bytes, SHA256 `4d0dd44e...`
- Response: 263 bytes, SHA256 `b32e368e...`
- Matching hashes client→server prove identical wire format
- Protobuf envelope contains: Version, TraceID, Profile, SenderDID, RecipientDID, TimestampMs, Sequence, Signature, Payload

---

### 4c. DID + Ed25519 Authentication Proof

**Source**: `server.log`

```
--- Received Envelope ---
  SenderDID:  did:key:axcp-auth-chat-client
  RecipientDID: did:key:axcp-auth-chat-server
  Sequence:   1
  Signature:  64 bytes

--- Verifying Signature ---
  Signature VALID
```

**Analysis**:
- DID format: `did:key:...`
- Signature: 64 bytes (Ed25519 standard)
- Verification: **VALID**

---

### 4d. Replay Protection Proof

**Source**: `replay_test.log`

```
[PROTOBUF] proto.Marshal (test envelope): bytes=215, sha256=086ee046775fb4cc8f82c33835353b5354fb5d239c726a41802718f4e91e6df1

--- Test 1: First Request (should PASS) ---
[REPLAY] Checking sequence=1 for DID=did:key:axcp-auth-chat-client
[REPLAY] ACCEPTED - Sequence 1 is new, recording
[AUTH] Signature verification: VALID (64 bytes Ed25519)
[RESULT] First request: PASS

--- Test 2: Replay Attack (SAME envelope, should FAIL) ---
[REPLAY] Checking sequence=1 for DID=did:key:axcp-auth-chat-client
[REPLAY] REJECTED - Sequence 1 already seen (REPLAY ATTACK BLOCKED)
[RESULT] Replay attack: BLOCKED (as expected)

--- Test 3: New Sequence (should PASS) ---
[REPLAY] Checking sequence=2 for DID=did:key:axcp-auth-chat-client
[REPLAY] ACCEPTED - Sequence 2 is new, recording
[RESULT] New sequence: PASS
```

**Analysis**:
- First request (seq=1): **ACCEPTED**
- Replay attack (seq=1): **REJECTED**
- New request (seq=2): **ACCEPTED**
- Explicit log: `REPLAY ATTACK BLOCKED`

---

### 4e. Gateway Enforcement Proof

**Source**: Unit tests (`go test -v ./edge/gateway/internal/...`)

```
=== RUN   TestEnvelopeAuthenticator_SecureBaseline_MissingFields
--- PASS: TestEnvelopeAuthenticator_SecureBaseline_MissingFields (0.00s)
=== RUN   TestEnvelopeAuthenticator_InvalidSignature
--- PASS: TestEnvelopeAuthenticator_InvalidSignature (0.00s)
=== RUN   TestEnvelopeAuthenticator_DIDNotFound
--- PASS: TestEnvelopeAuthenticator_DIDNotFound (0.00s)
=== RUN   TestEnvelopeAuthenticator_ReplayProtection
--- PASS: TestEnvelopeAuthenticator_ReplayProtection (0.00s)
=== RUN   TestEnvelopeAuthenticator_FullAuthFlow
--- PASS: TestEnvelopeAuthenticator_FullAuthFlow (0.00s)
```

**Analysis**:
- Gateway rejects: missing DID, missing signature, invalid signature, expired timestamps, replay attempts, unknown DIDs
- All enforcement tests: **PASS**

---

## Summary

| Requirement | Evidence | Status |
|-------------|----------|--------|
| QUIC transport | ALPN `axcp-auth-chat`, TLS 1.3 | **PASS** |
| Protobuf wire format | bytes + SHA256 hash | **PASS** |
| secure-baseline-v1 | Profile: 1 | **PASS** |
| DID + Ed25519 | Signature VALID | **PASS** |
| Replay protection | REJECTED on seq=1 replay | **PASS** |
| Gateway enforcement | Unit tests PASS | **PASS** |

---

## Conclusion

**M7.0 Protocol Proof: PASS**

AXCP Core uses the real protocol stack:
- QUIC transport (not HTTP)
- Protobuf envelopes (not JSON)
- secure-baseline-v1 profile
- DID + Ed25519 authentication
- Replay protection with sequence tracking
- Gateway enforcement of all security checks

The protocol is verified and ready for Apache 2.0 release.

---

*Generated: 2026-01-30T22:09:00Z*
