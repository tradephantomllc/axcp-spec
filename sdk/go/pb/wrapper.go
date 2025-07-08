// Package pb fornisce un wrapper pubblico per i tipi protobuf generati internamente
package pb

import (
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
)

// --------- Tipi Telemetria ---------

// TelemetryDatagram rappresenta un pacchetto di telemetria
type TelemetryDatagram = pb.TelemetryDatagram

// SystemStats contiene le statistiche di sistema
type SystemStats = pb.SystemStats

// TokenUsage contiene informazioni sull'utilizzo dei token
type TokenUsage = pb.TokenUsage

// TelemetryDatagram_System rappresenta il campo oneof per SystemStats
type TelemetryDatagram_System = pb.TelemetryDatagram_System

// TelemetryDatagram_Tokens rappresenta il campo oneof per TokenUsage
type TelemetryDatagram_Tokens = pb.TelemetryDatagram_Tokens

// --------- Tipi Envelope ---------

// AxcpEnvelope è l'envelope principale dei messaggi AXCP
type AxcpEnvelope = pb.AxcpEnvelope

// Envelope è un alias per AxcpEnvelope per mantenere compatibilità con il codice esistente
type Envelope = pb.AxcpEnvelope

// --------- Funzioni Helper ---------

// NewTelemetryDatagram crea un nuovo datagramma di telemetria
func NewTelemetryDatagram() *TelemetryDatagram {
	return &TelemetryDatagram{}
}

// NewSystemStats crea nuove statistiche di sistema
func NewSystemStats() *SystemStats {
	return &SystemStats{}
}

// NewTokenUsage crea un nuovo oggetto di utilizzo token
func NewTokenUsage() *TokenUsage {
	return &TokenUsage{}
}

// NewAxcpEnvelope crea un nuovo envelope AXCP
func NewAxcpEnvelope() *AxcpEnvelope {
	return &AxcpEnvelope{}
}
