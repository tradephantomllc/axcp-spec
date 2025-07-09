package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"google.golang.org/protobuf/proto"
)

var cfg = struct {
	Gateway string
	Profile uint32
}{
	Gateway: "192.168.1.10:7143",
	Profile: 0,
}

// getCPUPercent returns the current CPU usage percentage
func getCPUPercent() uint32 {
	cpuP, _ := cpu.Percent(0, false)
	return uint32(cpuP[0])
}

// getCPUTemperature returns the current CPU temperature in Celsius
// Note: This is a placeholder implementation that should be replaced with actual hardware-specific code
func getCPUTemperature() uint32 {
	// TODO: Implement actual temperature reading for your hardware
	// For Raspberry Pi, you might read from /sys/class/thermal/thermal_zone0/temp
	return 0
}

// sendTelemetry collects and logs system telemetry data (for UDP benchmarking)
func sendTelemetry() error {
	// Get system stats
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Errorf("error getting memory stats: %v", err)
	}

	// Create telemetry datagram
	tel := &axcp.TelemetryDatagram{
		TimestampMs: uint64(time.Now().UnixNano() / int64(time.Millisecond)),
		Payload: &axcp.TelemetryDatagram_System{
			System: &axcp.SystemStats{
				CpuPercent:   getCPUPercent(),
				MemBytes:     vmStat.Used,
				TemperatureC: getCPUTemperature(),
			},
		},
	}

	// Wrap in envelope
	env := &axcp.AxcpEnvelope{
		Version: 1,
		TraceId: uuid.New().String(),
		Profile: cfg.Profile, // Use configured profile
		Payload: &axcp.AxcpEnvelope_Telemetry{
			Telemetry: tel,
		},
	}

	// Serialize the envelope
	data, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("error marshaling telemetry: %v", err)
	}

	log.Printf("Collected telemetry: %d bytes (would send in non-benchmark mode)", len(data))
	return nil
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
