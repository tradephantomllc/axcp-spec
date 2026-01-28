// Package main demonstrates AXCP Secure Baseline authentication.
//
// This example shows end-to-end DID + Ed25519 signature verification
// using local, in-memory DID resolution (no external network dependencies).
//
// Run server: go run . -server
// Run client: go run .
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/quic-go/quic-go"
	"google.golang.org/protobuf/proto"

	"github.com/tradephantom/axcp-spec/sdk/go/auth"
	pb "github.com/tradephantom/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantom/axcp-spec/sdk/go/negotiate"
)

const (
	defaultAddr = "localhost:61301"
	alpnProto   = "axcp-auth-chat"
)

// DeterministicKeyPair generates a deterministic Ed25519 keypair from a seed.
// This allows consistent DIDs across runs for testing.
func DeterministicKeyPair(seed string) (ed25519.PublicKey, ed25519.PrivateKey) {
	// Create a deterministic seed by hashing the input
	seedBytes := make([]byte, ed25519.SeedSize)
	copy(seedBytes, []byte(seed))
	priv := ed25519.NewKeyFromSeed(seedBytes)
	return priv.Public().(ed25519.PublicKey), priv
}

// SharedDIDResolver returns a resolver pre-populated with both server and client DIDs.
// This enables fully offline operation without network DID resolution.
func SharedDIDResolver() *auth.MemoryDIDResolver {
	resolver := auth.NewMemoryDIDResolver()

	// Generate deterministic keypairs for consistent DIDs
	serverPub, _ := DeterministicKeyPair("axcp-test-server-key-001")
	clientPub, _ := DeterministicKeyPair("axcp-test-client-key-001")

	serverDID := "did:key:axcp-auth-chat-server"
	clientDID := "did:key:axcp-auth-chat-client"

	resolver.AddDID(serverDID, serverPub)
	resolver.AddDID(clientDID, clientPub)

	return resolver
}

func main() {
	serverMode := flag.Bool("server", false, "Run in server mode")
	flag.Parse()

	// Generate TLS certificate for QUIC transport
	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("Failed to generate certificate: %v", err)
	}

	tlsConf := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		NextProtos:         []string{alpnProto},
		InsecureSkipVerify: true, // Testing only
	}

	if *serverMode {
		runServer(tlsConf)
	} else {
		runClient(tlsConf)
	}
}

