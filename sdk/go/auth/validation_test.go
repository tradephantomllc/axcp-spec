package auth

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

// mockEnvelope implements EnvelopeFields for testing
type mockEnvelope struct {
	senderDID    string
	recipientDID string
	timestampMs  uint64
	sequence     uint64
	signature    []byte
}

func (m *mockEnvelope) GetSenderDid() string    { return m.senderDID }
func (m *mockEnvelope) GetRecipientDid() string { return m.recipientDID }
func (m *mockEnvelope) GetTimestampMs() uint64  { return m.timestampMs }
func (m *mockEnvelope) GetSequence() uint64     { return m.sequence }
func (m *mockEnvelope) GetSignature() []byte    { return m.signature }

func TestValidateRecipient_Valid(t *testing.T) {
	env := &mockEnvelope{
		recipientDID: "did:example:test-recipient",
	}

	err := ValidateRecipient(env, "did:example:test-recipient")
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestValidateRecipient_Nil(t *testing.T) {
	err := ValidateRecipient(nil, "did:example:test")
	if err != ErrEnvelopeNil {
		t.Errorf("expected ErrEnvelopeNil, got: %v", err)
	}
}

func TestValidateRecipient_Missing(t *testing.T) {
	env := &mockEnvelope{
		recipientDID: "",
	}

	err := ValidateRecipient(env, "did:example:test")
	if err != ErrRecipientMissing {
		t.Errorf("expected ErrRecipientMissing, got: %v", err)
	}
}

func TestValidateRecipient_Mismatch(t *testing.T) {
	env := &mockEnvelope{
		recipientDID: "did:example:other-recipient",
	}

	err := ValidateRecipient(env, "did:example:expected-recipient")
	if err != ErrRecipientMismatch {
		t.Errorf("expected ErrRecipientMismatch, got: %v", err)
	}
}

func TestValidateSender_Valid(t *testing.T) {
	env := &mockEnvelope{
		senderDID: "did:example:valid-sender",
	}

	err := ValidateSender(env)
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestValidateSender_Nil(t *testing.T) {
	err := ValidateSender(nil)
	if err != ErrEnvelopeNil {
		t.Errorf("expected ErrEnvelopeNil, got: %v", err)
	}
}

func TestValidateSender_Missing(t *testing.T) {
	env := &mockEnvelope{
		senderDID: "",
	}

	err := ValidateSender(env)
	if err != ErrSenderMissing {
		t.Errorf("expected ErrSenderMissing, got: %v", err)
	}
}

func TestValidateSender_InvalidFormat(t *testing.T) {
	env := &mockEnvelope{
		senderDID: "invalid-did-format",
	}

	err := ValidateSender(env)
	if err == nil {
		t.Error("expected error for invalid DID format")
	}
	if err == ErrSenderMissing {
		t.Error("expected DID format error, not ErrSenderMissing")
	}
}

func TestValidateSignaturePresent_Valid(t *testing.T) {
	env := &mockEnvelope{
		signature: testSignature(),
	}

	err := ValidateSignaturePresent(env)
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestValidateSignaturePresent_Nil(t *testing.T) {
	err := ValidateSignaturePresent(nil)
	if err != ErrEnvelopeNil {
		t.Errorf("expected ErrEnvelopeNil, got: %v", err)
	}
}

func TestValidateSignaturePresent_Missing(t *testing.T) {
	env := &mockEnvelope{
		signature: nil,
	}

	err := ValidateSignaturePresent(env)
	if err != ErrSignatureMissing {
		t.Errorf("expected ErrSignatureMissing, got: %v", err)
	}

	env.signature = []byte{}
	err = ValidateSignaturePresent(env)
	if err != ErrSignatureMissing {
		t.Errorf("expected ErrSignatureMissing for empty slice, got: %v", err)
	}
}

func TestQuickValidate_Valid(t *testing.T) {
	env := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  NowMs(),
		sequence:     1,
		signature:    testSignature(),
	}

	err := QuickValidate(env, "did:example:recipient")
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}

	env.sequence = 0
	if err := QuickValidate(env, "did:example:recipient"); err != ErrSequenceMissing {
		t.Errorf("zero sequence: got %v, want ErrSequenceMissing", err)
	}
	env.sequence = 1
	env.signature = make([]byte, 63)
	if err := QuickValidate(env, "did:example:recipient"); err != ErrSignatureLength {
		t.Errorf("short signature: got %v, want ErrSignatureLength", err)
	}
}

func TestQuickValidateWithTimestamp_UsesConfiguredPolicy(t *testing.T) {
	env := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  NowMs() - uint64((3 * time.Minute).Milliseconds()),
		sequence:     1,
		signature:    testSignature(),
	}
	strict := NewTimestampValidator(time.Minute, time.Second)
	if err := QuickValidateWithTimestamp(env, "did:example:recipient", strict); err != ErrTimestampExpired {
		t.Fatalf("got %v, want ErrTimestampExpired", err)
	}
	if err := QuickValidateWithTimestamp(env, "did:example:recipient", nil); err != nil {
		t.Fatalf("default policy: %v", err)
	}
}

