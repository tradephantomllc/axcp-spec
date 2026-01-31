// Package negotiate provides profile negotiation and handshake for AXCP protocol.
package negotiate

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

// Handshake errors
var (
	// ErrHandshakeTimeout indicates the handshake did not complete in time
	ErrHandshakeTimeout = errors.New("negotiate: handshake timeout")

	// ErrInvalidClientHello indicates the client hello was malformed
	ErrInvalidClientHello = errors.New("negotiate: invalid client hello")

	// ErrInvalidServerHello indicates the server hello was malformed
	ErrInvalidServerHello = errors.New("negotiate: invalid server hello")

	// ErrSignatureVerificationFailed indicates signature verification failed
	ErrSignatureVerificationFailed = errors.New("negotiate: signature verification failed")

	// ErrDIDResolutionFailed indicates the peer's DID could not be resolved
	ErrDIDResolutionFailed = errors.New("negotiate: DID resolution failed")

	// ErrHandshakeRejected indicates the peer rejected the handshake
	ErrHandshakeRejected = errors.New("negotiate: handshake rejected by peer")
)

// StreamIO defines the interface for reading/writing handshake messages.
type StreamIO interface {
	// SendMessage sends raw bytes with length prefix
	SendMessage(data []byte) error

	// ReceiveMessage receives raw bytes with length prefix
	ReceiveMessage() ([]byte, error)
}

// Handshaker performs AXCP profile handshake.
type Handshaker struct {
	// LocalDID is this node's DID
	LocalDID string

	// PrivateKey is the Ed25519 private key for signing
	PrivateKey ed25519.PrivateKey

	// PublicKey is the Ed25519 public key
	PublicKey ed25519.PublicKey

	// Resolver resolves DIDs to public keys
	Resolver auth.DIDResolver

	// Profiles lists supported profiles (ordered by preference)
	Profiles []Profile

	// Capabilities specifies this node's capabilities
	Capabilities Capabilities

	// Timeout is the maximum time for the handshake
	Timeout time.Duration

	// NegotiateOptions configures negotiation behavior
	NegotiateOptions NegotiateOptions
}

// HandshakerConfig is the configuration for creating a Handshaker.
type HandshakerConfig struct {
	LocalDID         string
	PrivateKey       ed25519.PrivateKey
	PublicKey        ed25519.PublicKey
	Resolver         auth.DIDResolver
	Profiles         []Profile
	Capabilities     Capabilities
	Timeout          time.Duration
	NegotiateOptions NegotiateOptions
}

// DefaultHandshakerConfig returns a config with production defaults.
func DefaultHandshakerConfig(localDID string, privateKey ed25519.PrivateKey, resolver auth.DIDResolver) HandshakerConfig {
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return HandshakerConfig{
		LocalDID:         localDID,
		PrivateKey:       privateKey,
		PublicKey:        publicKey,
		Resolver:         resolver,
		Profiles:         []Profile{ProfileSecureBaseline},
		Capabilities:     DefaultCapabilities(),
		Timeout:          30 * time.Second,
		NegotiateOptions: DefaultNegotiateOptions(),
	}
}

// NewHandshaker creates a new Handshaker with the given configuration.
func NewHandshaker(config HandshakerConfig) *Handshaker {
	return &Handshaker{
		LocalDID:         config.LocalDID,
		PrivateKey:       config.PrivateKey,
		PublicKey:        config.PublicKey,
		Resolver:         config.Resolver,
		Profiles:         config.Profiles,
		Capabilities:     config.Capabilities,
		Timeout:          config.Timeout,
		NegotiateOptions: config.NegotiateOptions,
	}
}

// HandshakeResult contains the result of a successful handshake.
type HandshakeResult struct {
	// RemoteDID is the authenticated peer's DID
	RemoteDID string

	// SelectedProfile is the negotiated security profile
	SelectedProfile Profile

	// Capabilities contains the resolved capabilities
	Capabilities Capabilities

	// RemotePublicKey is the peer's verified public key
	RemotePublicKey ed25519.PublicKey
}