func runServer(tlsConf *tls.Config) {
	log.Println("=== AXCP Authenticated Chat Server ===")
	log.Println("Profile: secure-baseline-v1")

	// Initialize server identity (deterministic for testing)
	_, serverPriv := DeterministicKeyPair("axcp-test-server-key-001")
	serverDID := "did:key:axcp-auth-chat-server"

	// Create server session
	session := auth.NewSession(auth.SessionConfig{
		LocalDID:   serverDID,
		PrivateKey: serverPriv,
	})

	// Initialize shared resolver (contains both server and client DIDs)
	resolver := SharedDIDResolver()
	log.Printf("DID Resolver initialized with %d identities", 2)
	log.Printf("Server DID: %s", serverDID)

	// Start QUIC listener
	listener, err := quic.ListenAddr(defaultAddr, tlsConf, &quic.Config{})
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("Listening on %s", defaultAddr)

	// Accept connection
	conn, err := listener.Accept(context.Background())
	if err != nil {
		log.Fatalf("Failed to accept connection: %v", err)
	}
	log.Println("Client connected")

	// Accept stream
	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		log.Fatalf("Failed to accept stream: %v", err)
	}
	defer stream.Close()

	// Read envelope from client
	buf := make([]byte, 8192)
	n, err := stream.Read(buf)
	if err != nil {
		log.Fatalf("Failed to read: %v", err)
	}

	// Deserialize envelope
	var env pb.AxcpEnvelope
	if err := proto.Unmarshal(buf[:n], &env); err != nil {
		log.Fatalf("Failed to unmarshal envelope: %v", err)
	}

	log.Printf("\n--- Received Envelope ---")
	log.Printf("  Version:    %d", env.Version)
	log.Printf("  TraceID:    %s", env.TraceId)
	log.Printf("  Profile:    %d (secure-baseline-v1)", env.Profile)
	log.Printf("  SenderDID:  %s", env.SenderDid)
	log.Printf("  RecipientDID: %s", env.RecipientDid)
	log.Printf("  TimestampMs: %d", env.TimestampMs)
	log.Printf("  Sequence:   %d", env.Sequence)
	log.Printf("  Signature:  %d bytes", len(env.Signature))

	// Verify signature
	log.Printf("\n--- Verifying Signature ---")
	clientDID := env.SenderDid

	// Resolve client's public key
	doc, err := resolver.Resolve(context.Background(), clientDID)
	if err != nil {
		log.Fatalf("Failed to resolve client DID: %v", err)
	}
	clientPubKey := doc.PublicKeys[0].PublicKeyBytes

	// Rebuild transcript for verification
	// Extract payload (envelope without auth fields)
	envCopy := &pb.AxcpEnvelope{
		Version: env.Version,
		TraceId: env.TraceId,
		Profile: env.Profile,
	}
	if env.GetContextPatch() != nil {
		envCopy.Payload = &pb.AxcpEnvelope_ContextPatch{ContextPatch: env.GetContextPatch()}
	}
	payloadBytes, _ := proto.Marshal(envCopy)

	timestamp := time.UnixMilli(int64(env.TimestampMs))
	transcript := auth.BuildDIDAuthTranscript(clientDID, env.RecipientDid, payloadBytes, timestamp)

	if auth.VerifyEd25519(clientPubKey, transcript, env.Signature) {
		log.Printf("  Signature VALID")
	} else {
		log.Fatalf("  Signature INVALID - rejecting message")
	}

	// Complete server session negotiation for response
	session.SetNegotiated(string(negotiate.ProfileSecureBaseline), clientDID)

	// Build response envelope
	respPatch := &pb.ContextPatch{
		ContextId:   "response-ctx",
		BaseVersion: 1,
		Ops: []*pb.DeltaOp{
			{
				Op:   pb.DeltaOp_ADD,
				Path: "/server_response",
				Data: []byte(`{"status":"authenticated","message":"Hello from server!"}`),
			},
		},
	}

	respEnv := &pb.AxcpEnvelope{
		Version: 1,
		TraceId: env.TraceId + "-resp",
		Profile: 1, // secure-baseline-v1
		Payload: &pb.AxcpEnvelope_ContextPatch{ContextPatch: respPatch},
	}

	// Sign response
	respPayloadBytes, _ := proto.Marshal(respEnv)
	sigOutput, err := session.SignEnvelope(context.Background(), auth.SignatureInput{
		Payload:      respPayloadBytes,
		RecipientDID: clientDID,
	})
	if err != nil {
		log.Fatalf("Failed to sign response: %v", err)
	}

	respEnv.SenderDid = sigOutput.SenderDID
	respEnv.RecipientDid = sigOutput.RecipientDID
	respEnv.TimestampMs = sigOutput.TimestampMs
	respEnv.Sequence = sigOutput.Sequence
	respEnv.Signature = sigOutput.Signature

	// Send response
	respBytes, _ := proto.Marshal(respEnv)
	if _, err := stream.Write(respBytes); err != nil {
		log.Fatalf("Failed to send response: %v", err)
	}

	log.Printf("\n--- Sent Authenticated Response ---")
	log.Printf("  Sequence: %d", respEnv.Sequence)
	log.Println("\nAuthentication flow completed successfully!")
}

