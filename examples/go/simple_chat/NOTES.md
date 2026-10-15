# Simple Chat Example Notes

This example is a plain QUIC echo client and server in one program (`main.go`). It demonstrates QUIC transport setup only; it does not use AXCP envelopes or the AXCP SDK.

## Key Components

1. **Server** (`-server` flag)
   - Listens with QUIC on `localhost:61300`
   - Accepts one connection and one stream, logs the received text and replies `Hello from server!`

2. **Client** (default)
   - Connects to `localhost:61300`
   - Sends `Hello, QUIC server!` and prints the reply

Both sides use a self-signed certificate generated at startup, `InsecureSkipVerify` and the example ALPN `simple-chat`. This is for local testing only.

## Dependencies

- Go 1.26.9 or later
- `github.com/quic-go/quic-go`
- Module: `github.com/tradephantomllc/axcp-spec` (repository root `go.mod`)

## Next Steps

- See `examples/go/authenticated_chat` for signed AXCP envelopes with Secure Baseline.
- See `docs/gateway-setup.md` for the reference gateway, which uses ALPN `axcp/1`.
