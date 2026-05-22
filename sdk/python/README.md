# AXCP Python SDK

Core Python SDK for AXCP Secure Baseline.

This package covers the application-layer security surface:

- Ed25519 identity generation
- `did:key` derivation and parsing
- deterministic protobuf envelope encoding
- AXCP DID auth transcript signing and verification
- timestamp validation
- sequence replay protection
- Secure Baseline profile negotiation

QUIC transport is intentionally out of scope for Phase 1 and will be added as
a separate transport package once the core envelope contract is stable.

## Install locally

```bash
python -m pip install -e sdk/python
```

## Minimal usage

```python
from axcp import Agent, Identity
from axcp.pb import axcp_pb2

alice = Agent(Identity.generate())
bob = Agent(Identity.generate())

env = alice.new_envelope(trace_id="trace-001")
env.context_patch.CopyFrom(
    axcp_pb2.ContextPatch(context_id="ctx", base_version=1)
)

alice.sign_message(env, recipient_did=bob.identity.did)
bob.verify(env)
```

## Verification

```bash
python -m pip install -e sdk/python
pytest -q sdk/python/tests
```

## Compatibility

The Python SDK uses the same `proto/axcp.proto` source as the Go SDK. The
signed transcript matches the Go Secure Baseline format:

```text
AXCP-DID-AUTH-v1
<sender DID>
<recipient DID>
<base64(deterministic protobuf payload without auth fields)>
<timestamp RFC3339 seconds>
```
