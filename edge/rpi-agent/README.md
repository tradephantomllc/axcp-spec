# AXCP rpi-agent

Edge node daemon for Raspberry Pi / ARM SBC.

## Installation

```bash
cd edge/rpi-agent
./build.sh arm64
scp bin/axcp-agent pi@raspberry:/usr/local/bin/
scp axcp-agent.service pi:/etc/systemd/system/
mkdir -p /etc/axcp
scp config.yaml pi:/etc/axcp/
ssh pi sudo systemctl enable --now axcp-agent
```

## Verifica

Per verificare i log dell'agente:

```bash
journalctl -u axcp-agent -f
```

Per monitorare i messaggi MQTT sul gateway:

```bash
mosquitto_sub -t axcp/# -h <gateway-ip>
```

## Configurazione

Modifica `config.yaml` per impostare:
- `gateway`: indirizzo del gateway AXCP
- `profile`: profilo di connettività (0=standard)
- `interval_sec`: intervallo di invio telemetria (secondi)
