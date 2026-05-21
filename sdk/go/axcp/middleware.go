package axcp

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

// Middleware wraps a Handler to add cross-cutting functionality.
type Middleware func(Handler) Handler

// Chain combines multiple middlewares into a single middleware.
// Middlewares are applied in order, so Chain(A, B, C)(handler)
// results in A(B(C(handler))).
func Chain(middlewares ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// RecoveryMiddleware recovers from panics in handlers and returns an error.
func RecoveryMiddleware() Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (resp *Envelope, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("%w: %v", ErrHandlerPanic, r)
					resp = nil
				}
			}()
			return next.HandleEnvelope(ctx, req)
		})
	}
}

// Logger defines the interface for middleware logging.
type Logger interface {
	Printf(format string, v ...interface{})
}

// defaultLogger wraps the standard log package.
type defaultLogger struct{}

func (l defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// LoggingMiddleware logs incoming requests and responses.
func LoggingMiddleware(logger Logger) Middleware {
	if logger == nil {
		logger = defaultLogger{}
	}
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			start := time.Now()
			traceID := req.GetTraceId()

			logger.Printf("[AXCP] --> %s sender=%s", traceID, req.GetSenderDid())

			resp, err := next.HandleEnvelope(ctx, req)

			duration := time.Since(start)
			if err != nil {
				logger.Printf("[AXCP] <-- %s error=%v duration=%v", traceID, err, duration)
			} else {
				logger.Printf("[AXCP] <-- %s ok duration=%v", traceID, duration)
			}

			return resp, err
		})
	}
}

// TimestampValidationMiddleware validates message timestamps.
func TimestampValidationMiddleware(validator *auth.TimestampValidator) Middleware {
	if validator == nil {
		validator = auth.DefaultTimestampValidator()
	}
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			ts := req.GetTimestampMs()
			if err := validator.Validate(ts); err != nil {
				return nil, fmt.Errorf("timestamp validation failed: %w", err)
			}
			return next.HandleEnvelope(ctx, req)
		})
	}
}

// ReplayProtectionMiddleware checks for replay attacks.
func ReplayProtectionMiddleware(protector *auth.ReplayProtector) Middleware {
	if protector == nil {
		// Use default protector if none provided
		var err error
		protector, err = auth.NewReplayProtector(5*time.Minute, 1000, nil)
		if err != nil {
			panic(fmt.Sprintf("failed to create default replay protector: %v", err))
		}
	}
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			peerID := req.GetSenderDid()
			seq := req.GetSequence()

			if err := protector.CheckAndMark(peerID, seq); err != nil {
				return nil, fmt.Errorf("replay protection failed: %w", err)
			}

			return next.HandleEnvelope(ctx, req)
		})
	}
}

// RecipientValidationMiddleware validates that the message is for this recipient.
func RecipientValidationMiddleware(localDID string) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			recipientDID := req.GetRecipientDid()

			if recipientDID == "" {
				return nil, auth.ErrRecipientMissing
			}
			if recipientDID != localDID {
				return nil, auth.ErrRecipientMismatch
			}

			return next.HandleEnvelope(ctx, req)
		})
	}
}

// SenderValidationMiddleware validates that the sender DID is valid.
func SenderValidationMiddleware() Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			senderDID := req.GetSenderDid()

			if senderDID == "" {
				return nil, auth.ErrSenderMissing
			}

			if _, err := auth.ParseDID(senderDID); err != nil {
				return nil, fmt.Errorf("invalid sender DID: %w", err)
			}

			return next.HandleEnvelope(ctx, req)
		})
	}
}

// SignatureValidationMiddleware validates the message signature.
// This requires a DID resolver to look up public keys.
func SignatureValidationMiddleware(resolver auth.DIDResolver) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			senderDID := req.GetSenderDid()
			signature := req.GetSignature()

			if len(signature) == 0 {
				return nil, auth.ErrSignatureMissing
			}

			// Resolve the sender's public key
			publicKey, err := auth.ResolveEd25519PublicKey(ctx, resolver, senderDID)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve sender public key: %w", err)
			}

			transcript, err := BuildAuthTranscript(req)
			if err != nil {
				return nil, fmt.Errorf("failed to build auth transcript: %w", err)
			}

			if !auth.VerifyEd25519(publicKey, transcript, signature) {
				return nil, auth.ErrVerificationFailed
			}

			return next.HandleEnvelope(ctx, req)
		})
	}
}

// buildEnvelopeTranscript builds the canonical auth transcript for an envelope.
func buildEnvelopeTranscript(env *Envelope) []byte {
	transcript, err := BuildAuthTranscript(env)
	if err != nil {
		return nil
	}
	return transcript
}

// TimeoutMiddleware adds a timeout to handler execution.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return next.HandleEnvelope(ctx, req)
		})
	}
}

// ContextValueMiddleware adds a value to the context for downstream handlers.
func ContextValueMiddleware(key, value interface{}) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			ctx = context.WithValue(ctx, key, value)
			return next.HandleEnvelope(ctx, req)
		})
	}
}

// SecureBaselineMiddleware creates the standard middleware chain for secure-baseline-v1 profile.
// This includes all security validations required by the profile.
func SecureBaselineMiddleware(localDID string, resolver auth.DIDResolver, replayProtector *auth.ReplayProtector) Middleware {
	return Chain(
		RecoveryMiddleware(),
		RecipientValidationMiddleware(localDID),
		SenderValidationMiddleware(),
		TimestampValidationMiddleware(nil),
		SignatureValidationMiddleware(resolver),
		ReplayProtectionMiddleware(replayProtector),
	)
}