// ClientHandshake performs the client-side handshake.
//
// Steps:
//  1. Build and sign ClientHello
//  2. Send ClientHello
//  3. Receive ServerHello
//  4. Verify ServerHello signature
//  5. Return HandshakeResult
func (h *Handshaker) ClientHandshake(ctx context.Context, stream StreamIO) (*HandshakeResult, error) {
	// Apply timeout
	if h.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}

	// Build ClientHello
	hello := &WireClientHello{
		Version:      1,
		SenderDID:    h.LocalDID,
		Profiles:     h.Profiles,
		Capabilities: h.Capabilities,
		TimestampMs:  auth.NowMs(),
	}

	// Sign the hello
	if err := hello.Sign(h.PrivateKey); err != nil {
		return nil, fmt.Errorf("negotiate: failed to sign client hello: %w", err)
	}

	// Marshal and send
	data, err := hello.Marshal()
	if err != nil {
		return nil, fmt.Errorf("negotiate: failed to marshal client hello: %w", err)
	}

	// Send with timeout check
	sendDone := make(chan error, 1)
	go func() {
		sendDone <- stream.SendMessage(data)
	}()

	select {
	case <-ctx.Done():
		return nil, ErrHandshakeTimeout
	case err := <-sendDone:
		if err != nil {
			return nil, fmt.Errorf("negotiate: failed to send client hello: %w", err)
		}
	}

	// Receive ServerHello
	recvDone := make(chan struct {
		data []byte
		err  error
	}, 1)
	go func() {
		data, err := stream.ReceiveMessage()
		recvDone <- struct {
			data []byte
			err  error
		}{data, err}
	}()

	var serverHelloData []byte
	select {
	case <-ctx.Done():
		return nil, ErrHandshakeTimeout
	case result := <-recvDone:
		if result.err != nil {
			return nil, fmt.Errorf("negotiate: failed to receive server hello: %w", result.err)
		}
		serverHelloData = result.data
	}

	// Unmarshal ServerHello
	serverHello, err := UnmarshalServerHello(serverHelloData)
	if err != nil {
		return nil, fmt.Errorf("negotiate: %w: %v", ErrInvalidServerHello, err)
	}

	// Resolve server's public key
	publicKey, err := auth.ResolveEd25519PublicKey(ctx, h.Resolver, serverHello.SenderDID)
	if err != nil {
		return nil, fmt.Errorf("negotiate: %w: %v", ErrDIDResolutionFailed, err)
	}

	// Verify signature
	if err := serverHello.Verify(publicKey); err != nil {
		return nil, ErrSignatureVerificationFailed
	}

	// Validate selected profile
	if err := ValidateProfile(serverHello.SelectedProfile); err != nil {
		return nil, fmt.Errorf("negotiate: server selected invalid profile: %w", err)
	}

	return &HandshakeResult{
		RemoteDID:       serverHello.SenderDID,
		SelectedProfile: serverHello.SelectedProfile,
		Capabilities:    serverHello.Capabilities,
		RemotePublicKey: publicKey,
	}, nil
}

// ServerHandshake performs the server-side handshake.
//
// Steps:
//  1. Receive ClientHello
//  2. Verify ClientHello signature
//  3. Run profile negotiation
//  4. Build and sign ServerHello
//  5. Send ServerHello
//  6. Return HandshakeResult
func (h *Handshaker) ServerHandshake(ctx context.Context, stream StreamIO) (*HandshakeResult, error) {
	// Apply timeout
	if h.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}

	// Receive ClientHello
	recvDone := make(chan struct {
		data []byte
		err  error
	}, 1)
	go func() {
		data, err := stream.ReceiveMessage()
		recvDone <- struct {
			data []byte
			err  error
		}{data, err}
	}()

	var clientHelloData []byte
	select {
	case <-ctx.Done():
		return nil, ErrHandshakeTimeout
	case result := <-recvDone:
		if result.err != nil {
			return nil, fmt.Errorf("negotiate: failed to receive client hello: %w", result.err)
		}
		clientHelloData = result.data
	}

	// Unmarshal ClientHello
	clientHello, err := UnmarshalClientHello(clientHelloData)
	if err != nil {
		return nil, fmt.Errorf("negotiate: %w: %v", ErrInvalidClientHello, err)
	}

	// Validate timestamp
	if err := auth.ValidateTimestamp(clientHello.TimestampMs); err != nil {
		return nil, fmt.Errorf("negotiate: client hello timestamp invalid: %w", err)
	}

	// Resolve client's public key
	publicKey, err := auth.ResolveEd25519PublicKey(ctx, h.Resolver, clientHello.SenderDID)
	if err != nil {
		return nil, fmt.Errorf("negotiate: %w: %v", ErrDIDResolutionFailed, err)
	}

	// Verify signature
	if err := clientHello.Verify(publicKey); err != nil {
		return nil, ErrSignatureVerificationFailed
	}

	// Run negotiation
	clientNegHello := ClientHello{
		Version:  fmt.Sprintf("%d", clientHello.Version),
		Profiles: clientHello.Profiles,
		Cap:      clientHello.Capabilities,
	}

	serverResult, err := Negotiate(clientNegHello, h.Profiles, h.Capabilities, h.NegotiateOptions)
	if err != nil {
		return nil, fmt.Errorf("negotiate: negotiation failed: %w", err)
	}

	// Build ServerHello
	serverHello := &WireServerHello{
		Version:         1,
		SenderDID:       h.LocalDID,
		SelectedProfile: serverResult.Profile,
		Capabilities:    serverResult.Cap,
		TimestampMs:     auth.NowMs(),
	}

	// Sign the hello
	if err := serverHello.Sign(h.PrivateKey); err != nil {
		return nil, fmt.Errorf("negotiate: failed to sign server hello: %w", err)
	}

	// Marshal and send
	data, err := serverHello.Marshal()
	if err != nil {
		return nil, fmt.Errorf("negotiate: failed to marshal server hello: %w", err)
	}

	sendDone := make(chan error, 1)
	go func() {
		sendDone <- stream.SendMessage(data)
	}()

	select {
	case <-ctx.Done():
		return nil, ErrHandshakeTimeout
	case err := <-sendDone:
		if err != nil {
			return nil, fmt.Errorf("negotiate: failed to send server hello: %w", err)
		}
	}

	return &HandshakeResult{
		RemoteDID:       clientHello.SenderDID,
		SelectedProfile: serverResult.Profile,
		Capabilities:    serverResult.Cap,
		RemotePublicKey: publicKey,
	}, nil
}
