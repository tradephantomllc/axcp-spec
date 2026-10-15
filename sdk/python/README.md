# AXCP Python SDK

Core Python SDK for AXCP Secure Baseline.

This package covers the application-layer security surface:

- Ed25519 identity generation
- `did:key` derivation and parsing
- deterministic protobuf envelope encoding
- AXCP DID auth transcript signing and verification
- timestamp validation
- sequence replay protection
- Secure Baseline profile selection helpers (explicit integration required)
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

# For bytes received from the network, decode strictly and verify in one step:
verified = bob.verify_bytes(encoded_bytes)
```

## Verification rules

`Agent.verify` / `verify_envelope` enforce the AXCP Secure Baseline rules shared with
the Go and TypeScript SDKs:

- **Recipient binding:** `recipient_did` must be non-empty and equal the verifier's
  DID. `sign_message` / `sign_envelope` require `recipient_did`; `verify_envelope`
  requires the keyword argument `expected_recipient_did`.
- **Envelope:** `version == 1`, `profile == 1`, exactly one payload member, no unknown
  fields at any level, defined enum values, valid UTF-8, `sequence > 0`, 64-byte
  signature. `decode_envelope(data, strict=True)` (used by `Agent.verify_bytes` and,
  by default, by `receive_envelope`) also rejects two different payload members on
  the wire, which lenient protobuf parsing silently collapses.
- **DIDs:** W3C DID syntax, ASCII only, no path/query/fragment, at most 512 bytes
  (checked first). `did:key` must be `z` + at most 128 base58btc characters decoding
  to `0xed01` + a 32-byte Ed25519 key. Small-order Ed25519 keys are rejected.
- **Resolver precedence:** a configured resolver is used for every sender, including
  `did:key` (allowlist); for `did:key` the resolved key must equal the DID key.
  Without a resolver `did:key` is decoded directly and other methods are rejected.
- **Replay:** per-sender bitmap sliding window (default 1000). Idle senders are
  evicted after `peer_ttl` (default 10 minutes, raised to at least the timestamp max
  age + clock skew); at most `max_peers` (default 100000) senders are tracked and
  new senders raise `ReplayCapacityError` when full. `ttl=` is a deprecated alias of
  `peer_ttl=`.
- **Sequences:** `Agent` starts at the current Unix time in microseconds (or
  `initial_sequence`); `sign_envelope` never signs sequence 0.
- **Errors:** unauthenticated input only raises `AXCPError` subclasses (for example
  `TimestampInvalidError` for timestamps beyond year 9999). Transport errors now
  derive from `AXCPError` too.

`axcp.pb` always imports the bundled bindings; an application package named `proto`
can no longer shadow them.

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

Transport bounds: unread stream data never exceeds `max_frame_size + 4` bytes per
stream (the connection is closed instead); received datagrams are queued up to
`max_datagram_queue` (default 1024) and further ones are dropped and counted
(`client.dropped_datagrams`); receives without an explicit timeout use
`read_timeout` (default 300 s). Stream and datagram handler exceptions are logged,
exposed through `server.handler_errors` and the first one is re-raised by
`server.close()`.

QUIC half-closes are directional: `STOP_SENDING` rejects further writes with
`TransportClosedError` while leaving a peer's error response readable.
`RESET_STREAM` interrupts reads with the peer's error code. Closing a client is
idempotent and releases the underlying connection after either event. Frame and
datagram timeouts consistently raise `TransportTimeoutError` on Python 3.10
and later.

The gateway uses a fixed profile and publishes accepted envelopes to MQTT; it
does not send a success acknowledgement. Only call `receive_envelope()` when
your peer implements replies. For gateway telemetry datagrams,
prepend `0xA1` to the encoded signed envelope; raw `0xA0` telemetry is restricted
to explicit transport-only mode. See [gateway setup](../../docs/gateway-setup.md).

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
