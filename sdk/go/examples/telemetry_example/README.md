# AXCP Telemetry Example

This example demonstrates how to use the AXCP telemetry functionality to send and receive telemetry data over QUIC.

This local example uses legacy `0xA0` telemetry and development TLS. It does not demonstrate Secure Baseline. For authenticated `0xA1` datagrams see the [gateway contract](../../../../docs/gateway-setup.md).

## Prerequisites

- Go 1.26.9 or later
- Git

## Building the Example

1. Clone the repository:
   ```bash
   git clone https://github.com/tradephantomllc/axcp-spec.git
   cd axcp-spec/sdk/go/examples/telemetry_example
   ```

2. Download dependencies:
   ```bash
   go mod download
   ```

3. Build the example:
   ```bash
   go build -o telemetry_example
   ```

## Running the Example

### Start the Server

In one terminal, start the server:

```bash
./telemetry_example -server -addr localhost:4242
```

### Run the Client

In another terminal, run the client to send telemetry data:

```bash
./telemetry_example -addr localhost:4242
```

## How It Works

1. The server starts a QUIC listener on the specified address.
2. The client connects to the server and sends telemetry data including:
   - System statistics (CPU, memory, temperature)
   - Token usage (prompt and completion tokens)
3. The server receives and displays the telemetry data.

## Code Structure

- `main.go`: Contains both client and server implementations.
- Dependencies are defined by the parent `sdk/go/go.mod`.

## Customization

You can modify the example to:
- Add more telemetry data types
- Implement custom processing of received telemetry data
- Add application authentication (the local example already uses self-signed QUIC/TLS)
- Implement a persistent storage backend for telemetry data

## License

This example is part of the AXCP specification and is licensed under the same terms.

## Troubleshooting

- If you see certificate errors, ensure your system clock is synchronized.
- Make sure the server is running before starting the client.
- Check that the specified port is not in use by another application.
