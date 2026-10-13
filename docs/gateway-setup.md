# AXCP gateway setup

The reference gateway receives AXCP envelopes over QUIC/TLS and publishes them to MQTT. Secure Baseline is enabled by default. It uses a fixed configured profile; the SDK wire handshake is not run automatically.

## Build and configure

From the repository root, with Go 1.26.9 or newer:

```sh
go build -o /tmp/axcp-gateway ./edge/gateway/cmd/gateway
/tmp/axcp-gateway -help
```

Provision a persistent TLS certificate/key and sender identities using your own deployment tooling. Configure MQTT access controls for the gateway and consumers. The gateway uses a repeatable allowlist of sender DID/public-key pairs; possession of a key outside that list does not grant access.

```sh
/tmp/axcp-gateway \
  -addr :7143 \
  -mqtt-url tcp://127.0.0.1:1883 \
  -tls-cert /etc/axcp/server.crt \
  -tls-key /etc/axcp/server.key \
  -server-did "$AXCP_SERVER_DID" \
  -trusted-did "$AXCP_TRUSTED_DID"
```

`AXCP_SERVER_DID` is the exact nonempty recipient DID clients must include and sign for; the gateway never fills in an omitted recipient. `AXCP_TRUSTED_DID` is `did:<method>:<identifier>=<base64-encoded raw 32-byte Ed25519 public key>`. Supply actual identities, not example labels. Keep private seeds out of arguments, logs and Git. Clients must validate the server's TLS certificate and use ALPN `axcp/1`.

Missing/invalid security settings are rejected before connecting to MQTT or starting retry workers. `-allow-insecure-demo-tls` enables an ephemeral self-signed certificate for local tests; it does not disable envelope authentication. `-secure-baseline=false` separately and explicitly enables deprecated transport-only mode. Profile 0 still uses TLS encryption.

## Container deployment

`docker-compose.yml` starts the gateway and a local development MQTT broker. Supply an absolute `AXCP_TLS_DIR` containing `server.crt` and `server.key`, readable by container UID 65532, plus the two public identity settings above:

```sh
export AXCP_TLS_DIR=/absolute/path/to/tls
export AXCP_SERVER_DID='did:example:gateway'
# Set this to the real sender DID and public key:
export AXCP_TRUSTED_DID='did:example:agent=BASE64_PUBLIC_KEY'
docker compose config
docker compose up --build
```

Replace the illustrative identities and key before starting. The development broker is bound to loopback on host port 1883 and permits anonymous local access. It is not a production MQTT security configuration. The gateway listens on host UDP 7143. Stop only this stack with `docker compose down`.

## Wire and delivery contract

- Stream: four-byte **little-endian** envelope length, then Protobuf `AxcpEnvelope`; maximum 10 MiB; empty frames are rejected. SDK raw handshake messages instead use **big-endian** framing.
- Signed telemetry: `0xA1 || AxcpEnvelope`, with a telemetry payload, `version=1`, `profile=1`, signature, recipient, timestamp and sequence. The same verifier/replay state is used for streams and datagrams.
- Legacy telemetry: `0xA0 || TelemetryDatagram`, accepted only in explicit transport-only mode. The previous Go helper's unprefixed-envelope datagram representation was incompatible with the gateway; migrate producers and consumers together.
- Datagram application limit: 1200 bytes including prefix. QUIC may impose a smaller negotiated limit; send errors must be handled. DATAGRAM remains unreliable, unordered and without application acknowledgement.
- Signed envelopes are Base64-encoded Protobuf on `axcp/envelope`, preserving authentication fields. Legacy telemetry is raw Protobuf on `telemetry/<trace_id>`.
- MQTT uses QoS 1 and waits up to 10 seconds for publisher acknowledgement. This is broker acceptance, not downstream processing. A timeout leaves delivery uncertain, and retries can produce duplicates.

Retry defaults: 1000 queued messages, 5 retry attempts, an immediate first retry and subsequent exponential delays from 1 second up to 5 minutes. The complete envelope and routing are preserved. Queue entries are copied so later caller mutation cannot change a retry. The queue is in memory, is not an exactly-once guarantee and is not drained persistently at shutdown. Use `-retry=false` to disable it. No retry is performed for rejected authentication.

## Authentication and errors

Secure mode validates envelope version/profile and payload, then DID/signature, timestamp and replay policy before invoking a handler. Default freshness: 5-minute maximum age and 30-second future tolerance; replay TTL: 10 minutes; sequence window: 1000, shared by a sender across transports/connections. Sequence state must be coordinated by an application using one identity concurrently. Restarting the gateway loses replay state; use application idempotency and operational controls if replay resistance across restart is required.

| Rejection | Wire error code |
|---|---|
| Missing/invalid signature | `10` (`AUTH_SIGNATURE_INVALID`) |
| Missing/unknown/invalid DID, missing/wrong recipient | `17` (`AUTH_DID_INVALID`) |
| Duplicate sequence | `11` (`AUTH_REPLAY_DETECTED`) |
| Expired/future timestamp or sequence outside window | `18` (`AUTH_TIMESTAMP_EXPIRED`) |
| Invalid envelope version/profile/payload/frame | `MALFORMED_REQUEST` |

A rejected stream receives an unsigned diagnostic error and its read side is cancelled; do not treat that diagnostic as a signed peer message. Invalid datagrams are discarded. A malformed connection need not close other streams. SIGINT/SIGTERM stops the listener and active connections, then stops retry workers and disconnects MQTT.

## Test the implementation

```sh
go test -race ./edge/gateway/... ./sdk/go/netquic/...
```

The suite exercises actual localhost QUIC streams and DATAGRAM negotiation, valid signed delivery, shared replay state, mutation/unsigned rejection, bounded framing and MQTT acknowledgement handling. The separate authenticated-chat example uses its own server and is not a gateway client.

See [the Core specification](../spec/axcp-v1.0.md) and [authentication](authentication.md) for transcript details. `edge/gateway/internal` is a reference binary implementation, not a public import surface.
