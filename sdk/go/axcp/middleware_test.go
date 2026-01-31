package axcp

import (
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

func TestChain(t *testing.T) {
	var order []string

	mw1 := func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			order = append(order, "mw1-before")
			resp, err := next.HandleEnvelope(ctx, req)
			order = append(order, "mw1-after")
			return resp, err
		})
	}

	mw2 := func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			order = append(order, "mw2-before")
			resp, err := next.HandleEnvelope(ctx, req)
			order = append(order, "mw2-after")
			return resp, err
		})
	}

	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		order = append(order, "handler")
		return nil, nil
	})

	chained := Chain(mw1, mw2)(handler)

	env := NewEnvelope("test", 1)
	_, _ = chained.HandleEnvelope(context.Background(), env)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("position %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		panic("test panic")
	})

	recovered := RecoveryMiddleware()(handler)

	env := NewEnvelope("test", 1)
	_, err := recovered.HandleEnvelope(context.Background(), env)

	if err == nil {
		t.Error("expected error from panic recovery")
	}
	if !errors.Is(err, ErrHandlerPanic) {
		t.Errorf("expected ErrHandlerPanic, got %v", err)
	}
}

func TestRecoveryMiddleware_NoError(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	recovered := RecoveryMiddleware()(handler)

	env := NewEnvelope("test", 1)
	resp, err := recovered.HandleEnvelope(context.Background(), env)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != env {
		t.Error("expected same envelope")
	}
}

// mockLogger for testing
type mockLogger struct {
	logs []string
}

func (l *mockLogger) Printf(format string, v ...interface{}) {
	l.logs = append(l.logs, format)
}

func TestLoggingMiddleware(t *testing.T) {
	logger := &mockLogger{}

	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	logged := LoggingMiddleware(logger)(handler)

	env := NewEnvelope("test-trace-123", 1)
	env.SenderDid = "did:key:sender"
	_, _ = logged.HandleEnvelope(context.Background(), env)

	if len(logger.logs) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(logger.logs))
	}
}

