package netquic

import (
	"context"
	"testing"
	"time"
)

func TestDefaultListenerConfig(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)

	if cfg.Address != "127.0.0.1:0" {
		t.Errorf("expected address 127.0.0.1:0, got %s", cfg.Address)
	}
	if cfg.MaxIdleTimeout != 5*time.Minute {
		t.Errorf("expected MaxIdleTimeout 5m, got %v", cfg.MaxIdleTimeout)
	}
	if cfg.KeepAlivePeriod != 30*time.Second {
		t.Errorf("expected KeepAlivePeriod 30s, got %v", cfg.KeepAlivePeriod)
	}
	if cfg.MaxConnections != 0 {
		t.Errorf("expected MaxConnections 0 (unlimited), got %d", cfg.MaxConnections)
	}
	if cfg.AcceptTimeout != 10*time.Second {
		t.Errorf("expected AcceptTimeout 10s, got %v", cfg.AcceptTimeout)
	}
}

func TestListen_Success(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)

	l, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	// Verify address is assigned
	addr := l.Addr()
	if addr == nil {
		t.Error("expected non-nil address")
	}

	// Verify connection count starts at 0
	if count := l.ConnectionCount(); count != 0 {
		t.Errorf("expected 0 connections, got %d", count)
	}
}

func TestListen_MissingAddress(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := ListenerConfig{
		TLSConfig: tlsConf,
	}

	_, err := Listen(cfg)
	if err == nil {
		t.Error("expected error for missing address")
	}
}

func TestListen_MissingTLSConfig(t *testing.T) {
	cfg := ListenerConfig{
		Address: "127.0.0.1:0",
	}

	_, err := Listen(cfg)
	if err == nil {
		t.Error("expected error for missing TLS config")
	}
}

func TestListener_Close(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)

	l, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	// Close should succeed
	if err := l.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Second close should be no-op
	if err := l.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}

	// Accept should fail after close
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = l.Accept(ctx)
	if err != ErrListenerClosed {
		t.Errorf("expected ErrListenerClosed, got %v", err)
	}
}

func TestListener_AcceptAndConnect(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)

	l, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	addr := l.Addr().String()
	t.Logf("Listening on %s", addr)

	// Start accepting in background
	acceptDone := make(chan *ServerConnection)
	acceptErr := make(chan error)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := l.Accept(ctx)
		if err != nil {
			acceptErr <- err
			return
		}
		acceptDone <- conn
	}()

	// Small delay to ensure listener is ready
	time.Sleep(50 * time.Millisecond)

	// Connect with client
	t.Logf("Dialing %s", addr)
	client, err := Dial(addr, tlsConf)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()
	t.Logf("Client connected")

	// Send initial data to announce the stream to the server
	// (QUIC streams are announced when data is sent)
	if err := client.SendMessage([]byte("hello")); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	t.Logf("Client sent initial message")

	// Wait for accept
	select {
	case conn := <-acceptDone:
		if conn == nil {
			t.Error("expected non-nil connection")
		}
		defer conn.Close()

		// Verify connection count
		if count := l.ConnectionCount(); count != 1 {
			t.Errorf("expected 1 connection, got %d", count)
		}

		// Verify remote address is set
		if conn.RemoteAddr() == nil {
			t.Error("expected non-nil remote address")
		}

	case err := <-acceptErr:
		t.Fatalf("Accept failed: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("Accept timeout")
	}
}

func TestListener_MaxConnections(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)
	cfg.MaxConnections = 1

	l, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	addr := l.Addr().String()

	// Connect first client and send data to announce stream
	client1, err := Dial(addr, tlsConf)
	if err != nil {
		t.Fatalf("first Dial failed: %v", err)
	}
	defer client1.Close()
	_ = client1.SendMessage([]byte("hello"))

	// Accept first connection
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	conn1, err := l.Accept(ctx1)
	if err != nil {
		t.Fatalf("first Accept failed: %v", err)
	}
	defer conn1.Close()

	// Try to accept second connection (should fail due to max connections)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	_, err = l.Accept(ctx2)
	if err != ErrMaxConnectionsReached {
		t.Errorf("expected ErrMaxConnectionsReached, got %v", err)
	}
}

