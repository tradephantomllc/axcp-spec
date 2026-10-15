# Simple Chat (QUIC loop-back)

A minimal QUIC echo between a local client and server, built with [quic-go](https://github.com/quic-go/quic-go). It belongs to the repository root module `github.com/tradephantomllc/axcp-spec`.

## What it is (and is not)

- The client sends the plain-text string `Hello, QUIC server!` over one QUIC stream; the server logs it and replies `Hello from server!`.
- It does **not** send AXCP envelopes, sign messages or use Secure Baseline. For an authenticated AXCP exchange see [`../authenticated_chat`](../authenticated_chat/README.md); for the gateway see [gateway setup](../../../docs/gateway-setup.md).
- It uses a throwaway self-signed certificate, skips certificate verification and uses its own ALPN `simple-chat` (AXCP endpoints use `axcp/1`). Use it only on localhost.

## Requirements

- Go 1.26.9 or newer

## Run

The server and the client are the same program; the `-server` flag selects the role. Both use `localhost:61300`.

Terminal 1, from the repository root:

```bash
go run ./examples/go/simple_chat -server
```

Terminal 2, from the repository root:

```bash
go run ./examples/go/simple_chat
```

The client prints the server's reply and exits. The server handles one connection and then keeps running; stop it with Ctrl+C.
