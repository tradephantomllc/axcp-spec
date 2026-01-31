package axcp

import (
	"errors"
	"fmt"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"
)

// ProtocolError represents an AXCP protocol error with error code and details.
type ProtocolError struct {
	// Code is the protocol error code
	Code pb.ErrorCode

	// Reason is a human-readable error message
	Reason string

	// Diagnostics contains optional diagnostic data (e.g., stack trace, context)
	Diagnostics []byte
}

// Error implements the error interface.
func (e *ProtocolError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("axcp: %s: %s", e.Code.String(), e.Reason)
	}
	return fmt.Sprintf("axcp: %s", e.Code.String())
}

// WithReason returns a copy of the error with the given reason.
func (e *ProtocolError) WithReason(reason string) *ProtocolError {
	return &ProtocolError{
		Code:        e.Code,
		Reason:      reason,
		Diagnostics: e.Diagnostics,
	}
}

// WithDiagnostics returns a copy of the error with the given diagnostics.
func (e *ProtocolError) WithDiagnostics(data []byte) *ProtocolError {
	return &ProtocolError{
		Code:        e.Code,
		Reason:      e.Reason,
		Diagnostics: data,
	}
}

// Is implements errors.Is for comparing ProtocolErrors by code.
func (e *ProtocolError) Is(target error) bool {
	if t, ok := target.(*ProtocolError); ok {
		return e.Code == t.Code
	}
	return false
}

// Protocol error constants for each error code.
var (
	// General errors
	ErrUnknown           = &ProtocolError{Code: pb.ErrorCode_UNKNOWN}
	ErrInvalidContext    = &ProtocolError{Code: pb.ErrorCode_INVALID_CONTEXT}
	ErrUnauthorized      = &ProtocolError{Code: pb.ErrorCode_UNAUTHORIZED}
	ErrToolNotFound      = &ProtocolError{Code: pb.ErrorCode_TOOL_NOT_FOUND}
	ErrTimeout           = &ProtocolError{Code: pb.ErrorCode_TIMEOUT}
	ErrUnsupportedVer    = &ProtocolError{Code: pb.ErrorCode_UNSUPPORTED_VERSION}
	ErrBadDelta          = &ProtocolError{Code: pb.ErrorCode_BAD_DELTA}
	ErrPayloadTooLarge   = &ProtocolError{Code: pb.ErrorCode_PAYLOAD_TOO_LARGE}
	ErrMalformedRequest  = &ProtocolError{Code: pb.ErrorCode_MALFORMED_REQUEST}
	ErrTooManyRequests   = &ProtocolError{Code: pb.ErrorCode_TOO_MANY_REQUESTS}
	ErrMissingPatchRange = &ProtocolError{Code: pb.ErrorCode_MISSING_PATCH_RANGE}

	// Authentication errors
	ErrAuthSignatureInvalid  = &ProtocolError{Code: pb.ErrorCode_AUTH_SIGNATURE_INVALID}
	ErrAuthReplayDetected    = &ProtocolError{Code: pb.ErrorCode_AUTH_REPLAY_DETECTED}
	ErrAuthDIDInvalid        = &ProtocolError{Code: pb.ErrorCode_AUTH_DID_INVALID}
	ErrAuthTimestampExpired  = &ProtocolError{Code: pb.ErrorCode_AUTH_TIMESTAMP_EXPIRED}

	// Profile negotiation errors
	ErrProfileMismatch      = &ProtocolError{Code: pb.ErrorCode_PROFILE_MISMATCH}
	ErrProfileUnsupported   = &ProtocolError{Code: pb.ErrorCode_PROFILE_UNSUPPORTED}
	ErrProfileNegFailed     = &ProtocolError{Code: pb.ErrorCode_PROFILE_NEGOTIATION_FAILED}
)

// NewProtocolError creates a new ProtocolError with the given code and reason.
func NewProtocolError(code pb.ErrorCode, reason string) *ProtocolError {
	return &ProtocolError{
		Code:   code,
		Reason: reason,
	}
}

// NewErrorEnvelope creates an AXCP envelope containing an error response.
func NewErrorEnvelope(traceID, senderDID, recipientDID string, err *ProtocolError) *Envelope {
	env := NewEnvelope(traceID, 1)
	env.SenderDid = senderDID
	env.RecipientDid = recipientDID
	env.TimestampMs = auth.NowMs()

	// Set the error payload
	env.Payload = &pb.AxcpEnvelope_Error{
		Error: &pb.ErrorMessage{
			Code:        uint32(err.Code),
			Reason:      err.Reason,
			Diagnostics: err.Diagnostics,
		},
	}

	return env
}

// ErrorFromEnvelope extracts a ProtocolError from an envelope if it contains an error.
// Returns nil, false if the envelope does not contain an error payload.
func ErrorFromEnvelope(env *Envelope) (*ProtocolError, bool) {
	if env == nil {
		return nil, false
	}

	errMsg := env.GetError()
	if errMsg == nil {
		return nil, false
	}

	return &ProtocolError{
		Code:        pb.ErrorCode(errMsg.GetCode()),
		Reason:      errMsg.GetReason(),
		Diagnostics: errMsg.GetDiagnostics(),
	}, true
}

// IsErrorEnvelope returns true if the envelope contains an error payload.
func IsErrorEnvelope(env *Envelope) bool {
	if env == nil {
		return false
	}
	return env.GetError() != nil
}

