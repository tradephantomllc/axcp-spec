package test

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
	"github.com/tradephantomllc/axcp-spec/sdk/go/netquic"
)

// TestIntegration_ClientServerExchange tests a complete client-server message exchange.
func TestIntegration_ClientServerExchange(t *testing.T) {
	// Generate server identity
	serverPub, serverPriv, _ := ed25519.GenerateKey(nil)
	serverDID := "did:key:server-test"

	// Generate client identity
	clientPub, clientPriv, _ := ed25519.GenerateKey(nil)
	clientDID := "did:key:client-test"

	// Create resolver with both DIDs
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(serverDID, serverPub)
	resolver.AddDID(clientDID, clientPub)

	// Generate TLS config for server
	serverTLS := netquic.InsecureTLSConfig()

	// Start server listener
	listenerConfig := netquic.ListenerConfig{
		Address:        "127.0.0.1:0",
		TLSConfig:      serverTLS,
		MaxConnections: 10,
		AcceptTimeout:  5 * time.Second,
	}

	listener, err := netquic.Listen(listenerConfig)
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()
	t.Logf("Server listening on %s", serverAddr)

	// Create validator for server
	validator := axcp.NewValidator(axcp.ValidatorConfig{
		LocalDID: serverDID,
		Resolver: resolver,
	})

	// Server handler - echoes back envelopes
	serverHandler := axcp.HandlerFunc(func(ctx context.Context, req *axcp.Envelope) (*axcp.Envelope, error) {
		// Validate request
		if perr := validator.ValidateAndVerify(ctx, req); perr != nil {
			return nil, perr
		}

		// Create response
		resp := axcp.NewEnvelope(req.TraceId, 1)
		resp.SenderDid = serverDID
		resp.RecipientDid = req.SenderDid
		resp.TimestampMs = auth.NowMs()
		resp.Sequence = 1

		// Sign response
		transcript := buildEnvelopeTranscript(resp)
		resp.Signature = ed25519.Sign(serverPriv, transcript)

		return resp, nil
	})

	// Channel to signal when client is done reading
	clientDone := make(chan struct{})

	// Start server goroutine
	var serverErr error
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		conn, err := listener.Accept(context.Background())
		if err != nil {
			serverErr = err
			return
		}
		defer conn.Close()

		// Receive message
		env, err := conn.RecvEnvelope()
		if err != nil {
			serverErr = err
			return
		}

		// Handle message
		resp, err := serverHandler.HandleEnvelope(context.Background(), env)
		if err != nil {
			serverErr = err
			return
		}

		// Send response
		if resp != nil {
			if err := conn.SendEnvelope(resp); err != nil {
				serverErr = err
				return
			}
		}

		// Wait for client to finish reading before closing
		<-clientDone
	}()

	// Create client
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"axcp/1"},
	}

	client, err := netquic.Dial(serverAddr, clientTLS)
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer client.Close()

	// Create request envelope
	traceID := uuid.NewString()
	req := axcp.NewEnvelope(traceID, 1)
	req.SenderDid = clientDID
	req.RecipientDid = serverDID
	req.TimestampMs = auth.NowMs()
	req.Sequence = 1

	// Sign request
	transcript := buildEnvelopeTranscript(req)
	req.Signature = ed25519.Sign(clientPriv, transcript)

	// Send request
	if err := client.SendEnvelope(req); err != nil {
		t.Fatalf("Failed to send envelope: %v", err)
	}

	// Receive response
	resp, err := client.RecvEnvelope()
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	// Verify response
	if resp.TraceId != traceID {
		t.Errorf("Response TraceId = %q, want %q", resp.TraceId, traceID)
	}
	if resp.SenderDid != serverDID {
		t.Errorf("Response SenderDid = %q, want %q", resp.SenderDid, serverDID)
	}
	if resp.RecipientDid != clientDID {
		t.Errorf("Response RecipientDid = %q, want %q", resp.RecipientDid, clientDID)
	}

	// Verify server signature
	respTranscript := buildEnvelopeTranscript(resp)
	if !auth.VerifyEd25519(serverPub, respTranscript, resp.Signature) {
		t.Error("Server signature verification failed")
	}

	// Signal client is done reading
	close(clientDone)

	// Wait for server to complete
	<-serverDone
	if serverErr != nil {
		t.Errorf("Server error: %v", serverErr)
	}
}

