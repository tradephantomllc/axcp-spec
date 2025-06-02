// Package pb fornisce un wrapper pubblico per i tipi protobuf interni
package pb

import (
	internal "github.com/tradephantom/axcp-spec/sdk/go/internal/pb"
	"google.golang.org/protobuf/proto"
)

// Esposizione dei tipi interni per compatibilità diretta
type (
	// AxcpEnvelope è un wrapper pubblico per internal.AxcpEnvelope
	AxcpEnvelope = internal.AxcpEnvelope

	// TelemetryDatagram è un wrapper pubblico per internal.TelemetryDatagram
	TelemetryDatagram = internal.TelemetryDatagram

	// TelemetryDatagram_System è un wrapper pubblico per internal.TelemetryDatagram_System
	TelemetryDatagram_System = internal.TelemetryDatagram_System

	// SystemStats è un wrapper pubblico per internal.SystemStats
	SystemStats = internal.SystemStats

	// AxcpEnvelope_Telemetry è un wrapper pubblico per internal.AxcpEnvelope_Telemetry
	AxcpEnvelope_Telemetry = internal.AxcpEnvelope_Telemetry
)

// Funzioni helper per lavorare con i tipi interni

// NewTelemetryDatagram crea un nuovo TelemetryDatagram
func NewTelemetryDatagram() *TelemetryDatagram {
	return &internal.TelemetryDatagram{}
}

// NewSystemStats crea un nuovo SystemStats
func NewSystemStats() *SystemStats {
	return &internal.SystemStats{}
}

// ToInternal converte un TelemetryDatagram pubblico in uno interno
// Questa funzione è necessaria per le API che si aspettano il tipo interno
func ToInternal(td *TelemetryDatagram) *internal.TelemetryDatagram {
	return td
}

// Marshal serializza il TelemetryDatagram in bytes
func Marshal(m proto.Message) ([]byte, error) {
	return proto.Marshal(m)
}

// Unmarshal deserializza i bytes in un TelemetryDatagram
func Unmarshal(data []byte, m proto.Message) error {
	return proto.Unmarshal(data, m)
}