// ToProtocolError converts a Go error to a ProtocolError.
// It recognizes common auth errors and maps them to appropriate protocol codes.
// Unknown errors are mapped to ErrUnknown with the error message as reason.
func ToProtocolError(err error) *ProtocolError {
	if err == nil {
		return nil
	}

	// If it's already a ProtocolError, return it
	var pe *ProtocolError
	if errors.As(err, &pe) {
		return pe
	}

	// Map auth package errors to protocol errors
	switch {
	case errors.Is(err, auth.ErrVerificationFailed):
		return ErrAuthSignatureInvalid.WithReason(err.Error())
	case errors.Is(err, auth.ErrSignatureMissing):
		return ErrAuthSignatureInvalid.WithReason("signature missing")
	case errors.Is(err, auth.ErrTimestampExpired):
		return ErrAuthTimestampExpired.WithReason(err.Error())
	case errors.Is(err, auth.ErrTimestampFuture):
		return ErrAuthTimestampExpired.WithReason("timestamp in future")
	case errors.Is(err, auth.ErrTimestampMissing):
		return ErrAuthTimestampExpired.WithReason("timestamp missing")
	case errors.Is(err, auth.ErrReplay):
		return ErrAuthReplayDetected.WithReason(err.Error())
	case errors.Is(err, auth.ErrRecipientMissing):
		return ErrMalformedRequest.WithReason("recipient missing")
	case errors.Is(err, auth.ErrRecipientMismatch):
		return ErrMalformedRequest.WithReason("wrong recipient")
	case errors.Is(err, auth.ErrSenderMissing):
		return ErrAuthDIDInvalid.WithReason("sender missing")
	case errors.Is(err, auth.ErrDIDNotFound):
		return ErrAuthDIDInvalid.WithReason("DID not found")
	case errors.Is(err, auth.ErrEnvelopeNil):
		return ErrMalformedRequest.WithReason("envelope is nil")
	}

	// Default: unknown error
	return ErrUnknown.WithReason(err.Error())
}

// FromErrorCode creates a ProtocolError from an error code.
func FromErrorCode(code pb.ErrorCode) *ProtocolError {
	switch code {
	case pb.ErrorCode_UNKNOWN:
		return ErrUnknown
	case pb.ErrorCode_INVALID_CONTEXT:
		return ErrInvalidContext
	case pb.ErrorCode_UNAUTHORIZED:
		return ErrUnauthorized
	case pb.ErrorCode_TOOL_NOT_FOUND:
		return ErrToolNotFound
	case pb.ErrorCode_TIMEOUT:
		return ErrTimeout
	case pb.ErrorCode_UNSUPPORTED_VERSION:
		return ErrUnsupportedVer
	case pb.ErrorCode_BAD_DELTA:
		return ErrBadDelta
	case pb.ErrorCode_PAYLOAD_TOO_LARGE:
		return ErrPayloadTooLarge
	case pb.ErrorCode_MALFORMED_REQUEST:
		return ErrMalformedRequest
	case pb.ErrorCode_TOO_MANY_REQUESTS:
		return ErrTooManyRequests
	case pb.ErrorCode_AUTH_SIGNATURE_INVALID:
		return ErrAuthSignatureInvalid
	case pb.ErrorCode_AUTH_REPLAY_DETECTED:
		return ErrAuthReplayDetected
	case pb.ErrorCode_PROFILE_MISMATCH:
		return ErrProfileMismatch
	case pb.ErrorCode_PROFILE_UNSUPPORTED:
		return ErrProfileUnsupported
	case pb.ErrorCode_PROFILE_NEGOTIATION_FAILED:
		return ErrProfileNegFailed
	case pb.ErrorCode_MISSING_PATCH_RANGE:
		return ErrMissingPatchRange
	case pb.ErrorCode_AUTH_DID_INVALID:
		return ErrAuthDIDInvalid
	case pb.ErrorCode_AUTH_TIMESTAMP_EXPIRED:
		return ErrAuthTimestampExpired
	default:
		return ErrUnknown
	}
}

// IsAuthError returns true if the error is an authentication-related error.
func IsAuthError(err error) bool {
	var pe *ProtocolError
	if !errors.As(err, &pe) {
		return false
	}

	switch pe.Code {
	case pb.ErrorCode_AUTH_SIGNATURE_INVALID,
		pb.ErrorCode_AUTH_REPLAY_DETECTED,
		pb.ErrorCode_AUTH_DID_INVALID,
		pb.ErrorCode_AUTH_TIMESTAMP_EXPIRED,
		pb.ErrorCode_UNAUTHORIZED:
		return true
	default:
		return false
	}
}

// IsProfileError returns true if the error is a profile negotiation error.
func IsProfileError(err error) bool {
	var pe *ProtocolError
	if !errors.As(err, &pe) {
		return false
	}

	switch pe.Code {
	case pb.ErrorCode_PROFILE_MISMATCH,
		pb.ErrorCode_PROFILE_UNSUPPORTED,
		pb.ErrorCode_PROFILE_NEGOTIATION_FAILED:
		return true
	default:
		return false
	}
}

// IsRetryable returns true if the error is potentially recoverable by retrying.
func IsRetryable(err error) bool {
	var pe *ProtocolError
	if !errors.As(err, &pe) {
		return false
	}

	switch pe.Code {
	case pb.ErrorCode_TIMEOUT,
		pb.ErrorCode_TOO_MANY_REQUESTS:
		return true
	default:
		return false
	}
}
