# AXCP Core Authentication

This document describes the authentication model in AXCP Core's **Secure Baseline (`secure-baseline-v1`)** profile.

> **Authoritative Reference**: The canonical specification is [AXCP Core Specification v1.0](../spec/axcp-v1.0.md). This document provides operational guidance aligned with that specification. If the two disagree, the specification wins.

> **Compatibility**: the Secure Baseline transcript changed after the initial 1.0.0 tag while keeping the label `AXCP-DID-AUTH-v1`. Peers built against the original 1.0.0 transcript cannot verify signatures from current peers, and vice versa. Upgrade all peers together. See the specification's [Revision History](../spec/axcp-v1.0.md#revision-history).

## Overview

AXCP Core uses DID-based message authentication with Ed25519 signatures. In Secure Baseline, every envelope must be:

1. Signed by the sender with Ed25519 over a transcript that binds the sender DID, the recipient DID, the canonical message bytes (including the replay sequence) and the timestamp
2. Addressed to an explicit recipient: the verifier only accepts envelopes whose `recipient_did` is its own DID
3. Verified by the recipient using the sender's public key, obtained through a DID resolver or from a `did:key`
4. Fresh (timestamp within the allowed age and clock skew) and not a replay

Secure Baseline is configured on each endpoint. The reference gateway uses a fixed configured profile; SDK handshake helpers are optional and are not required to establish Secure Baseline.

## Security Profiles

| Profile | Envelope `profile` | Authentication | Replay Protection | Production Use |
|---------|--------------------|----------------|-------------------|----------------|
| `secure-baseline-v1` | `1` | Required | Required | **Recommended** |
| `transport-only-v0` | `0` | Not required | Not required | **Deprecated** |

The `secure-baseline-v1` profile is the only profile suitable for production. A Secure Baseline verifier rejects every envelope whose `profile` is not `1` and whose `version` is not `1`.

## DID Model

### DID Format

