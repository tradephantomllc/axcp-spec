# Simple Chat (loop-back)

A minimal example demonstrating the Adaptive eXchange Context Protocol (AXCP) over QUIC.

## About AXCP

AXCP (Adaptive eXchange Context Protocol) is a protocol designed for efficient, secure communication between distributed AI agents with adaptive behavior based on context and requirements.

## Example Overview

This minimal program demonstrates:

1. spins up an **AXCP echo server** on QUIC/localhost  
2. dials the server with the Go SDK  
3. sends an `AxcpEnvelope` with a `CapabilityRequest`  
4. prints the echoed `trace_id`

Run:

```bash
cd examples/go/simple_chat
go run .
```
