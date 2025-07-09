// Package axcp implements the core data structures for the AXCP protocol.
package axcp

import (
	"time"

	"github.com/tradephantom/axcp-spec/sdk/go/axcp/internal/pb"
)

// NewTelemetryDatagram creates a new TelemetryDatagram with the current timestamp.
// The timestamp is set to the current time in milliseconds since epoch.
func NewTelemetryDatagram() *pb.TelemetryDatagram {
	return &pb.TelemetryDatagram{
		TimestampMs: uint64(time.Now().UnixNano() / int64(time.Millisecond)),
	}
}

// WithSystemStats adds system statistics to the telemetry datagram.
func WithSystemStats(td *pb.TelemetryDatagram, cpuPercent uint32, memBytes uint64, tempC uint32) *pb.TelemetryDatagram {
	systemStats := &pb.SystemStats{
		CpuPercent:   cpuPercent,
		MemBytes:     memBytes,
		TemperatureC: tempC,
	}
	td.Payload = &pb.TelemetryDatagram_System{
		System: systemStats,
	}
	return td
}

// WithTokenUsage adds token usage statistics to the telemetry datagram.
func WithTokenUsage(td *pb.TelemetryDatagram, promptTokens, completionTokens uint32) *pb.TelemetryDatagram {
	tokenUsage := &pb.TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}
	td.Payload = &pb.TelemetryDatagram_Tokens{
		Tokens: tokenUsage,
	}
	return td
}

// GetSystemStats returns the system stats if present, nil otherwise.
func GetSystemStats(td *pb.TelemetryDatagram) *pb.SystemStats {
	if system, ok := td.GetPayload().(*pb.TelemetryDatagram_System); ok {
		return system.System
	}
	return nil
}

// GetTokenUsage returns the token usage if present, nil otherwise.
func GetTokenUsage(td *pb.TelemetryDatagram) *pb.TokenUsage {
	if tokens, ok := td.GetPayload().(*pb.TelemetryDatagram_Tokens); ok {
		return tokens.Tokens
	}
	return nil
}
