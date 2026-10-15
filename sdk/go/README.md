# AXCP Go SDK

The reference SDK provides Protobuf envelopes, DID resolver interfaces, Ed25519 authentication primitives, `did:key` support, sequence-based replay protection, profile negotiation helpers and QUIC transport. Applications explicitly configure and connect these building blocks.

Requires Go 1.26.9 or newer on a supported Go release. From the repository root:

```sh
go test -race ./sdk/go/...
go build ./sdk/go/examples/telemetry_example
```

Public packages:

- `github.com/tradephantomllc/axcp-spec/sdk/go/axcp`: envelope/validation helpers
- `github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb`: public generated message aliases
- `github.com/tradephantomllc/axcp-spec/sdk/go/auth`: identity, signature, replay primitives
- `github.com/tradephantomllc/axcp-spec/sdk/go/negotiate`: policy and signed wire-handshake helpers
- `github.com/tradephantomllc/axcp-spec/sdk/go/netquic`: client/listener, framed streams and datagrams

`netquic` is part of the parent SDK module. See [module policy and migration](../../docs/dev/go-modules.md) before upgrading consumers that explicitly require the old nested module.

Envelope stream frames use a little-endian uint32 length, capped at 10 MiB before allocation. Raw `SendMessage`/`ReceiveMessage` frames use big-endian lengths with the same cap. Empty raw messages remain valid; empty envelope frames do not. A framing error requires discarding that stream. Stream transport does not itself authenticate envelopes; use the authentication/validation layer before handling a payload.

`SendTelemetry`/`ReceiveTelemetry` use legacy `0xA0 || TelemetryDatagram`, accepted by development transport-only gateways. `SendTelemetryEnvelope`/`ReceiveTelemetryEnvelope` use `0xA1 || AxcpEnvelope`. Sign outgoing envelopes and verify incoming envelopes (including recipient, timestamp and replay) at the application boundary. The datagram limit is 1200 bytes including prefix, subject to QUIC's negotiated limit. Delivery is unreliable.

`ClientHandshake` validates the signed response's version, freshness, offered profile, deprecated-profile policy and resolved capabilities. v1 ServerHello contains no client nonce or transcript binding: freshness is not a proof of a newly generated response. TLS peer verification and envelope replay protection remain required. Handshake helpers are not invoked automatically by the gateway or authenticated-chat demo. A timed-out custom StreamIO must be closed/cancelled by its owner to unblock pending I/O.

## Secure defaults

- `axcp.NewValidator` enables replay protection by default (opt out only with `DisableReplayProtection`, for tests) and raises the replay TTL to at least the timestamp max age plus clock skew. Every path (`ValidateAndVerify`, `ValidateWithResult`, `SecureValidatorMiddleware`, `SecureBaselineMiddleware`, `ServerConfig.BuildValidator`) checks version, profile, a single payload member, unknown fields, undefined enums, recipient, timestamp, signature length, sequence > 0 and the signature before replay state changes. `AllowedDIDs` restricts senders.
- `axcp.FromBytesStrict` is used by the QUIC receive paths and rejects envelopes with more than one payload member, unknown fields or undefined enum values.
- DIDs follow the W3C syntax with a 512-byte limit. `auth.EncodeDIDKey`, `auth.DecodeDIDKey`, `auth.GenerateIdentity` and `auth.DIDKeyResolver` implement Ed25519 `did:key`; a resolver can never bind a `did:key` to another key, and small-order keys are rejected.
- `auth.Session` starts sequences at the current Unix time in microseconds (restart-safe), requires a recipient and only signs payloads built with `PayloadForSequence`.
- `auth.CachingDIDResolver` caches validated keys, caches only definitive failures briefly and is bounded.
- Errors returned to peers carry fixed reasons, never internal error text.

See [quickstart](../../docs/quickstart.md) and [gateway setup](../../docs/gateway-setup.md) for runnable workflows. Core does not provide automatic context synchronization, discovery or orchestration.