DIDs have the form `did:<method>:<method-specific-id>`. The method name uses lowercase letters and digits; the identifier uses letters, digits, `.`, `-`, `_`, `:` and `%`-escapes. DIDs are ASCII only, contain no whitespace, path, query or fragment, and are at most 512 bytes long. See [specification §6.1](../spec/axcp-v1.0.md#61-did-syntax) for the exact grammar.

Examples:
- `did:key:z6MkehRgf7yJbgaGfYsdoAsKdBPE3dj2CYhowQdcjqSJgvVd`: a valid Ed25519 `did:key` (the public test identity of the shared SDK vector; never use it in a deployment)
- `did:example:my-agent`: a placeholder for a DID that your resolver knows

A `did:key` used with AXCP must encode an Ed25519 key: the identifier starts with `z` (base58btc), is at most 128 characters, and decodes to the multicodec prefix `0xed 0x01` followed by the 32-byte key. Strings such as `did:key:my-agent` are not valid `did:key` identifiers.

### DID Resolver Abstraction

AXCP Core does **not** provide a network-based DID resolver. Instead, it defines a `DIDResolver` interface that implementations inject:

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
resolver.AddDID("did:example:my-agent", publicKey)
```

**For production**, implement a resolver that queries your DID infrastructure (for example `did:web` or a private registry).

Resolution rules (specification [§6.4](../spec/axcp-v1.0.md#64-did-resolution)):

- When a resolver is configured, it is used for **every** DID method, including `did:key`. It acts as the allowlist of accepted senders. For a `did:key`, the key returned by the resolver must equal the key encoded in the DID.
- Without a resolver, only `did:key` is accepted and it is decoded directly ("open mode"). Open mode proves key possession, not authorization: anyone can generate a `did:key`. Add your own authorization policy or configure a resolver.
- Every resolution result, including cached results, is validated: the document ID must equal the DID, and the first key with an allowed type and a 32-byte length is used.

See `examples/go/authenticated_chat/main.go` for an example using the in-memory resolver.

## Ed25519 Signing

AXCP Core uses Ed25519 for all cryptographic signatures. Only the following key types are allowed in DID documents:

- `Ed25519VerificationKey2020`
- `Ed25519VerificationKey2018`

Verifiers also reject small-order (weak) Ed25519 public keys; the blocklist is in [specification §6.3](../spec/axcp-v1.0.md#63-key-type-allowlist-and-key-validation). Signatures are exactly 64 bytes.

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

The signature is computed over a **canonical transcript**. The exact format is defined in [AXCP Core Specification v1.0, Section 6.5](../spec/axcp-v1.0.md#65-canonical-transcript-format).

**Transcript version prefix**: `AXCP-DID-AUTH-v1`

**Format**:
```
AXCP-DID-AUTH-v1
<senderDID>
<recipientDID>
<base64(canonicalSigningPayload)>
<timestampRFC3339>
```

The canonical signing payload ([specification §6.6](../spec/axcp-v1.0.md#66-canonical-signing-payload)) is the byte-exact concatenation of envelope fields `version` (1), `trace_id` (2), `profile` (3), the single payload member (one of fields 4–11, deterministically encoded) and `sequence` (22). Zero-valued scalars are omitted. The detached authentication fields are excluded:

- `sender_did` (20)
- `timestamp_ms` (21)
- `recipient_did` (23)
- `signature` (100)
- `attestation_proof` (101)

The replay `sequence` field remains covered by the signing payload, so sequence tampering invalidates the signature before replay state is checked or mutated.

The timestamp is authenticated **at second precision**: the transcript contains `timestamp_ms` truncated to whole seconds as `YYYY-MM-DDTHH:MM:SSZ`. The millisecond part of `timestamp_ms` is not authenticated and must not be used for security decisions.

Because unknown fields would not be covered by the signature, verifiers reject envelopes with unknown fields, with more than one payload member, with undefined enum values or with invalid UTF-8.

**Important**: Do not construct envelope transcripts manually. Use the SDK envelope helper:

```go
transcript, err := axcp.BuildAuthTranscript(envelope)
```

For lower-level integrations, `auth.BuildDIDAuthTranscript(senderDID,
recipientDID, canonicalSigningPayload, timestamp)` remains the transcript
primitive. Its third argument is the canonical signing payload bytes, not a
standalone challenge.

A complete worked example with byte-level breakdown is in [specification Appendix A.2](../spec/axcp-v1.0.md#a2-canonical-signing-payload-and-transcript-example).

## Recipient Binding

Every Secure Baseline envelope must name its recipient. Signers require an explicit, non-empty recipient DID. Verifiers reject envelopes whose `recipient_did` is missing or differs from their own DID. This prevents an envelope addressed to one party from being replayed to another.

## Replay Protection

AXCP Core prevents replay attacks with per-sender sequence numbers and a sliding window. Secure Baseline envelopes have no nonce field, so nonce-based replay protection is not available in this profile.

### Configuration Parameters

| Parameter | Default | Constraint |
|-----------|---------|------------|
| Window `W` | 1000 | `W >= 1` |
| Peer-state TTL (idle sender eviction) | 10 minutes | Must be at least timestamp maximum age + clock skew |
| Timestamp maximum age | 5 minutes | |
| Future clock skew | 30 seconds | |
| Maximum tracked senders | 100 000 | New senders are rejected when full |

The peer-state TTL rule matters: if replay state for a sender expired before that sender's envelopes became stale, a recorded envelope could be accepted again. Implementations raise a smaller configured TTL to the sum of maximum age and clock skew.

### Sequence Semantics

`sequence` must be greater than 0. For each authenticated envelope with sequence `s`, where `H` is the highest accepted sequence for that sender:

1. **First message** from the sender: accept, `H = s`
2. **`s > H`**: accept, advance the window, `H = s`
3. **`H - W < s <= H`**: reject as a replay if `s` was already accepted, otherwise accept and record it
4. **`s <= H - W`**: reject as too old

Replay state is updated only after the signature and all other checks have passed.

### Sender Guidance

- Never reuse a sequence number for the same sender DID.
- Start each new signing session at the current Unix time in microseconds, so that sequences keep increasing across restarts.
- If one identity sends from several processes or connections, coordinate sequence allocation.

See [AXCP Core Specification v1.0, Section 7](../spec/axcp-v1.0.md#7-replay-protection) for complete semantics.

## Required Rejections

In Secure Baseline, verifiers MUST reject envelopes that:

- Cannot be parsed, or contain unknown fields, more than one (or no) payload member, undefined enum values or invalid UTF-8
- Have `version` other than 1 or `profile` other than 1
- Have a missing or invalid sender DID (syntax, 512-byte limit, invalid `did:key`)
- Have a missing recipient DID or one that is not the verifier's own DID
- Have a missing, stale or future timestamp
- Reference DIDs that cannot be resolved, whose document ID does not match, that lack an acceptable Ed25519 key, or whose key is small-order
- Have a missing signature, a signature that is not 64 bytes, or a signature that fails verification
- Have `sequence` 0, a replayed sequence, or a sequence outside the window

The full table and the error code to report for each condition are in [specification §8](../spec/axcp-v1.0.md#8-message-handling-requirements). There is no retry logic for authentication failures. Implementations must **fail fast**, and error replies to peers must not contain internal error strings.

## Code References

| Component | Location |
|-----------|----------|
| DID types and transcript | `sdk/go/auth/did.go` |
| Envelope signing payload | `sdk/go/axcp/signature.go` |
| Ed25519 signing | `sdk/go/auth/signer.go` |
| Ed25519 verification | `sdk/go/auth/verifier.go` |
| Timestamp validation | `sdk/go/auth/timestamp.go` |
| Replay protection | `sdk/go/auth/replay.go` |
| Session management | `sdk/go/auth/session.go` |
| Profile selection helper | `sdk/go/negotiate/profile.go` |

## Example

For a working example of authenticated communication, see:

```
examples/go/authenticated_chat/
```

This example demonstrates:
- Deterministic keypair generation
- Shared in-memory DID resolver
- Message signing and verification
- Bidirectional authenticated exchange

It is a localhost demonstration with fixed test keys and its own ALPN; it is not a gateway client.

## Integration boundaries

The reference gateway is configured for a fixed profile and defaults to Secure Baseline. SDK wire-handshake helpers must be explicitly integrated by applications; a fresh signed v1 hello is not bound to a client nonce. Authenticated datagrams (`0xA1`) and stream envelopes share the gateway's signature/replay boundary; raw legacy telemetry (`0xA0`) is rejected in secure mode.

Replay state is in memory and keyed by sender. Coordinate sequence allocation when one identity has concurrent connections, and account for receiver restarts: until the maximum age plus clock skew has passed, a restarted receiver cannot detect replays of envelopes it accepted before the restart. The MCP conversion example uses Python SDK signing on the complete envelope after sequence allocation; absent keys or an unconfigured secure session fail explicitly. It is a conversion demo, not a full MCP protocol adapter.
