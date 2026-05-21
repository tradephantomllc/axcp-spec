package internal

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/negotiate"
)

// mockClock provides a controllable time source for testing
type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

func (m *mockClock) Advance(d time.Duration) {
	m.now = m.now.Add(d)
}

// mockResolver provides a test DID resolver
type mockResolver struct {
	docs map[string]*auth.DIDDocument
	err  error
}

func newMockResolver() *mockResolver {
	return &mockResolver{
		docs: make(map[string]*auth.DIDDocument),
	}
}

func (m *mockResolver) AddKey(did string, pubKey ed25519.PublicKey) {
	m.docs[did] = &auth.DIDDocument{
		ID: did,
		PublicKeys: []auth.PublicKeyRecord{
			{
				ID:             "key-1",
				Type:           "Ed25519VerificationKey2020",
				PublicKeyBytes: pubKey,
			},
		},
	}
}

func (m *mockResolver) Resolve(ctx context.Context, did string) (*auth.DIDDocument, error) {
	if m.err != nil {
		return nil, m.err
	}
	doc, ok := m.docs[did]
	if !ok {
		return nil, auth.ErrDIDNotFound
	}
	return doc, nil
}

func TestEnvelopeAuthenticator_TransportOnlyProfile(t *testing.T) {
	clock := &mockClock{now: time.Now()}
	config := DefaultAuthConfig()

	ea, err := NewEnvelopeAuthenticator(nil, "did:key:server", config, clock)
	if err != nil {
		t.Fatalf("NewEnvelopeAuthenticator failed: %v", err)
	}

	// Transport-only profile should pass without auth
	result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileTransportOnly, EnvelopeAuth{})
	if !result.Authenticated {
		t.Errorf("Expected authenticated=true for transport-only profile, got error: %v", result.Error)
	}
}

func TestEnvelopeAuthenticator_SecureBaseline_MissingFields(t *testing.T) {
	clock := &mockClock{now: time.Now()}
	config := DefaultAuthConfig()
	resolver := newMockResolver()

	ea, err := NewEnvelopeAuthenticator(resolver, "did:key:server", config, clock)
	if err != nil {
		t.Fatalf("NewEnvelopeAuthenticator failed: %v", err)
	}

	tests := []struct {
		name      string
		auth      EnvelopeAuth
		wantCode  uint32
		wantError error
	}{
		{
			name:      "missing sender DID",
			auth:      EnvelopeAuth{},
			wantCode:  ErrorCodeAuthDIDInvalid,
			wantError: ErrMissingSenderDID,
		},
		{
			name: "missing signature",
			auth: EnvelopeAuth{
				SenderDID: "did:key:sender123",
			},
			wantCode:  ErrorCodeAuthSignatureInvalid,
			wantError: ErrMissingSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, tt.auth)
			if result.Authenticated {
				t.Error("Expected authentication to fail")
			}
			if result.ErrorCode != tt.wantCode {
				t.Errorf("ErrorCode = %d, want %d", result.ErrorCode, tt.wantCode)
			}
			if !errors.Is(result.Error, tt.wantError) {
				t.Errorf("Error = %v, want %v", result.Error, tt.wantError)
			}
		})
	}
}

func TestEnvelopeAuthenticator_TimestampValidation(t *testing.T) {
	now := time.Now()
	clock := &mockClock{now: now}
	config := DefaultAuthConfig()
	resolver := newMockResolver()

	ea, err := NewEnvelopeAuthenticator(resolver, "did:key:server", config, clock)
	if err != nil {
		t.Fatalf("NewEnvelopeAuthenticator failed: %v", err)
	}

	tests := []struct {
		name        string
		timestampMs uint64
		wantError   error
	}{
		{
			name:        "timestamp too old",
			timestampMs: uint64(now.Add(-10 * time.Minute).UnixMilli()),
			wantError:   ErrTimestampTooOld,
		},
		{
			name:        "timestamp in future",
			timestampMs: uint64(now.Add(5 * time.Minute).UnixMilli()),
			wantError:   ErrTimestampInFuture,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, EnvelopeAuth{
				SenderDID:   "did:key:sender123",
				TimestampMs: tt.timestampMs,
				Signature:   []byte("fake-signature"),
			})
			if result.Authenticated {
				t.Error("Expected authentication to fail")
			}
			if result.ErrorCode != ErrorCodeAuthTimestampExpired {
				t.Errorf("ErrorCode = %d, want %d", result.ErrorCode, ErrorCodeAuthTimestampExpired)
			}
			if !errors.Is(result.Error, tt.wantError) {
				t.Errorf("Error = %v, want %v", result.Error, tt.wantError)
			}
		})
	}
}

