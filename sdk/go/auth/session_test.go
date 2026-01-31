package auth

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestSession_SignEnvelope(t *testing.T) {
	// Generate test identity
	did, pubKey, privKey, err := GenerateTestIdentity()
	if err != nil {
		t.Fatalf("GenerateTestIdentity failed: %v", err)
	}

	// Verify DID format
	if did == "" {
		t.Error("Generated DID is empty")
	}

	// Create session
	session := NewSession(SessionConfig{
		LocalDID:   did,
		PrivateKey: privKey,
	})

	// Should fail before negotiation
	_, err = session.SignEnvelope(context.Background(), SignatureInput{
		Payload: []byte("test payload"),
	})
	if err != ErrSessionNotNegotiated {
		t.Errorf("Expected ErrSessionNotNegotiated, got %v", err)
	}

	// Complete negotiation
	remoteDID := "did:key:remote-server"
	session.SetNegotiated("secure-baseline-v1", remoteDID)

	// Now signing should work
	output, err := session.SignEnvelope(context.Background(), SignatureInput{
		Payload: []byte("test payload"),
	})
	if err != nil {
		t.Fatalf("SignEnvelope failed: %v", err)
	}

	// Verify output
	if output.SenderDID != did {
		t.Errorf("SenderDID = %q, want %q", output.SenderDID, did)
	}
	if output.RecipientDID != remoteDID {
		t.Errorf("RecipientDID = %q, want %q", output.RecipientDID, remoteDID)
	}
	if output.TimestampMs == 0 {
		t.Error("TimestampMs should not be zero")
	}
	if output.Sequence != 1 {
		t.Errorf("Sequence = %d, want 1", output.Sequence)
	}
	if len(output.Signature) != ed25519.SignatureSize {
		t.Errorf("Signature length = %d, want %d", len(output.Signature), ed25519.SignatureSize)
	}

	// Verify signature is valid
	transcript := BuildDIDAuthTranscript(did, remoteDID, []byte("test payload"), time.UnixMilli(int64(output.TimestampMs)))
	if !VerifyEd25519(pubKey, transcript, output.Signature) {
		t.Error("Signature verification failed")
	}
}

func TestSession_SequenceCounter(t *testing.T) {
	did, _, privKey, _ := GenerateTestIdentity()
	session := NewSession(SessionConfig{
		LocalDID:   did,
		PrivateKey: privKey,
	})
	session.SetNegotiated("secure-baseline-v1", "did:key:remote")

	// Sign multiple envelopes and verify sequence increments
	for i := uint64(1); i <= 5; i++ {
		output, err := session.SignEnvelope(context.Background(), SignatureInput{
			Payload: []byte("payload"),
		})
		if err != nil {
			t.Fatalf("SignEnvelope failed: %v", err)
		}
		if output.Sequence != i {
			t.Errorf("Sequence = %d, want %d", output.Sequence, i)
		}
	}
}

func TestSession_RecipientOverride(t *testing.T) {
	did, _, privKey, _ := GenerateTestIdentity()
	session := NewSession(SessionConfig{
		LocalDID:   did,
		PrivateKey: privKey,
	})
	session.SetNegotiated("secure-baseline-v1", "did:key:default-remote")

	// Override recipient
	overrideDID := "did:key:specific-recipient"
	output, err := session.SignEnvelope(context.Background(), SignatureInput{
		Payload:      []byte("payload"),
		RecipientDID: overrideDID,
	})
	if err != nil {
		t.Fatalf("SignEnvelope failed: %v", err)
	}
	if output.RecipientDID != overrideDID {
		t.Errorf("RecipientDID = %q, want %q", output.RecipientDID, overrideDID)
	}
}

