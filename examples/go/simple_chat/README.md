# Simple Chat (loop-back)

Minimal program that:

1. spins up an **AXCP echo server** on QUIC/localhost  
2. dials the server with the Go SDK  
3. sends an `AxcpEnvelope` with a `CapabilityRequest`  
4. prints the echoed `trace_id`

Run:

```bash
cd examples/go/simple_chat
go run .
```
