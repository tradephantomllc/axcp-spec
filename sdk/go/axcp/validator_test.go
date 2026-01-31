package axcp

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

func TestNewValidator(t *testing.T) {
	resolver := auth.NewMemoryDIDResolver()
	config := ValidatorConfig{
		LocalDID: "did:key:local",
		Resolver: resolver,
	}

	v := NewValidator(config)
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
}

func TestValidator_QuickValidate(t *testing.T) {
	resolver := auth.NewMemoryDIDResolver()
	v := NewValidator(ValidatorConfig{
		LocalDID: "did:key:local",
		Resolver: resolver,
	})

	tests := []struct {
		name    string
		env     *Envelope
		wantErr bool
	}{
		{
			name:    "nil envelope",
			env:     nil,
			wantErr: true,
		},
		{
			name: "missing recipient",
			env: func() *Envelope {
				e := NewEnvelope("trace", 1)
				e.SenderDid = "did:key:sender"
				e.TimestampMs = auth.NowMs()
				e.Signature = make([]byte, 64)
				return e
			}(),
			wantErr: true,
		},
		{
			name: "wrong recipient",
			env: func() *Envelope {
				e := NewEnvelope("trace", 1)
				e.SenderDid = "did:key:sender"
				e.RecipientDid = "did:key:other"
				e.TimestampMs = auth.NowMs()
				e.Signature = make([]byte, 64)
				return e
			}(),
			wantErr: true,
		},
		{
			name: "missing sender",
			env: func() *Envelope {
				e := NewEnvelope("trace", 1)
				e.RecipientDid = "did:key:local"
				e.TimestampMs = auth.NowMs()
				e.Signature = make([]byte, 64)
				return e
			}(),
			wantErr: true,
		},
		{
			name: "missing timestamp",
			env: func() *Envelope {
				e := NewEnvelope("trace", 1)
				e.SenderDid = "did:key:sender"
				e.RecipientDid = "did:key:local"
				e.Signature = make([]byte, 64)
				return e
			}(),
			wantErr: true,
		},
		{
			name: "missing signature",
			env: func() *Envelope {
				e := NewEnvelope("trace", 1)
				e.SenderDid = "did:key:sender"
				e.RecipientDid = "did:key:local"
				e.TimestampMs = auth.NowMs()
				return e
			}(),
			wantErr: true,
		},
		{
			name: "valid envelope",
			env: func() *Envelope {
				e := NewEnvelope("trace", 1)
				e.SenderDid = "did:key:sender"
				e.RecipientDid = "did:key:local"
				e.TimestampMs = auth.NowMs()
				e.Signature = make([]byte, 64)
				return e
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perr := v.QuickValidate(tt.env)
			if (perr != nil) != tt.wantErr {
				t.Errorf("QuickValidate() error = %v, wantErr %v", perr, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateAndVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	v := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})

	// Create valid signed envelope
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = senderDID
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = 1

	transcript := buildEnvelopeTranscript(env)
	env.Signature = ed25519.Sign(priv, transcript)

	// Should pass
	perr := v.ValidateAndVerify(context.Background(), env)
	if perr != nil {
		t.Errorf("ValidateAndVerify() unexpected error: %v", perr)
	}
}

func TestValidator_ValidateAndVerify_InvalidSignature(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	v := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})

	// Create envelope with invalid signature
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = senderDID
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Signature = make([]byte, 64) // Invalid signature

	perr := v.ValidateAndVerify(context.Background(), env)
	if perr == nil {
		t.Error("ValidateAndVerify() expected error for invalid signature")
	}
	if !errors.Is(perr, ErrAuthSignatureInvalid) {
		t.Errorf("expected ErrAuthSignatureInvalid, got %v", perr)
	}
}

func TestValidator_ValidateAndVerify_UnknownDID(t *testing.T) {
	localDID := "did:key:local"

	// Empty resolver - no DIDs registered
	resolver := auth.NewMemoryDIDResolver()

	v := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})

	env := NewEnvelope("trace-123", 1)
	env.SenderDid = "did:key:unknown"
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Signature = make([]byte, 64)

	perr := v.ValidateAndVerify(context.Background(), env)
	if perr == nil {
		t.Error("ValidateAndVerify() expected error for unknown DID")
	}
	if !errors.Is(perr, ErrAuthDIDInvalid) {
		t.Errorf("expected ErrAuthDIDInvalid, got %v", perr)
	}
}

func TestValidator_SkipSignatureVerification(t *testing.T) {
	localDID := "did:key:local"
	resolver := auth.NewMemoryDIDResolver()

	v := NewValidator(ValidatorConfig{
		LocalDID:                  localDID,
		Resolver:                  resolver,
		SkipSignatureVerification: true,
	})

	// Create envelope with invalid signature - should still pass
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = "did:key:sender"
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Signature = make([]byte, 64) // Invalid signature

	perr := v.ValidateAndVerify(context.Background(), env)
	if perr != nil {
		t.Errorf("ValidateAndVerify() with skip should not error: %v", perr)
	}
}

func TestValidator_WithReplayProtection(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	rp, _ := auth.NewReplayProtector(5*time.Minute, 100, nil)

	v := NewValidator(ValidatorConfig{
		LocalDID:        localDID,
		Resolver:        resolver,
		ReplayProtector: rp,
	})

	// Create valid signed envelope
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = senderDID
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = 1

	transcript := buildEnvelopeTranscript(env)
	env.Signature = ed25519.Sign(priv, transcript)

	// First request should pass
	perr := v.ValidateAndVerify(context.Background(), env)
	if perr != nil {
		t.Errorf("First ValidateAndVerify() unexpected error: %v", perr)
	}

	// Second request with same sequence should fail (replay)
	perr = v.ValidateAndVerify(context.Background(), env)
	if perr == nil {
		t.Error("Second ValidateAndVerify() expected replay error")
	}
	if !errors.Is(perr, ErrAuthReplayDetected) {
		t.Errorf("expected ErrAuthReplayDetected, got %v", perr)
	}
}

