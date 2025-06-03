package main

import (
	"log"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
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
	telemetryHandler := func(td *pb.TelemetryDatagram) {
		// Il campo 'Profile' non è presente in pb.TelemetryDatagram.
		// Impostiamo un valore di default. Questa logica potrebbe necessitare di revisione
		// per il corretto funzionamento dei test e della feature di DP.
		var profile uint32 = 0 // Default profile; con 0, il rumore DP non viene applicato.
		log.Printf("[gateway] Profilo telemetria (default): %d", profile)

		// Applica il rumore differenzialmente privato se il profilo è >= 3
		if profile >= 3 {
			log.Printf("[gateway] Applicazione rumore DP al profilo %d", profile)
			internal.ApplyNoise(td)
		} else {
			log.Printf("[gateway] Rumore DP non applicato per profilo %d", profile)
		}

		// Inoltra al broker MQTT
		if broker != nil {
			// Il campo 'TraceId' non è presente in pb.TelemetryDatagram.
			// Usiamo un valore di default per il traceID.
			traceID := "telemetry_datagram_default_trace"
			log.Printf("[gateway] Trace ID telemetria (default): %s", traceID)

			if err := broker.PublishTelemetry(td, traceID); err != nil {
				log.Printf("[gateway] Errore pubblicazione telemetria: %v", err)
			}
		}
	}

	// Avvia il server QUIC con gestione di stream e datagrammi
	if err := internal.RunQuicServer(addr, tlsConf, handler, telemetryHandler); err != nil {
		log.Fatal(err)
	}
}
