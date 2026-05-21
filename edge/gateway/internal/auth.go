// Package internal provides gateway authentication verification.
package internal

import (
	"context"
	"errors"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/negotiate"
)

// Auth verification errors
var (
	// ErrAuthRequired indicates the profile requires authentication but envelope is missing auth fields
	ErrAuthRequired = errors.New("auth: authentication required for this profile")

	// ErrMissingSignature indicates the envelope signature is empty
	ErrMissingSignature = errors.New("auth: envelope signature is empty")

	// ErrMissingSenderDID indicates the sender_did field is empty
	ErrMissingSenderDID = errors.New("auth: sender DID is empty")

	// ErrTimestampTooOld indicates the message timestamp is outside the acceptable window
	ErrTimestampTooOld = errors.New("auth: timestamp too old")

	// ErrTimestampInFuture indicates the message timestamp is in the future
	ErrTimestampInFuture = errors.New("auth: timestamp in the future")

	// ErrRecipientDIDMismatch indicates the envelope targets a different gateway DID
	ErrRecipientDIDMismatch = errors.New("auth: recipient DID does not match gateway DID")
)

// AuthConfig holds configuration for envelope authentication.
type AuthConfig struct {
	// MaxTimestampAge is the maximum age of a message timestamp (default: 5 minutes)
	MaxTimestampAge time.Duration

	// MaxTimestampFuture is the maximum time a timestamp can be in the future (default: 30 seconds)
	MaxTimestampFuture time.Duration

	// ReplayTTL is the TTL for replay protection (default: 10 minutes)
	ReplayTTL time.Duration

	// ReplayWindow is the sliding window size for sequence-based replay protection
	ReplayWindow uint64
}

// DefaultAuthConfig returns the default authentication configuration.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		MaxTimestampAge:    5 * time.Minute,
		MaxTimestampFuture: 30 * time.Second,
		ReplayTTL:          10 * time.Minute,
		ReplayWindow:       1000,
	}
}

// EnvelopeAuthenticator provides authentication verification for AXCP envelopes.
// It uses the auth package components (DID resolver, Ed25519 verifier, replay protection).
type EnvelopeAuthenticator struct {
	resolver auth.DIDResolver
	replay   *auth.ReplayProtector
	config   AuthConfig
	clock    auth.Clock

	// serverDID is this gateway's DID for transcript binding
	serverDID string
}

// NewEnvelopeAuthenticator creates a new envelope authenticator.
//
// Parameters:
//   - resolver: DID resolver for looking up sender public keys (must be non-nil for Secure Baseline)
//   - serverDID: this gateway's DID for transcript binding
//   - config: authentication configuration
//   - clock: time source (if nil, uses system clock)
func NewEnvelopeAuthenticator(resolver auth.DIDResolver, serverDID string, config AuthConfig, clock auth.Clock) (*EnvelopeAuthenticator, error) {
	if clock == nil {
		clock = auth.RealClock{}
	}

	replay, err := auth.NewReplayProtector(config.ReplayTTL, config.ReplayWindow, clock)
	if err != nil {
		return nil, err
	}

	return &EnvelopeAuthenticator{
		resolver:  resolver,
		replay:    replay,
		config:    config,
		clock:     clock,
		serverDID: serverDID,
	}, nil
}

// EnvelopeAuth contains the authentication fields from an AXCP envelope.
type EnvelopeAuth struct {
	SenderDID    string
	RecipientDID string
	TimestampMs  uint64
	Sequence     uint64
	Signature    []byte
	PayloadBytes []byte // The serialized payload for signature verification
}

// AuthResult contains the result of authentication verification.
type AuthResult struct {
	// Authenticated is true if the envelope passed all auth checks
	Authenticated bool

	// SenderDID is the verified sender DID
	SenderDID string

	// Error is set if authentication failed
	Error error

	// ErrorCode is the AXCP error code for the failure (0 if success)
	ErrorCode uint32
}

// Error code constants matching proto ErrorCode enum
const (
	ErrorCodeAuthSignatureInvalid = 10
	ErrorCodeAuthReplayDetected   = 11
	ErrorCodeAuthDIDInvalid       = 17
	ErrorCodeAuthTimestampExpired = 18
)

