// Secure Baseline example demonstrates a complete AXCP secure-baseline-v1
// client-server exchange with Ed25519 authentication and validation.
//
// Run as server:  go run main.go -server
// Run as client:  go run main.go -addr localhost:61300
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	"github.com/tradephantomllc/axcp-spec/sdk/go/netquic"
)

func main() {
	serverMode := flag.Bool("server", false, "Run in server mode")
	addr := flag.String("addr", "localhost:61300", "Server address")
	flag.Parse()

	if *serverMode {
		if err := runServer(*addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		if err := runClient(*addr); err != nil {
			log.Fatalf("Client error: %v", err)
		}
	}
}

func runServer(addr string) error {
	// Generate server identity
	serverPub, serverPriv, _ := ed25519.GenerateKey(nil)
	serverDID := "did:key:demo-server"
	log.Printf("Server DID: %s", serverDID)

	// Create DID resolver
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(serverDID, serverPub)

	// Create TLS config
	tlsConfig := netquic.InsecureTLSConfig()

	// Create listener
	listener, err := netquic.Listen(netquic.ListenerConfig{
		Address:        addr,
		TLSConfig:      tlsConfig,
		MaxConnections: 100,
		AcceptTimeout:  30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	log.Printf("AXCP Server listening on %s (secure-baseline-v1)", addr)

	// Create validator
	validator := axcp.NewValidator(axcp.ValidatorConfig{
		LocalDID:                  serverDID,
		Resolver:                  resolver,
		SkipSignatureVerification: true, // For demo, accept any client
	})

	// Accept connections
	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		go handleConnection(conn, serverDID, serverPriv, validator)
	}
}

func handleConnection(conn *netquic.ServerConnection, serverDID string, serverPriv ed25519.PrivateKey, validator *axcp.Validator) {
	defer conn.Close()
	log.Printf("New connection from %s", conn.RemoteAddr())

	for {
		// Receive envelope
		env, err := conn.RecvEnvelope()
		if err != nil {
			log.Printf("Recv error: %v", err)
			return
		}

		log.Printf("Received: TraceID=%s From=%s", env.TraceId, env.SenderDid)

		// Validate (skip signature verification for demo)
		if perr := validator.QuickValidate(env); perr != nil {
			log.Printf("Validation error: %v", perr)
			// Send error response
			errEnv := axcp.NewErrorEnvelope(env.TraceId, serverDID, env.SenderDid, perr)
			_ = conn.SendEnvelope(errEnv)
			continue
		}

		// Create response
		resp := axcp.NewEnvelope(env.TraceId, 1)
		resp.SenderDid = serverDID
		resp.RecipientDid = env.SenderDid
		resp.TimestampMs = auth.NowMs()
		resp.Sequence = 1

		// Sign response
		transcript, err := buildTranscript(resp)
		if err != nil {
			log.Printf("Failed to build response auth transcript: %v", err)
			perr := axcp.ErrMalformedRequest.WithReason("failed to build response auth transcript")
			_ = conn.SendEnvelope(axcp.NewErrorEnvelope(env.TraceId, serverDID, env.SenderDid, perr))
			continue
		}
		resp.Signature = ed25519.Sign(serverPriv, transcript)

		// Send response
		if err := conn.SendEnvelope(resp); err != nil {
			log.Printf("Send error: %v", err)
			return
		}

		log.Printf("Sent response for TraceID=%s", resp.TraceId)
	}
}

func runClient(serverAddr string) error {
	// Generate client identity
	clientPub, clientPriv, _ := ed25519.GenerateKey(nil)
	clientDID := "did:key:demo-client-" + uuid.NewString()[:8]
	log.Printf("Client DID: %s", clientDID)

	// Server DID (in real app, would be configured or discovered)
	serverDID := "did:key:demo-server"

	// Create TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"axcp/1"},
	}

	// Connect to server
	client, err := netquic.Dial(serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	log.Printf("Connected to %s", serverAddr)

	// Create resolver for verifying server responses
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(clientDID, clientPub)

	// Send 3 messages
	for i := 1; i <= 3; i++ {
		traceID := uuid.NewString()

		// Create envelope
		env := axcp.NewEnvelope(traceID, 1)
		env.SenderDid = clientDID
		env.RecipientDid = serverDID
		env.TimestampMs = auth.NowMs()
		env.Sequence = uint64(i)

		// Sign envelope
		transcript, err := buildTranscript(env)
		if err != nil {
			return fmt.Errorf("build auth transcript: %w", err)
		}
		env.Signature = ed25519.Sign(clientPriv, transcript)

		log.Printf("[%d] Sending: TraceID=%s", i, traceID[:8])

		// Send
		if err := client.SendEnvelope(env); err != nil {
			return fmt.Errorf("send: %w", err)
		}

		// Receive response
		resp, err := client.RecvEnvelope()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}

		// Check for error response
		if axcp.IsErrorEnvelope(resp) {
			perr, _ := axcp.ErrorFromEnvelope(resp)
			log.Printf("[%d] Error response: %v", i, perr)
			continue
		}

		log.Printf("[%d] Response received: TraceID=%s From=%s", i, resp.TraceId[:8], resp.SenderDid)

		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("AXCP exchange complete!")
	return nil
}

// buildTranscript creates the canonical DID auth transcript for an envelope.
func buildTranscript(env *axcp.Envelope) ([]byte, error) {
	return axcp.BuildAuthTranscript(env)
}
