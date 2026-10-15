// Package auth provides message validation for AXCP protocol.
package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
)

// Validation errors
var (
	// ErrRecipientMissing indicates the recipient_did field is empty
	ErrRecipientMissing = errors.New("auth: recipient DID is missing")

	// ErrRecipientMismatch indicates the message was not intended for this recipient
	ErrRecipientMismatch = errors.New("auth: message recipient does not match expected DID")

	// ErrSenderMissing indicates the sender_did field is empty
	ErrSenderMissing = errors.New("auth: sender DID is missing")

	// ErrSignatureMissing indicates the signature field is empty
	ErrSignatureMissing = errors.New("auth: signature is missing")

	// ErrEnvelopeNil indicates the envelope is nil
	ErrEnvelopeNil = errors.New("auth: envelope is nil")

	// ErrSignatureLength indicates a signature that is not 64 bytes
	ErrSignatureLength = errors.New("auth: signature must be 64 bytes")

	// ErrSequenceMissing indicates a zero sequence number
	ErrSequenceMissing = errors.New("auth: sequence must be greater than zero")
)

// EnvelopeFields contains the fields needed for envelope validation.
// This interface abstracts the protobuf envelope for easier testing
// and to avoid direct protobuf dependency in the auth package.
type EnvelopeFields interface {
	// GetSenderDid returns the sender's DID
	GetSenderDid() string

	// GetRecipientDid returns the recipient's DID
	GetRecipientDid() string

	// GetTimestampMs returns the timestamp in milliseconds since Unix epoch
	GetTimestampMs() uint64

	// GetSequence returns the message sequence number
	GetSequence() uint64

	// GetSignature returns the message signature
	GetSignature() []byte
}

// ValidateRecipient checks that the envelope's recipient matches the expected DID.
//
// Returns nil if the recipient matches, or an error if:
//   - The envelope is nil
//   - The recipient_did field is empty
//   - The recipient_did does not match the expected DID
func ValidateRecipient(env EnvelopeFields, expectedDID string) error {
	if env == nil {
		return ErrEnvelopeNil
	}

	recipientDID := env.GetRecipientDid()
	if recipientDID == "" {
		return ErrRecipientMissing
	}

	if recipientDID != expectedDID {
		return ErrRecipientMismatch
	}

	return nil
}

// ValidateSender checks that the envelope has a valid sender DID.
//
// Returns nil if the sender is valid, or an error if:
//   - The envelope is nil
//   - The sender_did field is empty
//   - The sender_did format is invalid
func ValidateSender(env EnvelopeFields) error {
	if env == nil {
		return ErrEnvelopeNil
	}

	senderDID := env.GetSenderDid()
	if senderDID == "" {
		return ErrSenderMissing
	}

	// Validate DID format
	_, err := ParseDID(senderDID)
	if err != nil {
		return err
	}

	return nil
}

// ValidateSignaturePresent checks that the envelope has a 64-byte signature.
//
// Returns nil if a signature of the right size is present, ErrSignatureMissing
// if empty and ErrSignatureLength for any other size.
func ValidateSignaturePresent(env EnvelopeFields) error {
	if env == nil {
		return ErrEnvelopeNil
	}

	switch n := len(env.GetSignature()); {
	case n == 0:
		return ErrSignatureMissing
	case n != ed25519.SignatureSize:
		return ErrSignatureLength
	}

	return nil
}

// ValidateSequence checks that the envelope carries a replay sequence (> 0).
func ValidateSequence(env EnvelopeFields) error {
	if env == nil {
		return ErrEnvelopeNil
	}
	if env.GetSequence() == 0 {
		return ErrSequenceMissing
	}
	return nil
}

// EnvelopeValidator provides comprehensive envelope validation.
type EnvelopeValidator struct {
	// Resolver resolves DIDs to public keys
	Resolver DIDResolver

	// TimestampValidator validates message timestamps (nil uses the default policy)
	TimestampValidator *TimestampValidator

	// ReplayProtector checks for replay attacks after signature verification.
	// NewEnvelopeValidator installs a default protector; set it to nil only in
	// tests or when replay protection is enforced elsewhere.
	ReplayProtector *ReplayProtector

	// LocalDID is this node's DID (used for recipient validation)
	LocalDID string
}

// NewEnvelopeValidator creates a new EnvelopeValidator with the default
// timestamp policy and a default replay protector.
func NewEnvelopeValidator(resolver DIDResolver, localDID string) *EnvelopeValidator {
	tv := DefaultTimestampValidator()
	rp := NewDefaultReplayProtector()
	rp.EnsureMinTTL(tv.MaxAge + tv.ClockSkew)
	return &EnvelopeValidator{
		Resolver:           resolver,
		TimestampValidator: tv,
		ReplayProtector:    rp,
		LocalDID:           localDID,
	}
}

