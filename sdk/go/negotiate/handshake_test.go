package negotiate

import (
	"context"
	"crypto/ed25519"
	"sync"
	"testing"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

// mockStream implements StreamIO for testing using channels for synchronization
type mockStream struct {
	sendChan chan []byte
	recvChan chan []byte
}

func newMockStreamPair() (*mockStream, *mockStream) {
	// Create bidirectional channels
	ch1 := make(chan []byte, 10)
	ch2 := make(chan []byte, 10)

	// Stream 1 sends to ch1, receives from ch2
	// Stream 2 sends to ch2, receives from ch1
	s1 := &mockStream{sendChan: ch1, recvChan: ch2}
	s2 := &mockStream{sendChan: ch2, recvChan: ch1}

	return s1, s2
}

func (s *mockStream) SendMessage(data []byte) error {
	// Make a copy to avoid data races
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	s.sendChan <- dataCopy
	return nil
}

func (s *mockStream) ReceiveMessage() ([]byte, error) {
	data := <-s.recvChan
	return data, nil
}

func TestWireClientHello_MarshalUnmarshal(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)

	hello := &WireClientHello{
		Version:   1,
		SenderDID: "did:key:client123",
		Profiles:  []Profile{ProfileSecureBaseline},
		Capabilities: Capabilities{
			RequireAuth:   true,
			RequireReplay: true,
			SigAlgos:      []string{SigAlgoEd25519},
		},
		TimestampMs: auth.NowMs(),
	}

	// Sign
	if err := hello.Sign(priv); err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// Marshal
	data, err := hello.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	restored, err := UnmarshalClientHello(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify fields
	if restored.Version != hello.Version {
		t.Errorf("Version mismatch: got %d, want %d", restored.Version, hello.Version)
	}
	if restored.SenderDID != hello.SenderDID {
		t.Errorf("SenderDID mismatch: got %s, want %s", restored.SenderDID, hello.SenderDID)
	}
	if len(restored.Profiles) != len(hello.Profiles) {
		t.Errorf("Profiles length mismatch: got %d, want %d", len(restored.Profiles), len(hello.Profiles))
	}
	if restored.TimestampMs != hello.TimestampMs {
		t.Errorf("TimestampMs mismatch: got %d, want %d", restored.TimestampMs, hello.TimestampMs)
	}
}

func TestWireServerHello_MarshalUnmarshal(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)

	hello := &WireServerHello{
		Version:         1,
		SenderDID:       "did:key:server456",
		SelectedProfile: ProfileSecureBaseline,
		Capabilities: Capabilities{
			RequireAuth:   true,
			RequireReplay: true,
			SigAlgos:      []string{SigAlgoEd25519},
		},
		TimestampMs: auth.NowMs(),
	}

	// Sign
	if err := hello.Sign(priv); err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// Marshal
	data, err := hello.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	restored, err := UnmarshalServerHello(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify fields
	if restored.Version != hello.Version {
		t.Errorf("Version mismatch")
	}
	if restored.SenderDID != hello.SenderDID {
		t.Errorf("SenderDID mismatch")
	}
	if restored.SelectedProfile != hello.SelectedProfile {
		t.Errorf("SelectedProfile mismatch")
	}
	if restored.TimestampMs != hello.TimestampMs {
		t.Errorf("TimestampMs mismatch")
	}
}

func TestWireClientHello_SignVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)

	hello := &WireClientHello{
		Version:      1,
		SenderDID:    "did:key:test",
		Profiles:     []Profile{ProfileSecureBaseline},
		Capabilities: DefaultCapabilities(),
		TimestampMs:  auth.NowMs(),
	}

	// Sign
	if err := hello.Sign(priv); err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// Verify with correct key
	if err := hello.Verify(pub); err != nil {
		t.Errorf("Verify failed with correct key: %v", err)
	}

	// Verify with wrong key
	wrongPub, _, _ := ed25519.GenerateKey(nil)
	if err := hello.Verify(wrongPub); err == nil {
		t.Error("Verify should fail with wrong key")
	}
}

func TestWireServerHello_SignVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)

	hello := &WireServerHello{
		Version:         1,
		SenderDID:       "did:key:test",
		SelectedProfile: ProfileSecureBaseline,
		Capabilities:    DefaultCapabilities(),
		TimestampMs:     auth.NowMs(),
	}

	// Sign
	if err := hello.Sign(priv); err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// Verify with correct key
	if err := hello.Verify(pub); err != nil {
		t.Errorf("Verify failed with correct key: %v", err)
	}

	// Verify with wrong key
	wrongPub, _, _ := ed25519.GenerateKey(nil)
	if err := hello.Verify(wrongPub); err == nil {
		t.Error("Verify should fail with wrong key")
	}
}

func TestUnmarshalClientHello_InvalidMagic(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01}
	_, err := UnmarshalClientHello(data)
	if err != ErrInvalidMagic {
		t.Errorf("expected ErrInvalidMagic, got %v", err)
	}
}

func TestUnmarshalServerHello_InvalidMagic(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01}
	_, err := UnmarshalServerHello(data)
	if err != ErrInvalidMagic {
		t.Errorf("expected ErrInvalidMagic, got %v", err)
	}
}

func TestUnmarshalClientHello_TooShort(t *testing.T) {
	data := []byte{0x41, 0x58, 0x43, 0x48} // just magic
	_, err := UnmarshalClientHello(data)
	if err != ErrDataTooShort {
		t.Errorf("expected ErrDataTooShort, got %v", err)
	}
}

