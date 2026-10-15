// Package main demonstrates AXCP Secure Baseline authentication.
//
// This example shows end-to-end DID + Ed25519 signature verification
// using local, in-memory DID resolution (no external network dependencies).
//
// Run server: go run . -server
// Run client: go run .
// Run with replay test: go run . -replay
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"time"

	"github.com/quic-go/quic-go"
	"google.golang.org/protobuf/proto"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"github.com/tradephantomllc/axcp-spec/sdk/go/negotiate"
)

const (
	defaultAddr      = "localhost:61301"
	alpnProto        = "axcp-auth-chat"
	maxEnvelopeBytes = 64 * 1024
)

// sha256Hex computes SHA256 hash of data and returns hex string
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// logProtobufEvidence logs Protobuf marshal/unmarshal evidence with bytes and SHA256
func logProtobufEvidence(operation string, data []byte) {
	log.Printf("[PROTOBUF] %s: bytes=%d, sha256=%s", operation, len(data), sha256Hex(data))
}

// logQUICEvidence logs QUIC connection evidence
func logQUICEvidence(conn *quic.Conn, role string) {
	state := conn.ConnectionState()
	log.Printf("[QUIC] %s Connection Established", role)
	log.Printf("[QUIC]   Local Addr:  %s", conn.LocalAddr())
	log.Printf("[QUIC]   Remote Addr: %s", conn.RemoteAddr())
	log.Printf("[QUIC]   ALPN:        %s", state.TLS.NegotiatedProtocol)
	log.Printf("[QUIC]   TLS Version: 0x%04x", state.TLS.Version)
	log.Printf("[QUIC]   Cipher Suite: 0x%04x", state.TLS.CipherSuite)
}

func readStreamMessage(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxEnvelopeBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty stream message")
	}
	if len(data) > maxEnvelopeBytes {
		return nil, fmt.Errorf("stream message exceeds %d bytes", maxEnvelopeBytes)
	}
	return data, nil
}

func writeStreamMessage(w io.Writer, data []byte) error {
	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

// DeterministicKeyPair generates a deterministic Ed25519 keypair from a seed.
// This allows consistent DIDs across runs for testing.
func DeterministicKeyPair(seed string) (ed25519.PublicKey, ed25519.PrivateKey) {
	// Create a deterministic seed by hashing the input
	seedBytes := make([]byte, ed25519.SeedSize)
	copy(seedBytes, []byte(seed))
	priv := ed25519.NewKeyFromSeed(seedBytes)
	return priv.Public().(ed25519.PublicKey), priv
}

const (
	serverSeed = "axcp-test-server-key-001"
	clientSeed = "axcp-test-client-key-001"
)

// identity returns the deterministic demo keypair for seed and its did:key.
func identity(seed string) (string, ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv := DeterministicKeyPair(seed)
	did, err := auth.EncodeDIDKey(pub)
	if err != nil {
		log.Fatalf("Failed to derive did:key: %v", err)
	}
	return did, pub, priv
}

// SharedDIDResolver returns a resolver pre-populated with both server and client DIDs.
// It acts as an allowlist: only these two identities are accepted.
func SharedDIDResolver() *auth.MemoryDIDResolver {
	resolver := auth.NewMemoryDIDResolver()
	serverDID, serverPub, _ := identity(serverSeed)
	clientDID, clientPub, _ := identity(clientSeed)
	resolver.AddDID(serverDID, serverPub)
	resolver.AddDID(clientDID, clientPub)
	return resolver
}

// decodeEnvelope decodes and structurally validates a received envelope.
func decodeEnvelope(data []byte) *axcp.Envelope {
	env, err := axcp.FromBytesStrict(data)
	if err != nil {
		log.Fatalf("Rejected envelope: %v", err)
	}
	return env
}

func main() {
	serverMode := flag.Bool("server", false, "Run in server mode")
	replayMode := flag.Bool("replay", false, "Run replay attack test (server must support it)")
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
	} else if *replayMode {
		runReplayTest(tlsConf)
	} else {
		runClient(tlsConf)
	}
}

