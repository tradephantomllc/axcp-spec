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

Transport is intentionally out of scope for this phase. QUIC support will be
introduced separately after the core contract is stable.

## Install locally

```bash
cd sdk/typescript
npm install
npm test
```

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
