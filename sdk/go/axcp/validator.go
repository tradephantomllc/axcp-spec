package axcp

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"sync"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
)

// ValidatorConfig holds configuration for the Validator.
type ValidatorConfig struct {
	// LocalDID is the local node's DID for recipient validation
	LocalDID string

	// Resolver resolves DIDs to public keys for signature verification
	Resolver auth.DIDResolver

	// TimestampValidator validates message timestamps (nil uses the default policy)
	TimestampValidator *auth.TimestampValidator

	// ReplayProtector checks for replay attacks after signature verification.
	// Nil selects a new default protector owned by this Validator; share one
	// protector between validators that accept messages from the same senders.
	// Its TTL is raised to at least the timestamp max age plus clock skew.
	ReplayProtector *auth.ReplayProtector

	// DisableReplayProtection turns replay protection off. Testing only.
	DisableReplayProtection bool

	// AllowedDIDs restricts accepted senders (empty = any resolvable sender).
	AllowedDIDs []string

	// AllowedProfiles lists accepted envelope profile numbers
	// (empty = secure-baseline-v1 only).
	AllowedProfiles []uint32

	// SkipSignatureVerification skips signature verification and replay
	// protection. Testing only: unauthenticated messages are accepted.
	SkipSignatureVerification bool
}

// Validator provides unified message validation for AXCP protocol.
// It wraps auth.EnvelopeValidator and integrates with the error framework.
type Validator struct {
	config   ValidatorConfig
	authVal  *auth.EnvelopeValidator
	allowed  map[string]struct{}
	profiles map[uint32]struct{}
}

// NewValidator creates a new Validator with the given configuration.
//
// Replay protection and timestamp validation are enabled by default.
func NewValidator(config ValidatorConfig) *Validator {
	authVal := auth.NewEnvelopeValidator(config.Resolver, config.LocalDID)

	if config.TimestampValidator != nil {
		authVal.WithTimestampValidator(config.TimestampValidator)
	}

	switch {
	case config.DisableReplayProtection:
		authVal.ReplayProtector = nil
	case config.ReplayProtector != nil:
		authVal.WithReplayProtector(config.ReplayProtector)
	}

	v := &Validator{
		config:   config,
		authVal:  authVal,
		profiles: map[uint32]struct{}{},
	}
	if len(config.AllowedDIDs) > 0 {
		v.allowed = make(map[string]struct{}, len(config.AllowedDIDs))
		for _, did := range config.AllowedDIDs {
			v.allowed[did] = struct{}{}
		}
	}
	if len(config.AllowedProfiles) == 0 {
		v.profiles[ProfileSecureBaselineNumber] = struct{}{}
	}
	for _, p := range config.AllowedProfiles {
		v.profiles[p] = struct{}{}
	}
	return v
}

// ReplayProtector returns the replay protector in use (nil when disabled).
func (v *Validator) ReplayProtector() *auth.ReplayProtector {
	return v.authVal.ReplayProtector
}

// checkEnvelope runs the structural and authorization checks shared by all
// validation paths.
func (v *Validator) checkEnvelope(env *Envelope) *ProtocolError {
	if env == nil {
		return ErrMalformedRequest.WithReason("envelope is nil")
	}
	if err := ValidateEnvelopeStructure(env); err != nil {
		return ToProtocolError(err)
	}
	if _, ok := v.profiles[env.GetProfile()]; !ok {
		return ErrProfileUnsupported.WithReason("envelope profile is not accepted")
	}
	if err := strictMarshalCheck(&env.AxcpEnvelope); err != nil {
		return ToProtocolError(err)
	}
	if v.allowed != nil {
		if _, ok := v.allowed[env.GetSenderDid()]; !ok {
			return ErrUnauthorized.WithReason("sender is not allowed")
		}
	}
	return nil
}

// ValidateEnvelope performs non-mutating preflight validation without signature verification.
// Replay protection is intentionally not applied here because replay state must only
// be mutated after successful signature verification.
// Returns a ProtocolError if validation fails.
func (v *Validator) ValidateEnvelope(ctx context.Context, env *Envelope) *ProtocolError {
	if perr := v.checkEnvelope(env); perr != nil {
		return perr
	}

	if err := v.authVal.ValidateEnvelope(ctx, env); err != nil {
		return ToProtocolError(err)
	}

	return nil
}

