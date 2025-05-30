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

	// Handler for telemetry datagrams
	telemetryHandler := func(td *axcp.TelemetryDatagram) {
		// Apply differential privacy noise if profile >= 3
		if td.Profile >= 3 {
			internal.ApplyNoise(td)
		}
		
		// Publish telemetry data with the trace ID as part of the topic
		traceID := "edge"
		if td.TraceId != "" {
			traceID = td.TraceId
		}
		
		if err := broker.PublishTelemetry(td, traceID); err != nil {
			log.Printf("[mqtt] telemetry publish failed: %v", err)
		}
	}

	// Start the QUIC server with both stream and datagram handlers
	if err := internal.RunQuicServer(addr, tlsConf, handler, telemetryHandler); err != nil {
		log.Fatal(err)
	}
}
