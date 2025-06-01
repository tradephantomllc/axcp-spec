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
	handler := func(env *axcp.Envelope) {
		if err := broker.Publish(env); err != nil {
			log.Printf("[mqtt] pub failed: %v", err)
		}
	}

	// Handler per i datagrammi di telemetria
	telemetryHandler := func(td *axcp.TelemetryDatagram) {
		// Estrai il profilo dal datagramma
		profile := td.GetProfile()

		// Applica il rumore differenzialmente privato se il profilo è >= 3
		if profile >= 3 {
			log.Printf("Applicazione rumore DP al profilo %d", profile)
			internal.ApplyNoise(td)
		}

		// Inoltra al broker MQTT
		if broker != nil {
			traceID := td.GetTraceId()
			if traceID == "" {
				traceID = "unknown"
			}

			if err := broker.PublishTelemetry(td, traceID); err != nil {
				log.Printf("Errore pubblicazione telemetria: %v", err)
			}
		}
	}

	// Avvia il server QUIC con gestione di stream e datagrammi
	if err := internal.RunQuicServer(addr, tlsConf, handler, telemetryHandler); err != nil {
		log.Fatal(err)
	}
}