func TestValidatorMiddleware(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	v := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})

	handlerCalled := false
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		handlerCalled = true
		return req, nil
	})

	wrapped := v.Middleware()(handler)

	// Create valid signed envelope
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = senderDID
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = 1

	transcript := buildEnvelopeTranscript(env)
	env.Signature = ed25519.Sign(priv, transcript)

	_, err := wrapped.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}

func TestValidatorMiddleware_RejectsInvalid(t *testing.T) {
	localDID := "did:key:local"
	resolver := auth.NewMemoryDIDResolver()

	v := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})

	handlerCalled := false
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		handlerCalled = true
		return req, nil
	})

	wrapped := v.Middleware()(handler)

	// Invalid envelope (no signature)
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = "did:key:sender"
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()

	_, err := wrapped.HandleEnvelope(context.Background(), env)
	if err == nil {
		t.Error("expected error for invalid envelope")
	}
	if handlerCalled {
		t.Error("handler should not be called for invalid envelope")
	}
}

func TestQuickRejectMiddleware(t *testing.T) {
	localDID := "did:key:local"
	resolver := auth.NewMemoryDIDResolver()

	v := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})

	handlerCalled := false
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		handlerCalled = true
		return req, nil
	})

	wrapped := v.QuickRejectMiddleware()(handler)

	// Invalid envelope (missing sender)
	env := NewEnvelope("trace-123", 1)
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Signature = make([]byte, 64)

	_, err := wrapped.HandleEnvelope(context.Background(), env)
	if err == nil {
		t.Error("expected error for invalid envelope")
	}
	if handlerCalled {
		t.Error("handler should not be called for invalid envelope")
	}
}

func TestValidateWithResult(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	v := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})

	// Create valid signed envelope
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = senderDID
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = 1

	transcript := buildEnvelopeTranscript(env)
	env.Signature = ed25519.Sign(priv, transcript)

	result := v.ValidateWithResult(context.Background(), env)
	if !result.Valid {
		t.Errorf("expected valid result, got error: %v", result.Error)
	}
	if len(result.SenderPublicKey) != ed25519.PublicKeySize {
		t.Errorf("expected public key, got %d bytes", len(result.SenderPublicKey))
	}
}

func TestValidateWithResult_Invalid(t *testing.T) {
	localDID := "did:key:local"
	resolver := auth.NewMemoryDIDResolver()

	v := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})

	result := v.ValidateWithResult(context.Background(), nil)
	if result.Valid {
		t.Error("expected invalid result for nil envelope")
	}
	if result.Error == nil {
		t.Error("expected error in result")
	}
}

func TestErrorResponseMiddleware(t *testing.T) {
	localDID := "did:key:local"

	testErr := errors.New("test error")
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return nil, testErr
	})

	wrapped := ErrorResponseMiddleware(localDID)(handler)

	env := NewEnvelope("trace-123", 1)
	env.SenderDid = "did:key:sender"
	env.RecipientDid = localDID

	resp, err := wrapped.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("ErrorResponseMiddleware should not return error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected error envelope response")
	}

	// Check it's an error envelope
	if !IsErrorEnvelope(resp) {
		t.Error("response should be an error envelope")
	}

	// Check the error was converted
	perr, ok := ErrorFromEnvelope(resp)
	if !ok {
		t.Fatal("could not extract error from envelope")
	}
	if perr.Code != ErrUnknown.Code {
		t.Errorf("expected UNKNOWN error code, got %v", perr.Code)
	}
}

func TestSecureValidatorMiddleware(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	config := ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	}

	handlerCalled := false
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		handlerCalled = true
		return req, nil
	})

	wrapped := SecureValidatorMiddleware(config)(handler)

	// Create valid signed envelope
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = senderDID
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = 1

	transcript := buildEnvelopeTranscript(env)
	env.Signature = ed25519.Sign(priv, transcript)

	_, err := wrapped.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}

func TestNewValidatorWithOptions(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	rp, _ := auth.NewReplayProtector(5*time.Minute, 100, nil)
	tv := auth.NewTimestampValidator(10*time.Minute, 1*time.Minute)

	v := NewValidatorWithOptions(
		WithLocalDID(localDID),
		WithResolver(resolver),
		WithReplayProtection(rp),
		WithTimestampValidation(tv),
	)

	if v == nil {
		t.Fatal("NewValidatorWithOptions returned nil")
	}

	if v.config.LocalDID != localDID {
		t.Errorf("LocalDID = %q, want %q", v.config.LocalDID, localDID)
	}
	if v.config.ReplayProtector == nil {
		t.Error("ReplayProtector should be set")
	}
	if v.config.TimestampValidator == nil {
		t.Error("TimestampValidator should be set")
	}
}

func TestValidateRequest(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	// Create valid signed envelope
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = senderDID
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = 1

	transcript := buildEnvelopeTranscript(env)
	env.Signature = ed25519.Sign(priv, transcript)

	perr := ValidateRequest(context.Background(), env, localDID, resolver)
	if perr != nil {
		t.Errorf("ValidateRequest() unexpected error: %v", perr)
	}
}
