package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantom/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
)

var BuildVersion = "dev" // overridden at build time with -ldflags "-X main.BuildVersion=<ver>"

func main() {
	// Parse command line flags
	var addr string
	var enableRetryBuffer bool
	var maxRetryCapacity int
	var maxRetryAttempts int
	var minRetryInterval time.Duration
	var maxRetryInterval time.Duration

	flag.StringVar(&addr, "addr", ":7143", "Address to listen on")
	flag.BoolVar(&enableRetryBuffer, "retry", true, "Enable retry buffer for failed messages")
	flag.IntVar(&maxRetryCapacity, "retry-capacity", 1000, "Maximum capacity of retry buffer")
	flag.IntVar(&maxRetryAttempts, "retry-attempts", 5, "Maximum retry attempts per message")
	flag.DurationVar(&minRetryInterval, "retry-min-interval", 1*time.Second, "Minimum retry interval")
	flag.DurationVar(&maxRetryInterval, "retry-max-interval", 5*time.Minute, "Maximum retry interval")

	flag.Parse()

	tlsConf := netquic.InsecureTLSConfig()

	// Set up context for graceful shutdown
	_, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize logger
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Initialize broker
	broker, err := internal.NewBroker(internal.BrokerConfig{
		URL: "tcp://mosquitto:1883",
	})
	if err != nil {
		log.Fatalf("Failed to initialize broker: %v", err)
	}

	// Initialize retry buffer if enabled
	var retryBuffer *internal.RetryBuffer
	if enableRetryBuffer {
		retryConfig := internal.RetryBufferConfig{
			MaxCapacity:      maxRetryCapacity,
			MinRetryInterval: minRetryInterval,
			MaxRetryInterval: maxRetryInterval,
			BackoffFactor:    2.0,
			MaxAttempts:      maxRetryAttempts,
		}

		// Crea il buffer di retry con la funzione di pubblicazione del broker
		retryBuffer = internal.NewRetryBuffer(&retryConfig, nil, func(env *axcp.Envelope) error {
			// Qui andrebbe la conversione da axcp.Envelope a pb.AxcpEnvelope
			return fmt.Errorf("not implemented")
		})

		// Avvia il buffer di retry
		retryBuffer.Start()
		defer retryBuffer.Close()

		log.Printf("Initializing retry buffer: capacity=%d, max_attempts=%d, min_interval=%s, max_interval=%s",
			maxRetryCapacity, maxRetryAttempts, minRetryInterval, maxRetryInterval)
	} else {
		log.Println("Retry buffer disabled")
	}

	// Handler per envelope AXCP compatibile con l'interfaccia EnvelopeHandler
	handler := func(pbEnv *pb.AxcpEnvelope) {
		// Usiamo il broker che è stato inizializzato all'interno del main
		if err := broker.Publish(pbEnv); err != nil {
			log.Printf("Failed to publish envelope: %v", err)

			// Se il retry buffer è abilitato, aggiungi l'envelope al buffer
			if retryBuffer != nil {
				traceID := fmt.Sprintf("env-%d", time.Now().UnixNano())
				axcpEnv := axcp.NewEnvelope(traceID, 0)

				// Usa l'ID traccia come identificatore univoco
				id := axcpEnv.GetTraceId()

				if err := retryBuffer.AddEnvelope(id, axcpEnv); err != nil {
					log.Printf("Failed to add envelope to retry buffer. id=%s, error=%v", id, err)
				} else {
					log.Printf("Added envelope to retry buffer. id=%s", id)
				}
			}
		}
	}

	// Telemetry datagram handler
	telemetryHandler := func(td *pb.TelemetryDatagram) {
		if broker == nil {
			return
		}

		// Generate trace ID
		traceID := fmt.Sprintf("telemetry-%d", td.GetTimestampMs())

		// Publish telemetry
		err := broker.PublishTelemetry(td, traceID)
		if err != nil {
			log.Printf("Failed to publish telemetry. trace_id=%s, error=%v", traceID, err)

			// Se il retry buffer è abilitato, aggiungi la telemetria al buffer
			if retryBuffer != nil {
				axcpEnv := axcp.NewEnvelope(traceID, 0)

				if err := retryBuffer.AddEnvelope(traceID, axcpEnv); err != nil {
					log.Printf("Failed to add telemetry to retry buffer. trace_id=%s, error=%v", traceID, err)
				} else {
					log.Printf("Added telemetry to retry buffer. trace_id=%s", traceID)
				}
			}
		}
	}

	// Start server
	log.Printf("Starting AXCP gateway server %s on %s...", BuildVersion, addr)
	if err := internal.RunQuicServer(addr, tlsConf, handler, telemetryHandler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
