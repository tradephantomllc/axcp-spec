# Getting Started with AXCP Core

This guide helps you set up and run AXCP Core with Secure Baseline authentication.

## Prerequisites

- **Go 1.25.6+** (CI uses Go 1.25.6)
- Git

Verify your Go installation:

```bash
go version
# Expected: go version go1.25.6 or later
```

## Installation

Clone the repository:

```bash
git clone https://github.com/tradephantomllc/axcp-spec.git
cd axcp-spec
```

## Verify Build

Run tests to verify everything is working:

```bash
go test ./sdk/go/...
```

All tests should pass. If you see import errors, ensure you're using Go 1.25.6+.

## Python SDK Core

The Python SDK covers the Secure Baseline application-layer contract:

- Ed25519 identity generation
- `did:key` derivation
- protobuf envelope encode/decode
- AXCP auth transcript signing and verification
- timestamp and replay validation
- profile negotiation primitives
- async QUIC stream transport

Install it locally from the repository:

```bash
python -m pip install -e sdk/python
pytest -q sdk/python/tests
```

Minimal signed-envelope roundtrip:

```python
from axcp import Agent, Identity
from axcp.pb import axcp_pb2

alice = Agent(Identity.generate())
bob = Agent(Identity.generate())

env = alice.new_envelope(trace_id="trace-001")
env.context_patch.CopyFrom(axcp_pb2.ContextPatch(context_id="ctx", base_version=1))

alice.sign_message(env, recipient_did=bob.identity.did)
bob.verify(env)
```

Python QUIC transport is available through `axcp.transport` and preserves the
Go SDK framing contract for both raw messages and protobuf envelopes.

## TypeScript SDK Core

The TypeScript SDK covers the same Secure Baseline application-layer contract
for Node applications:

- Ed25519 identity generation
- `did:key` derivation
- protobuf envelope encode/decode
- AXCP auth transcript signing and verification
- timestamp and replay validation
- profile negotiation primitives
- TLS stream transport and AXCP framing helpers

Install it locally from the repository:

```bash
cd sdk/typescript
npm install
npm test
```

Minimal signed-envelope roundtrip:

```typescript
import { Agent, Identity } from "@tradephantom/axcp";

const alice = new Agent(Identity.generate());
const bob = new Agent(Identity.generate());

const env = alice.newEnvelope({ traceId: "trace-001" });
env.contextPatch = { contextId: "ctx", baseVersion: 1, ops: [] };

alice.signMessage(env, { recipientDid: bob.identity.did });
await bob.verify(env);
```

TypeScript transport is available through `@tradephantom/axcp/transport`.
It provides the stable AXCP stream framing surface and a Node TLS adapter.
Native QUIC remains an optional adapter decision because the supported Node
runtime does not expose a stable built-in QUIC module.

## Running the Authenticated Chat Example

The recommended way to experience AXCP is through the `authenticated_chat` example, which demonstrates the **Secure Baseline (`secure-baseline-v1`)** profile with:

- DID-based identity
- Ed25519 signature generation and verification
- Local, in-memory DID resolution (fully offline)
- Bidirectional authenticated message exchange

### Terminal 1 - Start Server

```bash
cd examples/go/authenticated_chat
go run . -server
```

Expected output:
```
=== AXCP Authenticated Chat Server ===
Profile: secure-baseline-v1
DID Resolver initialized with 2 identities
Server DID: did:key:axcp-auth-chat-server
Listening on localhost:61301
```

### Terminal 2 - Run Client

```bash
cd examples/go/authenticated_chat
go run .
```

Expected output:
```
=== AXCP Authenticated Chat Client ===
Profile: secure-baseline-v1
Client DID: did:key:axcp-auth-chat-client
Server DID: did:key:axcp-auth-chat-server
...
--- Verifying Server Signature ---
  Server Signature VALID

Bidirectional authenticated exchange completed!
```

### What Happens

1. **Profile Negotiation**: Client and server agree on `secure-baseline-v1`
2. **DID Resolution**: Both parties resolve each other's DIDs from the shared in-memory resolver
3. **Message Signing**: Client signs its message with Ed25519
4. **Signature Verification**: Server verifies the client's signature
5. **Response Signing**: Server signs its response
6. **Bidirectional Verification**: Client verifies the server's signature

## Troubleshooting

### "go: command not found"

Install Go from https://go.dev/dl/ and ensure it's in your PATH.

### "go version" shows < 1.25.6

Update Go to version 1.25.6 or later. AXCP Core requires Go 1.25.6+.

### Import errors when running tests

Ensure you're in the repository root directory when running `go test ./sdk/go/...`.

### "connection refused" when running client

Make sure the server is running in a separate terminal before starting the client.

## Next Steps

- [Authentication](authentication.md) - Understand DID authentication and Ed25519 signing
- [Gateway Setup](gateway-setup.md) - Configure the gateway for Secure Baseline
- [Specification v1.0](../spec/axcp-v1.0.md) - Full protocol specification