func TestEnvelopeAuthenticator_ReplayProtection(t *testing.T) {
	now := time.Now()
	clock := &mockClock{now: now}
	config := DefaultAuthConfig()

	// Create a real key pair for signing
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	resolver := newMockResolver()
	senderDID := "did:key:sender123"
	serverDID := "did:key:server456"
	resolver.AddKey(senderDID, pub)

	ea, err := NewEnvelopeAuthenticator(resolver, serverDID, config, clock)
	if err != nil {
		t.Fatalf("NewEnvelopeAuthenticator failed: %v", err)
	}

	// Create valid envelope auth
	timestampMs := uint64(now.UnixMilli())
	payload := []byte("test payload")
	timestamp := time.UnixMilli(int64(timestampMs))
	transcript := auth.BuildDIDAuthTranscript(senderDID, serverDID, payload, timestamp)
	signature := ed25519.Sign(priv, transcript)

	envAuth := EnvelopeAuth{
		SenderDID:    senderDID,
		RecipientDID: serverDID,
		TimestampMs:  timestampMs,
		Sequence:     1,
		Signature:    signature,
		PayloadBytes: payload,
	}

	// First request should succeed
	result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, envAuth)
	if !result.Authenticated {
		t.Errorf("First request should succeed, got error: %v", result.Error)
	}

	// Replay with same sequence should fail
	result = ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, envAuth)
	if result.Authenticated {
		t.Error("Replay should be detected")
	}
	if result.ErrorCode != ErrorCodeAuthReplayDetected {
		t.Errorf("ErrorCode = %d, want %d", result.ErrorCode, ErrorCodeAuthReplayDetected)
	}

	// New sequence should succeed
	envAuth.Sequence = 2
	// Need to re-sign with new transcript (sequence doesn't affect transcript, but this tests the flow)
	result = ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, envAuth)
	if !result.Authenticated {
		t.Errorf("New sequence should succeed, got error: %v", result.Error)
	}
}

func TestEnvelopeAuthenticator_FullAuthFlow(t *testing.T) {
	now := time.Now()
	clock := &mockClock{now: now}
	config := DefaultAuthConfig()

	// Generate key pair
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	resolver := newMockResolver()
	senderDID := "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
	serverDID := "did:key:z6MkiTBz1ymuepAQ4HEHYSF1H8quG5GLVVQR3djdX3mDooWp"
	resolver.AddKey(senderDID, pub)

	ea, err := NewEnvelopeAuthenticator(resolver, serverDID, config, clock)
	if err != nil {
		t.Fatalf("NewEnvelopeAuthenticator failed: %v", err)
	}

	// Build valid envelope
	timestampMs := uint64(now.UnixMilli())
	payload := []byte(`{"type":"context_patch","data":"test"}`)
	timestamp := time.UnixMilli(int64(timestampMs))
	transcript := auth.BuildDIDAuthTranscript(senderDID, serverDID, payload, timestamp)
	signature := ed25519.Sign(priv, transcript)

	result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, EnvelopeAuth{
		SenderDID:    senderDID,
		RecipientDID: serverDID,
		TimestampMs:  timestampMs,
		Sequence:     100,
		Signature:    signature,
		PayloadBytes: payload,
	})

	if !result.Authenticated {
		t.Errorf("Expected authentication to succeed, got error: %v", result.Error)
	}
	if result.SenderDID != senderDID {
		t.Errorf("SenderDID = %q, want %q", result.SenderDID, senderDID)
	}
}

