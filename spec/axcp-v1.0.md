# AXCP Core Specification v1.0 (Secure Baseline)

**Version:** 1.0 (revised 2026-10-15)
**Status:** Stable security contract. Read the [Revision History](#revision-history) before implementing: the Secure Baseline transcript changed after the initial 1.0.0 tag.
**Initial publication:** 2026-01-29
**License:** Apache 2.0

---

## Table of Contents

- [Revision History](#revision-history)
1. [Scope and Non-Goals](#1-scope-and-non-goals)
2. [Terminology](#2-terminology)
3. [Versioning](#3-versioning)
4. [Security Profiles](#4-security-profiles)
5. [Profile Negotiation](#5-profile-negotiation)
6. [Identity and Authentication](#6-identity-and-authentication)
7. [Replay Protection](#7-replay-protection)
8. [Message Handling Requirements](#8-message-handling-requirements)
9. [Deprecations](#9-deprecations)
10. [Security Considerations](#10-security-considerations)
- [Interoperability Appendix](#interoperability-appendix)

---

## Revision History

| Revision | Date | Summary |
|----------|------|---------|
| 1.0.0 | 2026-01-29 | Initial release (Git tag `v1.0.0`). Transcript label `AXCP-DID-AUTH-v1` over `<clientDID>`, `<serverDID>`, `<base64(challenge)>` and an RFC 3339 timestamp. |
| 1.0, transcript revision | 2026-07-05 | The Secure Baseline transcript was redefined to bind the **sender DID**, the **recipient DID**, the **canonical signing payload** of the envelope (which includes the replay `sequence`) and the **timestamp at second precision**. The label `AXCP-DID-AUTH-v1` was kept. |
| 1.0, transport clarification | 2026-10-13 | Added [§8.4](#84-reference-transport-and-telemetry-encoding): ALPN `axcp/1`, stream framing, authenticated `0xA1` telemetry datagrams and rejection of legacy `0xA0` telemetry in Secure Baseline. |
| 1.0, hardening revision | 2026-10-15 | Byte-exact canonical signing payload ([§6.6](#66-canonical-signing-payload)); strict envelope parsing ([§8.2](#82-envelope-validation-rules)); DID syntax and length limits, `did:key` derivation and resolver precedence ([§6.1](#61-did-syntax), [§6.4](#64-did-resolution)); small-order key rejection ([§6.3](#63-key-type-allowlist-and-key-validation)); replay window and peer-state lifetime rules ([§7](#7-replay-protection)); nonce mode withdrawn (reserved); expanded required rejections and error-code mapping ([§8](#8-message-handling-requirements)); profile negotiation wire format declared informative ([§5](#5-profile-negotiation)). The transcript itself is unchanged from the 2026-07-05 revision. |

**Interoperability notice.** Implementations of the original 1.0.0 tag are **not interoperable** with the revised transcript: a signature produced under one transcript does not verify under the other, and the shared label `AXCP-DID-AUTH-v1` does not let a peer detect which variant was used. All peers of a deployment MUST be upgraded together. The 2026-10-15 revision makes verifiers stricter; envelopes accepted by earlier implementations (for example envelopes with unknown fields, an empty recipient, `sequence = 0` or a `did:key` identifier that is not a multibase Ed25519 key) are rejected by conforming verifiers.

---

## 1. Scope and Non-Goals

### 1.1 Scope

This specification defines **AXCP Core v1.0**, the open-source foundation for the Adaptive eXchange Context Protocol. AXCP Core provides:

- Security profile definitions and profile selection rules
- DID-based message authentication with Ed25519 signatures, bound to sender, recipient, canonical message bytes and time
- Replay protection using per-sender sequence numbers
- Deterministic encoding rules so that independent implementations accept and reject the same envelopes

### 1.2 Non-Goals (Out of Scope for Core)

The following capabilities are **NOT part of AXCP Core**; commercial capabilities are offered separately:

| Feature | Distribution |
|---------|--------------|
| Differential privacy and privacy budgets | Private commercial product |
| Automatic context synchronization (CRDT) | Private commercial product |
| Managed mTLS certificates and attestation | Private commercial product |
| PII filtering, governance and audit tooling | Private commercial product |
| Automatic orchestration and routing | Private commercial product |

Commercial capability availability is determined by license. Advanced/Enterprise are historical repository labels. Compliance tooling is not a certification claim.

This public Core repository MUST NOT import proprietary commercial implementations. This product boundary does not restrict independently extending Apache-licensed Core.

### 1.3 Conventions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY" and "OPTIONAL" in this document are to be interpreted as described in BCP 14 (RFC 2119, RFC 8174) when, and only when, they appear in all capitals.

Field names and numbers refer to the `AxcpEnvelope` message and related messages in [`proto/axcp.proto`](../proto/axcp.proto). Sections marked *informative* do not contain requirements.

---

## 2. Terminology

| Term | Definition |
|------|------------|
| **Envelope** | An `AxcpEnvelope` Protocol Buffers message |
| **Sender** | The party that signs an envelope; identified by `sender_did` (field 20) |
| **Verifier** | The party that receives and validates an envelope; its own DID is the expected `recipient_did` (field 23) |
| **Profile** | A named security configuration that defines required authentication and protection mechanisms |
| **DID** | Decentralized Identifier with the syntax defined in [§6.1](#61-did-syntax) |
| **DID Document** | The resolved description of a DID that carries its verification keys. This specification relies only on the abstract structure in [§6.2](#62-did-document-structure) and does not mandate a serialization |
| **Resolver** | A deployment-provided function that maps a DID to a DID Document ([§6.4](#64-did-resolution)) |
| **Canonical signing payload** | The byte string defined in [§6.6](#66-canonical-signing-payload) |
| **Transcript** | The byte sequence defined in [§6.5](#65-canonical-transcript-format) that is signed with Ed25519 |
| **Replay** | An attack where a valid message is retransmitted to cause unauthorized effects |
| **Window** | The number of most recent sequence numbers per sender for which a verifier tracks individual use |
| **Peer-state TTL** | How long a verifier keeps replay state for a sender that has sent nothing |
| **Nonce** | Reserved. Secure Baseline v1 has no nonce field ([§7.5](#75-nonce-mode-reserved)) |

---

## 3. Versioning

### 3.1 Protocol Version

The current protocol version is `1`. Every envelope carries it in `version` (field 1). Verifiers MUST reject an envelope whose `version` is not `1`.

### 3.2 Specification Versioning

This document is **AXCP Core Specification v1.0**. Future minor revisions (1.1, 1.2, etc.) are intended to remain backward compatible; major revisions (2.0) may introduce breaking changes.

The transcript revision recorded in the [Revision History](#revision-history) is an exception to this rule: it changed the Secure Baseline transcript incompatibly within v1.0. Future changes to the transcript or to the canonical signing payload will be published under a new specification version and a new transcript label.

---

## 4. Security Profiles

### 4.1 Profile Definitions

AXCP Core defines two profiles. The envelope carries the profile as a number in `profile` (field 3).

| Profile | Envelope `profile` value | Status |
|---------|--------------------------|--------|
| `secure-baseline-v1` | `1` | Production default |
| `transport-only-v0` | `0` | Deprecated |

Values `2` and `3` are reserved and are not defined by this specification.

#### 4.1.1 `secure-baseline-v1` (Production Default)

**String constant:** `"secure-baseline-v1"`

Requirements:
- DID-based authentication: **REQUIRED**
- Ed25519 signature verification: **REQUIRED**
- Recipient binding and timestamp freshness: **REQUIRED**
- Replay protection: **REQUIRED**

This profile MUST be used for all production deployments. It provides application-layer security on top of transport-layer protections (QUIC/TLS). A verifier operating in Secure Baseline MUST reject every envelope whose `profile` is not `1`.

#### 4.1.2 `transport-only-v0` (Deprecated)

**String constant:** `"transport-only-v0"`

Requirements:
- DID-based authentication: Not required
- Ed25519 signature verification: Not required
- Replay protection: Not required

**WARNING:** This profile is DEPRECATED and MUST NOT be used in production. It relies solely on transport-layer security (QUIC/TLS) without application-layer authentication.

### 4.2 Profile Selection Rules

1. Implementations MUST prefer `secure-baseline-v1` over `transport-only-v0`.
2. Implementations MUST reject `transport-only-v0` by default.
3. An implementation MAY offer an explicit, non-default configuration option that permits `transport-only-v0` for testing. It MUST be disabled by default and MUST NOT be enabled implicitly.

---

## 5. Profile Negotiation

### 5.1 Status in v1.0

The profile negotiation **wire format is informative in v1.0**. Secure Baseline is configured out-of-band: each deployment configures the profile its endpoints use, and every verifier enforces it on every envelope ([§4.1.1](#411-secure-baseline-v1-production-default), [§8](#8-message-handling-requirements)). No on-wire exchange is required to establish Secure Baseline, and the reference gateway uses a fixed configured profile.

The Go SDK provides an optional handshake helper that implements the selection algorithm in [§5.3](#53-reference-selection-algorithm-informative). Its message encoding is specific to that SDK and is not part of this specification.

The `ProfileNegotiate` and `ProfileAck` messages in `proto/axcp.proto` (envelope payload fields 8 and 9) are **reserved**. v1.0 does not define their semantics, and implementations MUST NOT use them to select or lower the profile of a session.

### 5.2 Normative Selection Rules

Any mechanism that selects a profile, whether configuration or an optional handshake, MUST follow these rules:

1. **Security first.** If both parties support `secure-baseline-v1`, it MUST be selected.
2. **No silent downgrade.** `transport-only-v0` MUST NOT be selected unless both parties support only it *and* the explicit test option of [§4.2](#42-profile-selection-rules) is enabled. If `secure-baseline-v1` cannot be established, the connection MUST fail.
3. **Unknown values fail.** An unknown protocol version, an unknown profile or an empty profile list MUST cause failure.
4. **Signature algorithm.** Core supports only Ed25519 (`"ed25519"`). If the parties cannot agree on Ed25519, the connection MUST fail.

### 5.3 Reference Selection Algorithm (informative)

The Go SDK helper models the exchange as a client offer and a server answer:

```
ClientHello {
    Version:   string        // Protocol version, "1"
    Profiles:  []Profile     // Supported profiles in preference order
    Cap:       Capabilities  // Client capability requirements
}

Capabilities {
    RequireAuth:   bool      // Whether DID auth is required
    RequireReplay: bool      // Whether replay protection is required
    SigAlgos:      []string  // Acceptable signature algorithms
}

ServerHello {
    Version:  string        // Negotiated protocol version
    Profile:  Profile       // Selected security profile
    Cap:      Capabilities  // Resolved capabilities
}
```

The server:

1. Rejects a version other than `"1"` (*unsupported version*).
2. Rejects an empty profile list (*empty profile list*) or any unknown profile (*unknown profile*).
3. Selects `secure-baseline-v1` if both sides support it; otherwise, if both support only `transport-only-v0`, rejects (*deprecated profile*) unless the explicit test option is enabled; otherwise rejects (*no profile overlap*).
4. For `secure-baseline-v1`, sets `RequireAuth = true` and `RequireReplay = true`.
5. Resolves the signature algorithm to Ed25519 when both sides accept it (an empty list in the Go helper means the implicit Ed25519 default; an explicit list that excludes Ed25519 is not overridden), otherwise rejects (*no algorithm overlap*).

The client verifies the signed answer: protocol version, timestamp freshness, that the selected profile was offered (*profile not offered*), explicit permission for any deprecated profile, the mandatory auth/replay flags and exactly one supported and offered signature algorithm.

The helper's answer contains no client nonce or transcript binding, so timestamp freshness is not session binding. Deployments that use it MUST still authenticate the TLS peer and enforce envelope replay protection.

| Outcome | Condition |
|---------|-----------|
| unsupported version | Protocol version is not `"1"` |
| empty profile list | Client sent no profiles |
| unknown profile | Client sent an unrecognized profile |
| no profile overlap | No common profile between client and server |
| deprecated profile | Only `transport-only-v0` overlaps and the test option is disabled |
| no algorithm overlap | No common signature algorithm |
| profile not offered | Server selected a recognized profile the client did not offer |

---

## 6. Identity and Authentication

### 6.1 DID Syntax

Every DID used by AXCP (`sender_did`, `recipient_did` and a verifier's own DID) MUST match the following ABNF (RFC 5234), a subset of W3C DID Core syntax:

```
did                = "did:" method-name ":" method-specific-id
method-name        = 1*method-char
method-char        = %x61-7A / DIGIT          ; lowercase a-z, 0-9
method-specific-id = *( *idchar ":" ) 1*idchar
idchar             = ALPHA / DIGIT / "." / "-" / "_" / pct-encoded
pct-encoded        = "%" HEXDIG HEXDIG
```

Consequently a DID is ASCII only and contains no whitespace, no control characters, no path, query or fragment (`/`, `?`, `#`) and does not end with `:`.

The total length of a DID MUST NOT exceed **512 bytes**. Verifiers MUST check the length before any other parsing or decoding and MUST reject longer values.

### 6.2 DID Document Structure

This specification uses the following abstract DID Document structure. It does not require JSON, JSON-LD or any other serialization; a resolver may obtain documents in any format and present them in this form.

```
DIDDocument {
    ID:         string           // The DID this document describes
    PublicKeys: []PublicKeyRecord
}

PublicKeyRecord {
    ID:             string   // Key identifier (e.g., "key-1")
    Type:           string   // Key type
    PublicKeyBytes: []byte   // Raw public key bytes
}
```

### 6.3 Key Type Allowlist and Key Validation

For Ed25519 verification, the following key types are allowed:

- `Ed25519VerificationKey2020`
- `Ed25519VerificationKey2018`

The verification key of a DID Document is the **first** `PublicKeyRecord`, in document order, whose `Type` is in the allowlist and whose `PublicKeyBytes` is exactly 32 bytes long. Records with other types or lengths are ignored. If no record qualifies, verification MUST fail.

**Small-order keys.** Verifiers MUST reject a public key, whether obtained from a resolver or decoded from a `did:key`, if after clearing the sign bit (`key[31] &= 0x7f`) it equals one of the following little-endian point encodings (hex):

```
0000000000000000000000000000000000000000000000000000000000000000
0100000000000000000000000000000000000000000000000000000000000000
ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f
26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05
c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a
edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f
eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f
```

### 6.4 DID Resolution

#### 6.4.1 Resolver abstraction

A resolver maps a DID to a DID Document or fails:

```
resolve(did) -> DIDDocument | error
```

Core does not provide a network-based resolver. Deployments inject a resolver that may be in-memory, file-based or network-based. A resolver acts as the verifier's trust policy: only DIDs it resolves are accepted.

Every resolution result, including a result served from a cache, MUST be validated before use:

1. The document `ID` MUST equal the requested DID exactly.
2. The verification key is selected as in [§6.3](#63-key-type-allowlist-and-key-validation), and the small-order check of §6.3 applies.

A cache MUST NOT bypass these checks.

#### 6.4.2 `did:key` (Ed25519)

A `did:key` DID used with AXCP MUST identify an Ed25519 key as follows:

- The method-specific identifier starts with the multibase prefix `z` (base58btc, Bitcoin alphabet).
- The method-specific identifier, including the `z`, is at most **128 characters**. Verifiers MUST check this limit **before** base58 decoding.
- Base58btc decoding of the characters after `z` yields exactly **34 bytes**: the multicodec prefix `0xed 0x01` (Ed25519 public key) followed by the 32-byte public key.
- The decoded key MUST pass the small-order check of [§6.3](#63-key-type-allowlist-and-key-validation).

Derivation: `did = "did:key:z" || base58btc(0xed || 0x01 || publicKey)`. A `did:key` value that does not meet these rules MUST be rejected. See [A.2](#a2-canonical-signing-payload-and-transcript-example) for a worked example.

#### 6.4.3 Resolver precedence

1. If the verifier is configured with a resolver, the resolver is used for **all** DID methods, including `did:key`. The resolver acts as an allowlist.
2. When the DID is a `did:key` and the resolver returns a key, that key MUST equal the key encoded in the DID; otherwise verification MUST fail (DID mismatch).
3. Without a configured resolver, a `did:key` is decoded directly ("open mode") and any other DID method MUST be rejected.

In open mode any party that holds an Ed25519 key can authenticate as the `did:key` derived from it. Open mode proves possession of the key, not authorization; deployments MUST apply their own authorization policy or configure a resolver.

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
- Fields are joined with `\n` (no trailing newline); the transcript is encoded as UTF-8 (all fields are ASCII)
- `<senderDID>` is the value of `sender_did` (field 20)
- `<recipientDID>` is the value of `recipient_did` (field 23); it MUST be non-empty
- `<base64(canonicalSigningPayload)>` is the canonical signing payload of [§6.6](#66-canonical-signing-payload) in standard Base64 with padding (RFC 4648, Section 4)
- `<timestampRFC3339>` is defined in [§6.7](#67-timestamp-precision)

The Ed25519 signature (RFC 8032, pure Ed25519) over the transcript bytes is carried in `signature` (field 100) and MUST be exactly 64 bytes.

Because DIDs cannot contain newlines ([§6.1](#61-did-syntax)) and Base64 has no newline, the transcript is unambiguous.

### 6.6 Canonical Signing Payload

The canonical signing payload is the concatenation of the following Protocol Buffers fields of the envelope, in this order. Field numbers and wire types follow `proto/axcp.proto`; all varints use the minimal encoding.

| Order | Field | Encoding | Omitted when |
|-------|-------|----------|--------------|
| 1 | `version` (1) | tag `0x08`, varint | value is 0 |
| 2 | `trace_id` (2) | tag `0x12`, varint length, UTF-8 bytes | empty string |
| 3 | `profile` (3) | tag `0x18`, varint | value is 0 |
| 4 | the single payload member (one of fields 4–11) | tag `(n << 3) \| 2`, varint length, canonical encoding of the member message | never (exactly one member is present, [§8.2](#82-envelope-validation-rules)) |
| 5 | `sequence` (22) | tag `0xb0 0x01`, varint | value is 0 |

The following fields are **excluded**: `sender_did` (20), `timestamp_ms` (21), `recipient_did` (23), `signature` (100) and `attestation_proof` (101). Sender, recipient and timestamp are bound by the transcript instead.

The payload member message is encoded deterministically:

- Fields appear in ascending field-number order.
- Singular scalar, string, bytes and enum fields without explicit presence are omitted when they hold the default value (0, empty, or the zero enum value).
- A singular message field or a `oneof` member that is set is always emitted, even when the nested message is empty (zero-length) or the member holds a default value.
- Repeated fields emit every element in order, including elements that hold a default value (for example an empty string in a repeated string field).
- Nested messages are encoded recursively with the same rules. The schema has no map fields. Unknown fields cannot occur because they are rejected ([§8.2](#82-envelope-validation-rules)).

For any envelope that satisfies [§8.2](#82-envelope-validation-rules), the result equals the deterministic Protocol Buffers encoding of the envelope after clearing fields 20, 21, 23, 100 and 101.

The replay `sequence` is covered. Changing `sequence` changes the signing payload and invalidates the signature before replay state is touched. Fields that may be added to the envelope in the future are not covered by this v1 payload; that is one reason why unknown fields are rejected.

### 6.7 Timestamp Precision

`timestamp_ms` (field 21) is the sender's clock in milliseconds since the Unix epoch. The transcript authenticates it **at second precision only**:

- `<timestampRFC3339>` is the UTC time `floor(timestamp_ms / 1000)` seconds after the epoch, formatted as `YYYY-MM-DDTHH:MM:SSZ` (RFC 3339 `date-time`, uppercase `T` and `Z`, no fractional seconds, no numeric offset). Example: `timestamp_ms = 1700000000123` gives `2023-11-14T22:13:20Z`.
- The sub-second part (`timestamp_ms mod 1000`) is **not authenticated**. An on-path party can change it by up to 999 ms without invalidating the signature. Implementations MUST NOT use it for security decisions.

Freshness is evaluated on the received `timestamp_ms` ([§8.2](#82-envelope-validation-rules)). Defaults: maximum age 5 minutes, future clock skew 30 seconds.

### 6.8 Signature Verification

To verify an envelope signature, a verifier:

1. Validates the sender DID ([§6.1](#61-did-syntax)), checking its length first.
2. Obtains the sender's public key according to [§6.4](#64-did-resolution), including document-ID, key-type, key-length and small-order checks.
3. Builds the canonical signing payload ([§6.6](#66-canonical-signing-payload)) and the transcript ([§6.5](#65-canonical-transcript-format)) from the received envelope.
4. Verifies the 64-byte Ed25519 signature over the transcript bytes.

If any step fails, authentication MUST be rejected. The key comes only from the resolver or, in `did:key` open mode, from the sender DID itself; a verifier never accepts a key supplied in any other envelope field.

---

## 7. Replay Protection

### 7.1 Replay Token

Secure Baseline envelopes carry a per-sender sequence number in `sequence` (field 22, `uint64`). `sequence` MUST be greater than 0. Replay state is keyed by the exact `sender_did` string.

### 7.2 Replay Window

A verifier keeps, per sender, the highest accepted sequence `H` and a record (for example a bitmap) of which of the last `W` sequence numbers were accepted. `W` is the window size (default **1000**, `W >= 1`). For an authenticated envelope with sequence `s`:

1. **First message** from this sender (no state): accept; `H = s`.
2. **`s > H`**: advance the window, accept; `H = s`.
3. **`H - W < s <= H`**: if `s` was already accepted, reject (*replay*); otherwise mark `s` and accept.
4. **`s <= H - W`**: reject (*too old*).

Comparisons use unbounded integer arithmetic (in particular, rule 4 cannot apply while `H < W`). Accepted sequence numbers within the window do not expire while the sender's state is retained. Check-and-mark MUST be atomic per sender.

Replay state MUST be mutated only after the envelope has passed every other check of [§8.3](#83-verification-order), including signature verification, so that unauthenticated traffic cannot consume sequence numbers.

### 7.3 Peer-State Lifetime

A verifier MAY evict the state of a sender that has sent no accepted envelope for the peer-state TTL (default **10 minutes**).

The peer-state TTL MUST be at least the timestamp maximum age plus the allowed future clock skew. If a smaller TTL is configured together with a timestamp policy, the implementation MUST raise it to that sum. This guarantees that an envelope whose sender state has been evicted can no longer pass the freshness check.

A verifier MUST bound the number of tracked senders (default **100 000**). When the limit is reached after evicting idle senders, envelopes from new senders MUST be rejected (fail closed). Cleanup of idle senders SHOULD be amortized rather than performed as a full scan on every message.

### 7.4 Sender Sequence Allocation

- A sender MUST NOT reuse a sequence number under the same sender DID; a reused number is rejected as a replay.
- A new signing session SHOULD start its counter at the current Unix time in **microseconds** and increment it by one per envelope, so that sequences stay monotonic across sender restarts while the average sending rate stays below one envelope per microsecond. An explicit initial sequence MAY be configured, for example for tests or resumption.
- Senders that share one DID across processes or connections MUST coordinate allocation; envelopes delivered more than `W` sequence numbers behind the highest accepted one are rejected as too old.

### 7.5 Nonce Mode (Reserved)

`AxcpEnvelope` has no nonce field, so nonce-based replay protection **is not available in Secure Baseline v1**. Earlier text describing a nonce mode is withdrawn. A future profile may define one together with a signed nonce field.

### 7.6 Replay Outcomes

| Outcome | Condition | Error code ([§8.5](#85-error-codes)) |
|---------|-----------|------------|
| replay | Sequence already accepted within the window | `AUTH_REPLAY_DETECTED` (11) |
| too old | Sequence at or below `H - W` | `AUTH_REPLAY_DETECTED` (11) |
| invalid sequence | `sequence = 0` | `MALFORMED_REQUEST` (8) |
| capacity exceeded | Sender limit reached ([§7.3](#73-peer-state-lifetime)) | `TOO_MANY_REQUESTS` (9) |

---

## 8. Message Handling Requirements

### 8.1 Required Rejections

In Secure Baseline, verifiers MUST reject envelopes that meet any of the following conditions:

| Condition | Reference |
|-----------|-----------|
| Envelope cannot be parsed, contains unknown fields at any nesting level, contains more than one payload member or none, contains an undefined enum value or invalid UTF-8 | [§8.2](#82-envelope-validation-rules) |
| `version` is not 1 | [§3.1](#31-protocol-version) |
| `profile` is not 1 | [§4.1.1](#411-secure-baseline-v1-production-default) |
| Sender DID missing, longer than 512 bytes or not matching the DID syntax | [§6.1](#61-did-syntax) |
| `did:key` identifier not a valid multibase Ed25519 key (prefix, length, multicodec) | [§6.4.2](#642-didkey-ed25519) |
| DID method not accepted (no resolver and not `did:key`) | [§6.4.3](#643-resolver-precedence) |
| DID not resolvable, DID Document ID mismatch, or resolver key different from the `did:key` key | [§6.4](#64-did-resolution) |
| No acceptable Ed25519 key in the document | [§6.3](#63-key-type-allowlist-and-key-validation) |
| Small-order public key | [§6.3](#63-key-type-allowlist-and-key-validation) |
| Recipient DID missing, invalid, or not equal to the verifier's own DID | [§8.2](#82-envelope-validation-rules) |
| Timestamp missing (0), not representable, older than the maximum age, or later than now plus the clock skew | [§6.7](#67-timestamp-precision) |
| Signature missing, not 64 bytes, or verification failed | [§6.8](#68-signature-verification) |
| `sequence` is 0, already accepted (replay) or too old | [§7](#7-replay-protection) |
| Replay capacity exceeded | [§7.3](#73-peer-state-lifetime) |
| Frame larger than the maximum frame size | [§8.4](#84-reference-transport-and-telemetry-encoding) |
| Profile selection failed | [§5.2](#52-normative-selection-rules) (reject the connection) |

### 8.2 Envelope Validation Rules

Verification fails closed. In Secure Baseline:

- **Strict parsing.** The envelope MUST contain no unknown fields at any nesting level (including field numbers marked `reserved` in the schema). Enum values MUST be defined in the schema. All strings MUST be valid UTF-8.
- **Exactly one payload.** The wire encoding MUST contain exactly one payload member (one of fields 4–11). Envelopes with two or more payload members MUST be rejected, because Protocol Buffers parsers would otherwise silently keep only one of them. Senders MUST encode the payload member once.
- **Version and profile.** `version` MUST be 1 and `profile` MUST be 1.
- **Sender.** `sender_did` MUST satisfy [§6.1](#61-did-syntax).
- **Recipient binding.** `recipient_did` MUST be present, MUST satisfy [§6.1](#61-did-syntax) and MUST equal the verifier's own DID. Signers MUST require an explicit, non-empty recipient DID and MUST NOT sign envelopes without one.
- **Timestamp.** `timestamp_ms` MUST be non-zero, MUST be representable as an RFC 3339 date-time, MUST NOT be older than the maximum age (default 5 minutes) and MUST NOT be later than now plus the clock skew (default 30 seconds). Out-of-range values (for example values of 2^63 or more) MUST be rejected through the implementation's normal error reporting, never through an unhandled runtime fault.
- **Sequence.** `sequence` MUST be greater than 0.
- **Signature.** `signature` MUST be exactly 64 bytes.
- **Attestation.** `attestation_proof` (field 101) is not interpreted by Secure Baseline and is not covered by the signature; verifiers MUST NOT rely on its content.

### 8.3 Verification Order

1. Enforce the frame size limit ([§8.4](#84-reference-transport-and-telemetry-encoding)).
2. Parse the envelope under the strict rules of [§8.2](#82-envelope-validation-rules).
3. Check `version`, `profile`, sender and recipient DID syntax (length first), recipient binding, `sequence > 0` and signature length.
4. Check timestamp freshness.
5. Obtain and validate the sender's key ([§6.4](#64-did-resolution)).
6. Build the canonical signing payload and transcript and verify the signature ([§6.8](#68-signature-verification)).
7. Perform the atomic replay check-and-mark ([§7.2](#72-replay-window)).
8. Deliver the envelope to the application.

Steps 3–6 do not touch replay state and MAY be performed in a different order where their inputs allow, but steps 1–6 MUST all succeed before step 7 mutates replay state, and an envelope MUST NOT be delivered before step 7 succeeds.

### 8.4 Reference transport and telemetry encoding

The current protobuf schema is `proto/axcp.proto`; schemas in superseded drafts are historical. QUIC uses TLS 1.3 and ALPN `axcp/1`. Envelope stream frames are a four-byte little-endian length followed by the serialized envelope. Reference implementations cap frames at 10 MiB; the Go/gateway envelope reader rejects zero length. Raw Go/Python handshake frames use big-endian lengths instead.

Receivers MUST bound their resource use before authentication:

- A frame header announcing more than the maximum frame size MUST be rejected immediately, before reading the body.
- Receivers MUST NOT pre-allocate the announced frame size; buffers grow as bytes arrive.
- Buffered, unread stream bytes MUST NOT exceed the maximum frame size plus 4 bytes; otherwise the connection is closed.
- Datagram receive queues MUST be bounded (RECOMMENDED default 1024 entries); excess datagrams are dropped and counted.
- Reads MUST have timeouts.

Authenticated telemetry datagrams MUST be `0xA1 || AxcpEnvelope` with protocol version 1, numeric profile 1 and a populated telemetry payload. All normal signature, recipient, freshness and replay checks MUST run before delivery to a handler. Streams and datagrams MUST share replay state for a sender.

`0xA0 || TelemetryDatagram` is legacy transport-only telemetry and MUST be rejected in Secure Baseline. Unprefixed envelope datagrams are not a gateway wire format. The application limit is 1200 bytes including prefix, subject to the lower QUIC negotiated limit. DATAGRAM delivery may be lost or reordered and conveys no processing acknowledgement.

The gateway forwards signed envelopes intact to MQTT. Publisher acknowledgement is not consumer execution; bounded in-memory retry can duplicate or lose messages. Replay state is also in memory and does not survive gateway restart. Applications needing durable idempotency must supply it. Core makes no automatic orchestration, context-synchronization or exactly-once delivery promise.

### 8.5 Error Codes

When a verifier reports a rejection to its peer, it SHOULD use the `ErrorMessage.code` values of `proto/axcp.proto` as follows:

| Rejection | `ErrorCode` |
|-----------|-------------|
| Frame larger than the maximum | `PAYLOAD_TOO_LARGE` (7) |
| Parse failure, unknown field, payload member count not 1, undefined enum value, invalid UTF-8, `sequence = 0`, signature length not 64 | `MALFORMED_REQUEST` (8) |
| `version` not 1 | `UNSUPPORTED_VERSION` (5) |
| `profile` not 1 | `PROFILE_UNSUPPORTED` (13) |
| Sender DID syntax or length, unaccepted method, invalid `did:key`, resolution failure, document ID mismatch, no acceptable key, small-order key, resolver/`did:key` key mismatch | `AUTH_DID_INVALID` (17) |
| Recipient DID missing, invalid or not the verifier's DID | `AUTH_DID_INVALID` (17) |
| Signature missing or verification failed | `AUTH_SIGNATURE_INVALID` (10) |
| Timestamp missing, unrepresentable, too old or too far in the future | `AUTH_TIMESTAMP_EXPIRED` (18) |
| Replay or sequence too old | `AUTH_REPLAY_DETECTED` (11) |
| Replay capacity exceeded | `TOO_MANY_REQUESTS` (9) |
| Profile selection failed | `PROFILE_NEGOTIATION_FAILED` (14) |
| Any other failure | `UNKNOWN` (0) |

Error codes are diagnostic. Some v1.0 implementations report a sequence outside the window as `AUTH_TIMESTAMP_EXPIRED` (18), or version, profile and recipient failures as `MALFORMED_REQUEST` (8). Receivers MUST accept any defined code and MUST treat an undefined code value as `UNKNOWN`. Codes 15 (`MISSING_PATCH_RANGE`) and the remaining application codes are outside the scope of this section; code 16 is reserved.

### 8.6 Error Envelopes and Fail-Fast

- Implementations MUST fail fast on authentication errors. There is no retry logic for authentication failures.
- In v1.0 error envelopes are not required to be signed. A receiver MUST NOT treat an unsigned error envelope as an authenticated statement of its peer.
- Errors returned to a remote peer MUST NOT include internal error strings (library messages, stack traces, resolver responses). The `reason` field SHOULD be a short fixed description of the rejection category.
- Processing unauthenticated input MUST only produce the implementation's own error types, never unhandled runtime faults.

---

## 9. Deprecations

### 9.1 Deprecated Profiles

| Profile | Status | Rationale |
|---------|--------|-----------|
| `transport-only-v0` | DEPRECATED | Provides only transport-layer security without application-layer authentication. Not suitable for production environments where mutual authentication is required. |

### 9.2 Withdrawn Mechanisms

| Mechanism | Status |
|-----------|--------|
| Nonce-based replay protection | Withdrawn from Secure Baseline v1; reserved ([§7.5](#75-nonce-mode-reserved)) |
| `ProfileNegotiate` / `ProfileAck` payloads | Reserved; no v1.0 semantics ([§5.1](#51-status-in-v10)) |
| 1.0.0 transcript (`<clientDID>`, `<serverDID>`, `<base64(challenge)>`) | Replaced ([Revision History](#revision-history)) |

### 9.3 Migration Path

Implementations currently using `transport-only-v0` SHOULD migrate to `secure-baseline-v1`:

1. Provision DIDs (and, where a resolver is used, DID Documents) for all participants
2. Implement or inject a DID resolver, or deliberately choose `did:key` open mode with application-level authorization
3. Configure all endpoints for `secure-baseline-v1` and sign every envelope with an explicit recipient
4. Remove the deprecated-profile test option from production configurations

---

## 10. Security Considerations

- **Secrets.** Implementations MUST NOT expose private keys or seeds through debug representations, string conversion, serialization helpers or logs.
- **Open mode.** `did:key` open mode ([§6.4.3](#643-resolver-precedence)) authenticates key possession only. Configure a resolver or an application authorization policy to restrict which senders are accepted.
- **Receiver restarts.** Replay state kept only in memory is lost on restart. Until the maximum age plus clock skew has elapsed, envelopes accepted before the restart could be accepted again. Deployments that need replay resistance across restarts SHOULD persist replay state or refuse traffic for that interval, and SHOULD add application-level idempotency.
- **Timestamp precision.** Only whole seconds are authenticated ([§6.7](#67-timestamp-precision)).
- **Error envelopes.** Unsigned error envelopes can be forged by an on-path party that can inject into the transport; rely on TLS peer authentication and treat codes as diagnostics ([§8.6](#86-error-envelopes-and-fail-fast)).
- **Resource exhaustion.** Bound frame sizes, buffers, queues, tracked senders and resolver caches before authentication ([§7.3](#73-peer-state-lifetime), [§8.4](#84-reference-transport-and-telemetry-encoding)). Length limits on DIDs (512 bytes) and `did:key` identifiers (128 characters) exist so that decoding cost is bounded before any signature is checked.

---

## Interoperability Appendix

### A.1 Negotiation Examples (informative)

These examples use the model of [§5.3](#53-reference-selection-algorithm-informative).

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

#### A.1.3 Negotiation Failure (Deprecated Profile Only)

```
ClientHello:
  Version: "1"
  Profiles: ["transport-only-v0"]
  Cap: {...}

Server (deprecated-profile test option disabled):
  Error: deprecated profile
```

### A.2 Canonical Signing Payload and Transcript Example

This example is the shared vector [`testdata/sdk/secure_baseline_vector.json`](../testdata/sdk/secure_baseline_vector.json). Its key is a published test key and its timestamp is in the past, so a verifier with a real clock rejects it as stale; it is meant for checking encoding and signing.

**Sender identity (test only):**
- Ed25519 seed: `000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f`
- Public key: `03a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8`
- `did:key`: `did:key:z6MkehRgf7yJbgaGfYsdoAsKdBPE3dj2CYhowQdcjqSJgvVd` (base58btc of `ed01` followed by the public key)

**Envelope fields:** `version = 1`, `trace_id = "trace-vector"`, `profile = 1`, `context_patch = {context_id: "ctx", base_version: 7, ops: [{op: ADD, path: "/x", data: "1", ts: 1}]}`, `sequence = 42`, `recipient_did = "did:example:bob"`, `timestamp_ms = 1700000000123`.

**Canonical signing payload (41 bytes):**

```
08 01                                   version = 1
12 0c 74726163652d766563746f72          trace_id = "trace-vector"
18 01                                   profile = 1
22 12                                   context_patch (field 4), 18 bytes:
   0a 03 637478                           context_id = "ctx"
   10 07                                  base_version = 7
   1a 09                                  ops[0], 9 bytes:
      12 02 2f78                            path = "/x" (op = ADD = 0 omitted)
      1a 01 31                              data = "1"
      20 01                                 ts = 1
b0 01 2a                                sequence = 42 (field 22)
```

Base64: `CAESDHRyYWNlLXZlY3RvchgBIhIKA2N0eBAHGgkSAi94GgExIAGwASo=`

**Transcript (the sub-second part `123` is not included):**
```
AXCP-DID-AUTH-v1
did:key:z6MkehRgf7yJbgaGfYsdoAsKdBPE3dj2CYhowQdcjqSJgvVd
did:example:bob
CAESDHRyYWNlLXZlY3RvchgBIhIKA2N0eBAHGgkSAi94GgExIAGwASo=
2023-11-14T22:13:20Z
```

**Signature (Base64):** `+wNWeLDbJj+8fau341hY8OOkZN0tIzr0yjqQW8P3L30x8xWqNai0G71Qc0HHgt7q9+8PhkZAczQgtorNr6HvCQ==`

The vector file also contains the complete encoded envelope. [`testdata/sdk/repeated_defaults_vector.json`](../testdata/sdk/repeated_defaults_vector.json) covers default-valued elements in repeated fields.

### A.3 Implementation Reference (informative)

The Go SDK is the reference implementation of this specification:

| Component | Location |
|-----------|----------|
| Canonical signing payload and envelope transcript | `sdk/go/axcp/signature.go` |
| DID syntax, resolution and transcript primitive | `sdk/go/auth/did.go` |
| Ed25519 signing | `sdk/go/auth/signer.go` |
| Ed25519 verification | `sdk/go/auth/verifier.go` |
| Timestamp validation | `sdk/go/auth/timestamp.go` |
| Replay protection | `sdk/go/auth/replay.go` |
| Envelope validation | `sdk/go/axcp/validator.go` |
| Profile selection helper | `sdk/go/negotiate/profile.go` |
| Error codes | `proto/axcp.proto`, `sdk/go/axcp/errors.go` |

The Python and TypeScript SDKs MUST accept and reject the same envelopes as the Go SDK; the shared vectors in `testdata/sdk/` are part of their test suites.

---

*End of AXCP Core Specification v1.0*
