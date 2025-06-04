# Simple Chat Example Notes

This example demonstrates a basic AXCP (Adaptive eXchange Context Protocol) echo client-server implementation using QUIC for transport.

## Key Components

1. **Server**
   - Listens on localhost using QUIC
   - Implements a simple echo handler that responds to AXCP messages
   - Runs on port 4242 by default

2. **Client**
   - Connects to the local server
   - Sends a CapabilityRequest message
   - Prints the trace ID from the server's response

## Purpose

This example serves as a minimal working example (MWE) for:
- Setting up an AXCP server with QUIC transport
- Creating an AXCP client
- Basic message exchange using the Adaptive eXchange Context Protocol

## Dependencies

- Go 1.23.4 or later
- github.com/quic-go/quic-go for QUIC transport
- github.com/tradephantom/axcp-spec/sdk/go/axcp for AXCP protocol implementation

## Next Steps

- Modify the message handler to implement custom logic
- Add more complex message types
- Implement proper error handling and timeouts
- Add logging for better debugging
