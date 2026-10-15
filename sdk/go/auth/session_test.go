package auth

import (
	"context"
	"crypto/ed25519"
	"strings"
	"testing"
	"time"
)

// fixedPayload returns a PayloadForSequence builder whose output covers the sequence.
func fixedPayload(body string) func(uint64) ([]byte, error) {
	return func(sequence uint64) ([]byte, error) {
		return sequencePayload(body, sequence), nil
	}
}

func sequencePayload(body string, sequence uint64) []byte {
	out := []byte(body)
	for i := 0; i < 8; i++ {
		out = append(out, byte(sequence>>(8*i)))
	}
	return out
}

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

	if !strings.HasPrefix(did, "did:key:z6Mk") {
		t.Errorf("GenerateTestIdentity must return a standard Ed25519 did:key, got %q", did)
	}
	if decoded, err := DecodeDIDKey(did); err != nil || !decoded.Equal(pubKey) {
		t.Errorf("did:key does not encode the public key: %v", err)
	}

	// Create session
	session := NewSession(SessionConfig{
		LocalDID:      did,
		PrivateKey:    privKey,
		FirstSequence: 1,
	})

	// Should fail before negotiation
	_, err = session.SignEnvelope(context.Background(), SignatureInput{
		PayloadForSequence: fixedPayload("test payload"),
	})
	if err != ErrSessionNotNegotiated {
		t.Errorf("Expected ErrSessionNotNegotiated, got %v", err)
	}

	// Complete negotiation
	remoteDID := "did:example:remote-server"
	session.SetNegotiated("secure-baseline-v1", remoteDID)

	// A prebuilt payload cannot cover the sequence and is rejected.
	if _, err := session.SignEnvelope(context.Background(), SignatureInput{Payload: []byte("test payload")}); err != ErrSequenceNotBound {
		t.Fatalf("legacy Payload: got %v, want ErrSequenceNotBound", err)
	}

	// Now signing should work
	output, err := session.SignEnvelope(context.Background(), SignatureInput{
		PayloadForSequence: fixedPayload("test payload"),
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
	transcript := BuildDIDAuthTranscript(did, remoteDID, sequencePayload("test payload", output.Sequence), time.UnixMilli(int64(output.TimestampMs)))
	if !VerifyEd25519(pubKey, transcript, output.Signature) {
		t.Error("Signature verification failed")
	}
}

func TestSession_SequenceCounter(t *testing.T) {
	did, _, privKey, _ := GenerateTestIdentity()
	session := NewSession(SessionConfig{
		LocalDID:      did,
		PrivateKey:    privKey,
		FirstSequence: 1,
	})
	session.SetNegotiated("secure-baseline-v1", "did:example:remote")

	// Sign multiple envelopes and verify sequence increments
	for i := uint64(1); i <= 5; i++ {
		output, err := session.SignEnvelope(context.Background(), SignatureInput{
			PayloadForSequence: fixedPayload("payload"),
		})
		if err != nil {
			t.Fatalf("SignEnvelope failed: %v", err)
		}
		if output.Sequence != i {
			t.Errorf("Sequence = %d, want %d", output.Sequence, i)
		}
	}
}

func TestSession_SignEnvelopePayloadForSequence(t *testing.T) {
	did, pubKey, privKey, err := GenerateTestIdentity()
	if err != nil {
		t.Fatalf("GenerateTestIdentity failed: %v", err)
	}

	remoteDID := "did:example:remote"
	session := NewSession(SessionConfig{
		LocalDID:      did,
		PrivateKey:    privKey,
		FirstSequence: 1,
	})
	session.SetNegotiated("secure-baseline-v1", remoteDID)

	output, err := session.SignEnvelope(context.Background(), SignatureInput{
		PayloadForSequence: func(sequence uint64) ([]byte, error) {
			return []byte{byte(sequence), 'p', 'a', 'y', 'l', 'o', 'a', 'd'}, nil
		},
	})
	if err != nil {
		t.Fatalf("SignEnvelope failed: %v", err)
	}

	if output.Sequence != 1 {
		t.Fatalf("Sequence = %d, want 1", output.Sequence)
	}

	wantPayload := []byte{byte(output.Sequence), 'p', 'a', 'y', 'l', 'o', 'a', 'd'}
	transcript := BuildDIDAuthTranscript(did, remoteDID, wantPayload, time.UnixMilli(int64(output.TimestampMs)))
	if !VerifyEd25519(pubKey, transcript, output.Signature) {
		t.Fatal("signature did not cover sequence-aware signing payload")
	}

	wrongPayload := []byte{byte(output.Sequence + 1), 'p', 'a', 'y', 'l', 'o', 'a', 'd'}
	wrongTranscript := BuildDIDAuthTranscript(did, remoteDID, wrongPayload, time.UnixMilli(int64(output.TimestampMs)))
	if VerifyEd25519(pubKey, wrongTranscript, output.Signature) {
		t.Fatal("signature verified with payload for a different sequence")
	}
}

func TestSession_RecipientOverride(t *testing.T) {
	did, _, privKey, _ := GenerateTestIdentity()
	session := NewSession(SessionConfig{
		LocalDID:   did,
		PrivateKey: privKey,
	})
	session.SetNegotiated("secure-baseline-v1", "did:example:default-remote")

	// Override recipient
	overrideDID := "did:example:specific-recipient"
	output, err := session.SignEnvelope(context.Background(), SignatureInput{
		PayloadForSequence: fixedPayload("payload"),
		RecipientDID:       overrideDID,
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
		LocalDID: "did:example:verify-only",
		// No private key
	})
	session.SetNegotiated("secure-baseline-v1", "did:example:remote")

	_, err := session.SignEnvelope(context.Background(), SignatureInput{
		PayloadForSequence: fixedPayload("payload"),
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
	session.SetNegotiated("secure-baseline-v1", "did:example:remote")

	// Close session
	session.Close()

	if !session.IsClosed() {
		t.Error("Session should be closed")
	}

	_, err := session.SignEnvelope(context.Background(), SignatureInput{
		PayloadForSequence: fixedPayload("payload"),
	})
	if err != ErrSessionClosed {
		t.Errorf("Expected ErrSessionClosed, got %v", err)
	}
}

func TestMemoryDIDResolver(t *testing.T) {
	resolver := NewMemoryDIDResolver()

	did := "did:example:test-abc123"
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
	localDID := "did:example:local"
	remoteDID := "did:example:remote"
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

func TestSession_TimeBasedFirstSequenceSurvivesRestart(t *testing.T) {
	did, _, privKey, _ := GenerateIdentity()
	clock := newFakeClock()

	first := NewSession(SessionConfig{LocalDID: did, PrivateKey: privKey, Clock: clock})
	first.SetNegotiated("secure-baseline-v1", "did:example:remote")
	var last uint64
	for i := 0; i < 3; i++ {
		out, err := first.SignEnvelope(context.Background(), SignatureInput{PayloadForSequence: fixedPayload("p")})
		if err != nil {
			t.Fatal(err)
		}
		last = out.Sequence
	}

	// A restarted sender must continue above the previous sequences.
	clock.Advance(time.Millisecond)
	restarted := NewSession(SessionConfig{LocalDID: did, PrivateKey: privKey, Clock: clock})
	restarted.SetNegotiated("secure-baseline-v1", "did:example:remote")
	out, err := restarted.SignEnvelope(context.Background(), SignatureInput{PayloadForSequence: fixedPayload("p")})
	if err != nil {
		t.Fatal(err)
	}
	if out.Sequence <= last {
		t.Fatalf("restarted sequence %d must exceed previous %d", out.Sequence, last)
	}
	rp, _ := NewReplayProtector(time.Hour, 1000, clock)
	if err := rp.CheckAndMark(did, last); err != nil {
		t.Fatal(err)
	}
	if err := rp.CheckAndMark(did, out.Sequence); err != nil {
		t.Fatalf("receiver rejected restarted sender: %v", err)
	}
}

func TestSession_RequiresRecipientAndValidKey(t *testing.T) {
	did, _, privKey, _ := GenerateIdentity()

	noRecipient := NewSession(SessionConfig{LocalDID: did, PrivateKey: privKey})
	noRecipient.SetNegotiated("secure-baseline-v1", "")
	if _, err := noRecipient.SignEnvelope(context.Background(), SignatureInput{PayloadForSequence: fixedPayload("p")}); err != ErrRecipientMissing {
		t.Fatalf("missing recipient: got %v", err)
	}

	badKey := NewSession(SessionConfig{LocalDID: did, PrivateKey: privKey[:31]})
	badKey.SetNegotiated("secure-baseline-v1", "did:example:remote")
	if _, err := badKey.SignEnvelope(context.Background(), SignatureInput{PayloadForSequence: fixedPayload("p")}); err != ErrInvalidPrivateKeySize {
		t.Fatalf("short key: got %v", err)
	}
}
