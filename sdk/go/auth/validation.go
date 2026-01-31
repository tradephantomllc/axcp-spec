// Package auth provides message validation for AXCP protocol.
package auth

import (
	"context"
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

// ValidateSignaturePresent checks that the envelope has a signature.
//
// Returns nil if signature is present, ErrSignatureMissing if empty.
func ValidateSignaturePresent(env EnvelopeFields) error {
	if env == nil {
		return ErrEnvelopeNil
	}

	if len(env.GetSignature()) == 0 {
		return ErrSignatureMissing
	}

	return nil
}

// EnvelopeValidator provides comprehensive envelope validation.
type EnvelopeValidator struct {
	// Resolver resolves DIDs to public keys
	Resolver DIDResolver

	// TimestampValidator validates message timestamps
	TimestampValidator *TimestampValidator

	// ReplayProtector checks for replay attacks (optional)
	ReplayProtector *ReplayProtector

	// LocalDID is this node's DID (used for recipient validation)
	LocalDID string
}

// NewEnvelopeValidator creates a new EnvelopeValidator with the given configuration.
func NewEnvelopeValidator(resolver DIDResolver, localDID string) *EnvelopeValidator {
	return &EnvelopeValidator{
		Resolver:           resolver,
		TimestampValidator: DefaultTimestampValidator(),
		LocalDID:           localDID,
	}
}

// WithTimestampValidator sets a custom timestamp validator.
func (v *EnvelopeValidator) WithTimestampValidator(tv *TimestampValidator) *EnvelopeValidator {
	v.TimestampValidator = tv
	return v
}

// WithReplayProtector sets a replay protector for replay attack detection.
func (v *EnvelopeValidator) WithReplayProtector(rp *ReplayProtector) *EnvelopeValidator {
	v.ReplayProtector = rp
	return v
}

// ValidateEnvelope performs comprehensive validation of an incoming envelope.
//
// Validation steps (in order):
//  1. Check envelope is not nil
//  2. Validate recipient matches LocalDID
//  3. Validate sender DID format
//  4. Validate timestamp is within acceptable window
//  5. Validate signature is present
//  6. Resolve sender's public key
//  7. Verify signature (requires transcript)
//  8. Check replay protection (if configured)
//
// Note: This method does NOT verify the signature because it doesn't have
// access to the full transcript. Use ValidateAndVerify for complete validation.
func (v *EnvelopeValidator) ValidateEnvelope(ctx context.Context, env EnvelopeFields) error {
	// 1. Check envelope not nil
	if env == nil {
		return ErrEnvelopeNil
	}

	// 2. Validate recipient
	if err := ValidateRecipient(env, v.LocalDID); err != nil {
		return err
	}

	// 3. Validate sender
	if err := ValidateSender(env); err != nil {
		return err
	}

	// 4. Validate timestamp
	if v.TimestampValidator != nil {
		if err := v.TimestampValidator.Validate(env.GetTimestampMs()); err != nil {
			return err
		}
	}

	// 5. Validate signature present
	if err := ValidateSignaturePresent(env); err != nil {
		return err
	}

	// 6. Resolve sender's public key (validates DID is resolvable)
	_, err := ResolveEd25519PublicKey(ctx, v.Resolver, env.GetSenderDid())
	if err != nil {
		return err
	}

	// 7. Replay protection (if configured)
	if v.ReplayProtector != nil {
		peerID := env.GetSenderDid()
		seq := env.GetSequence()
		if err := v.ReplayProtector.CheckAndMark(peerID, seq); err != nil {
			return err
		}
	}

	return nil
}

// ValidateAndVerifySignature validates the envelope and verifies the signature.
//
// This is the complete validation flow:
//  1. All basic validations from ValidateEnvelope
//  2. Verify Ed25519 signature over the provided transcript
//
// The transcript should be built using BuildDIDAuthTranscript or similar
// to ensure proper binding of the signature to the message context.
func (v *EnvelopeValidator) ValidateAndVerifySignature(ctx context.Context, env EnvelopeFields, transcript []byte) error {
	// Run basic validations (except replay - we do it after signature verification)
	if env == nil {
		return ErrEnvelopeNil
	}

	if err := ValidateRecipient(env, v.LocalDID); err != nil {
		return err
	}

	if err := ValidateSender(env); err != nil {
		return err
	}

	if v.TimestampValidator != nil {
		if err := v.TimestampValidator.Validate(env.GetTimestampMs()); err != nil {
			return err
		}
	}

	if err := ValidateSignaturePresent(env); err != nil {
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
		peerID := env.GetSenderDid()
		seq := env.GetSequence()
		if err := v.ReplayProtector.CheckAndMark(peerID, seq); err != nil {
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
//   - Timestamp valid
//   - Signature present
func QuickValidate(env EnvelopeFields, expectedRecipient string) error {
	if env == nil {
		return ErrEnvelopeNil
	}

	if err := ValidateRecipient(env, expectedRecipient); err != nil {
		return err
	}

	if err := ValidateSender(env); err != nil {
		return err
	}

	if err := ValidateTimestamp(env.GetTimestampMs()); err != nil {
		return err
	}

	if err := ValidateSignaturePresent(env); err != nil {
		return err
	}

	return nil
}
