package axcp

import (
	"context"
	"fmt"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

// ValidatorConfig holds configuration for the Validator.
type ValidatorConfig struct {
	// LocalDID is the local node's DID for recipient validation
	LocalDID string

	// Resolver resolves DIDs to public keys for signature verification
	Resolver auth.DIDResolver

	// TimestampValidator validates message timestamps (nil uses default)
	TimestampValidator *auth.TimestampValidator

	// ReplayProtector checks for replay attacks (nil disables replay protection)
	ReplayProtector *auth.ReplayProtector

	// SkipSignatureVerification skips signature verification (for testing only)
	SkipSignatureVerification bool
}

// Validator provides unified message validation for AXCP protocol.
// It wraps auth.EnvelopeValidator and integrates with the error framework.
type Validator struct {
	config  ValidatorConfig
	authVal *auth.EnvelopeValidator
}

// NewValidator creates a new Validator with the given configuration.
func NewValidator(config ValidatorConfig) *Validator {
	authVal := auth.NewEnvelopeValidator(config.Resolver, config.LocalDID)

	if config.TimestampValidator != nil {
		authVal.WithTimestampValidator(config.TimestampValidator)
	}

	if config.ReplayProtector != nil {
		authVal.WithReplayProtector(config.ReplayProtector)
	}

	return &Validator{
		config:  config,
		authVal: authVal,
	}
}

// ValidateEnvelope validates an envelope without signature verification.
// Returns a ProtocolError if validation fails.
func (v *Validator) ValidateEnvelope(ctx context.Context, env *Envelope) *ProtocolError {
	if env == nil {
		return ErrMalformedRequest.WithReason("envelope is nil")
	}

	// Use auth validator for basic validation
	if err := v.authVal.ValidateEnvelope(ctx, env); err != nil {
		return ToProtocolError(err)
	}

	return nil
}

// ValidateAndVerify validates an envelope including signature verification.
// Returns a ProtocolError if validation fails.
func (v *Validator) ValidateAndVerify(ctx context.Context, env *Envelope) *ProtocolError {
	if env == nil {
		return ErrMalformedRequest.WithReason("envelope is nil")
	}

	// Skip signature verification if configured (testing only)
	if v.config.SkipSignatureVerification {
		// Do quick validation only (no DID resolution)
		return v.QuickValidate(env)
	}

	// Build transcript for signature verification
	transcript := buildEnvelopeTranscript(env)

	// Use auth validator for full validation with signature verification
	if err := v.authVal.ValidateAndVerifySignature(ctx, env, transcript); err != nil {
		return ToProtocolError(err)
	}

	return nil
}

// QuickValidate performs fast validation without DID resolution.
// Returns a ProtocolError if validation fails.
func (v *Validator) QuickValidate(env *Envelope) *ProtocolError {
	if env == nil {
		return ErrMalformedRequest.WithReason("envelope is nil")
	}

	if err := auth.QuickValidate(env, v.config.LocalDID); err != nil {
		return ToProtocolError(err)
	}

	return nil
}

// Middleware returns a middleware that validates envelopes using this validator.
// If validation fails, it returns an error response envelope.
func (v *Validator) Middleware() Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			// Validate and verify the envelope
			if perr := v.ValidateAndVerify(ctx, req); perr != nil {
				return nil, perr
			}

			return next.HandleEnvelope(ctx, req)
		})
	}
}

// QuickRejectMiddleware returns a middleware that does quick validation.
// Use this for early rejection of obviously invalid messages.
func (v *Validator) QuickRejectMiddleware() Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			if perr := v.QuickValidate(req); perr != nil {
				return nil, perr
			}

			return next.HandleEnvelope(ctx, req)
		})
	}
}

// ValidationResult contains the result of envelope validation.
type ValidationResult struct {
	// Valid is true if validation passed
	Valid bool

	// Error contains the validation error if Valid is false
	Error *ProtocolError

	// SenderPublicKey is the sender's resolved public key (if signature was verified)
	SenderPublicKey []byte
}