func TestQuickValidate_AllErrors(t *testing.T) {
	tests := []struct {
		name     string
		env      EnvelopeFields
		expected error
	}{
		{
			name:     "nil envelope",
			env:      nil,
			expected: ErrEnvelopeNil,
		},
		{
			name: "missing recipient",
			env: &mockEnvelope{
				senderDID:   "did:example:sender",
				timestampMs: NowMs(),
				signature:   testSignature(),
			},
			expected: ErrRecipientMissing,
		},
		{
			name: "wrong recipient",
			env: &mockEnvelope{
				senderDID:    "did:example:sender",
				recipientDID: "did:example:wrong",
				timestampMs:  NowMs(),
				signature:    testSignature(),
			},
			expected: ErrRecipientMismatch,
		},
		{
			name: "missing sender",
			env: &mockEnvelope{
				recipientDID: "did:example:recipient",
				timestampMs:  NowMs(),
				signature:    testSignature(),
			},
			expected: ErrSenderMissing,
		},
		{
			name: "missing timestamp",
			env: &mockEnvelope{
				senderDID:    "did:example:sender",
				recipientDID: "did:example:recipient",
				timestampMs:  0,
				signature:    testSignature(),
			},
			expected: ErrTimestampMissing,
		},
		{
			name: "expired timestamp",
			env: &mockEnvelope{
				senderDID:    "did:example:sender",
				recipientDID: "did:example:recipient",
				timestampMs:  uint64(time.Now().Add(-10 * time.Minute).UnixMilli()),
				signature:    testSignature(),
			},
			expected: ErrTimestampExpired,
		},
		{
			name: "missing signature",
			env: &mockEnvelope{
				senderDID:    "did:example:sender",
				recipientDID: "did:example:recipient",
				timestampMs:  NowMs(),
				signature:    nil,
			},
			expected: ErrSignatureMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := QuickValidate(tt.env, "did:example:recipient")
			if err != tt.expected {
				t.Errorf("expected %v, got: %v", tt.expected, err)
			}
		})
	}
}

func TestEnvelopeValidator_ValidateEnvelope(t *testing.T) {
	// Generate a test key pair
	pub, _, _ := ed25519.GenerateKey(nil)

	// Create a mock resolver
	resolver := NewMemoryDIDResolver()
	resolver.AddDID("did:example:sender", pub)

	// Create validator
	v := NewEnvelopeValidator(resolver, "did:example:recipient")

	// Valid envelope
	env := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  NowMs(),
		sequence:     1,
		signature:    make([]byte, 64), // ed25519 signature size
	}

	err := v.ValidateEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestEnvelopeValidator_ValidateEnvelope_Errors(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)

	resolver := NewMemoryDIDResolver()
	resolver.AddDID("did:example:sender", pub)

	v := NewEnvelopeValidator(resolver, "did:example:recipient")

	tests := []struct {
		name string
		env  EnvelopeFields
	}{
		{
			name: "nil envelope",
			env:  nil,
		},
		{
			name: "wrong recipient",
			env: &mockEnvelope{
				senderDID:    "did:example:sender",
				recipientDID: "did:example:wrong",
				timestampMs:  NowMs(),
				signature:    make([]byte, 64),
			},
		},
		{
			name: "unresolvable sender",
			env: &mockEnvelope{
				senderDID:    "did:example:unknown",
				recipientDID: "did:example:recipient",
				timestampMs:  NowMs(),
				signature:    make([]byte, 64),
			},
		},
		{
			name: "expired timestamp",
			env: &mockEnvelope{
				senderDID:    "did:example:sender",
				recipientDID: "did:example:recipient",
				timestampMs:  uint64(time.Now().Add(-10 * time.Minute).UnixMilli()),
				signature:    make([]byte, 64),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateEnvelope(context.Background(), tt.env)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestEnvelopeValidator_ValidateEnvelopeDoesNotMutateReplayProtector(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)

	resolver := NewMemoryDIDResolver()
	resolver.AddDID("did:example:sender", pub)

	rp, err := NewReplayProtector(5*time.Minute, 100, nil)
	if err != nil {
		t.Fatalf("failed to create replay protector: %v", err)
	}

	v := NewEnvelopeValidator(resolver, "did:example:recipient").
		WithReplayProtector(rp)

	env := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  NowMs(),
		sequence:     1,
		signature:    make([]byte, 64),
	}

	// ValidateEnvelope is preflight-only and should not mark replay state.
	err = v.ValidateEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("first call should succeed: %v", err)
	}

	err = v.ValidateEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("second preflight call with same sequence should not fail as replay: %v", err)
	}
}

