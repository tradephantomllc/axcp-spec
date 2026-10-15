# Raspberry Pi system-statistics collector (experimental)

This executable samples CPU/memory statistics and logs the encoded message size every five seconds. It reads the CPU temperature from `/sys/class/thermal/thermal_zone0/temp` when available (0 otherwise). It currently does **not** connect to a gateway, send QUIC/MQTT messages or load `config.yaml`. Installing its hardened systemd unit does not establish an AXCP integration.

From the repository root:

```sh
go test ./edge/rpi-agent/...
go run ./edge/rpi-agent/cmd/agent
```

Stop with Ctrl+C. Use the [Core SDK](../../sdk/go/README.md) to build an authenticated sender, and the [gateway guide](../../docs/gateway-setup.md) for transport configuration. This collector is not a supported production agent daemon.
