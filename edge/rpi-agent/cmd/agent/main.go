package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
	pb "github.com/tradephantom/axcp-spec/sdk/go/pb"
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
	traceID := "rpi-agent-hello-" + time.Now().Format("20060102-150405.000")
	env := &axcp.Envelope{
		AxcpEnvelope: pb.AxcpEnvelope{
			Version: 1,
			TraceId: traceID,
			Profile: profile,
		},
	}
	return c.SendEnvelope(env)
}

func buildTelemetry(profile uint32) *pb.TelemetryDatagram {
	cpuP, _ := cpu.Percent(0, false)
	vmem, _ := mem.VirtualMemory()
	
	td := &pb.TelemetryDatagram{
		TimestampMs: uint64(time.Now().UnixNano() / int64(time.Millisecond)),
		Payload: &pb.TelemetryDatagram_System{
			System: &pb.SystemStats{
				CpuPercent:   uint32(cpuP[0]),
				MemBytes:     vmem.Used,
				TemperatureC: 0, // Not available
			},
		},
	}
	return td
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
