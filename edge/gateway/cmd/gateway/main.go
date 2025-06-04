package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal"
	"github.com/tradephantom/axcp-spec/edge/gateway/internal/buffer"
	"github.com/tradephantom/axcp-spec/edge/gateway/internal/metrics"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
	"google.golang.org/protobuf/proto"
)

func main() {
	addr := ":7143"
	tlsConf := netquic.InsecureTLSConfig()

	// Initialize broker
	broker := internal.NewBroker("tcp://mosquitto:1883")

	// Set up context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize retry buffer
	db, err := buffer.Open("retry.db")
	if err != nil {
		log.Fatalf("Failed to open retry buffer: %v", err)
	}
	defer db.Close()

	queue := buffer.NewQueue(db)

	// Start retry loop in a goroutine
	stopRetry := make(chan struct{})
	defer close(stopRetry)
	go buffer.StartRetryLoop(queue, broker, stopRetry)

	// Handler for AXCP envelopes over streams
	handler := func(env *pb.Envelope) {
		if err := broker.Publish(env); err != nil {
			log.Printf("[mqtt] pub failed: %v", err)
		}
	}

	// Telemetry datagram handler
	telemetryHandler := func(td *pb.TelemetryDatagram) {
		// Apply DP noise
		internal.ApplyNoise(td)

		if broker == nil {
			return
		}

		// Generate trace ID
		traceID := fmt.Sprintf("telemetry-%d", td.GetTimestampMs())

		// First try to publish directly
		err := broker.PublishTelemetry(td, traceID)
		if err != nil {
			// If direct publish fails, add to retry queue
			log.Printf("Publish failed, adding to retry queue: %v", err)
			
				// Serialize the telemetry data using protobuf
			data, err := proto.Marshal(td)
			if err != nil {
				log.Printf("Failed to marshal telemetry data: %v", err)
				metrics.RetryDropped.Inc()
				return
			}

			// Add to retry queue with trace ID as key
			if err := queue.Push([]byte(traceID), data); err != nil {
				log.Printf("Failed to add to retry queue: %v", err)
				metrics.RetryDropped.Inc()
				return
			}

			// Update metrics
			metrics.RetryAttempts.Inc()
			if count, err := queue.Len(); err == nil {
				metrics.RetryQueue.Set(float64(count))
			}
		} else {
			// Update success metric
			metrics.RetrySuccess.Inc()
		}
	}

	// Start the QUIC server
	errChan := make(chan error, 1)
	go func() {
		errChan <- internal.RunQuicServer(addr, tlsConf, handler, telemetryHandler)
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		log.Fatalf("QUIC server error: %v", err)
	case <-ctx.Done():
		log.Println("Shutting down gracefully...")
		// Allow time for in-flight requests to complete
		time.Sleep(2 * time.Second)
	}
}