func TestDefaultHandshakerConfig(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	resolver := auth.NewMemoryDIDResolver()

	config := DefaultHandshakerConfig("did:key:test", priv, resolver)

	if config.LocalDID != "did:key:test" {
		t.Errorf("unexpected LocalDID: %s", config.LocalDID)
	}
	if len(config.Profiles) != 1 || config.Profiles[0] != ProfileSecureBaseline {
		t.Error("unexpected profiles")
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("unexpected timeout: %v", config.Timeout)
	}
}

func TestHandshaker_FullHandshake(t *testing.T) {
	// Setup client identity
	clientPub, clientPriv, _ := ed25519.GenerateKey(nil)
	clientDID := "did:key:client"

	// Setup server identity
	serverPub, serverPriv, _ := ed25519.GenerateKey(nil)
	serverDID := "did:key:server"

	// Create resolver with both DIDs
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(clientDID, clientPub)
	resolver.AddDID(serverDID, serverPub)

	// Create handshakers
	clientConfig := DefaultHandshakerConfig(clientDID, clientPriv, resolver)
	clientHandshaker := NewHandshaker(clientConfig)

	serverConfig := DefaultHandshakerConfig(serverDID, serverPriv, resolver)
	serverHandshaker := NewHandshaker(serverConfig)

	// Create connected mock streams
	clientStream, serverStream := newMockStreamPair()

	// Run handshake in parallel
	ctx := context.Background()
	var clientResult, serverResult *HandshakeResult
	var clientErr, serverErr error
	var wg sync.WaitGroup

	wg.Add(2)

	// Client handshake
	go func() {
		defer wg.Done()
		// Small delay to ensure server is ready
		time.Sleep(10 * time.Millisecond)
		clientResult, clientErr = clientHandshaker.ClientHandshake(ctx, clientStream)
	}()

	// Server handshake
	go func() {
		defer wg.Done()
		serverResult, serverErr = serverHandshaker.ServerHandshake(ctx, serverStream)
	}()

	wg.Wait()

	// Check results
	if clientErr != nil {
		t.Fatalf("client handshake failed: %v", clientErr)
	}
	if serverErr != nil {
		t.Fatalf("server handshake failed: %v", serverErr)
	}

	// Verify client result
	if clientResult.RemoteDID != serverDID {
		t.Errorf("client got wrong remote DID: %s", clientResult.RemoteDID)
	}
	if clientResult.SelectedProfile != ProfileSecureBaseline {
		t.Errorf("client got wrong profile: %s", clientResult.SelectedProfile)
	}

	// Verify server result
	if serverResult.RemoteDID != clientDID {
		t.Errorf("server got wrong remote DID: %s", serverResult.RemoteDID)
	}
	if serverResult.SelectedProfile != ProfileSecureBaseline {
		t.Errorf("server got wrong profile: %s", serverResult.SelectedProfile)
	}
}

func TestHandshaker_ProfileMismatch(t *testing.T) {
	// Setup client with only deprecated profile
	clientPub, clientPriv, _ := ed25519.GenerateKey(nil)
	clientDID := "did:key:client"

	// Setup server with only secure profile
	serverPub, serverPriv, _ := ed25519.GenerateKey(nil)
	serverDID := "did:key:server"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(clientDID, clientPub)
	resolver.AddDID(serverDID, serverPub)

	// Server only supports secure profile (and doesn't allow deprecated)
	serverConfig := DefaultHandshakerConfig(serverDID, serverPriv, resolver)
	serverConfig.Timeout = 5 * time.Second
	serverHandshaker := NewHandshaker(serverConfig)

	// Create a ClientHello with only deprecated profile
	clientHello := &WireClientHello{
		Version:      1,
		SenderDID:    clientDID,
		Profiles:     []Profile{ProfileTransportOnly},
		Capabilities: DefaultCapabilities(),
		TimestampMs:  auth.NowMs(),
	}
	_ = clientHello.Sign(clientPriv)

	// Create a mock stream that provides the client hello
	clientHelloData, _ := clientHello.Marshal()
	serverStream := &mockStream{
		sendChan: make(chan []byte, 10),
		recvChan: make(chan []byte, 10),
	}
	serverStream.recvChan <- clientHelloData

	ctx := context.Background()
	_, serverErr := serverHandshaker.ServerHandshake(ctx, serverStream)

	// Server should reject due to profile mismatch
	if serverErr == nil {
		t.Error("expected server handshake to fail due to profile mismatch")
	}
}

// blockingStream implements StreamIO but blocks forever on receive
type blockingStream struct {
	sendChan chan []byte
}

func (s *blockingStream) SendMessage(data []byte) error {
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	s.sendChan <- dataCopy
	return nil
}

func (s *blockingStream) ReceiveMessage() ([]byte, error) {
	// Block forever
	select {}
}

func TestHandshaker_Timeout(t *testing.T) {
	clientPub, clientPriv, _ := ed25519.GenerateKey(nil)
	clientDID := "did:key:client"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(clientDID, clientPub)

	clientConfig := DefaultHandshakerConfig(clientDID, clientPriv, resolver)
	clientConfig.Timeout = 100 * time.Millisecond
	clientHandshaker := NewHandshaker(clientConfig)

	// Create a stream that never responds
	clientStream := &blockingStream{
		sendChan: make(chan []byte, 10),
	}

	ctx := context.Background()
	_, err := clientHandshaker.ClientHandshake(ctx, clientStream)

	if err != ErrHandshakeTimeout {
		t.Errorf("expected ErrHandshakeTimeout, got %v", err)
	}
}
