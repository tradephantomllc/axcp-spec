# AXCP Protocol Proof Report - M7.0

**Date**: 2026-01-30 21:53 UTC
**Commit SHA**: `612ed0aabeb0bc2502813a697ab33e3094eabac2`
**Branch**: main
**Go Version**: 1.23.12

---

## OVERALL RESULT: **PASS**

---

## 1. QUIC Transport Proof

**Evidence**: `authenticated_chat_server.log`

```
2026/01/30 21:54:44 Listening on localhost:61301
2026/01/30 21:54:58 Client connected
```

**Analysis**:
- Server uses `quic.ListenAddr()` from quic-go library
- Client uses `quic.DialAddr()` for connection
- ALPN protocol: `axcp-auth-chat`

**Verdict**: **PASS** - QUIC transport is active and functioning.

---

## 2. Protobuf Serialization Proof

**Evidence**: `protobuf_roundtrip_test.log`

```
=== RUN   TestRoundTrip
--- PASS: TestRoundTrip (0.00s)
PASS
```

**Evidence from E2E**: `authenticated_chat_server.log`

```
--- Received Envelope ---
  Version:    1
  TraceID:    auth-chat-001
  Profile:    1 (secure-baseline-v1)
  SenderDID:  did:key:axcp-auth-chat-client
  RecipientDID: did:key:axcp-auth-chat-server
  TimestampMs: 1769806498439
  Sequence:   1
  Signature:  64 bytes
```

**Analysis**:
- `proto.Marshal()` / `proto.Unmarshal()` used for `pb.AxcpEnvelope`
- All Protobuf fields correctly serialized/deserialized
- Envelope contains: Version, TraceID, Profile, SenderDID, RecipientDID, TimestampMs, Sequence, Signature, Payload

**Verdict**: **PASS** - Protobuf is the wire format.

---

## 3. Secure Baseline Profile Negotiation Proof

**Evidence**: `negotiate_profile_tests.log`

```
=== RUN   TestNegotiate_Success_SecureBaseline
--- PASS: TestNegotiate_Success_SecureBaseline (0.00s)
=== RUN   TestNegotiate_BothSupportBoth_SelectsSecureFirst
--- PASS: TestNegotiate_BothSupportBoth_SelectsSecureFirst (0.00s)
=== RUN   TestNegotiate_Deterministic
--- PASS: TestNegotiate_Deterministic (0.00s)
=== RUN   TestNegotiate_DeprecatedProfile_Rejected
--- PASS: TestNegotiate_DeprecatedProfile_Rejected (0.00s)
```

**Evidence from E2E**: `authenticated_chat_server.log`

```
Profile: secure-baseline-v1
Profile:    1 (secure-baseline-v1)
```

**Analysis**:
- `secure-baseline-v1` is the active profile
- Security-first selection: secure profile preferred over deprecated profiles
- Deprecated profiles (transport-only) are rejected by default

**Verdict**: **PASS** - Profile negotiation works and enforces secure-baseline-v1.

---

## 4. DID + Ed25519 Authentication Proof

**Evidence**: `did_auth_tests.log`

```
=== RUN   TestVerifyDIDAuthSignature_Success
--- PASS: TestVerifyDIDAuthSignature_Success (0.00s)
=== RUN   TestDIDAuth_FullFlow
--- PASS: TestDIDAuth_FullFlow (0.00s)
=== RUN   TestVerifyDIDAuthSignature_ChallengeBinding
--- PASS: TestVerifyDIDAuthSignature_ChallengeBinding (0.00s)
```

**Evidence from E2E**: `authenticated_chat_server.log`

```
--- Verifying Signature ---
  Signature VALID
--- Sent Authenticated Response ---
  Sequence: 1
Authentication flow completed successfully!
```

**Analysis**:
- DID format: `did:key:axcp-auth-chat-client`
- Signature size: 64 bytes (Ed25519 standard)
- Transcript includes: SenderDID, RecipientDID, Payload, Timestamp
- Signature verified using resolved public key from DID document

**Verdict**: **PASS** - DID + Ed25519 authentication is working end-to-end.

---

## 5. Replay Protection Proof

**Evidence**: `gateway_auth_tests.log`

```
=== RUN   TestEnvelopeAuthenticator_ReplayProtection
--- PASS: TestEnvelopeAuthenticator_ReplayProtection (0.00s)
```

**Test Details** (from source code):
```go
// First request should succeed
result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, envAuth)
if !result.Authenticated {
    t.Errorf("First request should succeed, got error: %v", result.Error)
}

// Replay with same sequence should fail
result = ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, envAuth)
if result.Authenticated {
    t.Error("Replay should be detected")
}
if result.ErrorCode != ErrorCodeAuthReplayDetected {
    t.Errorf("ErrorCode = %d, want %d", result.ErrorCode, ErrorCodeAuthReplayDetected)
}
```

**Analysis**:
- Sequence number per DID tracked
- Same sequence number rejected on second attempt
- ErrorCode: `ErrorCodeAuthReplayDetected` returned for replays

**Verdict**: **PASS** - Replay protection is active and blocks duplicates.

---

## 6. Gateway Enforcement Proof

**Evidence**: `gateway_auth_tests.log`

```
=== RUN   TestEnvelopeAuthenticator_SecureBaseline_MissingFields
--- PASS: TestEnvelopeAuthenticator_SecureBaseline_MissingFields (0.00s)
=== RUN   TestEnvelopeAuthenticator_InvalidSignature
--- PASS: TestEnvelopeAuthenticator_InvalidSignature (0.00s)
=== RUN   TestEnvelopeAuthenticator_DIDNotFound
--- PASS: TestEnvelopeAuthenticator_DIDNotFound (0.00s)
=== RUN   TestEnvelopeAuthenticator_FullAuthFlow
--- PASS: TestEnvelopeAuthenticator_FullAuthFlow (0.00s)
```

**Analysis**:
- Gateway verifies all auth fields are present
- Invalid signatures are rejected
- Unknown DIDs are rejected
- Timestamps are validated (not too old, not in future)

**Verdict**: **PASS** - Gateway enforces full authentication.

---

## Artifacts Included

| File | Description |
|------|-------------|
| `authenticated_chat_server.log` | E2E QUIC server log showing auth flow |
| `did_auth_tests.log` | DID + Ed25519 unit tests |
| `gateway_auth_tests.log` | Gateway authentication unit tests |
| `negotiate_profile_tests.log` | Profile negotiation unit tests |
| `protobuf_roundtrip_test.log` | Protobuf serialization test |
| `PROTOCOL_PROOF_REPORT.md` | This report |

---

## Summary

| Requirement | Status |
|-------------|--------|
| QUIC Transport | **PASS** |
| Protobuf Serialization | **PASS** |
| secure-baseline-v1 Profile | **PASS** |
| DID + Ed25519 Auth | **PASS** |
| Replay Protection | **PASS** |
| Gateway Enforcement | **PASS** |

---

## Conclusion

**AXCP Protocol Proof M7.0: PASS**

All five M7.0 requirements have been demonstrated with evidence:
1. QUIC transport via quic-go (not HTTP)
2. Protobuf envelopes (not JSON)
3. secure-baseline-v1 profile negotiation
4. DID + Ed25519 signature verification
5. Replay protection with sequence tracking

The protocol is ready for production deployment evaluation.

---

*Generated by AXCP Protocol Proof Runner*
*Report timestamp: 2026-01-30T21:56:00Z*
