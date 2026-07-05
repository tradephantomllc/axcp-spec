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
- async QUIC stream transport with AXCP envelope framing
- QUIC DATAGRAM helpers for telemetry-sized payloads

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

## QUIC transport

The transport module uses `aioquic` and preserves the Go SDK framing contract:

- `send_message` / `receive_message`: 4-byte big-endian length prefix
- `send_envelope` / `receive_envelope`: 4-byte little-endian length prefix
- `send_datagram` / `receive_datagram`: QUIC DATAGRAM payloads without stream framing

Minimal client usage:

```python
from axcp import QuicClient, QuicClientConfig

client = await QuicClient.connect(
    QuicClientConfig(host="127.0.0.1", port=61300, server_name="localhost")
)
try:
    await client.send_envelope(env)
    reply = await client.receive_envelope()
finally:
    await client.close()
```

Certificate verification is enabled by default. Use `ca_file` for private CA
deployments. `insecure_skip_verify=True` is available only for explicit local
development and tests.

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

The signed payload excludes `sender_did`, `recipient_did`, `timestamp_ms`,
`signature`, and `attestation_proof`. `sequence` remains covered by the
signature because replay protection consumes it.