// TestIntegration_ValidationRejectsInvalid tests that the server rejects invalid messages.
func TestIntegration_ValidationRejectsInvalid(t *testing.T) {
	// Generate server identity
	serverPub, _, _ := ed25519.GenerateKey(nil)
	serverDID := "did:key:server-test"

	// Create resolver
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(serverDID, serverPub)

	// Generate TLS config for server
	serverTLS := netquic.InsecureTLSConfig()

	// Start server listener
	listener, err := netquic.Listen(netquic.DefaultListenerConfig("127.0.0.1:0", serverTLS))
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Create validator
	validator := axcp.NewValidator(axcp.ValidatorConfig{
		LocalDID: serverDID,
		Resolver: resolver,
	})

	// Server goroutine
	var validationErr *axcp.ProtocolError
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		conn, err := listener.Accept(context.Background())
		if err != nil {
			return
		}
		defer conn.Close()

		env, err := conn.RecvEnvelope()
		if err != nil {
			return
		}

		// Validate - should fail
		validationErr = validator.ValidateAndVerify(context.Background(), env)
	}()

	// Create client
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"axcp/1"},
	}

	client, err := netquic.Dial(serverAddr, clientTLS)
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer client.Close()

	// Send envelope with invalid signature (zeros)
	req := axcp.NewEnvelope(uuid.NewString(), 1)
	req.SenderDid = "did:key:unknown-client" // Unknown DID
	req.RecipientDid = serverDID
	req.TimestampMs = auth.NowMs()
	req.Signature = make([]byte, 64) // Invalid signature

	if err := client.SendEnvelope(req); err != nil {
		t.Fatalf("Failed to send envelope: %v", err)
	}

	// Wait for server validation
	<-serverDone

	// Validation should have failed
	if validationErr == nil {
		t.Error("Expected validation error for invalid signature")
	}
}

// TestIntegration_MultipleMessages tests multiple message exchange.
func TestIntegration_MultipleMessages(t *testing.T) {
	// Generate identities
	serverPub, serverPriv, _ := ed25519.GenerateKey(nil)
	serverDID := "did:key:server-multi"
	clientPub, clientPriv, _ := ed25519.GenerateKey(nil)
	clientDID := "did:key:client-multi"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(serverDID, serverPub)
	resolver.AddDID(clientDID, clientPub)

	serverTLS := netquic.InsecureTLSConfig()
	listener, err := netquic.Listen(netquic.DefaultListenerConfig("127.0.0.1:0", serverTLS))
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Channel to signal when client is done
	clientDone := make(chan struct{})

	// Server goroutine - handles multiple messages
	serverDone := make(chan struct{})
	messageCount := 5

	go func() {
		defer close(serverDone)

		conn, err := listener.Accept(context.Background())
		if err != nil {
			return
		}
		defer conn.Close()

		for i := 0; i < messageCount; i++ {
			env, err := conn.RecvEnvelope()
			if err != nil {
				t.Logf("Server recv error: %v", err)
				return
			}

			// Echo back
			resp := axcp.NewEnvelope(env.TraceId, 1)
			resp.SenderDid = serverDID
			resp.RecipientDid = env.SenderDid
			resp.TimestampMs = auth.NowMs()
			resp.Sequence = uint64(i + 1)

			transcript := buildEnvelopeTranscript(resp)
			resp.Signature = ed25519.Sign(serverPriv, transcript)

			if err := conn.SendEnvelope(resp); err != nil {
				t.Logf("Server send error: %v", err)
				return
			}
		}

		// Wait for client to finish
		<-clientDone
	}()

	// Client sends multiple messages
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"axcp/1"},
	}

	client, err := netquic.Dial(serverAddr, clientTLS)
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer client.Close()

	for i := 0; i < messageCount; i++ {
		traceID := uuid.NewString()
		req := axcp.NewEnvelope(traceID, 1)
		req.SenderDid = clientDID
		req.RecipientDid = serverDID
		req.TimestampMs = auth.NowMs()
		req.Sequence = uint64(i + 1)

		transcript := buildEnvelopeTranscript(req)
		req.Signature = ed25519.Sign(clientPriv, transcript)

		if err := client.SendEnvelope(req); err != nil {
			t.Fatalf("Failed to send message %d: %v", i, err)
		}

		resp, err := client.RecvEnvelope()
		if err != nil {
			t.Fatalf("Failed to receive response %d: %v", i, err)
		}

		if resp.TraceId != traceID {
			t.Errorf("Response %d TraceId mismatch", i)
		}
	}

	close(clientDone)
	<-serverDone
}

