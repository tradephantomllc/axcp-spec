package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tradephantomllc/axcp-spec/edge/gateway/internal"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantomllc/axcp-spec/sdk/go/negotiate"
	"github.com/tradephantomllc/axcp-spec/sdk/go/netquic"
)

var BuildVersion = "dev" // overridden at build time with -ldflags "-X main.BuildVersion=<ver>"

type trustedDIDFlag struct {
	entries map[string]ed25519.PublicKey
}

func (f *trustedDIDFlag) String() string {
	if f == nil {
		return "0 trusted DIDs"
	}
	return fmt.Sprintf("%d trusted DIDs", len(f.entries))
}

func (f *trustedDIDFlag) Set(value string) error {
	did, rawKey, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(did) == "" || strings.TrimSpace(rawKey) == "" {
		return fmt.Errorf("expected format did:key:...=<base64 raw Ed25519 public key>")
	}
	if _, err := auth.ParseDID(did); err != nil {
		return fmt.Errorf("invalid trusted DID %q: %w", did, err)
	}

	key, err := decodePublicKey(rawKey)
	if err != nil {
		return fmt.Errorf("invalid public key for %s: %w", did, err)
	}
	if len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key for %s: got %d bytes, want %d", did, len(key), ed25519.PublicKeySize)
	}

	if f.entries == nil {
		f.entries = make(map[string]ed25519.PublicKey)
	}
	f.entries[did] = ed25519.PublicKey(append([]byte(nil), key...))
	return nil
}

func (f *trustedDIDFlag) Len() int {
	if f == nil {
		return 0
	}
	return len(f.entries)
}

func (f *trustedDIDFlag) Resolver() *auth.MemoryDIDResolver {
	resolver := auth.NewMemoryDIDResolver()
	for did, key := range f.entries {
		resolver.AddDID(did, key)
	}
	return resolver
}

func decodePublicKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		key, err := enc.DecodeString(value)
		if err == nil {
			return key, nil
		}
	}
	return nil, fmt.Errorf("must be standard or URL-safe base64")
}

