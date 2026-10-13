package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
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
	"google.golang.org/protobuf/proto"
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

const defaultSecureBaseline = true

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Parse command line flags
	var addr string
	var brokerURL string
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
	flag.StringVar(&brokerURL, "mqtt-url", "tcp://127.0.0.1:1883", "MQTT broker URL")
	flag.BoolVar(&enableRetryBuffer, "retry", true, "Enable retry buffer for failed messages")
	flag.IntVar(&maxRetryCapacity, "retry-capacity", 1000, "Maximum capacity of retry buffer")
	flag.IntVar(&maxRetryAttempts, "retry-attempts", 5, "Maximum retry attempts per message")
	flag.DurationVar(&minRetryInterval, "retry-min-interval", 1*time.Second, "Minimum retry interval")
	flag.DurationVar(&maxRetryInterval, "retry-max-interval", 5*time.Minute, "Maximum retry interval")
	flag.BoolVar(&secureBaseline, "secure-baseline", defaultSecureBaseline, "Enable Secure Baseline authentication (DID + Ed25519 + replay protection)")
	flag.StringVar(&serverDID, "server-did", "", "Server DID for auth transcript binding (required with -secure-baseline)")
	flag.Var(&trustedDIDs, "trusted-did", "Trusted sender DID and raw Ed25519 public key; repeatable format did:key:...=<base64-public-key>")
	flag.StringVar(&tlsCertFile, "tls-cert", "", "Server TLS certificate PEM file")
	flag.StringVar(&tlsKeyFile, "tls-key", "", "Server TLS private key PEM file")
	flag.BoolVar(&allowInsecureDemoTLS, "allow-insecure-demo-tls", false, "Allow ephemeral self-signed TLS for local demos only")

	flag.Parse()

	// Validate all security and retry settings before connecting to MQTT or starting workers.
	if err := validateSecuritySettings(secureBaseline, serverDID, trustedDIDs.Len()); err != nil {
		return err
	}
	retryConfig := internal.RetryBufferConfig{
		MaxCapacity: maxRetryCapacity, MaxAttempts: maxRetryAttempts,
		MinRetryInterval: minRetryInterval, MaxRetryInterval: maxRetryInterval, BackoffFactor: 2,
	}
	if enableRetryBuffer {
		if err := retryConfig.Validate(); err != nil {
			return err
		}
	}
	tlsConf, err := buildGatewayTLSConfig(tlsCertFile, tlsKeyFile, allowInsecureDemoTLS)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config := internal.ServerConfig{Addr: addr, TLSConf: tlsConf, Context: ctx, Profile: negotiate.ProfileTransportOnly}
	if secureBaseline {
		config.Profile = negotiate.ProfileSecureBaseline
		config.ServerDID = serverDID
		config.DIDResolver = trustedDIDs.Resolver()
		config.Authenticator, err = internal.NewEnvelopeAuthenticator(config.DIDResolver, serverDID, internal.DefaultAuthConfig(), nil)
		if err != nil {
			return fmt.Errorf("configure authentication: %w", err)
		}
	} else {
		log.Print("WARNING: explicit transport-only mode; application authentication and replay protection are disabled")
	}
	broker, err := internal.NewBroker(internal.BrokerConfig{URL: brokerURL})
	if err != nil {
		return err
	}
	defer broker.Close()
	publisher := gatewayPublisher{broker}
	var retries *internal.RetryBuffer
	if enableRetryBuffer {
		retries = internal.NewRetryBuffer(&retryConfig, nil, func(env *axcp.Envelope) error {
			if env == nil {
				return fmt.Errorf("retry envelope is required")
			}
			return publisher.publish(&env.AxcpEnvelope)
		})
		retries.Start()
		defer retries.Close()
	}
	handler := func(env *pb.AxcpEnvelope, _ *internal.AuthResult) {
		if err := publishOrQueue(publisher, retries, env); err != nil {
			log.Printf("publish failed: %v", err)
		}
	}
	telemetry := func(td *pb.TelemetryDatagram) {
		env := &pb.AxcpEnvelope{Version: 1, TraceId: "telemetry-" + rand.Text(), Payload: &pb.AxcpEnvelope_Telemetry{Telemetry: td}}
		handler(env, nil)
	}
	log.Printf("Starting AXCP gateway %s on %s (%s)", BuildVersion, addr, config.Profile)
	return internal.RunAuthenticatedQuicServer(config, handler, telemetry)
}

func validateSecuritySettings(secure bool, serverDID string, trustedCount int) error {
	if !secure {
		return nil
	}
	if serverDID == "" {
		return fmt.Errorf("-server-did is required; authentication is enabled by default")
	}
	if _, err := auth.ParseDID(serverDID); err != nil {
		return fmt.Errorf("invalid -server-did: %w", err)
	}
	if trustedCount == 0 {
		return fmt.Errorf("at least one -trusted-did is required for secure-baseline")
	}
	return nil
}

type mqttPublisher interface {
	Publish(*pb.AxcpEnvelope) error
	PublishTelemetry(*pb.TelemetryDatagram, string) error
}

type gatewayPublisher struct{ mqttPublisher }

func (p gatewayPublisher) publish(env *pb.AxcpEnvelope) error {
	if env == nil {
		return fmt.Errorf("publish envelope is required")
	}
	// Legacy telemetry keeps its raw topic. Signed envelopes retain all authentication fields.
	if env.Profile == 0 && len(env.Signature) == 0 && env.GetTelemetry() != nil {
		return p.PublishTelemetry(env.GetTelemetry(), env.TraceId)
	}
	return p.Publish(env)
}

func publishOrQueue(p gatewayPublisher, retries *internal.RetryBuffer, env *pb.AxcpEnvelope) error {
	if err := p.publish(env); err != nil {
		if retries == nil {
			return err
		}
		clone := &axcp.Envelope{AxcpEnvelope: *proto.Clone(env).(*pb.AxcpEnvelope)}
		if queueErr := retries.AddEnvelope(rand.Text(), clone); queueErr != nil {
			return fmt.Errorf("publish: %v; enqueue: %w", err, queueErr)
		}
		log.Printf("MQTT publish failed; original message queued for bounded retry: %v", err)
	}
	return nil
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
