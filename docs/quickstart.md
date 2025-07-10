# AXCP Quick Start

Step-by-step for a local mesh of two echo-agents + gateway (profilo 0).

```bash
make gateway
./bin/gateway --listen :4433 &
./bin/echo_agent --id agent1 --connect quic://127.0.0.1:4433 &
…
```

End with `axcpctl trace --live`.