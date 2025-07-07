# AXCP Core – Quick Start (Local Mesh)

This guide spins up a minimal **local mesh** with two echo agents and a gateway using **Profile 0** (development mode).

## Prerequisites

* Go 1.20+
* `make` and a POSIX-compatible shell (or adapt the commands for PowerShell)

## Steps

```bash
# 1. Build gateway and agent binaries
make gateway echo_agent

# 2. Start the gateway (listens on QUIC port 4433)
./bin/gateway --listen :4433 &

# 3. Launch two echo agents and connect them to the gateway
./bin/echo_agent --id agent1 --connect quic://127.0.0.1:4433 &
./bin/echo_agent --id agent2 --connect quic://127.0.0.1:4433 &

# 4. Verify live context exchange
axcpctl trace --live
```

`axcpctl trace` should display the delta traffic between *agent1* and *agent2* as they exchange echo messages.

> **Tip**: Stop the processes with `Ctrl+C` or `kill %1 %2 %3` depending on your shell.

For production-grade deployments refer to the upcoming **v0.4** documentation (Gateway clustering, encrypted profiles, remote OTEL streaming).
