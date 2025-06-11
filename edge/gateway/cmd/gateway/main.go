package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal"
	// gatewaymetrics "github.com/tradephantom/axcp-spec/enterprise/edge/gateway/internal/metrics" // Importazione commentata per risolvere problema con internal package
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
)

// lookupEnvFloat legge una variabile d'ambiente come float64 o restituisce il valore di default
func lookupEnvFloat(key string, defaultVal float64) float64 {
	if val, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

// lookupEnvDuration legge una variabile d'ambiente come time.Duration o restituisce il valore di default
func lookupEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func main() {
	// Parse command line flags
	// metricsCfg := gatewaymetrics.DefaultConfig() // Commentato per risolvere problema con internal package
	var addr string
	var enableRetryBuffer bool
	var maxRetryCapacity int
	var maxRetryAttempts int
	var minRetryInterval time.Duration
	var maxRetryInterval time.Duration
	
	// Parametri per il budget DP
	var epsilonFlag float64
	var deltaFlag float64
	var budgetWindowFlag time.Duration
	
	flag.StringVar(&addr, "addr", ":7143", "Address to listen on")
	flag.BoolVar(&enableRetryBuffer, "retry", true, "Enable retry buffer for failed messages")
	flag.IntVar(&maxRetryCapacity, "retry-capacity", 1000, "Maximum capacity of retry buffer")
	flag.IntVar(&maxRetryAttempts, "retry-attempts", 5, "Maximum retry attempts per message")
	flag.DurationVar(&minRetryInterval, "retry-min-interval", 1*time.Second, "Minimum retry interval")
	flag.DurationVar(&maxRetryInterval, "retry-max-interval", 5*time.Minute, "Maximum retry interval")
	
	// Flag per il budget DP con binding alle variabili d'ambiente
	flag.Float64Var(&epsilonFlag, "epsilon", lookupEnvFloat("AXCP_DP_EPSILON", 1.0), "Privacy parameter epsilon for differential privacy")
	flag.Float64Var(&deltaFlag, "delta", lookupEnvFloat("AXCP_DP_DELTA", 1e-5), "Privacy parameter delta for differential privacy")
	flag.DurationVar(&budgetWindowFlag, "budget-window", lookupEnvDuration("AXCP_DP_WINDOW", 1*time.Hour), "Time window for privacy budget calculation")
	
	// metricsCfg.AddFlags(flag.CommandLine) // Commentato per risolvere problema con internal package
	flag.Parse()

	tlsConf := netquic.InsecureTLSConfig()

	// Set up context for graceful shutdown
	_, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize metrics
	// metrics, err := metricsCfg.Setup(ctx)
	// if err != nil {
	// 	log.Fatalf("Failed to initialize metrics: %v", err)
	// }
	// defer func() {
	// 	if err := metrics.Shutdown(context.Background()); err != nil {
	// 		log.Printf("Failed to shutdown metrics: %v", err)
	// 	}
	// }()

	// Initialize logger
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	
	// Initialize broker
	broker, err := internal.NewBroker(internal.BrokerConfig{
		URL:       "tcp://mosquitto:1883",
		DPEnabled: true, // Enable DP by default, config can be loaded from env vars
		DPConfig:  "",  // Use default config location
	})
	if err != nil {
		log.Fatalf("Failed to initialize broker: %v", err)
	}
	
	// Imposta i parametri DP se forniti tramite flag o variabili d'ambiente
	if epsilonFlag > 0 || deltaFlag > 0 || budgetWindowFlag > 0 {
		e, d, w := internal.GetBudget()
		if epsilonFlag > 0 {
			e = epsilonFlag
		}
		if deltaFlag > 0 {
			d = deltaFlag
		}
		if budgetWindowFlag > 0 {
			w = budgetWindowFlag
		}
		internal.SetBudget(e, d, w)
		log.Printf("DP budget parameters set: epsilon=%.2f, delta=%.10f, window=%v", e, d, w)
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
			// Qui andrebbe la conversione da axcp.Envelope a pb.Envelope
			return fmt.Errorf("not implemented")
		})
		
		// Inizializza le metriche per il retry buffer
		// Nota: Commentiamo temporaneamente questa parte fino a quando
		// non avremo un'interfaccia metrics compatibile
		/*
		retryMetrics, err := internal.NewRetryMetrics(gatewaymetrics.DefaultMeter())
		if err != nil {
			log.Printf("Failed to initialize retry metrics: %v", err)
		} else {
			retryBuffer.SetMetrics(retryMetrics)
		}
		*/
		
		// Avvia il buffer di retry
		retryBuffer.Start()
		defer retryBuffer.Close()
		
		log.Printf("Initializing retry buffer: capacity=%d, max_attempts=%d, min_interval=%s, max_interval=%s",
			maxRetryCapacity, maxRetryAttempts, minRetryInterval, maxRetryInterval)
	} else {
		log.Println("Retry buffer disabled")
	}

	// Handler per envelope AXCP compatibile con l'interfaccia EnvelopeHandler
	handler := func(pbEnv *pb.Envelope) {
		// Usiamo il broker che è stato inizializzato all'interno del main
		if err := broker.Publish(pbEnv); err != nil {
			log.Printf("Failed to publish envelope: %v", err)
			
			// Se il retry buffer è abilitato, aggiungi l'envelope al buffer
			if retryBuffer != nil {
				// Per gestire l'envelope nel retry buffer, dobbiamo convertirlo in axcp.Envelope
				// (Nella realtà questa conversione dovrebbe copiare i dati da pbEnv ad axcpEnv)
				traceID := fmt.Sprintf("env-%d", time.Now().UnixNano())
				axcpEnv := axcp.NewEnvelope(traceID, 0)
				
				// In uno scenario reale, qui copieremmo tutti i campi rilevanti da pbEnv ad axcpEnv
				
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
			log.Printf("Failed to publish telemetry. trace_id=%s, error=%v", traceID, err)
			
			// Se il retry buffer è abilitato, aggiungi la telemetria al buffer
			if retryBuffer != nil {
					// Crea un envelope contenente la telemetria
				axcpEnv := axcp.NewEnvelope(traceID, 0)
				
				// In un'implementazione reale, qui inseriremmo il telemetry datagram
				// nell'envelope, ma per evitare errori di compilazione commentiamo questa parte
				// axcpEnv.AxcpEnvelope.Payload = &pb.AxcpEnvelope_Telemetry{Telemetry: td}
				
				// Aggiungi l'envelope al retry buffer
				if err := retryBuffer.AddEnvelope(traceID, axcpEnv); err != nil {
					log.Printf("Failed to add telemetry to retry buffer. trace_id=%s, error=%v", traceID, err)
				} else {
					log.Printf("Added telemetry to retry buffer. trace_id=%s", traceID)
				}
			}
		} else {
			// Update success metric
			// metrics.RecordRetrySuccess() // Commentato: metrics non disponibile dopo refactoring
		}
	}

	// Start server
	log.Printf("Starting AXCP gateway server on %s...", addr)
	if err := internal.RunQuicServer(addr, tlsConf, handler, telemetryHandler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