func TestSession_NoPrivateKey(t *testing.T) {
	session := NewSession(SessionConfig{
		LocalDID: "did:key:verify-only",
		// No private key
	})
	session.SetNegotiated("secure-baseline-v1", "did:key:remote")

	_, err := session.SignEnvelope(context.Background(), SignatureInput{
		Payload: []byte("payload"),
	})
	if err != ErrNoPrivateKey {
		t.Errorf("Expected ErrNoPrivateKey, got %v", err)
	}
}

func TestSession_Closed(t *testing.T) {
	did, _, privKey, _ := GenerateTestIdentity()
	session := NewSession(SessionConfig{
		LocalDID:   did,
		PrivateKey: privKey,
	})
	session.SetNegotiated("secure-baseline-v1", "did:key:remote")

	// Close session
	session.Close()

	if !session.IsClosed() {
		t.Error("Session should be closed")
	}

	_, err := session.SignEnvelope(context.Background(), SignatureInput{
		Payload: []byte("payload"),
	})
	if err != ErrSessionClosed {
		t.Errorf("Expected ErrSessionClosed, got %v", err)
	}
}

func TestMemoryDIDResolver(t *testing.T) {
	resolver := NewMemoryDIDResolver()

	did := "did:key:test-abc123"
	_, pubKey, _, _ := GenerateTestIdentity()

	// Should not find before adding
	_, err := resolver.Resolve(context.Background(), did)
	if err != ErrDIDNotFound {
		t.Errorf("Expected ErrDIDNotFound, got %v", err)
	}

	// Add DID
	resolver.AddDID(did, pubKey)

	// Should find now
	doc, err := resolver.Resolve(context.Background(), did)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if doc.ID != did {
		t.Errorf("Document ID = %q, want %q", doc.ID, did)
	}
	if len(doc.PublicKeys) != 1 {
		t.Errorf("PublicKeys count = %d, want 1", len(doc.PublicKeys))
	}
}

func TestGenerateTestIdentity(t *testing.T) {
	did1, pub1, priv1, err := GenerateTestIdentity()
	if err != nil {
		t.Fatalf("GenerateTestIdentity failed: %v", err)
	}

	did2, pub2, priv2, err := GenerateTestIdentity()
	if err != nil {
		t.Fatalf("GenerateTestIdentity failed: %v", err)
	}

	// DIDs should be different
	if did1 == did2 {
		t.Error("Generated DIDs should be unique")
	}

	// Keys should be different
	if string(pub1) == string(pub2) {
		t.Error("Generated public keys should be unique")
	}
	if string(priv1) == string(priv2) {
		t.Error("Generated private keys should be unique")
	}

	// Keys should work for signing
	msg := []byte("test message")
	sig := ed25519.Sign(priv1, msg)
	if !ed25519.Verify(pub1, msg, sig) {
		t.Error("Generated keypair should be valid for signing")
	}
}

func TestSession_Getters(t *testing.T) {
	localDID := "did:key:local"
	remoteDID := "did:key:remote"
	profile := "secure-baseline-v1"

	_, _, privKey, _ := GenerateTestIdentity()
	session := NewSession(SessionConfig{
		LocalDID:   localDID,
		PrivateKey: privKey,
	})

	// Before negotiation
	if session.IsNegotiated() {
		t.Error("Should not be negotiated initially")
	}
	if session.LocalDID() != localDID {
		t.Errorf("LocalDID = %q, want %q", session.LocalDID(), localDID)
	}
	if session.RemoteDID() != "" {
		t.Errorf("RemoteDID should be empty before negotiation")
	}
	if session.Profile() != "" {
		t.Errorf("Profile should be empty before negotiation")
	}

	// After negotiation
	session.SetNegotiated(profile, remoteDID)

	if !session.IsNegotiated() {
		t.Error("Should be negotiated after SetNegotiated")
	}
	if session.RemoteDID() != remoteDID {
		t.Errorf("RemoteDID = %q, want %q", session.RemoteDID(), remoteDID)
	}
	if session.Profile() != profile {
		t.Errorf("Profile = %q, want %q", session.Profile(), profile)
	}
}