// TestIntegration_ConcurrentClients tests multiple concurrent clients.
func TestIntegration_ConcurrentClients(t *testing.T) {
	// Generate server identity
	serverPub, serverPriv, _ := ed25519.GenerateKey(nil)
	serverDID := "did:key:server-concurrent"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(serverDID, serverPub)

	serverTLS := netquic.InsecureTLSConfig()
	listener, err := netquic.Listen(netquic.ListenerConfig{
		Address:        "127.0.0.1:0",
		TLSConfig:      serverTLS,
		MaxConnections: 10,
		AcceptTimeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Server goroutine - handles multiple connections
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var serverWg sync.WaitGroup

	go func() {
		for {
			conn, err := listener.Accept(ctx)
			if err != nil {
				return
			}

			serverWg.Add(1)
			go func(c *netquic.ServerConnection) {
				defer serverWg.Done()
				defer c.Close()

				env, err := c.RecvEnvelope()
				if err != nil {
					return
				}

				resp := axcp.NewEnvelope(env.TraceId, 1)
				resp.SenderDid = serverDID
				resp.RecipientDid = env.SenderDid
				resp.TimestampMs = auth.NowMs()
				resp.Sequence = 1

				transcript := buildEnvelopeTranscript(resp)
				resp.Signature = ed25519.Sign(serverPriv, transcript)

				_ = c.SendEnvelope(resp)

				// Wait a bit for client to read before closing
				time.Sleep(100 * time.Millisecond)
			}(conn)
		}
	}()

	// Launch multiple concurrent clients
	numClients := 5
	var clientWg sync.WaitGroup
	errors := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		clientWg.Add(1)
		go func(clientID int) {
			defer clientWg.Done()

			// Generate client identity
			clientPub, clientPriv, _ := ed25519.GenerateKey(nil)
			clientDID := "did:key:client-" + uuid.NewString()[:8]
			resolver.AddDID(clientDID, clientPub)

			clientTLS := &tls.Config{
				InsecureSkipVerify: true,
				NextProtos:         []string{"axcp/1"},
			}

			client, err := netquic.Dial(serverAddr, clientTLS)
			if err != nil {
				errors <- err
				return
			}
			defer client.Close()

			// Send request
			traceID := uuid.NewString()
			req := axcp.NewEnvelope(traceID, 1)
			req.SenderDid = clientDID
			req.RecipientDid = serverDID
			req.TimestampMs = auth.NowMs()
			req.Sequence = 1

			transcript := buildEnvelopeTranscript(req)
			req.Signature = ed25519.Sign(clientPriv, transcript)

			if err := client.SendEnvelope(req); err != nil {
				errors <- err
				return
			}

			// Receive response
			resp, err := client.RecvEnvelope()
			if err != nil {
				errors <- err
				return
			}

			if resp.TraceId != traceID {
				errors <- err
				return
			}
		}(i)
	}

	clientWg.Wait()
	cancel()
	serverWg.Wait()

	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("Client error: %v", err)
		}
	}
}

// TestIntegration_ErrorEnvelope tests error envelope handling.
func TestIntegration_ErrorEnvelope(t *testing.T) {
	// Generate identities
	serverPub, serverPriv, _ := ed25519.GenerateKey(nil)
	serverDID := "did:key:server-error"
	clientPub, clientPriv, _ := ed25519.GenerateKey(nil)
	clientDID := "did:key:client-error"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(serverDID, serverPub)
	resolver.AddDID(clientDID, clientPub)

	serverTLS := netquic.InsecureTLSConfig()
	listener, err := netquic.Listen(netquic.DefaultListenerConfig("127.0.0.1:0", serverTLS))
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Channel to signal when client is done
	clientDone := make(chan struct{})

	// Server sends error response
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		conn, err := listener.Accept(context.Background())
		if err != nil {
			return
		}
		defer conn.Close()

		_, err = conn.RecvEnvelope()
		if err != nil {
			return
		}

		// Send error envelope
		errEnv := axcp.NewErrorEnvelope(
			uuid.NewString(),
			serverDID,
			clientDID,
			axcp.ErrToolNotFound.WithReason("requested tool not available"),
		)
		errEnv.TimestampMs = auth.NowMs()
		errEnv.Sequence = 1

		transcript := buildEnvelopeTranscript(errEnv)
		errEnv.Signature = ed25519.Sign(serverPriv, transcript)

		_ = conn.SendEnvelope(errEnv)

		// Wait for client to finish reading
		<-clientDone
	}()

	// Client
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"axcp/1"},
	}

	client, err := netquic.Dial(serverAddr, clientTLS)
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer client.Close()

	// Send request
	req := axcp.NewEnvelope(uuid.NewString(), 1)
	req.SenderDid = clientDID
	req.RecipientDid = serverDID
	req.TimestampMs = auth.NowMs()
	req.Sequence = 1

	transcript := buildEnvelopeTranscript(req)
	req.Signature = ed25519.Sign(clientPriv, transcript)

	if err := client.SendEnvelope(req); err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Receive error response
	resp, err := client.RecvEnvelope()
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	// Check it's an error envelope
	if !axcp.IsErrorEnvelope(resp) {
		t.Error("Expected error envelope")
	}

	// Extract error
	perr, ok := axcp.ErrorFromEnvelope(resp)
	if !ok {
		t.Fatal("Failed to extract error from envelope")
	}

	if perr.Reason != "requested tool not available" {
		t.Errorf("Error reason = %q, want %q", perr.Reason, "requested tool not available")
	}

	close(clientDone)
	<-serverDone
}

// buildEnvelopeTranscript builds the canonical transcript for signing/verifying.
func buildEnvelopeTranscript(env *axcp.Envelope) []byte {
	transcript, err := axcp.BuildAuthTranscript(env)
	if err != nil {
		panic(err)
	}
	return transcript
}
