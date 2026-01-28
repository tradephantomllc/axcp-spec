package internal

import (
	"math/rand"
	"time"
)

// TelemetryData rappresenta i dati essenziali estratti da un datagramma di telemetria
type TelemetryData struct {
	TimestampMs  uint64
	SystemStats  *SystemStats
	TokenUsage   *TokenUsage
	LatencyStats *LatencyStats
	TraceID      string
}

// SystemStats contiene statistiche di sistema
type SystemStats struct {
	CPUPercent   uint32
	MemBytes     uint64
	TemperatureC uint32
}

// TokenUsage contiene statistiche sull'utilizzo dei token
type TokenUsage struct {
	PromptTokens     uint32
	CompletionTokens uint32
}

// LatencyStats contiene statistiche sulla latenza
type LatencyStats struct {
	RequestLatencyMs  uint32
	ResponseLatencyMs uint32
}

// ExtractTelemetry estrae i dati di telemetria dai dati grezzi
func ExtractTelemetry(data []byte, profile uint32) (*TelemetryData, error) {
	// Implementazione semplificata: nella pratica useresti proto.Unmarshal
	// con le strutture generate da protobuf

	// Per questa dimostrazione, creiamo dati fittizi
	td := &TelemetryData{
		TimestampMs: uint64(time.Now().UnixNano() / int64(time.Millisecond)),
		SystemStats: &SystemStats{
			CPUPercent:   rand.Uint32() % 100,
			MemBytes:     uint64(rand.Uint32()) * 1024 * 1024,
			TemperatureC: 40 + rand.Uint32()%20,
		},
		TraceID: "edge",
	}

	return td, nil
}
