# AXCP • Go SDK (reference)

Minimal façade over the generated `axcp.proto` definitions.
Designed for experimentation and PoC clients.

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