// WithTimestampValidator sets a custom timestamp validator.
func (v *EnvelopeValidator) WithTimestampValidator(tv *TimestampValidator) *EnvelopeValidator {
	v.TimestampValidator = tv
	v.alignReplayTTL()
	return v
}

// WithReplayProtector sets the replay protector used after signature verification.
//
// Replay state is mutated only by validation paths that verify the envelope signature
// first, such as ValidateAndVerifySignature. ValidateEnvelope is intentionally
// non-mutating and does not call the replay protector. The protector's TTL is
// raised to at least the timestamp max age plus clock skew.
func (v *EnvelopeValidator) WithReplayProtector(rp *ReplayProtector) *EnvelopeValidator {
	v.ReplayProtector = rp
	v.alignReplayTTL()
	return v
}

func (v *EnvelopeValidator) timestampValidator() *TimestampValidator {
	if v.TimestampValidator == nil {
		return DefaultTimestampValidator()
	}
	return v.TimestampValidator
}

func (v *EnvelopeValidator) alignReplayTTL() {
	if v.ReplayProtector == nil {
		return
	}
	tv := v.timestampValidator()
	v.ReplayProtector.EnsureMinTTL(tv.MaxAge + tv.ClockSkew)
}

// preflight runs the checks shared by every validation path.
func (v *EnvelopeValidator) preflight(env EnvelopeFields) error {
	if env == nil {
		return ErrEnvelopeNil
	}
	if err := ValidateRecipient(env, v.LocalDID); err != nil {
		return err
	}
	if err := ValidateSender(env); err != nil {
		return err
	}
	if err := v.timestampValidator().Validate(env.GetTimestampMs()); err != nil {
		return err
	}
	if err := ValidateSignaturePresent(env); err != nil {
		return err
	}
	return ValidateSequence(env)
}

// ValidateEnvelope performs non-mutating preflight validation of an incoming envelope.
//
// Validation steps (in order):
//  1. Check envelope is not nil
//  2. Validate recipient matches LocalDID
//  3. Validate sender DID format
//  4. Validate timestamp is within acceptable window
//  5. Validate signature is present (64 bytes) and sequence is non-zero
//  6. Resolve sender's public key
//
// Note: This method does NOT verify the signature because it doesn't have
// access to the full transcript. It also does NOT mutate replay state. Use
// ValidateAndVerifySignature for complete validation with post-signature replay
// protection.
func (v *EnvelopeValidator) ValidateEnvelope(ctx context.Context, env EnvelopeFields) error {
	if err := v.preflight(env); err != nil {
		return err
	}

	// Resolve sender's public key (validates DID is resolvable)
	_, err := ResolveEd25519PublicKey(ctx, v.Resolver, env.GetSenderDid())
	return err
}

// ValidateAndVerifySignature validates the envelope and verifies the signature.
//
// This is the complete validation flow:
//  1. All non-mutating preflight validations from ValidateEnvelope
//  2. Verify Ed25519 signature over the provided transcript
//  3. Check replay protection, mutating replay state only after successful signature verification
//
// The transcript MUST be built from this same envelope. For AXCP envelopes use
// axcp.BuildAuthTranscript, which covers version, trace ID, profile, payload,
// sequence, sender, recipient and timestamp. A transcript that does not cover
// the sequence would let an attacker resend a signed message with a different
// sequence; prefer axcp.Validator, which builds the transcript itself.
func (v *EnvelopeValidator) ValidateAndVerifySignature(ctx context.Context, env EnvelopeFields, transcript []byte) error {
	if err := v.preflight(env); err != nil {
		return err
	}

	// Resolve public key and verify signature
	publicKey, err := ResolveEd25519PublicKey(ctx, v.Resolver, env.GetSenderDid())
	if err != nil {
		return err
	}

	if !VerifyEd25519(publicKey, transcript, env.GetSignature()) {
		return ErrVerificationFailed
	}

	// Replay protection AFTER signature verification
	// (to prevent DoS by flooding with invalid signatures)
	if v.ReplayProtector != nil {
		if err := v.ReplayProtector.CheckAndMark(env.GetSenderDid(), env.GetSequence()); err != nil {
			return err
		}
	}

	return nil
}

// QuickValidate performs a quick validation without DID resolution or signature verification.
// Useful for fast rejection of obviously invalid messages.
//
// Checks:
//   - Envelope not nil
//   - Recipient matches
//   - Sender present and valid format
//   - Timestamp valid (default policy)
//   - Signature present (64 bytes) and sequence non-zero
func QuickValidate(env EnvelopeFields, expectedRecipient string) error {
	return QuickValidateWithTimestamp(env, expectedRecipient, nil)
}

// QuickValidateWithTimestamp is QuickValidate with an explicit timestamp
// policy; a nil validator selects the default policy.
func QuickValidateWithTimestamp(env EnvelopeFields, expectedRecipient string, tv *TimestampValidator) error {
	v := EnvelopeValidator{LocalDID: expectedRecipient, TimestampValidator: tv}
	return v.preflight(env)
}
