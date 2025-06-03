package main

import (
	"fmt"
	"log"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
)

func main() {
	addr := ":7143"
	tlsConf := netquic.InsecureTLSConfig()

	broker := internal.NewBroker("tcp://mosquitto:1883")

	// Handler for AXCP envelopes over streams
	handler := func(env *pb.Envelope) {
		if err := broker.Publish(env); err != nil {
			log.Printf("[mqtt] pub failed: %v", err)
		}
	}

	// Handler per i datagrammi di telemetria
	telemetryHandler := func(td *pb.TelemetryDatagram) {
		// In questa versione del protobuf, applichiamo sempre il rumore DP
		// poiché il campo Profile non è più disponibile nel protobuf
		log.Printf("Applicazione rumore DP al datagramma di telemetria")
		internal.ApplyNoise(td)

		// Inoltra al broker MQTT
		if broker != nil {
			// Usiamo il timestamp come identificatore in assenza del campo TraceId
			traceID := fmt.Sprintf("telemetry-%d", td.GetTimestampMs())

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