func main() {
	// Parse command line flags
	var addr string
	var enableRetryBuffer bool
	var maxRetryCapacity int
	var maxRetryAttempts int
	var minRetryInterval time.Duration
	var maxRetryInterval time.Duration
	var secureBaseline bool
	var serverDID string
	var trustedDIDs trustedDIDFlag
	var tlsCertFile string
	var tlsKeyFile string
	var allowInsecureDemoTLS bool

	flag.StringVar(&addr, "addr", ":7143", "Address to listen on")
	flag.BoolVar(&enableRetryBuffer, "retry", true, "Enable retry buffer for failed messages")
	flag.IntVar(&maxRetryCapacity, "retry-capacity", 1000, "Maximum capacity of retry buffer")
	flag.IntVar(&maxRetryAttempts, "retry-attempts", 5, "Maximum retry attempts per message")
	flag.DurationVar(&minRetryInterval, "retry-min-interval", 1*time.Second, "Minimum retry interval")
	flag.DurationVar(&maxRetryInterval, "retry-max-interval", 5*time.Minute, "Maximum retry interval")
	flag.BoolVar(&secureBaseline, "secure-baseline", false, "Enable Secure Baseline authentication (DID + Ed25519 + replay protection)")
	flag.StringVar(&serverDID, "server-did", "", "Server DID for auth transcript binding (required with -secure-baseline)")
	flag.Var(&trustedDIDs, "trusted-did", "Trusted sender DID and raw Ed25519 public key; repeatable format did:key:...=<base64-public-key>")
	flag.StringVar(&tlsCertFile, "tls-cert", "", "Server TLS certificate PEM file")
	flag.StringVar(&tlsKeyFile, "tls-key", "", "Server TLS private key PEM file")
	flag.BoolVar(&allowInsecureDemoTLS, "allow-insecure-demo-tls", false, "Allow ephemeral self-signed TLS for local demos only")

	flag.Parse()

	tlsConf, err := buildGatewayTLSConfig(tlsCertFile, tlsKeyFile, allowInsecureDemoTLS)
	if err != nil {
		log.Fatal(err)
	}

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

		retryBuffer = internal.NewRetryBuffer(&retryConfig, nil, func(env *axcp.Envelope) error {
			if env == nil {
				return fmt.Errorf("retry envelope is nil")
			}
			return broker.Publish(&env.AxcpEnvelope)
		})

		// Avvia il buffer di retry
		retryBuffer.Start()
		defer retryBuffer.Close()

		log.Printf("Initializing retry buffer: capacity=%d, max_attempts=%d, min_interval=%s, max_interval=%s",
			maxRetryCapacity, maxRetryAttempts, minRetryInterval, maxRetryInterval)
	} else {
		log.Println("Retry buffer disabled")
	}

	// Telemetry datagram handler (shared between authenticated and non-authenticated modes)
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

	// Start server based on profile
	if secureBaseline {
		// Validate secure baseline requirements
		if serverDID == "" {
			log.Fatal("--server-did is required when --secure-baseline is enabled")
		}
		if _, err := auth.ParseDID(serverDID); err != nil {
			log.Fatalf("--server-did is invalid: %v", err)
		}
		if trustedDIDs.Len() == 0 {
			log.Fatal("--trusted-did is required at least once when --secure-baseline is enabled")
		}

		log.Printf("Starting AXCP gateway server %s on %s (Secure Baseline)...", BuildVersion, addr)

		// Initialize authenticator
		resolver := trustedDIDs.Resolver()
		authConfig := internal.DefaultAuthConfig()
		authenticator, err := internal.NewEnvelopeAuthenticator(resolver, serverDID, authConfig, nil)
		if err != nil {
			log.Fatalf("Failed to create authenticator: %v", err)
		}

		// Authenticated envelope handler
		authHandler := func(pbEnv *pb.AxcpEnvelope, authResult *internal.AuthResult) {
			if authResult != nil && authResult.Authenticated {
				log.Printf("Authenticated envelope from %s, trace_id=%s", authResult.SenderDID, pbEnv.GetTraceId())
			}

			if err := broker.Publish(pbEnv); err != nil {
				log.Printf("Failed to publish envelope: %v", err)

				if retryBuffer != nil {
					traceID := fmt.Sprintf("env-%d", time.Now().UnixNano())
					axcpEnv := axcp.NewEnvelope(traceID, 0)
					id := axcpEnv.GetTraceId()

					if err := retryBuffer.AddEnvelope(id, axcpEnv); err != nil {
						log.Printf("Failed to add envelope to retry buffer. id=%s, error=%v", id, err)
					} else {
						log.Printf("Added envelope to retry buffer. id=%s", id)
					}
				}
			}
		}

		serverConfig := internal.ServerConfig{
			Addr:          addr,
			TLSConf:       tlsConf,
			Profile:       negotiate.ProfileSecureBaseline,
			Authenticator: authenticator,
			DIDResolver:   resolver,
			ServerDID:     serverDID,
		}

		if err := internal.RunAuthenticatedQuicServer(serverConfig, authHandler, telemetryHandler); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		// Legacy non-authenticated mode (transport-only)
		log.Printf("Starting AXCP gateway server %s on %s (transport-only)...", BuildVersion, addr)

		// Handler per envelope AXCP compatibile con l'interfaccia EnvelopeHandler
		handler := func(pbEnv *pb.AxcpEnvelope) {
			if err := broker.Publish(pbEnv); err != nil {
				log.Printf("Failed to publish envelope: %v", err)

				if retryBuffer != nil {
					traceID := fmt.Sprintf("env-%d", time.Now().UnixNano())
					axcpEnv := axcp.NewEnvelope(traceID, 0)
					id := axcpEnv.GetTraceId()

					if err := retryBuffer.AddEnvelope(id, axcpEnv); err != nil {
						log.Printf("Failed to add envelope to retry buffer. id=%s, error=%v", id, err)
					} else {
						log.Printf("Added envelope to retry buffer. id=%s", id)
					}
				}
			}
		}

		if err := internal.RunQuicServer(addr, tlsConf, handler, telemetryHandler); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
}

func buildGatewayTLSConfig(certFile, keyFile string, allowInsecureDemoTLS bool) (*tls.Config, error) {
	if certFile != "" || keyFile != "" {
		if certFile == "" || keyFile == "" {
			return nil, fmt.Errorf("-tls-cert and -tls-key must be provided together")
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load server TLS keypair: %w", err)
		}
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
			NextProtos:   []string{"axcp/1"},
		}, nil
	}

	if allowInsecureDemoTLS {
		log.Print("WARNING: using ephemeral self-signed TLS; this mode is for local demos only")
		return netquic.InsecureTLSConfig(), nil
	}

	return nil, fmt.Errorf("server TLS requires -tls-cert and -tls-key; use -allow-insecure-demo-tls only for local demos")
}