// VerifyEnvelope verifies the authentication of an AXCP envelope.
//
// For Secure Baseline profile:
//  1. Validates sender_did is present and parseable
//  2. Validates timestamp is within acceptable window
//  3. Checks replay protection (sequence number)
//  4. Resolves sender's public key via DID resolver
//  5. Verifies Ed25519 signature over the auth transcript
//
// Returns AuthResult with Authenticated=true on success, or with Error and ErrorCode on failure.
func (ea *EnvelopeAuthenticator) VerifyEnvelope(ctx context.Context, profile negotiate.Profile, envAuth EnvelopeAuth) AuthResult {
	// Check if profile requires authentication
	if !negotiate.IsSecureProfile(profile) {
		// Transport-only profile doesn't require app-layer auth
		return AuthResult{Authenticated: true}
	}

	// Validate required fields
	if envAuth.SenderDID == "" {
		return AuthResult{
			Error:     ErrMissingSenderDID,
			ErrorCode: ErrorCodeAuthDIDInvalid,
		}
	}

	if len(envAuth.Signature) == 0 {
		return AuthResult{
			Error:     ErrMissingSignature,
			ErrorCode: ErrorCodeAuthSignatureInvalid,
		}
	}

	if ea.serverDID != "" && envAuth.RecipientDID != "" && envAuth.RecipientDID != ea.serverDID {
		return AuthResult{
			Error:     ErrRecipientDIDMismatch,
			ErrorCode: ErrorCodeAuthDIDInvalid,
		}
	}

	// Validate timestamp
	if err := ea.validateTimestamp(envAuth.TimestampMs); err != nil {
		return AuthResult{
			Error:     err,
			ErrorCode: ErrorCodeAuthTimestampExpired,
		}
	}

	// Check replay protection
	if err := ea.replay.CheckAndMark(envAuth.SenderDID, envAuth.Sequence); err != nil {
		code := ErrorCodeAuthReplayDetected
		if errors.Is(err, auth.ErrTooOld) {
			code = ErrorCodeAuthTimestampExpired // Sequence too old is similar to timestamp expired
		}
		return AuthResult{
			Error:     err,
			ErrorCode: uint32(code),
		}
	}

	// Build auth transcript for signature verification
	timestamp := time.UnixMilli(int64(envAuth.TimestampMs))
	recipientDID := envAuth.RecipientDID
	if recipientDID == "" {
		recipientDID = ea.serverDID
	}
	transcript := auth.BuildDIDAuthTranscript(envAuth.SenderDID, recipientDID, envAuth.PayloadBytes, timestamp)

	// Verify signature via DID resolver
	if ea.resolver == nil {
		return AuthResult{
			Error:     errors.New("auth: DID resolver not configured"),
			ErrorCode: ErrorCodeAuthDIDInvalid,
		}
	}

	if err := auth.VerifyDIDAuthSignature(ctx, ea.resolver, envAuth.SenderDID, transcript, envAuth.Signature); err != nil {
		code := ErrorCodeAuthSignatureInvalid
		if errors.Is(err, auth.ErrDIDNotFound) || errors.Is(err, auth.ErrInvalidDIDFormat) {
			code = ErrorCodeAuthDIDInvalid
		}
		return AuthResult{
			Error:     err,
			ErrorCode: uint32(code),
		}
	}

	return AuthResult{
		Authenticated: true,
		SenderDID:     envAuth.SenderDID,
	}
}

// validateTimestamp checks if the timestamp is within acceptable bounds.
func (ea *EnvelopeAuthenticator) validateTimestamp(timestampMs uint64) error {
	now := ea.clock.Now()
	msgTime := time.UnixMilli(int64(timestampMs))

	// Check if too old
	if now.Sub(msgTime) > ea.config.MaxTimestampAge {
		return ErrTimestampTooOld
	}

	// Check if too far in future
	if msgTime.Sub(now) > ea.config.MaxTimestampFuture {
		return ErrTimestampInFuture
	}

	return nil
}

// Reset clears all replay protection state.
func (ea *EnvelopeAuthenticator) Reset() {
	ea.replay.Reset()
}

// ResetPeer clears replay state for a specific peer.
func (ea *EnvelopeAuthenticator) ResetPeer(peerDID string) {
	ea.replay.ResetPeer(peerDID)
}
