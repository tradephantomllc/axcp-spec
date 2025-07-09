# AXCP Quick Start Guide

This guide will help you set up a local AXCP mesh with two agents and a gateway in just a few minutes.

## Prerequisites

- **Go 1.21+** (for building the gateway and agents)
- **Protocol Buffers** compiler (`protoc`)
- **Git** (for cloning the repository)

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/tradephantom/axcp-spec.git
cd axcp-spec
```

### 2. Build the Components

```bash
# Build the gateway
make gateway

# Build the example agents
make examples
```

If you don't have `make` available, you can build manually:

```bash
# Build gateway
cd edge/gateway
go build -o ../../bin/gateway ./cmd/gateway

# Build example echo agent
cd ../../examples/go/echo_agent
go build -o ../../../bin/echo_agent .
```

## Running Your First AXCP Mesh

### Step 1: Start the Gateway

The gateway acts as the central coordination hub. Start it with basic configuration:

```bash
./bin/gateway --listen :4433 --profile 0
```

**Parameters**:
- `--listen :4433`: Listen on port 4433
- `--profile 0`: Use basic profile (no privacy constraints for development)

You should see output similar to:

```
2024-01-09 10:30:00 INFO  Starting AXCP Gateway
2024-01-09 10:30:00 INFO  Listening on :4433
2024-01-09 10:30:00 INFO  Profile: 0 (development)
2024-01-09 10:30:00 INFO  Ready to accept connections
```

### Step 2: Connect the First Agent

In a new terminal, start the first echo agent:

```bash
./bin/echo_agent --id agent1 --connect quic://127.0.0.1:4433
```

**Parameters**:
- `--id agent1`: Unique identifier for this agent
- `--connect quic://127.0.0.1:4433`: Connect to the gateway via QUIC

Expected output:

```
2024-01-09 10:30:05 INFO  Echo Agent starting (ID: agent1)
2024-01-09 10:30:05 INFO  Connecting to gateway at quic://127.0.0.1:4433
2024-01-09 10:30:05 INFO  Connection established
2024-01-09 10:30:05 INFO  Capabilities registered: [echo, ping]
2024-01-09 10:30:05 INFO  Agent ready
```

### Step 3: Connect the Second Agent

In another terminal, start the second echo agent:

```bash
./bin/echo_agent --id agent2 --connect quic://127.0.0.1:4433
```

You should see similar output, and the gateway will log the new connection.

### Step 4: Test Agent Communication

The echo agents automatically discover each other and can exchange messages. You should see periodic heartbeat messages and capability advertisements in the logs.

To send a test message from agent1 to agent2:

```bash
# In agent1's terminal, type:
echo "Hello from agent1!" | ./bin/echo_agent --id agent1 --connect quic://127.0.0.1:4433 --send-to agent2
```

### Step 5: Monitor with AXCP Control

Use the control tool to monitor the mesh:

```bash
# Install axcpctl (if not already built)
go install ./cmd/axcpctl

# Monitor live activity
axcpctl trace --live --gateway quic://127.0.0.1:4433
```

You should see a live stream of AXCP messages:

```
2024-01-09 10:30:15 agent1 → gateway: CapabilityOffer (echo, ping)
2024-01-09 10:30:15 gateway → agent2: CapabilityAnnouncement (agent1: echo, ping)
2024-01-09 10:30:20 agent2 → agent1: Message (echo: "Hello from agent1!")
2024-01-09 10:30:20 agent1 → agent2: Message (echo_response: "Hello from agent1!")
```

## Advanced Configuration

### Custom Profiles

Try different security profiles:

```bash
# Basic privacy (Profile 1)
./bin/gateway --listen :4433 --profile 1

# Enhanced privacy (Profile 2)
./bin/gateway --listen :4433 --profile 2 --dp-epsilon 1.0
```

### Multiple Gateways

Run a distributed setup with multiple gateways:

```bash
# Terminal 1: Primary gateway
./bin/gateway --listen :4433 --cluster-id primary

# Terminal 2: Secondary gateway
./bin/gateway --listen :4434 --cluster-id secondary --peer quic://127.0.0.1:4433

# Terminal 3: Agent connected to secondary
./bin/echo_agent --id agent3 --connect quic://127.0.0.1:4434
```

### Python Client

If you have Python installed, try the Python client:

```bash
pip install -r requirements.txt

# Run Python echo agent
python examples/python/echo_agent.py --id python-agent --gateway quic://127.0.0.1:4433
```

## Troubleshooting

### Common Issues

**Connection Refused**:
- Ensure the gateway is running and listening on the correct port
- Check firewall settings
- Verify the gateway address format: `quic://host:port`

**Certificate Errors**:
- For development, use `--insecure` flag with agents
- For production, ensure proper TLS certificates are configured

**Performance Issues**:
- Try different profiles (0 for development, 1-2 for production)
- Check system resources with `axcpctl status`
- Monitor logs for error messages

### Debug Mode

Enable debug logging for detailed troubleshooting:

```bash
# Gateway with debug logging
./bin/gateway --listen :4433 --log-level debug

# Agent with debug logging
./bin/echo_agent --id agent1 --connect quic://127.0.0.1:4433 --log-level debug
```

## What's Next?

Now that you have a basic AXCP mesh running, explore these topics:

1. **[Technical Specification](spec.md)**: Deep dive into the protocol details
2. **[Architecture](architecture.md)**: Understand the system design
3. **[Contributing](../CONTRIBUTING.md)**: Help improve AXCP
4. **[Roadmap](../ROADMAP.md)**: See what's coming next

### Building Your Own Agent

Create a custom agent using the AXCP SDK:

```go
// Go example
package main

import (
    "context"
    "log"
    "github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

func main() {
    client, err := axcp.NewClient(axcp.Config{
        GatewayURL: "quic://127.0.0.1:4433",
        AgentID:    "my-agent",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Register capabilities
    client.RegisterCapability("custom-task", handleCustomTask)
    
    // Start the agent
    ctx := context.Background()
    if err := client.Run(ctx); err != nil {
        log.Fatal(err)
    }
}

func handleCustomTask(ctx context.Context, req *axcp.Request) (*axcp.Response, error) {
    // Your custom logic here
    return &axcp.Response{
        Data: []byte("Task completed successfully"),
    }, nil
}
```

## Need Help?

- **GitHub Issues**: [Report bugs or ask questions](https://github.com/tradephantom/axcp-spec/issues)
- **Discussions**: [Join the community discussions](https://github.com/tradephantom/axcp-spec/discussions)
- **Documentation**: [Browse the full documentation](https://github.com/tradephantom/axcp-spec/tree/main/docs)

Happy coding with AXCP! 🚀