# TypeScript Transport

The TypeScript SDK exposes the AXCP stream framing contract without pretending
that Node has production-ready native QUIC in core.

## Decision

Phase 4 keeps QUIC support out of the default TypeScript dependency graph.
The local Node runtime does not expose `node:quic`, and the available QUIC or
WebTransport packages depend on native bindings. Pulling those into the core SDK
would make installability and CI portability worse for the main adoption path.

Instead, the SDK now provides:

- transport-independent AXCP frame encoding and decoding
- `NodeStreamConnection` for any ordered duplex stream
- `TlsClient` and `TlsServer` for production-oriented encrypted Node transport
- `TcpClient` and `TcpServer` for local development and harnesses only
- explicit QUIC capability detection through `detectNativeQuic()`

The public connection methods match the Go and Python framing contract:

- `sendMessage` / `receiveMessage`: 4-byte big-endian length prefix
- `sendEnvelope` / `receiveEnvelope`: 4-byte little-endian length prefix

## TLS Usage

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

Certificate verification is enabled by default. `insecureSkipVerify` exists only
as an explicit development escape hatch and should not be used in production.

## QUIC Roadmap

The future QUIC adapter should implement the same `NodeStreamConnection`
surface and preserve the existing framing tests. It should remain optional until
the runtime or dependency choice is stable enough for public SDK users.
