# AXCP • Go SDK (reference)

Minimal façade over the generated `axcp.proto` definitions.
Designed for experimentation and PoC clients.

## Module Policy

> **Important**: `sdk/go` is the **single Go module** for AXCP Core.

- Nested modules under `sdk/go/` are **not allowed**
- The `internal/` packages are only consumed within the `sdk/go` module
- External packages (Advanced, Enterprise) import `github.com/tradephantomllc/axcp-spec/sdk/go`
- Public types are re-exported via `axcp/pb/export.go` for external consumption

This policy prevents module boundary violations with `internal/` packages.

## Usage

```bash
# generate/update protobuf stubs
protoc -I ../../proto \
      --go_out=internal/pb --go_opt=paths=source_relative \
      ../../proto/axcp.proto

# run unit tests
go test ./...
```

## Roadmap

- [ ] QUIC client helpers (`netquic`)
- [ ] Automatic profile negotiation
- [ ] Streaming context-sync examples
