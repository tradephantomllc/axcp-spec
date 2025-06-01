package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
)

type Config struct {
	Gateway     string `json:"gateway" yaml:"gateway"`
	Profile     uint32 `json:"profile" yaml:"profile"`
	IntervalSec int    `json:"interval_sec" yaml:"interval_sec"`
}

func loadConfig(path string) Config {
	b, _ := ioutil.ReadFile(path)
	var c Config
	_ = json.Unmarshal(b, &c)
	if c.Gateway == "" { c.Gateway = "127.0.0.1:7143" }
	if c.IntervalSec == 0 { c.IntervalSec = 5 }
	return c
}

func sendHello(c *netquic.Client, profile uint32) error {
	env := axcp.NewEnvelope(uuid.NewString(), profile)
	return c.SendEnvelope(env)
}

func buildTelemetry(profile uint32) *axcp.TelemetryDatagram {
	cpuP, _ := cpu.Percent(0, false)
	vmem, _ := mem.VirtualMemory()
	return &axcp.TelemetryDatagram{
		TimestampMs: uint64(time.Now().UnixMilli()),
		Profile:     profile,
		Payload: &axcp.TelemetryDatagram_System{
			System: &axcp.SystemStats{
				CpuPercent: uint32(cpuP[0]),
				MemBytes:   vmem.Used,
			},
		},
	}
}

func main() {
	cfg := loadConfig("/etc/axcp/config.yaml")
	tlsConf := netquic.InsecureTLSConfig()

	client, err := netquic.Dial(cfg.Gateway, tlsConf)
	if err != nil { log.Fatal(err) }
	defer client.Close()

	// Send "hello" via stream
	if err := sendHello(client, cfg.Profile); err != nil {
		log.Printf("hello failed: %v", err)
	}

	ticker := time.NewTicker(time.Duration(cfg.IntervalSec) * time.Second)
	for range ticker.C {
		td := buildTelemetry(cfg.Profile)
		if err := client.SendTelemetry(td); err != nil {
			log.Printf("telemetry err: %v", err)
		}
	}
}