// ValidateAndVerify validates an envelope including signature verification
// and, after the signature is verified, replay protection.
// Returns a ProtocolError if validation fails.
func (v *Validator) ValidateAndVerify(ctx context.Context, env *Envelope) *ProtocolError {
	if perr := v.checkEnvelope(env); perr != nil {
		return perr
	}

	// Skip signature verification if configured (testing only)
	if v.config.SkipSignatureVerification {
		return v.quickValidate(env)
	}

	transcript, err := BuildAuthTranscript(env)
	if err != nil {
		return ErrMalformedRequest.WithReason("cannot build authentication transcript")
	}

	if err := v.authVal.ValidateAndVerifySignature(ctx, env, transcript); err != nil {
		return ToProtocolError(err)
	}

	return nil
}

// QuickValidate performs fast validation without DID resolution.
// Returns a ProtocolError if validation fails.
func (v *Validator) QuickValidate(env *Envelope) *ProtocolError {
	if perr := v.checkEnvelope(env); perr != nil {
		return perr
	}
	return v.quickValidate(env)
}

func (v *Validator) quickValidate(env *Envelope) *ProtocolError {
	if err := auth.QuickValidateWithTimestamp(env, v.config.LocalDID, v.authVal.TimestampValidator); err != nil {
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

// ValidateWithResult validates an envelope exactly like ValidateAndVerify and
// returns detailed results.
func (v *Validator) ValidateWithResult(ctx context.Context, env *Envelope) ValidationResult {
	if perr := v.ValidateAndVerify(ctx, env); perr != nil {
		return ValidationResult{Valid: false, Error: perr}
	}
	if v.config.SkipSignatureVerification {
		return ValidationResult{Valid: true}
	}

	publicKey, err := auth.ResolveEd25519PublicKey(ctx, v.config.Resolver, env.GetSenderDid())
	if err != nil {
		return ValidationResult{Valid: false, Error: ToProtocolError(err)}
	}
	return ValidationResult{Valid: true, SenderPublicKey: ed25519.PublicKey(publicKey)}
}

// ErrorResponseMiddleware wraps a handler to convert errors to error envelopes.
// This middleware should be placed early in the chain to catch errors from inner handlers.
//
// Error envelopes are not signed: receivers must treat them as advisory.
// Internal error details are never copied into the response.
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

// WithReplayProtection sets the replay protector.
func WithReplayProtection(rp *auth.ReplayProtector) ValidatorOption {
	return func(c *ValidatorConfig) {
		c.ReplayProtector = rp
	}
}

// WithAllowedDIDs restricts accepted senders.
func WithAllowedDIDs(dids ...string) ValidatorOption {
	return func(c *ValidatorConfig) {
		c.AllowedDIDs = append(c.AllowedDIDs, dids...)
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

var (
	sharedReplayOnce sync.Once
	sharedReplay     *auth.ReplayProtector
)

// SharedReplayProtector returns a process-wide default replay protector.
// It is used where callers do not supply one (ValidateRequest,
// ReplayProtectionMiddleware(nil), SecureBaselineMiddleware with nil), so that
// replay protection still spans connections and calls.
func SharedReplayProtector() *auth.ReplayProtector {
	sharedReplayOnce.Do(func() {
		sharedReplay = auth.NewDefaultReplayProtector()
		sharedReplay.EnsureMinTTL(auth.DefaultMaxTimestampAge + auth.DefaultClockSkew)
	})
	return sharedReplay
}

// ValidateRequest is a convenience function for one-off validation.
// It uses the process-wide SharedReplayProtector. For repeated validation,
// create a Validator instance instead.
func ValidateRequest(ctx context.Context, env *Envelope, localDID string, resolver auth.DIDResolver) *ProtocolError {
	validator := NewValidator(ValidatorConfig{
		LocalDID:        localDID,
		Resolver:        resolver,
		ReplayProtector: SharedReplayProtector(),
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
