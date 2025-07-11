# AXCP Quickstart

> This guide shows how to spin up a minimal local mesh with two echo-agents and a gateway using **Profile 0** (no encryption, ideal for experimentation on localhost).

## Prerequisites

* Go 1.21+ (for building the reference binaries)
* `make`

```bash
# Build gateway and echo agents
make gateway echo_agent axcpctl
```

## Start the gateway

```bash
./bin/gateway --listen :4433 &
```

The gateway now listens for QUIC connections on port 4433.

## Start two echo agents

```bash
./bin/echo_agent --id agent1 --connect quic://127.0.0.1:4433 &
./bin/echo_agent --id agent2 --connect quic://127.0.0.1:4433 &
```

Each agent will register with the gateway and advertise a simple echo capability.

## Live trace

```bash
axcpctl trace --live
```

You should see envelopes flowing between agents and the gateway in real-time.

Press <kbd>Ctrl+C</kbd> to stop each background process when you are done.