func TestListener_Shutdown(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)

	l, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	addr := l.Addr().String()

	// Connect a client and send data to announce stream
	client, err := Dial(addr, tlsConf)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()
	_ = client.SendMessage([]byte("hello"))

	// Accept the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := l.Accept(ctx)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	// Close the connection before shutdown
	conn.Close()
	client.Close()

	// Give some time for connection tracking to update
	time.Sleep(100 * time.Millisecond)

	// Shutdown should succeed
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := l.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestServerConnection_SendRecvEnvelope(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)

	l, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	addr := l.Addr().String()

	// Start accepting
	connChan := make(chan *ServerConnection)
	errChan := make(chan error)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := l.Accept(ctx)
		if err != nil {
			errChan <- err
			return
		}
		connChan <- conn
	}()

	// Connect client and send data to announce stream
	client, err := Dial(addr, tlsConf)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	// Client sends message (this announces the stream)
	testData := []byte("hello from client")
	if err := client.SendMessage(testData); err != nil {
		t.Fatalf("client SendMessage failed: %v", err)
	}

	// Get server connection
	var serverConn *ServerConnection
	select {
	case serverConn = <-connChan:
		// continue
	case err := <-errChan:
		t.Fatalf("Accept failed: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("Accept timeout")
	}
	defer serverConn.Close()

	// Server receives message
	received, err := serverConn.ReceiveMessage()
	if err != nil {
		t.Fatalf("server ReceiveMessage failed: %v", err)
	}
	if string(received) != string(testData) {
		t.Errorf("expected %q, got %q", testData, received)
	}

	// Server sends reply
	replyData := []byte("hello from server")
	if err := serverConn.SendMessage(replyData); err != nil {
		t.Fatalf("server SendMessage failed: %v", err)
	}

	// Client receives reply
	reply, err := client.ReceiveMessage()
	if err != nil {
		t.Fatalf("client ReceiveMessage failed: %v", err)
	}
	if string(reply) != string(replyData) {
		t.Errorf("expected %q, got %q", replyData, reply)
	}
}

func TestServerConnection_SetAuthenticated(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)

	l, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	addr := l.Addr().String()

	// Start accepting
	connChan := make(chan *ServerConnection)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, _ := l.Accept(ctx)
		connChan <- conn
	}()

	// Connect client and send data to announce stream
	client, err := Dial(addr, tlsConf)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()
	_ = client.SendMessage([]byte("hello"))

	// Get server connection
	serverConn := <-connChan
	if serverConn == nil {
		t.Fatal("failed to accept connection")
	}
	defer serverConn.Close()
	// Drain the initial message
	_, _ = serverConn.ReceiveMessage()

	// Initially not authenticated
	if serverConn.IsNegotiated() {
		t.Error("expected not negotiated initially")
	}
	if serverConn.RemoteDID() != "" {
		t.Error("expected empty remote DID initially")
	}

	// Set authenticated
	serverConn.SetAuthenticated("did:key:local", "did:key:remote")

	// Verify state
	if !serverConn.IsNegotiated() {
		t.Error("expected negotiated after SetAuthenticated")
	}
	if serverConn.LocalDID() != "did:key:local" {
		t.Errorf("expected local DID did:key:local, got %s", serverConn.LocalDID())
	}
	if serverConn.RemoteDID() != "did:key:remote" {
		t.Errorf("expected remote DID did:key:remote, got %s", serverConn.RemoteDID())
	}
}

func TestServerConnection_CloseWithError(t *testing.T) {
	tlsConf := InsecureTLSConfig()
	cfg := DefaultListenerConfig("127.0.0.1:0", tlsConf)

	l, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	addr := l.Addr().String()

	// Start accepting
	connChan := make(chan *ServerConnection)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, _ := l.Accept(ctx)
		connChan <- conn
	}()

	// Connect client and send data to announce stream
	client, err := Dial(addr, tlsConf)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()
	_ = client.SendMessage([]byte("hello"))

	// Get server connection
	serverConn := <-connChan
	if serverConn == nil {
		t.Fatal("failed to accept connection")
	}

	// Close with error
	if err := serverConn.CloseWithError(1, "test error"); err != nil {
		t.Errorf("CloseWithError failed: %v", err)
	}

	// Further operations should fail
	_, err = serverConn.ReceiveMessage()
	if err != ErrNotConnected {
		t.Errorf("expected ErrNotConnected after close, got %v", err)
	}
}
