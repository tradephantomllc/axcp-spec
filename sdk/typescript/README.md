# AXCP TypeScript SDK

Core TypeScript SDK for AXCP Secure Baseline.

This package covers the application-layer security surface:

- Ed25519 identity generation with `did:key` derivation
- DID parsing and minimal resolver interfaces
- deterministic AXCP protobuf envelope encoding
- AXCP DID auth transcript signing and verification
- timestamp validation
- sequence and nonce replay protection
- Secure Baseline profile selection helpers (explicit integration required)
- transport-independent stream framing
- Node TLS stream transport

The TypeScript SDK does not bundle a native QUIC dependency yet. Node does not
currently expose a stable `node:quic` module in the supported runtime path, so
the SDK provides a stable stream transport surface plus a TLS adapter while QUIC
remains an optional adapter decision.

## Install locally

```bash
cd sdk/typescript
npm ci
npm test
```

The transport tests generate a short-lived local certificate with `openssl`.

## Minimal usage

```typescript
import { Agent, Identity } from "@tradephantom/axcp";

const alice = new Agent(Identity.generate());
const bob = new Agent(Identity.generate());

const envelope = alice.newEnvelope({ traceId: "trace-001" });
envelope.contextPatch = {
  contextId: "ctx",
  baseVersion: 1,
  ops: [],
};

alice.signMessage(envelope, { recipientDid: bob.identity.did });
await bob.verify(envelope);

// For bytes received from the network, decode strictly and verify in one step:
const verified = await bob.verifyBytes(encodedBytes);
```

## Verification rules

`Agent.verify` / `verifyEnvelope` enforce the AXCP Secure Baseline rules shared with
the Go and Python SDKs:

- **Recipient binding:** `recipient_did` must be non-empty and equal the verifier's DID.
  `signMessage` / `signEnvelope` require `recipientDid`; `verifyEnvelope` requires
  `expectedRecipientDid`.
- **Envelope:** `version == 1`, `profile == 1`, exactly one payload member, defined
  enum values, valid UTF-8, `sequence > 0`, 64-byte signature.
  `decodeEnvelope(bytes, { strict: true })` (used by `Agent.verifyBytes` and, by default,
  by `receiveEnvelope`) additionally rejects unknown fields at any level and two
  different payload members on the wire. Repeated occurrences of the same payload
  member are merged as in Go/Python.
- **DIDs:** W3C DID syntax, ASCII only, no path/query/fragment, at most 512 bytes
  (checked first). `did:key` must be `z` + at most 128 base58btc characters decoding to
  `0xed01` + a 32-byte Ed25519 key. Small-order Ed25519 keys are rejected everywhere.
- **Resolver precedence:** when a resolver is configured it is used for every sender,
  including `did:key` (it acts as an allowlist); for `did:key` the resolved key must
  equal the DID key. Without a resolver `did:key` is decoded directly and other
  methods are rejected.
- **Replay:** per-sender bitmap sliding window (default 1000). Idle senders are evicted
  after `peerTtlMs` (default 10 minutes, raised to at least the timestamp max age +
  clock skew); at most `maxPeers` (default 100000) senders are tracked and new
  senders are rejected with `ReplayCapacityError` when full.
- **Sequences:** `Agent` starts at the current Unix time in microseconds (or
  `initialSequence`), so restarted senders stay ahead of the verifier's window.
  `signEnvelope` never signs sequence 0.
- **Errors:** unauthenticated input only raises SDK errors (`AXCPError` subclasses);
  uint64 values above 2^53 - 1 are rejected with `InvalidEnvelopeError`, timestamps
  outside year 0-9999 with `TimestampInvalidError`.

`Identity` never exposes its private seed through `util.inspect`, `console.log` or
`JSON.stringify`; use `exportPrivateSeed()` or `toJson()` (persistence format,
includes the seed) explicitly. The `privateSeed` getter is deprecated.

## TLS transport

```typescript
import { TlsClient } from "@tradephantom/axcp/transport";

const client = await TlsClient.connect({
  host: "127.0.0.1",
  port: 61300,
  serverName: "localhost",
  ca: trustedCaPem,
});

try {
  await client.sendEnvelope(envelope);
  const reply = await client.receiveEnvelope({ timeoutMs: 5_000 });
} finally {
  await client.close();
}
```

The transport framing matches the Go and Python SDKs:

- `sendMessage` / `receiveMessage`: 4-byte big-endian length prefix
- `sendEnvelope` / `receiveEnvelope`: 4-byte little-endian length prefix

Transport bounds: TLS 1.3 is the default minimum version (`minVersion`). Unread
buffered bytes never exceed `maxFrameSize + 4` (the connection is closed instead),
oversized frame headers fail immediately, and receives without `timeoutMs` use
`readTimeoutMs` (default 5 minutes). Stream-handler errors go to
`onConnectionError`; without it they are kept in `server.handlerErrors` and the
first one is re-thrown by `server.close()`. `TcpClient` / `TcpServer` are plaintext
and intended for local development only.

TLS-over-TCP is a separate transport: `TlsClient` cannot connect directly to the
reference QUIC gateway. Use a compatible TLS peer or integrate a QUIC adapter.

## Compatibility

The TypeScript SDK uses the same `proto/axcp.proto` field numbers and the same
Secure Baseline transcript shape as the Go and Python SDKs:

```text
AXCP-DID-AUTH-v1
<sender DID>
<recipient DID>
<base64(deterministic protobuf payload without auth fields)>
<timestamp RFC3339 seconds>
```

The signed payload excludes `sender_did`, `recipient_did`, `timestamp_ms`,
`signature`, and `attestation_proof`. `sequence` remains covered by the
signature because replay protection consumes it, matching the reference Go and
Python implementations.

Repeated scalar values retain their positions, including empty strings and zero
values. Canonical encoding/signatures are checked against shared Go/Python
vectors in `testdata/sdk`; omitted singular proto3 defaults are a different case.
