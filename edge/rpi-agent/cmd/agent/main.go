package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
)

var cfg = struct {
	Gateway string
	Profile uint32
}{
	Gateway: "192.168.1.10:7143",
	Profile: 0,
}

// thermalZonePath is the Linux thermal sensor used by Raspberry Pi OS.
const thermalZonePath = "/sys/class/thermal/thermal_zone0/temp"

// getCPUPercent returns the current CPU usage percentage (0 if unavailable).
func getCPUPercent() uint32 {
	cpuP, err := cpu.Percent(0, false)
	if err != nil || len(cpuP) == 0 || cpuP[0] < 0 {
		return 0
	}
	return uint32(cpuP[0])
}

// getCPUTemperature returns the CPU temperature in Celsius (0 if unavailable).
func getCPUTemperature() uint32 {
	return readTemperature(thermalZonePath)
}

// readTemperature parses a Linux thermal zone file, which reports millidegrees Celsius.
func readTemperature(path string) uint32 {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	milli, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil || milli < 0 || milli > 200_000 {
		return 0
	}
	return uint32(milli / 1000)
}

// sendTelemetry collects and logs system telemetry data (for UDP benchmarking)
func sendTelemetry() error {
	// Get system stats
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Errorf("error getting memory stats: %v", err)
	}

	env := buildTelemetryEnvelope(uuid.New().String(), time.Now(), &pb.SystemStats{
		CpuPercent:   getCPUPercent(),
		MemBytes:     vmStat.Used,
		TemperatureC: getCPUTemperature(),
	})

	// Serialize the envelope
	data, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("error marshaling telemetry: %v", err)
	}

	log.Printf("Collected telemetry: %d bytes (would send in non-benchmark mode)", len(data))
	return nil
}

// buildTelemetryEnvelope wraps system statistics in an AXCP telemetry envelope.
func buildTelemetryEnvelope(traceID string, now time.Time, stats *pb.SystemStats) *pb.AxcpEnvelope {
	return &pb.AxcpEnvelope{
		Version: 1,
		TraceId: traceID,
		Profile: cfg.Profile,
		Payload: &pb.AxcpEnvelope_Telemetry{
			Telemetry: &pb.TelemetryDatagram{
				TimestampMs: uint64(now.UnixMilli()),
				Payload:     &pb.TelemetryDatagram_System{System: stats},
			},
		},
	}
}

// sendHello simulates sending a hello message (for UDP benchmarking)
func sendHello() error {
	log.Printf("Would send hello message in UDP benchmark mode")
	return nil
}

func main() {
	log.Println("AXCP Agent starting in UDP benchmark mode...")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Send initial hello message
	if err := sendHello(); err != nil {
		log.Printf("Warning: Failed to send hello: %v", err)
	}

	// Main loop - collect telemetry periodically
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Println("AXCP Agent started in UDP benchmark mode")
	log.Println("Press Ctrl+C to exit")

	for {
		select {
		case <-ticker.C:
			if err := sendTelemetry(); err != nil {
				log.Printf("Error collecting telemetry: %v", err)
			} else {
				log.Printf("Telemetry collected successfully")
			}

		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			return
		}
	}
}
