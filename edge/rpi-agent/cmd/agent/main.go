package main

import (
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
)

var cfg = struct {
	Gateway string
	Profile uint32
}{
	Gateway: "192.168.1.10:7143",
	Profile: 0,
}

func sendTelemetry(client *netquic.Client) error {
	cpuP, _ := cpu.Percent(0, false)
	vmem, _ := mem.VirtualMemory()
	td := &axcp.TelemetryDatagram{
		TimestampMs: uint64(time.Now().UnixMilli()),
		Payload: &axcp.TelemetryDatagram_System{
			System: &axcp.SystemStats{
				CpuPercent:   uint32(cpuP[0]),
				MemBytes:     vmem.Used,
				TemperatureC: 0,
			},
		},
	}
	return client.SendTelemetry(td)
}

func sendHello(client *netquic.Client) error {
	env := axcp.NewEnvelope(uuid.NewString(), cfg.Profile)
	env.Payload = &axcp.Envelope_ContextPatch{
		ContextPatch: &axcp.ContextPatch{
			ContextId:   "hello",
			BaseVersion: 0,
		},
	}
	return client.SendEnvelope(env)
}

func main() {
	tlsConf := netquic.InsecureTLSConfig()
	client, err := netquic.Dial(cfg.Gateway, tlsConf)
	if err != nil { log.Fatal(err) }
	defer client.Close()

	if err := sendHello(client); err != nil {
		log.Printf("hello failed: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		if err := sendTelemetry(client); err != nil {
			log.Printf("telemetry error: %v", err)
		}
	}
}
