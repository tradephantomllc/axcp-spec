package main

import (
	"log"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
)

func main() {
	addr := ":7143"
	tlsConf := netquic.InsecureTLSConfig()

	broker := internal.NewBroker("tcp://mosquitto:1883")
	
	// Handler for AXCP envelopes over streams
	handler := func(data []byte) {
		// In una implementazione reale, deserializzeremmo i dati
		// Per ora, creiamo un envelope semplice usando il costruttore appropriato
		env := axcp.NewEnvelope("stream", 1) // Trace ID "stream", profilo base 1
		
		if err := broker.Publish(env); err != nil {
			log.Printf("[mqtt] pub failed: %v", err)
		}
	}

	// Handler for telemetry datagrams with profile information
	telemetryHandler := func(rawData []byte, profile uint32) {
		// Estrae i dati di telemetria dal payload grezzo
		td, err := internal.ExtractTelemetry(rawData, profile)
		if err != nil {
			log.Printf("[telemetry] failed to extract telemetry data: %v", err)
			return
		}

		// Abilita la privacy differenziale solo per profilo >= 3
		td.DifferentialDP = profile >= 3
		
		// Applica la privacy differenziale se il profilo >= 3
		internal.ApplyNoiseToTelemetryData(td)
		
		// Pubblica i dati di telemetria con il trace ID come parte del topic
		traceID := td.TraceID
		if traceID == "" {
			traceID = "edge"
		}
		
		// Crea una versione semplificata del messaggio per il broker
		message := map[string]interface{}{
			"timestamp_ms": td.TimestampMs,
			"trace_id": traceID,
			"profile": profile,
		}
		
		if td.SystemStats != nil {
			message["system"] = map[string]interface{}{
				"cpu_percent": td.SystemStats.CPUPercent,
				"mem_bytes": td.SystemStats.MemBytes,
				"temperature_c": td.SystemStats.TemperatureC,
			}
		}
		
		if td.TokenUsage != nil {
			message["tokens"] = map[string]interface{}{
				"prompt": td.TokenUsage.PromptTokens,
				"completion": td.TokenUsage.CompletionTokens,
			}
		}
		
		if err := broker.PublishTelemetryData(message, traceID); err != nil {
			log.Printf("[mqtt] telemetry publish failed: %v", err)
		}
	}

	// Start the QUIC server with both stream and datagram handlers
	if err := internal.RunQuicServer(addr, tlsConf, handler, telemetryHandler); err != nil {
		log.Fatal(err)
	}
}
