package internal

import (
	"log"
	"math/rand"
	"time"

	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

// TelemetryData rappresenta i dati essenziali estratti da un datagramma di telemetria
type TelemetryData struct {
	TimestampMs    uint64
	SystemStats    *SystemStats
	TokenUsage     *TokenUsage
	LatencyStats   *LatencyStats
	TraceID        string
	DifferentialDP bool
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
		DifferentialDP: profile >= 3,
		TraceID:        "edge",
	}

	return td, nil
}

// ApplyNoiseToProtobuf applica rumore differenziale a un datagramma di telemetria nel formato protobuf
// Nota: questa è una funzione segnaposto che delega alla funzione ApplyNoise nel file dp_noise.go
func ApplyNoiseToProtobuf(td *axcp.TelemetryDatagram) {
	// In una implementazione completa, deleghiamo alla funzione esistente
	log.Printf("[dp] Applying differential privacy noise to telemetry protobuf data")

	// Delega alla funzione esistente in dp_noise.go
	// ApplyNoise(td)
}

// ApplyNoiseToData applica rumore differenziale ai dati di telemetria estratti
func ApplyNoiseToData(td *TelemetryData) {
	if !td.DifferentialDP {
		return
	}

	log.Printf("[dp] Applying differential privacy noise to telemetry data")

	// Esempio: aggiungi rumore gaussiano alle statistiche di sistema
	if td.SystemStats != nil {
		// Aggiungi rumore ±5% al CPU
		noise := (rand.Uint32() % 10) - 5
		if td.SystemStats.CPUPercent+noise <= 100 {
			td.SystemStats.CPUPercent += noise
		}

		// Aggiungi rumore ±2% alla memoria
		memNoise := uint64(float64(td.SystemStats.MemBytes) * (float64(rand.Intn(5)-2) / 100.0))
		td.SystemStats.MemBytes += memNoise

		// Aggiungi rumore ±1°C alla temperatura
		tempNoise := rand.Uint32()%3 - 1
		td.SystemStats.TemperatureC += tempNoise
	}

	// Esempio: aggiungi rumore ai token
	if td.TokenUsage != nil {
		promptNoise := rand.Uint32() % 5
		td.TokenUsage.PromptTokens += promptNoise

		completionNoise := rand.Uint32() % 10
		td.TokenUsage.CompletionTokens += completionNoise
	}
}