func runServer(tlsConf *tls.Config) {
	log.Println("=== AXCP Authenticated Chat Server ===")
	log.Println("Profile: secure-baseline-v1")

	// Initialize server identity (deterministic for testing)
	serverDID, _, serverPriv := identity(serverSeed)

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

	// Log QUIC connection evidence
	logQUICEvidence(conn, "Server")

	// Accept request stream
	reqStream, err := conn.AcceptStream(context.Background())
	if err != nil {
		log.Fatalf("Failed to accept stream: %v", err)
	}
	defer reqStream.Close()

	// Read envelope from client. The client closes its request stream after
	// sending one envelope, so ReadAll correctly handles data followed by EOF.
	reqBytes, err := readStreamMessage(reqStream)
	if err != nil {
		log.Fatalf("Failed to read request envelope: %v", err)
	}

	// Deserialize envelope (strict: unknown fields and ambiguous payloads are rejected)
	env := decodeEnvelope(reqBytes)

	// Log Protobuf evidence
	logProtobufEvidence("proto.Unmarshal (received)", reqBytes)

	log.Printf("\n--- Received Envelope ---")
	log.Printf("  Version:    %d", env.Version)
	log.Printf("  TraceID:    %s", env.TraceId)
	log.Printf("  Profile:    %d (secure-baseline-v1)", env.Profile)
	log.Printf("  SenderDID:  %s", env.SenderDid)
	log.Printf("  RecipientDID: %s", env.RecipientDid)
	log.Printf("  TimestampMs: %d", env.TimestampMs)
	log.Printf("  Sequence:   %d", env.Sequence)
	log.Printf("  Signature:  %d bytes", len(env.Signature))

	// Full Secure Baseline verification with the SDK validator: structure,
	// recipient, timestamp, signature and, after the signature, replay.
	log.Printf("\n--- Verifying Envelope ---")
	clientDID := env.SenderDid
	validator := axcp.NewValidator(axcp.ValidatorConfig{LocalDID: serverDID, Resolver: resolver})
	if perr := validator.ValidateAndVerify(context.Background(), env); perr != nil {
		log.Fatalf("  Envelope REJECTED: %v", perr)
	}
	log.Printf("  Signature VALID, recipient, timestamp and replay checks passed")

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

	// Sign response. The session allocates sequence first, then the payload
	// builder inserts that sequence into the canonical signing payload.
	sigOutput, err := session.SignEnvelope(context.Background(), auth.SignatureInput{
		RecipientDID: clientDID,
		PayloadForSequence: func(sequence uint64) ([]byte, error) {
			respEnv.Sequence = sequence
			return axcp.SigningPayloadFromProto(respEnv)
		},
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

	// Log Protobuf evidence
	logProtobufEvidence("proto.Marshal (response)", respBytes)

	respStream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatalf("Failed to open response stream: %v", err)
	}
	if err := writeStreamMessage(respStream, respBytes); err != nil {
		log.Fatalf("Failed to send response: %v", err)
	}
	if err := respStream.Close(); err != nil {
		log.Fatalf("Failed to close response stream: %v", err)
	}

	log.Printf("\n--- Sent Authenticated Response ---")
	log.Printf("  Sequence: %d", respEnv.Sequence)
	log.Println("\nAuthentication flow completed successfully!")

	select {
	case <-conn.Context().Done():
		log.Println("Client connection closed")
	case <-time.After(5 * time.Second):
		log.Fatalf("Timed out waiting for client connection close")
	}
}

func runClient(tlsConf *tls.Config) {
	log.Println("=== AXCP Authenticated Chat Client ===")
	log.Println("Profile: secure-baseline-v1")

	// Initialize client identity (deterministic for testing)
	clientDID, clientPub, clientPriv := identity(clientSeed)
	serverDID, _, _ := identity(serverSeed)

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

	// Log QUIC connection evidence
	logQUICEvidence(conn, "Client")

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

	// Sign the envelope. The sequence is part of the signed payload, so it must
	// be assigned before canonical signing payload bytes are built.
	sigOutput, err := session.SignEnvelope(context.Background(), auth.SignatureInput{
		PayloadForSequence: func(sequence uint64) ([]byte, error) {
			reqEnv.Sequence = sequence
			return axcp.SigningPayloadFromProto(reqEnv)
		},
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

	// Log Protobuf evidence
	logProtobufEvidence("proto.Marshal (request)", reqBytes)

	if err := writeStreamMessage(stream, reqBytes); err != nil {
		log.Fatalf("Failed to send: %v", err)
	}
	if err := stream.Close(); err != nil {
		log.Fatalf("Failed to close request stream: %v", err)
	}

	// Read response on a server-initiated stream.
	respStream, err := conn.AcceptStream(context.Background())
	if err != nil {
		log.Fatalf("Failed to accept response stream: %v", err)
	}
	defer respStream.Close()

	respBytes, err := readStreamMessage(respStream)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	// Deserialize response
	respEnv := decodeEnvelope(respBytes)

	// Log Protobuf evidence
	logProtobufEvidence("proto.Unmarshal (response)", respBytes)

	log.Printf("\n--- Received Response ---")
	log.Printf("  TraceID:    %s", respEnv.TraceId)
	log.Printf("  SenderDID:  %s", respEnv.SenderDid)
	log.Printf("  Sequence:   %d", respEnv.Sequence)

	// Verify the server's response with the same SDK validator; only the
	// expected server identity is accepted.
	log.Printf("\n--- Verifying Server Response ---")
	validator := axcp.NewValidator(axcp.ValidatorConfig{
		LocalDID:    clientDID,
		Resolver:    resolver,
		AllowedDIDs: []string{serverDID},
	})
	if perr := validator.ValidateAndVerify(context.Background(), respEnv); perr != nil {
		log.Fatalf("  Server response REJECTED: %v", perr)
	}
	log.Printf("  Server Signature VALID")

	log.Println("\nBidirectional authenticated exchange completed!")
}

// runReplayTest demonstrates replay protection with the SDK validator and its
// ReplayProtector: replay state is updated only after the signature verifies.
// It runs locally; no server is needed.
func runReplayTest(_ *tls.Config) {
	log.Println("=== AXCP Replay Protection Test ===")
	log.Println("This test demonstrates that replayed messages are REJECTED")

	clientDID, _, clientPriv := identity(clientSeed)
	serverDID, _, _ := identity(serverSeed)

	session := auth.NewSession(auth.SessionConfig{LocalDID: clientDID, PrivateKey: clientPriv})
	session.SetNegotiated(string(negotiate.ProfileSecureBaseline), serverDID)

	// The receiving side: full Secure Baseline validation with replay protection.
	validator := axcp.NewValidator(axcp.ValidatorConfig{LocalDID: serverDID, Resolver: SharedDIDResolver()})

	newSigned := func(traceID string) *axcp.Envelope {
		env := axcp.NewEnvelope(traceID, 1)
		env.Payload = &pb.AxcpEnvelope_ContextPatch{ContextPatch: &pb.ContextPatch{
			ContextId: "replay-test-ctx",
			Ops:       []*pb.DeltaOp{{Op: pb.DeltaOp_ADD, Path: "/test_data", Data: []byte(`{"test":"replay"}`)}},
		}}
		out, err := session.SignEnvelope(context.Background(), auth.SignatureInput{
			PayloadForSequence: func(sequence uint64) ([]byte, error) {
				env.Sequence = sequence
				return axcp.SigningPayload(env)
			},
		})
		if err != nil {
			log.Fatalf("Failed to sign: %v", err)
		}
		env.SenderDid, env.RecipientDid = out.SenderDID, out.RecipientDID
		env.TimestampMs, env.Sequence, env.Signature = out.TimestampMs, out.Sequence, out.Signature
		return env
	}

	// Serialize and decode like a network peer would.
	wire := func(env *axcp.Envelope) *axcp.Envelope {
		raw, err := axcp.ToBytes(env)
		if err != nil {
			log.Fatalf("Failed to encode: %v", err)
		}
		logProtobufEvidence("axcp.ToBytes (test envelope)", raw)
		return decodeEnvelope(raw)
	}

	first := newSigned("replay-test-001")
	failures := 0
	check := func(label string, env *axcp.Envelope, wantReplay bool) {
		perr := validator.ValidateAndVerify(context.Background(), wire(env))
		switch {
		case !wantReplay && perr == nil:
			log.Printf("[RESULT] %s (seq=%d): ACCEPTED", label, env.Sequence)
		case wantReplay && errors.Is(perr, axcp.ErrAuthReplayDetected):
			log.Printf("[RESULT] %s (seq=%d): REJECTED as replay (expected)", label, env.Sequence)
		default:
			failures++
			log.Printf("[RESULT] %s (seq=%d): UNEXPECTED result %v", label, env.Sequence, perr)
		}
	}

	log.Println("\n--- Test 1: First request (should PASS) ---")
	check("First request", first, false)
	log.Println("\n--- Test 2: Replay of the SAME envelope (should FAIL) ---")
	check("Replay attack", first, true)
	log.Println("\n--- Test 3: New signed request (should PASS) ---")
	check("New request", newSigned("replay-test-002"), false)

	if failures > 0 {
		log.Fatalf("Replay protection test FAILED (%d unexpected results)", failures)
	}
	log.Println("\nReplay protection is working correctly!")
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
