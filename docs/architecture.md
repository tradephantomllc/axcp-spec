# AXCP Core reference architecture

```text
Application / agent
  explicit identity + peer configuration
  construct payload -> sign envelope
                  |
           QUIC + TLS 1.3
       stream / 0xA1 telemetry
                  |
Reference gateway
  version/profile/payload validation
  trusted DID -> Ed25519 -> timestamp -> replay window
                  |
     MQTT QoS 1 publish / bounded retry
                  |
Application-owned MQTT consumer
```

Core supplies protocol and transport building blocks. The application owns routing, tool execution, context state, persistence, provisioning and authorization beyond the gateway's configured sender trust. Protobuf context patches and capability messages do not imply an automatic CRDT store or discovery service.

The Go and Python SDKs provide QUIC. TypeScript provides compatible framing and a Node TLS adapter; that TCP/TLS adapter cannot directly connect to the QUIC gateway. Rust's experimental JSON/HTTP telemetry adapter expects a separate HTTP service.

The gateway starts in Secure Baseline mode. It accepts configured sender DIDs and verifies signed envelopes before forwarding them to MQTT. Streams and signed datagrams share replay state. The gateway's profile is fixed by configuration; the SDK's signed ClientHello/ServerHello exchange is a separate opt-in integration primitive.

MQTT publisher acknowledgement confirms broker acceptance, not consumer processing. Retry is bounded and in memory; exhaustion, capacity limits, shutdown and crashes can lose pending messages. Uncertain acknowledgements can produce duplicates. Consumers must supply business-level idempotency if required.

Automatic orchestration, smart routing, CRDT synchronization, privacy budgets, managed mTLS/attestation and commercial governance are outside Core. A long-running deployment is operational evidence for that deployment, not proof that every public SDK, example or optional path has been exercised.