// ValidateWithResult validates an envelope and returns detailed results.
func (v *Validator) ValidateWithResult(ctx context.Context, env *Envelope) ValidationResult {
	if env == nil {
		return ValidationResult{
			Valid: false,
			Error: ErrMalformedRequest.WithReason("envelope is nil"),
		}
	}

	// Quick validation first
	if perr := v.QuickValidate(env); perr != nil {
		return ValidationResult{
			Valid: false,
			Error: perr,
		}
	}

	// Skip signature verification if configured
	if v.config.SkipSignatureVerification {
		return ValidationResult{Valid: true}
	}

	// Resolve public key
	publicKey, err := auth.ResolveEd25519PublicKey(ctx, v.config.Resolver, env.GetSenderDid())
	if err != nil {
		return ValidationResult{
			Valid: false,
			Error: ToProtocolError(err),
		}
	}

	// Verify signature
	transcript := buildEnvelopeTranscript(env)
	if !auth.VerifyEd25519(publicKey, transcript, env.GetSignature()) {
		return ValidationResult{
			Valid: false,
			Error: ErrAuthSignatureInvalid.WithReason("signature verification failed"),
		}
	}

	// Check replay (if configured)
	if v.config.ReplayProtector != nil {
		if err := v.config.ReplayProtector.CheckAndMark(env.GetSenderDid(), env.GetSequence()); err != nil {
			return ValidationResult{
				Valid: false,
				Error: ToProtocolError(err),
			}
		}
	}

	return ValidationResult{
		Valid:           true,
		SenderPublicKey: publicKey,
	}
}

// ErrorResponseMiddleware wraps a handler to convert errors to error envelopes.
// This middleware should be placed early in the chain to catch errors from inner handlers.
func ErrorResponseMiddleware(localDID string) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, req *Envelope) (*Envelope, error) {
			resp, err := next.HandleEnvelope(ctx, req)
			if err != nil {
				// Convert error to ProtocolError
				perr := ToProtocolError(err)

				// Create error envelope as response
				errEnv := NewErrorEnvelope(
					req.GetTraceId(),
					localDID,
					req.GetSenderDid(),
					perr,
				)

				return errEnv, nil
			}

			return resp, nil
		})
	}
}

// SecureValidatorMiddleware creates the complete validation middleware chain.
// This is the recommended way to add validation to handlers.
func SecureValidatorMiddleware(config ValidatorConfig) Middleware {
	validator := NewValidator(config)

	return Chain(
		RecoveryMiddleware(),
		ErrorResponseMiddleware(config.LocalDID),
		validator.Middleware(),
	)
}

// ValidatorOption is a function that configures a ValidatorConfig.
type ValidatorOption func(*ValidatorConfig)

// WithLocalDID sets the local DID for recipient validation.
func WithLocalDID(did string) ValidatorOption {
	return func(c *ValidatorConfig) {
		c.LocalDID = did
	}
}

// WithResolver sets the DID resolver for public key resolution.
func WithResolver(resolver auth.DIDResolver) ValidatorOption {
	return func(c *ValidatorConfig) {
		c.Resolver = resolver
	}
}

// WithTimestampValidation sets a custom timestamp validator.
func WithTimestampValidation(tv *auth.TimestampValidator) ValidatorOption {
	return func(c *ValidatorConfig) {
		c.TimestampValidator = tv
	}
}

// WithReplayProtection enables replay protection with the given protector.
func WithReplayProtection(rp *auth.ReplayProtector) ValidatorOption {
	return func(c *ValidatorConfig) {
		c.ReplayProtector = rp
	}
}

// NewValidatorWithOptions creates a Validator with functional options.
func NewValidatorWithOptions(opts ...ValidatorOption) *Validator {
	config := ValidatorConfig{}
	for _, opt := range opts {
		opt(&config)
	}
	return NewValidator(config)
}

// ValidateRequest is a convenience function for one-off validation.
// For repeated validation, create a Validator instance instead.
func ValidateRequest(ctx context.Context, env *Envelope, localDID string, resolver auth.DIDResolver) *ProtocolError {
	validator := NewValidator(ValidatorConfig{
		LocalDID: localDID,
		Resolver: resolver,
	})
	return validator.ValidateAndVerify(ctx, env)
}

// MustValidate validates an envelope and panics if validation fails.
// Use only in tests or initialization code.
func MustValidate(ctx context.Context, env *Envelope, localDID string, resolver auth.DIDResolver) {
	if perr := ValidateRequest(ctx, env, localDID, resolver); perr != nil {
		panic(fmt.Sprintf("validation failed: %v", perr))
	}
}
