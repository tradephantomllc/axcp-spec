# AXCP Core Authentication

This document describes the authentication model in AXCP Core's **Secure Baseline (`secure-baseline-v1`)** profile.

> **Authoritative Reference**: The canonical specification is [AXCP Core Specification v1.0](../spec/axcp-v1.0.md). This document provides operational guidance aligned with that specification.

## Overview

AXCP Core uses DID-based mutual authentication with Ed25519 signatures. When the `secure-baseline-v1` profile is negotiated, all messages must be:

1. Signed by the sender using Ed25519
2. Verified by the recipient using the sender's public key (resolved via DID)
3. Protected against replay attacks

## Security Profiles

| Profile | Authentication | Replay Protection | Production Use |
|---------|----------------|-------------------|----------------|
| `secure-baseline-v1` | Required | Required | **Recommended** |
| `transport-only-v0` | Not required | Not required | **Deprecated** |

The `secure-baseline-v1` profile is the only profile suitable for production. The `transport-only-v0` profile is deprecated and should not be used.

## DID Model

### DID Format

DIDs follow the standard format:

```
did:<method>:<id>
```

Examples:
- `did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK`
- `did:key:axcp-auth-chat-server`

### DID Resolver Abstraction

AXCP Core does **not** provide a network-based DID resolver. Instead, it defines a `DIDResolver` interface that implementations must inject:

```go
// DIDResolver is an interface for resolving DIDs to DID documents.
// See: sdk/go/auth/did.go
type DIDResolver interface {
    Resolve(ctx context.Context, did string) (*DIDDocument, error)
}
```

**For development/testing**, use `MemoryDIDResolver`:

```go
resolver := auth.NewMemoryDIDResolver()
resolver.AddDID("did:key:my-agent", publicKey)
```

**For production**, implement a resolver that queries your DID infrastructure (e.g., DID:Web, DID:ION, or a private registry).

See `examples/go/authenticated_chat/main.go` for a complete example using the in-memory resolver.

## Ed25519 Signing

AXCP Core uses Ed25519 for all cryptographic signatures. Only the following key types are allowed in DID documents:

- `Ed25519VerificationKey2020`
- `Ed25519VerificationKey2018`

### Key Generation

```go
publicKey, privateKey, err := auth.GenerateEd25519Keypair()
```

### Signing

```go
signature, err := auth.SignEd25519(privateKey, transcript)
```

### Verification

```go
valid := auth.VerifyEd25519(publicKey, transcript, signature)
```

## Transcript Canonicalization

The signature is computed over a **canonical transcript** that includes all authentication-relevant fields. The exact format is defined in [AXCP Core Specification v1.0, Section 6.5](../spec/axcp-v1.0.md#65-canonical-transcript-format).

**Transcript version prefix**: `AXCP-DID-AUTH-v1`

**Format**:
```
AXCP-DID-AUTH-v1
<clientDID>
<serverDID>
<base64(challenge)>
<timestampRFC3339>
```

**Important**: Do not construct transcripts manually. Use the SDK function:

```go
transcript := auth.BuildDIDAuthTranscript(clientDID, serverDID, challenge, timestamp)
```

This ensures the transcript matches the specification exactly.

## Replay Protection

AXCP Core prevents replay attacks using sequence numbers with a sliding window.

### Configuration Parameters

| Parameter | Description | Constraints |
|-----------|-------------|-------------|
| TTL | Time-to-live for seen entries | Must be > 0 |
| Window | Sliding window size | Must be >= 1 |

### Sequence Mode Semantics

For each incoming message with sequence number `seq`:

1. **Cleanup**: Remove entries older than TTL
2. **Replay check**: If `seq` was seen within TTL, reject with `ErrReplay`
3. **Window check**: If `seq < (highest - window + 1)`, reject with `ErrTooOld`
4. **Accept**: Mark `seq` as seen, update highest if `seq > highest`

### Errors

| Error | Meaning |
|-------|---------|
| `ErrReplay` | Sequence was already seen within TTL |
| `ErrTooOld` | Sequence is outside the sliding window |
| `ErrEmptyPeerID` | Peer identifier is missing |

See [AXCP Core Specification v1.0, Section 7](../spec/axcp-v1.0.md#7-replay-protection) for complete semantics.

## Required Rejections

When `secure-baseline-v1` is negotiated, implementations MUST reject messages that:

- Have missing or invalid signatures
- Have invalid DID format
- Reference DIDs that cannot be resolved
- Have DID document ID mismatches
- Lack an acceptable Ed25519 key in the document
- Fail signature verification
- Are replays (same sequence within TTL)
- Have sequences outside the sliding window

There is no retry logic for authentication failures. Implementations must **fail fast**.

## Code References

| Component | Location |
|-----------|----------|
| DID types and transcript | `sdk/go/auth/did.go` |
| Ed25519 operations | `sdk/go/auth/ed25519.go` |
| Replay protection | `sdk/go/auth/replay.go` |
| Session management | `sdk/go/auth/session.go` |
| Profile negotiation | `sdk/go/negotiate/profile.go` |

## Example

For a complete working example of authenticated communication, see:

```
examples/go/authenticated_chat/
```

This example demonstrates:
- Deterministic keypair generation
- Shared in-memory DID resolver
- Message signing and verification
- Bidirectional authenticated exchange