func TestEnvelopeAuthenticator_InvalidSignature(t *testing.T) {
	now := time.Now()
	clock := &mockClock{now: now}
	config := DefaultAuthConfig()

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	resolver := newMockResolver()
	senderDID := "did:key:sender123"
	resolver.AddKey(senderDID, pub)

	ea, err := NewEnvelopeAuthenticator(resolver, "did:key:server", config, clock)
	if err != nil {
		t.Fatalf("NewEnvelopeAuthenticator failed: %v", err)
	}

	// Use invalid signature
	result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, EnvelopeAuth{
		SenderDID:    senderDID,
		TimestampMs:  uint64(now.UnixMilli()),
		Sequence:     1,
		Signature:    make([]byte, 64), // Wrong signature
		PayloadBytes: []byte("payload"),
	})

	if result.Authenticated {
		t.Error("Expected authentication to fail with invalid signature")
	}
	if result.ErrorCode != ErrorCodeAuthSignatureInvalid {
		t.Errorf("ErrorCode = %d, want %d", result.ErrorCode, ErrorCodeAuthSignatureInvalid)
	}
}

func TestEnvelopeAuthenticator_RecipientDIDMismatch(t *testing.T) {
	now := time.Now()
	clock := &mockClock{now: now}
	config := DefaultAuthConfig()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	resolver := newMockResolver()
	senderDID := "did:key:sender123"
	serverDID := "did:key:server"
	otherDID := "did:key:other"
	payload := []byte("payload")
	resolver.AddKey(senderDID, pub)

	ea, err := NewEnvelopeAuthenticator(resolver, serverDID, config, clock)
	if err != nil {
		t.Fatalf("NewEnvelopeAuthenticator failed: %v", err)
	}

	transcript := auth.BuildDIDAuthTranscript(senderDID, otherDID, payload, now)
	signature := ed25519.Sign(priv, transcript)

	result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, EnvelopeAuth{
		SenderDID:    senderDID,
		RecipientDID: otherDID,
		TimestampMs:  uint64(now.UnixMilli()),
		Sequence:     1,
		Signature:    signature,
		PayloadBytes: payload,
	})

	if result.Authenticated {
		t.Error("expected authentication to fail")
	}
	if !errors.Is(result.Error, ErrRecipientDIDMismatch) {
		t.Fatalf("error = %v, want %v", result.Error, ErrRecipientDIDMismatch)
	}
	if result.ErrorCode != ErrorCodeAuthDIDInvalid {
		t.Fatalf("ErrorCode = %d, want %d", result.ErrorCode, ErrorCodeAuthDIDInvalid)
	}
}

func TestEnvelopeAuthenticator_DIDNotFound(t *testing.T) {
	now := time.Now()
	clock := &mockClock{now: now}
	config := DefaultAuthConfig()
	resolver := newMockResolver() // Empty resolver

	ea, err := NewEnvelopeAuthenticator(resolver, "did:key:server", config, clock)
	if err != nil {
		t.Fatalf("NewEnvelopeAuthenticator failed: %v", err)
	}

	result := ea.VerifyEnvelope(context.Background(), negotiate.ProfileSecureBaseline, EnvelopeAuth{
		SenderDID:    "did:key:unknown",
		TimestampMs:  uint64(now.UnixMilli()),
		Sequence:     1,
		Signature:    make([]byte, 64),
		PayloadBytes: []byte("payload"),
	})

	if result.Authenticated {
		t.Error("Expected authentication to fail when DID not found")
	}
	if result.ErrorCode != ErrorCodeAuthDIDInvalid {
		t.Errorf("ErrorCode = %d, want %d", result.ErrorCode, ErrorCodeAuthDIDInvalid)
	}
}

func TestDefaultAuthConfig(t *testing.T) {
	config := DefaultAuthConfig()

	if config.MaxTimestampAge != 5*time.Minute {
		t.Errorf("MaxTimestampAge = %v, want 5m", config.MaxTimestampAge)
	}
	if config.MaxTimestampFuture != 30*time.Second {
		t.Errorf("MaxTimestampFuture = %v, want 30s", config.MaxTimestampFuture)
	}
	if config.ReplayTTL != 10*time.Minute {
		t.Errorf("ReplayTTL = %v, want 10m", config.ReplayTTL)
	}
	if config.ReplayWindow != 1000 {
		t.Errorf("ReplayWindow = %d, want 1000", config.ReplayWindow)
	}
}