func TestLoggingMiddleware_NilLogger(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	// Should not panic with nil logger
	logged := LoggingMiddleware(nil)(handler)

	env := NewEnvelope("test", 1)
	_, err := logged.HandleEnvelope(context.Background(), env)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTimestampValidationMiddleware_Valid(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := TimestampValidationMiddleware(nil)(handler)

	env := NewEnvelope("test", 1)
	env.TimestampMs = auth.NowMs()

	_, err := validated.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTimestampValidationMiddleware_Expired(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := TimestampValidationMiddleware(nil)(handler)

	env := NewEnvelope("test", 1)
	env.TimestampMs = uint64(time.Now().Add(-10 * time.Minute).UnixMilli())

	_, err := validated.HandleEnvelope(context.Background(), env)
	if err == nil {
		t.Error("expected error for expired timestamp")
	}
}

func TestReplayProtectionMiddleware_Valid(t *testing.T) {
	protector, _ := auth.NewReplayProtector(5*time.Minute, 100, nil)

	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	protected := ReplayProtectionMiddleware(protector)(handler)

	env := NewEnvelope("test", 1)
	env.SenderDid = "did:key:sender"
	env.Sequence = 1

	// First call should succeed
	_, err := protected.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Second call with same sequence should fail
	_, err = protected.HandleEnvelope(context.Background(), env)
	if err == nil {
		t.Error("expected error for replay")
	}
}

func TestRecipientValidationMiddleware_Valid(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := RecipientValidationMiddleware("did:key:local")(handler)

	env := NewEnvelope("test", 1)
	env.RecipientDid = "did:key:local"

	_, err := validated.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecipientValidationMiddleware_Missing(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := RecipientValidationMiddleware("did:key:local")(handler)

	env := NewEnvelope("test", 1)
	// No recipient set

	_, err := validated.HandleEnvelope(context.Background(), env)
	if !errors.Is(err, auth.ErrRecipientMissing) {
		t.Errorf("expected ErrRecipientMissing, got %v", err)
	}
}

func TestRecipientValidationMiddleware_Mismatch(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := RecipientValidationMiddleware("did:key:local")(handler)

	env := NewEnvelope("test", 1)
	env.RecipientDid = "did:key:other"

	_, err := validated.HandleEnvelope(context.Background(), env)
	if !errors.Is(err, auth.ErrRecipientMismatch) {
		t.Errorf("expected ErrRecipientMismatch, got %v", err)
	}
}

func TestSenderValidationMiddleware_Valid(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := SenderValidationMiddleware()(handler)

	env := NewEnvelope("test", 1)
	env.SenderDid = "did:key:sender"

	_, err := validated.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSenderValidationMiddleware_Missing(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := SenderValidationMiddleware()(handler)

	env := NewEnvelope("test", 1)
	// No sender set

	_, err := validated.HandleEnvelope(context.Background(), env)
	if !errors.Is(err, auth.ErrSenderMissing) {
		t.Errorf("expected ErrSenderMissing, got %v", err)
	}
}

func TestSenderValidationMiddleware_Invalid(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := SenderValidationMiddleware()(handler)

	env := NewEnvelope("test", 1)
	env.SenderDid = "invalid-did-format"

	_, err := validated.HandleEnvelope(context.Background(), env)
	if err == nil {
		t.Error("expected error for invalid DID")
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		// Check that context has deadline
		if _, ok := ctx.Deadline(); !ok {
			return nil, errors.New("expected deadline in context")
		}
		return req, nil
	})

	timed := TimeoutMiddleware(5 * time.Second)(handler)

	env := NewEnvelope("test", 1)
	_, err := timed.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestContextValueMiddleware(t *testing.T) {
	type ctxKey string
	key := ctxKey("test-key")

	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		value := ctx.Value(key)
		if value != "test-value" {
			return nil, errors.New("expected value in context")
		}
		return req, nil
	})

	valued := ContextValueMiddleware(key, "test-value")(handler)

	env := NewEnvelope("test", 1)
	_, err := valued.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSignatureValidationMiddleware_Valid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	recipientDID := "did:key:recipient"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := SignatureValidationMiddleware(resolver)(handler)

	// Create and sign envelope
	env := NewEnvelope("test-trace", 1)
	env.SenderDid = senderDID
	env.RecipientDid = recipientDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = 1

	// Sign using the same transcript format as the middleware
	transcript := buildEnvelopeTranscript(env)
	env.Signature = ed25519.Sign(priv, transcript)

	_, err := validated.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSignatureValidationMiddleware_Missing(t *testing.T) {
	resolver := auth.NewMemoryDIDResolver()

	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := SignatureValidationMiddleware(resolver)(handler)

	env := NewEnvelope("test", 1)
	env.SenderDid = "did:key:sender"
	// No signature

	_, err := validated.HandleEnvelope(context.Background(), env)
	if !errors.Is(err, auth.ErrSignatureMissing) {
		t.Errorf("expected ErrSignatureMissing, got %v", err)
	}
}

func TestSignatureValidationMiddleware_Invalid(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		return req, nil
	})

	validated := SignatureValidationMiddleware(resolver)(handler)

	env := NewEnvelope("test", 1)
	env.SenderDid = senderDID
	env.RecipientDid = "did:key:recipient"
	env.TimestampMs = auth.NowMs()
	env.Signature = make([]byte, 64) // Invalid signature (zeros)

	_, err := validated.HandleEnvelope(context.Background(), env)
	if err == nil {
		t.Error("expected error for invalid signature")
	}
	if !strings.Contains(err.Error(), "verification") {
		t.Errorf("expected verification error, got: %v", err)
	}
}

func TestSecureBaselineMiddleware(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	senderDID := "did:key:sender"
	localDID := "did:key:local"

	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, pub)

	protector, _ := auth.NewReplayProtector(5*time.Minute, 100, nil)

	handlerCalled := false
	handler := HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
		handlerCalled = true
		return req, nil
	})

	secured := SecureBaselineMiddleware(localDID, resolver, protector)(handler)

	// Create valid envelope
	env := NewEnvelope("test-trace", 1)
	env.SenderDid = senderDID
	env.RecipientDid = localDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = 1

	// Sign using the same transcript format as the middleware
	transcript := buildEnvelopeTranscript(env)
	env.Signature = ed25519.Sign(priv, transcript)

	_, err := secured.HandleEnvelope(context.Background(), env)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}
