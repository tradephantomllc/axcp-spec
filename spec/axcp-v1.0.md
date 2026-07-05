# AXCP Core Specification v1.0 (Secure Baseline)

**Version:** 1.0
**Status:** Stable
**Date:** 2026-01-29
**License:** Apache 2.0

---

## Table of Contents

1. [Scope and Non-Goals](#1-scope-and-non-goals)
2. [Terminology](#2-terminology)
3. [Versioning](#3-versioning)
4. [Security Profiles](#4-security-profiles)
5. [Profile Negotiation](#5-profile-negotiation)
6. [Identity and Authentication](#6-identity-and-authentication)
7. [Replay Protection](#7-replay-protection)
8. [Message Handling Requirements](#8-message-handling-requirements)
9. [Deprecations](#9-deprecations)
10. [Interoperability Appendix](#interoperability-appendix)

---

## 1. Scope and Non-Goals

### 1.1 Scope

This specification defines **AXCP Core v1.0**, the open-source foundation for the Adaptive eXchange Context Protocol. AXCP Core provides:

- Secure profile negotiation between clients and servers
- DID-based mutual authentication with Ed25519 signatures
- Replay attack protection using sequence numbers or nonces
- Deterministic behavior for reproducible security guarantees

### 1.2 Non-Goals (Out of Scope for Core)

The following capabilities are **NOT part of AXCP Core** and belong to commercial tiers:

| Feature | Tier |
|---------|------|
| Differential Privacy (DP) noise mechanisms | Advanced / Enterprise |
| Privacy budget management | Advanced / Enterprise |
| Context synchronization (CRDT) | Advanced |
| mTLS certificate management | Advanced |
| SGX/SEV enclave integration | Advanced |
| PII filtering and redaction | Enterprise |
| Compliance reporting (GDPR/HIPAA/SOC2) | Enterprise |
| Audit trails | Enterprise |

Core implementations MUST NOT include these features. Core MUST NOT import code from Advanced or Enterprise tiers.

---

## 2. Terminology

| Term | Definition |
|------|------------|
| **Profile** | A named security configuration that defines required authentication and protection mechanisms |
| **DID** | Decentralized Identifier, format: `did:<method>:<id>` |
| **DID Document** | A JSON-LD document containing public keys and verification methods for a DID |
| **Transcript** | A canonical byte sequence used as input for cryptographic signing |
| **Replay** | An attack where a valid message is retransmitted to cause unauthorized effects |
| **TTL** | Time-to-live; the duration for which replay protection entries are retained |
| **Window** | The sliding window size for sequence-based replay protection |
| **Nonce** | A unique value used exactly once to prevent replay attacks |

---

## 3. Versioning

### 3.1 Protocol Version

The current protocol version is `"1"`.

Implementations MUST reject negotiations with unknown protocol versions by returning `ErrInvalidVersion`.

### 3.2 Specification Versioning

This document is **AXCP Core Specification v1.0**. Future minor revisions (1.1, 1.2, etc.) will maintain backward compatibility. Major revisions (2.0) may introduce breaking changes.

---

## 4. Security Profiles

### 4.1 Profile Definitions

AXCP Core defines two profiles:

#### 4.1.1 `secure-baseline-v1` (Production Default)

**String constant:** `"secure-baseline-v1"`

Requirements:
- DID-based authentication: **REQUIRED**
- Ed25519 signature verification: **REQUIRED**
- Replay protection: **REQUIRED**

This profile MUST be used for all production deployments. It provides application-layer security on top of transport-layer protections (QUIC/TLS).

#### 4.1.2 `transport-only-v0` (Deprecated)

**String constant:** `"transport-only-v0"`

Requirements:
- DID-based authentication: Not required
- Ed25519 signature verification: Not required
- Replay protection: Not required

**WARNING:** This profile is DEPRECATED and MUST NOT be used in production. It relies solely on transport-layer security (QUIC/TLS) without application-layer authentication.

### 4.2 Profile Selection Rules

1. Implementations MUST prefer `secure-baseline-v1` over `transport-only-v0`
2. Implementations MUST reject `transport-only-v0` by default
3. The `AllowDeprecated` option MAY be used to permit `transport-only-v0` for testing only

---

## 5. Profile Negotiation

### 5.1 Negotiation Messages

#### 5.1.1 ClientHello

```
ClientHello {
    Version:   string        // Protocol version, MUST be "1"
    Profiles:  []Profile     // Supported profiles in preference order
    Cap:       Capabilities  // Client capability requirements
}

Capabilities {
    RequireAuth:   bool      // Whether DID auth is required
    RequireReplay: bool      // Whether replay protection is required
    SigAlgos:      []string  // Acceptable signature algorithms
}
```

#### 5.1.2 ServerHello

```
ServerHello {
    Version:  string        // Negotiated protocol version
    Profile:  Profile       // Selected security profile
    Cap:      Capabilities  // Resolved capabilities
}
```

### 5.2 Negotiation Algorithm

The server performs negotiation as follows:

1. **Validate protocol version**
   - If `ClientHello.Version != "1"`, return `ErrInvalidVersion`

2. **Validate client profiles**
   - If `ClientHello.Profiles` is empty, return `ErrEmptyProfiles`
   - If any profile is unknown, return `ErrUnknownProfile`

3. **Select profile (security-first)**
   - Iterate through secure profiles in order: `[secure-baseline-v1]`
   - Select the first profile supported by both client and server
   - If no secure profile matches and both support `transport-only-v0`:
     - If `AllowDeprecated == false`, return `ErrDeprecatedProfile`
     - Otherwise, select `transport-only-v0`
   - If no overlap, return `ErrNoProfileOverlap`

4. **Validate capabilities**
   - For `secure-baseline-v1`, server enforces `RequireAuth=true` and `RequireReplay=true`

5. **Resolve signature algorithm**
   - Find intersection of client and server algorithms
   - Prefer `"ed25519"` if available
   - If no intersection and either list is empty, default to `"ed25519"`
   - If no intersection and both lists non-empty, return `ErrNoAlgoOverlap`

6. **Return ServerHello**

### 5.3 No Silent Downgrade

Implementations MUST NOT silently downgrade from `secure-baseline-v1` to `transport-only-v0`. If `secure-baseline-v1` is the only secure option and it cannot be negotiated, the connection MUST fail.

### 5.4 Negotiation Errors

| Error | Condition |
|-------|-----------|
| `ErrInvalidVersion` | Protocol version is not `"1"` |
| `ErrEmptyProfiles` | Client sent no profiles |
| `ErrUnknownProfile` | Client sent an unrecognized profile |
| `ErrNoProfileOverlap` | No common profile between client and server |
| `ErrDeprecatedProfile` | Only `transport-only-v0` overlaps and `AllowDeprecated=false` |
| `ErrNoAlgoOverlap` | No common signature algorithm |

---

## 6. Identity and Authentication

### 6.1 DID Format

DIDs MUST follow the format:

```
did:<method>:<id>
```

Where:
- `<method>` is a non-empty method identifier (e.g., `key`, `web`, `peer`)
- `<id>` is a non-empty method-specific identifier

Validation rules:
- DIDs MUST NOT contain spaces or control characters
- DIDs MUST start with `did:`
- Method and ID components MUST be non-empty

### 6.2 DID Document Structure

A minimal DID Document MUST contain:

```
DIDDocument {
    ID:         string           // The DID this document describes
    PublicKeys: []PublicKeyRecord
}

PublicKeyRecord {
    ID:             string   // Key identifier (e.g., "key-1")
    Type:           string   // Key type from allowlist
    PublicKeyBytes: []byte   // Raw public key bytes
}
```

### 6.3 Key Type Allowlist

For Ed25519 verification, the following key types are allowed:

- `Ed25519VerificationKey2020`
- `Ed25519VerificationKey2018`

Implementations MUST reject keys with types not in this allowlist.

### 6.4 DID Resolver Abstraction

The `DIDResolver` interface:

```
interface DIDResolver {
    Resolve(ctx, did string) -> (DIDDocument, error)
}
```

Core does NOT provide a network-based resolver implementation. Implementations MUST inject a resolver (which may be in-memory, file-based, or network-based depending on deployment).

### 6.5 Canonical Transcript Format

The authentication transcript MUST be constructed exactly as follows:

**Version prefix:** `AXCP-DID-AUTH-v1`

**Format:**
```
AXCP-DID-AUTH-v1\n
<senderDID>\n
<recipientDID>\n
<base64(canonicalSigningPayload)>\n
<timestampRFC3339>
```

Where:
- `\n` is the newline character (0x0A)
- Fields are joined with `\n` (no trailing newline)
- `<senderDID>` is the DID of the signing agent or gateway
- `<recipientDID>` is the intended recipient DID
- `<base64(canonicalSigningPayload)>` uses standard Base64 encoding (RFC 4648)
- `<timestampRFC3339>` is UTC time formatted as RFC3339 (e.g., `2026-01-29T12:00:00Z`)

The canonical signing payload is the deterministic Protobuf encoding of the
AXCP envelope after clearing detached authentication fields:

- `sender_did`
- `recipient_did`
- `timestamp_ms`
- `signature`
- `attestation_proof`

The replay `sequence` field MUST remain in the canonical signing payload.
Changing `sequence` MUST change the signing payload and MUST invalidate the
detached signature before replay state is mutated.

**Example transcript:**
```
AXCP-DID-AUTH-v1
did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK
did:key:z6MknGc3ocHs3zdPiJbnaaqDi5QSrGmCaXz5Y8Jrd8hVbuYZ
SGVsbG8gV29ybGQh
2026-01-29T12:00:00Z
```

### 6.6 Signature Verification

To verify a DID authentication signature:

1. Parse and validate the signer DID format
2. Resolve the DID document using the injected resolver
3. Verify the document ID matches the requested DID exactly
4. Extract the first public key with an allowed Ed25519 type
5. Verify key length is exactly 32 bytes (Ed25519 public key size)
6. Verify the Ed25519 signature over the transcript bytes

If any step fails, authentication MUST be rejected.

---

## 7. Replay Protection

### 7.1 Replay Token

Messages MUST include a replay token, either:
- **Sequence number** (`uint64`): Monotonically increasing per peer
- **Nonce** (`string`): Unique random value per message

### 7.2 Replay Protector Configuration

| Parameter | Constraints | Description |
|-----------|-------------|-------------|
| TTL | > 0 | Time-to-live for seen entries. After expiration, the same sequence/nonce may be accepted again. |
| Window | >= 1 | Sliding window size for sequence mode. |

### 7.3 Sequence Mode

For sequence-based replay protection:

1. **Cleanup**: Remove entries older than TTL
2. **Replay check**: If sequence was seen within TTL, reject with `ErrReplay`
3. **Window check**: Let H = highest seen sequence for this peer
   - Calculate minimum acceptable: `minAcceptable = H - window + 1` (or 0 if H < window)
   - If `seq < minAcceptable`, reject with `ErrTooOld`
4. **Accept**: Mark sequence as seen, update highest if `seq > H`

### 7.4 Nonce Mode

For nonce-based replay protection:

1. **Cleanup**: Remove nonces older than TTL
2. **Replay check**: If nonce was seen within TTL, reject with `ErrReplay`
3. **Accept**: Mark nonce as seen

### 7.5 Replay Protection Errors

| Error | Condition |
|-------|-----------|
| `ErrReplay` | Sequence/nonce already seen within TTL |
| `ErrTooOld` | Sequence is outside the sliding window (below `H - window + 1`) |
| `ErrEmptyPeerID` | Peer identifier is empty |
| `ErrEmptyNonce` | Nonce is empty (nonce mode only) |

---

## 8. Message Handling Requirements

### 8.1 Required Rejections

When `secure-baseline-v1` is negotiated, implementations MUST reject messages that:

| Condition | Action |
|-----------|--------|
| Missing signature | Reject |
| Invalid DID format | Reject |
| DID document not found | Reject |
| DID document ID mismatch | Reject |
| No acceptable Ed25519 key in document | Reject |
| Signature verification failed | Reject |
| Replay detected (same seq/nonce within TTL) | Reject |
| Sequence too old (outside sliding window) | Reject |
| Negotiation failed | Reject connection |

### 8.2 Error Categories

- **Authentication errors**: DID parsing, resolution, key extraction, signature verification
- **Replay errors**: Duplicate sequence/nonce, sequence too old
- **Negotiation errors**: Version mismatch, no profile overlap, deprecated profile

### 8.3 Fail-Fast Behavior

Implementations MUST fail fast on authentication errors. There is no retry logic for authentication failures.

---

## 9. Deprecations

### 9.1 Deprecated Profiles

| Profile | Status | Rationale |
|---------|--------|-----------|
| `transport-only-v0` | DEPRECATED | Provides only transport-layer security without application-layer authentication. Not suitable for production environments where mutual authentication is required. |

### 9.2 Migration Path

Implementations currently using `transport-only-v0` SHOULD migrate to `secure-baseline-v1`:

1. Provision DID documents for all participants
2. Implement or inject a DID resolver
3. Update clients and servers to negotiate `secure-baseline-v1`
4. Remove `AllowDeprecated` option from production configurations

---

## Interoperability Appendix

### A.1 Negotiation Examples

#### A.1.1 Successful Secure Baseline Negotiation

```
ClientHello:
  Version: "1"
  Profiles: ["secure-baseline-v1"]
  Cap: {RequireAuth: true, RequireReplay: true, SigAlgos: ["ed25519"]}

ServerHello:
  Version: "1"
  Profile: "secure-baseline-v1"
  Cap: {RequireAuth: true, RequireReplay: true, SigAlgos: ["ed25519"]}
```

#### A.1.2 Client Offers Multiple Profiles

```
ClientHello:
  Version: "1"
  Profiles: ["transport-only-v0", "secure-baseline-v1"]
  Cap: {RequireAuth: false, RequireReplay: false, SigAlgos: ["ed25519"]}

ServerHello:
  Version: "1"
  Profile: "secure-baseline-v1"  # Security-first selection
  Cap: {RequireAuth: true, RequireReplay: true, SigAlgos: ["ed25519"]}
```

#### A.1.3 Negotiation Failure (No Overlap)

```
ClientHello:
  Version: "1"
  Profiles: ["transport-only-v0"]
  Cap: {...}

Server (AllowDeprecated=false):
  Error: ErrDeprecatedProfile
```

### A.2 Transcript Composition Example

Given:
- Sender DID: `did:key:z6MkClient123`
- Recipient DID: `did:key:z6MkServer456`
- Canonical signing payload bytes: `[0x08, 0x01, 0x12, 0x09, 0x74, 0x72, 0x61, 0x63, 0x65, 0x2d, 0x31, 0x32, 0x33, 0x18, 0x01, 0xb0, 0x01, 0x2a]`
- Timestamp: 2026-01-29T15:30:00Z

Canonical signing payload Base64: `CAESCXRyYWNlLTEyMxgBsAEq`

Transcript bytes (UTF-8):
```
AXCP-DID-AUTH-v1
did:key:z6MkClient123
did:key:z6MkServer456
CAESCXRyYWNlLTEyMxgBsAEq
2026-01-29T15:30:00Z
```

### A.3 Implementation Reference

The canonical implementation of these specifications can be found in:

| Component | Location |
|-----------|----------|
| Profile negotiation | `sdk/go/negotiate/profile.go` |
| DID authentication | `sdk/go/auth/did.go` |
| Replay protection | `sdk/go/auth/replay.go` |
| Ed25519 signing/verifying | `sdk/go/auth/ed25519.go` |

---

*End of AXCP Core Specification v1.0*
