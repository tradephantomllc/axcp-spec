# Getting Started with AXCP Core

This guide helps you set up and run AXCP Core with Secure Baseline authentication.

## Prerequisites

- **Go 1.23+** (CI uses Go 1.23.4)
- Git

Verify your Go installation:

```bash
go version
# Expected: go version go1.23.x ...
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

All tests should pass. If you see import errors, ensure you're using Go 1.23+.

## Python SDK Core

The Python SDK covers the Secure Baseline application-layer contract:

- Ed25519 identity generation
- `did:key` derivation
- protobuf envelope encode/decode
- AXCP auth transcript signing and verification
- timestamp and replay validation
- profile negotiation primitives

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

Python QUIC transport is intentionally separate from this core SDK and will be
added after the signing/protobuf/replay contract remains stable.

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

### "go version" shows < 1.23

Update Go to version 1.23 or later. AXCP Core requires Go 1.23+.

### Import errors when running tests

Ensure you're in the repository root directory when running `go test ./sdk/go/...`.

### "connection refused" when running client

Make sure the server is running in a separate terminal before starting the client.

## Next Steps

- [Authentication](authentication.md) - Understand DID authentication and Ed25519 signing
- [Gateway Setup](gateway-setup.md) - Configure the gateway for Secure Baseline
- [Specification v1.0](../spec/axcp-v1.0.md) - Full protocol specification
