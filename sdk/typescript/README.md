# AXCP TypeScript SDK

Core TypeScript SDK for AXCP Secure Baseline.

This package covers the application-layer security surface:

- Ed25519 identity generation with `did:key` derivation
- DID parsing and minimal resolver interfaces
- deterministic AXCP protobuf envelope encoding
- AXCP DID auth transcript signing and verification
- timestamp validation
- sequence and nonce replay protection
- Secure Baseline profile negotiation
- transport-independent stream framing
- Node TLS stream transport

The TypeScript SDK does not bundle a native QUIC dependency yet. Node does not
currently expose a stable `node:quic` module in the supported runtime path, so
the SDK provides a stable stream transport surface plus a TLS adapter while QUIC
remains an optional adapter decision.

## Install locally

```bash
cd sdk/typescript
npm install
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
```

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
`sequence`, `signature`, and `attestation_proof`, matching the reference Go and
Python implementations.
