// Package auth provides authentication session management for AXCP.
package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Session errors
var (
	// ErrSessionNotNegotiated indicates the session has not completed negotiation
	ErrSessionNotNegotiated = errors.New("auth: session not negotiated")

	// ErrNoPrivateKey indicates no private key is available for signing
	ErrNoPrivateKey = errors.New("auth: no private key available for signing")

	// ErrSessionClosed indicates the session has been closed
	ErrSessionClosed = errors.New("auth: session closed")
)

// Session manages authentication state for an AXCP connection.
// It tracks the negotiated profile, sequence numbers, and provides
// signing capabilities for outgoing messages.
//
// Thread-safe: can be used concurrently from multiple goroutines.
type Session struct {
	mu sync.RWMutex

	// Identity
	localDID  string
	remoteDID string
	privateKey ed25519.PrivateKey

	// Negotiation state
	negotiated bool
	profile    string // e.g., "secure-baseline-v1"

	// Sequence counter for replay protection (atomically incremented)
	sequence uint64

	// Clock for timestamps
	clock Clock

	// Closed flag
	closed bool
}

// SessionConfig holds configuration for creating a new session.
type SessionConfig struct {
	// LocalDID is this endpoint's DID
	LocalDID string

	// PrivateKey is the Ed25519 private key for signing (optional for verify-only sessions)
	PrivateKey ed25519.PrivateKey

	// Clock is the time source (if nil, uses system clock)
	Clock Clock
}

// NewSession creates a new authentication session.
func NewSession(config SessionConfig) *Session {
	clock := config.Clock
	if clock == nil {
		clock = RealClock{}
	}

	return &Session{
		localDID:   config.LocalDID,
		privateKey: config.PrivateKey,
		clock:      clock,
		sequence:   0,
	}
}

// SetNegotiated marks the session as having completed negotiation.
func (s *Session) SetNegotiated(profile string, remoteDID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.negotiated = true
	s.profile = profile
	s.remoteDID = remoteDID
}

// IsNegotiated returns whether the session has completed negotiation.
func (s *Session) IsNegotiated() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.negotiated
}

// Profile returns the negotiated profile.
func (s *Session) Profile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.profile
}

// LocalDID returns the local endpoint's DID.
func (s *Session) LocalDID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.localDID
}

// RemoteDID returns the remote endpoint's DID.
func (s *Session) RemoteDID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.remoteDID
}

// NextSequence returns the next sequence number and increments the counter.
func (s *Session) NextSequence() uint64 {
	return atomic.AddUint64(&s.sequence, 1)
}

// CurrentSequence returns the current sequence number without incrementing.
func (s *Session) CurrentSequence() uint64 {
	return atomic.LoadUint64(&s.sequence)
}

// SignatureInput contains the data needed to create a signed envelope.
type SignatureInput struct {
	// Payload is the serialized envelope payload (without auth fields)
	Payload []byte

	// RecipientDID overrides the remote DID for directed messages (optional)
	RecipientDID string
}

// SignatureOutput contains the auth fields to add to an envelope.
type SignatureOutput struct {
	SenderDID    string
	RecipientDID string
	TimestampMs  uint64
	Sequence     uint64
	Signature    []byte
}

// SignEnvelope creates authentication fields for an outgoing envelope.
// The caller should add these fields to the envelope before sending.
//
// Returns ErrSessionNotNegotiated if negotiation hasn't completed.
// Returns ErrNoPrivateKey if no private key is available.
func (s *Session) SignEnvelope(ctx context.Context, input SignatureInput) (*SignatureOutput, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSessionClosed
	}
	if !s.negotiated {
		s.mu.RUnlock()
		return nil, ErrSessionNotNegotiated
	}
	if s.privateKey == nil {
		s.mu.RUnlock()
		return nil, ErrNoPrivateKey
	}

	localDID := s.localDID
	remoteDID := s.remoteDID
	privateKey := s.privateKey
	s.mu.RUnlock()

	// Use override recipient if provided
	recipientDID := remoteDID
	if input.RecipientDID != "" {
		recipientDID = input.RecipientDID
	}

	// Get timestamp and sequence
	now := s.clock.Now()
	timestampMs := uint64(now.UnixMilli())
	sequence := s.NextSequence()

	// Build transcript for signing
	transcript := BuildDIDAuthTranscript(localDID, recipientDID, input.Payload, now)

	// Sign the transcript
	signature := ed25519.Sign(privateKey, transcript)

	return &SignatureOutput{
		SenderDID:    localDID,
		RecipientDID: recipientDID,
		TimestampMs:  timestampMs,
		Sequence:     sequence,
		Signature:    signature,
	}, nil
}

// Close marks the session as closed.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

// IsClosed returns whether the session has been closed.
func (s *Session) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// MemoryDIDResolver is an in-memory DID resolver for testing and examples.
// It maps DIDs to their public keys.
type MemoryDIDResolver struct {
	mu   sync.RWMutex
	docs map[string]*DIDDocument
}

// NewMemoryDIDResolver creates a new in-memory DID resolver.
func NewMemoryDIDResolver() *MemoryDIDResolver {
	return &MemoryDIDResolver{
		docs: make(map[string]*DIDDocument),
	}
}

// AddDID adds a DID and its public key to the resolver.
func (r *MemoryDIDResolver) AddDID(did string, publicKey ed25519.PublicKey) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.docs[did] = &DIDDocument{
		ID: did,
		PublicKeys: []PublicKeyRecord{
			{
				ID:             "key-1",
				Type:           "Ed25519VerificationKey2020",
				PublicKeyBytes: publicKey,
			},
		},
	}
}

// RemoveDID removes a DID from the resolver.
func (r *MemoryDIDResolver) RemoveDID(did string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.docs, did)
}

// Resolve implements DIDResolver.
func (r *MemoryDIDResolver) Resolve(ctx context.Context, did string) (*DIDDocument, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	doc, ok := r.docs[did]
	if !ok {
		return nil, ErrDIDNotFound
	}
	return doc, nil
}

// GenerateTestIdentity creates a new DID and keypair for testing.
// The DID is in the format did:key:test-<random-suffix>.
func GenerateTestIdentity() (did string, publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey, err error) {
	publicKey, privateKey, err = ed25519.GenerateKey(nil)
	if err != nil {
		return "", nil, nil, err
	}

	// Create a simple test DID using base64url of first 8 bytes of public key
	// In production, use proper did:key encoding
	did = "did:key:test-" + encodeBase64URL(publicKey[:8])
	return did, publicKey, privateKey, nil
}

// encodeBase64URL encodes bytes to base64url without padding
func encodeBase64URL(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, 0, (len(data)*8+5)/6)

	var bits uint32
	var numBits uint

	for _, b := range data {
		bits = (bits << 8) | uint32(b)
		numBits += 8
		for numBits >= 6 {
			numBits -= 6
			result = append(result, alphabet[(bits>>numBits)&0x3f])
		}
	}

	if numBits > 0 {
		result = append(result, alphabet[(bits<<(6-numBits))&0x3f])
	}

	return string(result)
}

// TimestampNow returns the current timestamp in milliseconds.
func TimestampNow() uint64 {
	return uint64(time.Now().UnixMilli())
}
