package main

import "testing"

func TestBuildTelemetry(t *testing.T) {
	// Nella versione locale di TelemetryDatagram (localpb) non esiste un campo Profile
	// Il profilo viene invece impostato nell'AxcpEnvelope che contiene il TelemetryDatagram
	// Quindi verifichiamo solo che buildTelemetry restituisca un valore non nil
	td := buildTelemetry(3)
	if td == nil {
		t.Fatal("telemetry datagram is nil")
	}
	
	// Verifichiamo che i campi fondamentali siano impostati
	if td.TimestampMs == 0 {
		t.Fatal("timestamp not set")
	}
	
	// Verifichiamo che il payload System sia impostato
	system := td.GetSystem()
	if system == nil {
		t.Fatal("system stats not set")
	}
}