func TestEnvelopeValidator_WithReplayProtector(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)

	resolver := NewMemoryDIDResolver()
	resolver.AddDID("did:example:sender", pub)

	rp, err := NewReplayProtector(5*time.Minute, 100, nil)
	if err != nil {
		t.Fatalf("failed to create replay protector: %v", err)
	}

	v := NewEnvelopeValidator(resolver, "did:example:recipient").
		WithReplayProtector(rp)

	transcript := []byte("test-transcript-data")
	signature := ed25519.Sign(priv, transcript)
	env := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  NowMs(),
		sequence:     1,
		signature:    signature,
	}

	// First verified call should succeed and mark replay state.
	err = v.ValidateAndVerifySignature(context.Background(), env, transcript)
	if err != nil {
		t.Errorf("first call should succeed: %v", err)
	}

	// Second verified call with same sequence should fail.
	err = v.ValidateAndVerifySignature(context.Background(), env, transcript)
	if err == nil {
		t.Fatal("second call should fail due to replay detection")
	}

	// New sequence should succeed.
	env.sequence = 2
	err = v.ValidateAndVerifySignature(context.Background(), env, transcript)
	if err != nil {
		t.Errorf("new sequence should succeed: %v", err)
	}
}

func TestEnvelopeValidator_InvalidSignatureDoesNotConsumeReplaySequence(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)

	resolver := NewMemoryDIDResolver()
	resolver.AddDID("did:example:sender", pub)

	rp, err := NewReplayProtector(5*time.Minute, 100, nil)
	if err != nil {
		t.Fatalf("failed to create replay protector: %v", err)
	}

	v := NewEnvelopeValidator(resolver, "did:example:recipient").
		WithReplayProtector(rp)

	transcript := []byte("test-transcript-data")
	validSignature := ed25519.Sign(priv, transcript)
	env := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  NowMs(),
		sequence:     42,
		signature:    make([]byte, ed25519.SignatureSize),
	}

	err = v.ValidateAndVerifySignature(context.Background(), env, transcript)
	if err != ErrVerificationFailed {
		t.Fatalf("invalid signature error = %v, want %v", err, ErrVerificationFailed)
	}

	env.signature = validSignature
	err = v.ValidateAndVerifySignature(context.Background(), env, transcript)
	if err != nil {
		t.Fatalf("valid signature with same sequence should succeed after invalid attempt: %v", err)
	}
}

func TestEnvelopeValidator_WithCustomTimestampValidator(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)

	resolver := NewMemoryDIDResolver()
	resolver.AddDID("did:example:sender", pub)

	// Use strict timestamp validator (1 minute max age)
	tv := NewTimestampValidator(1*time.Minute, 10*time.Second)

	v := NewEnvelopeValidator(resolver, "did:example:recipient").
		WithTimestampValidator(tv)

	// 2 minutes old should be rejected
	env := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  uint64(time.Now().Add(-2 * time.Minute).UnixMilli()),
		sequence:     1,
		signature:    make([]byte, 64),
	}

	err := v.ValidateEnvelope(context.Background(), env)
	if err != ErrTimestampExpired {
		t.Errorf("expected ErrTimestampExpired, got: %v", err)
	}

	// Current time should succeed
	env.timestampMs = NowMs()
	err = v.ValidateEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("current timestamp should succeed: %v", err)
	}
}

func TestEnvelopeValidator_ValidateAndVerifySignature(t *testing.T) {
	// Generate a test key pair
	pub, priv, _ := ed25519.GenerateKey(nil)

	resolver := NewMemoryDIDResolver()
	resolver.AddDID("did:example:sender", pub)

	v := NewEnvelopeValidator(resolver, "did:example:recipient")

	// Create a transcript and sign it
	transcript := []byte("test-transcript-data")
	signature := ed25519.Sign(priv, transcript)

	env := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  NowMs(),
		sequence:     1,
		signature:    signature,
	}

	// Valid signature should succeed
	err := v.ValidateAndVerifySignature(context.Background(), env, transcript)
	if err != nil {
		t.Errorf("valid signature should succeed: %v", err)
	}

	// Wrong transcript should fail
	wrongTranscript := []byte("wrong-transcript")
	err = v.ValidateAndVerifySignature(context.Background(), env, wrongTranscript)
	if err != ErrVerificationFailed {
		t.Errorf("wrong transcript should fail with ErrVerificationFailed, got: %v", err)
	}

	// Tampered signature should fail
	tamperedEnv := &mockEnvelope{
		senderDID:    "did:example:sender",
		recipientDID: "did:example:recipient",
		timestampMs:  NowMs(),
		sequence:     2,
		signature:    make([]byte, 64), // zeros instead of valid signature
	}
	err = v.ValidateAndVerifySignature(context.Background(), tamperedEnv, transcript)
	if err != ErrVerificationFailed {
		t.Errorf("tampered signature should fail, got: %v", err)
	}
}

// testSignature returns a placeholder signature of the correct length.
func testSignature() []byte {
	sig := make([]byte, 64)
	sig[0] = 1
	return sig
}