func runClient(tlsConf *tls.Config) {
	log.Println("=== AXCP Authenticated Chat Client ===")
	log.Println("Profile: secure-baseline-v1")

	// Initialize client identity (deterministic for testing)
	clientPub, clientPriv := DeterministicKeyPair("axcp-test-client-key-001")
	clientDID := "did:key:axcp-auth-chat-client"
	serverDID := "did:key:axcp-auth-chat-server"

	log.Printf("Client DID: %s", clientDID)
	log.Printf("Server DID: %s", serverDID)
	log.Printf("Client Public Key: %x...", clientPub[:8])

	// Create client session
	session := auth.NewSession(auth.SessionConfig{
		LocalDID:   clientDID,
		PrivateKey: clientPriv,
	})

	// Simulate profile negotiation (in real usage, this happens via handshake)
	session.SetNegotiated(string(negotiate.ProfileSecureBaseline), serverDID)

	// Initialize shared resolver for response verification
	resolver := SharedDIDResolver()

	// Connect to server
	conn, err := quic.DialAddr(context.Background(), defaultAddr, tlsConf, &quic.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.CloseWithError(0, "done")

	log.Printf("Connected to %s", defaultAddr)

	// Open stream
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// Build request envelope
	reqPatch := &pb.ContextPatch{
		ContextId:   "request-ctx-001",
		BaseVersion: 0,
		Ops: []*pb.DeltaOp{
			{
				Op:   pb.DeltaOp_ADD,
				Path: "/client_message",
				Data: []byte(`{"action":"greet","content":"Hello, authenticated server!"}`),
			},
		},
	}

	reqEnv := &pb.AxcpEnvelope{
		Version: 1,
		TraceId: "auth-chat-001",
		Profile: 1, // secure-baseline-v1
		Payload: &pb.AxcpEnvelope_ContextPatch{ContextPatch: reqPatch},
	}

	// Sign the envelope
	payloadBytes, _ := proto.Marshal(reqEnv)
	sigOutput, err := session.SignEnvelope(context.Background(), auth.SignatureInput{
		Payload: payloadBytes,
	})
	if err != nil {
		log.Fatalf("Failed to sign envelope: %v", err)
	}

	// Add auth fields
	reqEnv.SenderDid = sigOutput.SenderDID
	reqEnv.RecipientDid = sigOutput.RecipientDID
	reqEnv.TimestampMs = sigOutput.TimestampMs
	reqEnv.Sequence = sigOutput.Sequence
	reqEnv.Signature = sigOutput.Signature

	log.Printf("\n--- Sending Authenticated Envelope ---")
	log.Printf("  TraceID:    %s", reqEnv.TraceId)
	log.Printf("  SenderDID:  %s", reqEnv.SenderDid)
	log.Printf("  RecipientDID: %s", reqEnv.RecipientDid)
	log.Printf("  TimestampMs: %d", reqEnv.TimestampMs)
	log.Printf("  Sequence:   %d", reqEnv.Sequence)
	log.Printf("  Signature:  %d bytes", len(reqEnv.Signature))

	// Send envelope
	reqBytes, _ := proto.Marshal(reqEnv)
	if _, err := stream.Write(reqBytes); err != nil {
		log.Fatalf("Failed to send: %v", err)
	}

	// Read response
	buf := make([]byte, 8192)
	n, err := stream.Read(buf)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	// Deserialize response
	var respEnv pb.AxcpEnvelope
	if err := proto.Unmarshal(buf[:n], &respEnv); err != nil {
		log.Fatalf("Failed to unmarshal response: %v", err)
	}

	log.Printf("\n--- Received Response ---")
	log.Printf("  TraceID:    %s", respEnv.TraceId)
	log.Printf("  SenderDID:  %s", respEnv.SenderDid)
	log.Printf("  Sequence:   %d", respEnv.Sequence)

	// Verify server's signature
	log.Printf("\n--- Verifying Server Signature ---")
	doc, err := resolver.Resolve(context.Background(), respEnv.SenderDid)
	if err != nil {
		log.Fatalf("Failed to resolve server DID: %v", err)
	}
	serverPubKey := doc.PublicKeys[0].PublicKeyBytes

	// Rebuild transcript
	respCopy := &pb.AxcpEnvelope{
		Version: respEnv.Version,
		TraceId: respEnv.TraceId,
		Profile: respEnv.Profile,
	}
	if respEnv.GetContextPatch() != nil {
		respCopy.Payload = &pb.AxcpEnvelope_ContextPatch{ContextPatch: respEnv.GetContextPatch()}
	}
	respPayloadBytes, _ := proto.Marshal(respCopy)

	respTimestamp := time.UnixMilli(int64(respEnv.TimestampMs))
	respTranscript := auth.BuildDIDAuthTranscript(respEnv.SenderDid, respEnv.RecipientDid, respPayloadBytes, respTimestamp)

	if auth.VerifyEd25519(serverPubKey, respTranscript, respEnv.Signature) {
		log.Printf("  Server Signature VALID")
	} else {
		log.Fatalf("  Server Signature INVALID")
	}

	log.Println("\nBidirectional authenticated exchange completed!")
}

func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("key generation failed: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"AXCP Auth Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certificate creation failed: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return tls.X509KeyPair(certPEM, keyPEM)
}
