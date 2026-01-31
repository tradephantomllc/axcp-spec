package axcp

import (
	"errors"
	"testing"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"
)

func TestProtocolError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ProtocolError
		expected string
	}{
		{
			name:     "code only",
			err:      ErrUnknown,
			expected: "axcp: UNKNOWN",
		},
		{
			name:     "code with reason",
			err:      ErrUnknown.WithReason("something went wrong"),
			expected: "axcp: UNKNOWN: something went wrong",
		},
		{
			name:     "auth error",
			err:      ErrAuthSignatureInvalid,
			expected: "axcp: AUTH_SIGNATURE_INVALID",
		},
		{
			name:     "auth error with reason",
			err:      ErrAuthSignatureInvalid.WithReason("bad signature"),
			expected: "axcp: AUTH_SIGNATURE_INVALID: bad signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProtocolError_WithReason(t *testing.T) {
	original := ErrUnknown
	withReason := original.WithReason("test reason")

	// Should be a new instance
	if withReason == original {
		t.Error("WithReason should return a new instance")
	}

	// Should have the reason
	if withReason.Reason != "test reason" {
		t.Errorf("Reason = %q, want %q", withReason.Reason, "test reason")
	}

	// Code should be preserved
	if withReason.Code != original.Code {
		t.Errorf("Code = %v, want %v", withReason.Code, original.Code)
	}

	// Original should be unchanged
	if original.Reason != "" {
		t.Errorf("Original Reason changed to %q", original.Reason)
	}
}

func TestProtocolError_WithDiagnostics(t *testing.T) {
	original := ErrUnknown.WithReason("test")
	diagnostics := []byte("stack trace here")
	withDiag := original.WithDiagnostics(diagnostics)

	// Should be a new instance
	if withDiag == original {
		t.Error("WithDiagnostics should return a new instance")
	}

	// Should have the diagnostics
	if string(withDiag.Diagnostics) != string(diagnostics) {
		t.Errorf("Diagnostics = %q, want %q", withDiag.Diagnostics, diagnostics)
	}

	// Reason should be preserved
	if withDiag.Reason != original.Reason {
		t.Errorf("Reason = %q, want %q", withDiag.Reason, original.Reason)
	}

	// Code should be preserved
	if withDiag.Code != original.Code {
		t.Errorf("Code = %v, want %v", withDiag.Code, original.Code)
	}
}

func TestProtocolError_Is(t *testing.T) {
	tests := []struct {
		name     string
		err      *ProtocolError
		target   error
		expected bool
	}{
		{
			name:     "same code matches",
			err:      ErrUnknown.WithReason("different reason"),
			target:   ErrUnknown,
			expected: true,
		},
		{
			name:     "different code does not match",
			err:      ErrUnknown,
			target:   ErrTimeout,
			expected: false,
		},
		{
			name:     "non-ProtocolError does not match",
			err:      ErrUnknown,
			target:   errors.New("some error"),
			expected: false,
		},
		{
			name:     "auth errors match by code",
			err:      ErrAuthSignatureInvalid.WithReason("bad sig"),
			target:   ErrAuthSignatureInvalid,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Is(tt.target)
			if got != tt.expected {
				t.Errorf("Is() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProtocolError_ErrorsIs(t *testing.T) {
	// Test that errors.Is works with wrapped errors
	wrapped := &ProtocolError{
		Code:   pb.ErrorCode_TIMEOUT,
		Reason: "wrapped timeout",
	}

	if !errors.Is(wrapped, ErrTimeout) {
		t.Error("errors.Is should match by code")
	}

	if errors.Is(wrapped, ErrUnknown) {
		t.Error("errors.Is should not match different codes")
	}
}

func TestNewProtocolError(t *testing.T) {
	err := NewProtocolError(pb.ErrorCode_TIMEOUT, "request timed out")

	if err.Code != pb.ErrorCode_TIMEOUT {
		t.Errorf("Code = %v, want TIMEOUT", err.Code)
	}
	if err.Reason != "request timed out" {
		t.Errorf("Reason = %q, want %q", err.Reason, "request timed out")
	}
}

func TestNewErrorEnvelope(t *testing.T) {
	err := ErrUnauthorized.WithReason("access denied")
	env := NewErrorEnvelope("trace-123", "did:key:sender", "did:key:recipient", err)

	if env == nil {
		t.Fatal("NewErrorEnvelope returned nil")
	}

	if env.TraceId != "trace-123" {
		t.Errorf("TraceId = %q, want %q", env.TraceId, "trace-123")
	}
	if env.SenderDid != "did:key:sender" {
		t.Errorf("SenderDid = %q, want %q", env.SenderDid, "did:key:sender")
	}
	if env.RecipientDid != "did:key:recipient" {
		t.Errorf("RecipientDid = %q, want %q", env.RecipientDid, "did:key:recipient")
	}

	// Check error payload
	errMsg := env.GetError()
	if errMsg == nil {
		t.Fatal("GetError() returned nil")
	}
	if errMsg.Code != uint32(pb.ErrorCode_UNAUTHORIZED) {
		t.Errorf("Error Code = %d, want %d", errMsg.Code, pb.ErrorCode_UNAUTHORIZED)
	}
	if errMsg.Reason != "access denied" {
		t.Errorf("Error Reason = %q, want %q", errMsg.Reason, "access denied")
	}
}

func TestErrorFromEnvelope(t *testing.T) {
	// Create an error envelope
	origErr := ErrToolNotFound.WithReason("tool xyz not found").WithDiagnostics([]byte("debug info"))
	env := NewErrorEnvelope("trace-456", "did:key:s", "did:key:r", origErr)

	// Extract error from envelope
	extractedErr, ok := ErrorFromEnvelope(env)
	if !ok {
		t.Fatal("ErrorFromEnvelope returned false")
	}
	if extractedErr == nil {
		t.Fatal("ErrorFromEnvelope returned nil error")
	}

	if extractedErr.Code != pb.ErrorCode_TOOL_NOT_FOUND {
		t.Errorf("Code = %v, want TOOL_NOT_FOUND", extractedErr.Code)
	}
	if extractedErr.Reason != "tool xyz not found" {
		t.Errorf("Reason = %q, want %q", extractedErr.Reason, "tool xyz not found")
	}
	if string(extractedErr.Diagnostics) != "debug info" {
		t.Errorf("Diagnostics = %q, want %q", extractedErr.Diagnostics, "debug info")
	}
}

func TestErrorFromEnvelope_NilEnvelope(t *testing.T) {
	err, ok := ErrorFromEnvelope(nil)
	if ok {
		t.Error("ErrorFromEnvelope(nil) should return false")
	}
	if err != nil {
		t.Error("ErrorFromEnvelope(nil) should return nil error")
	}
}

func TestErrorFromEnvelope_NotErrorEnvelope(t *testing.T) {
	// Create a non-error envelope
	env := NewEnvelope("trace-789", 1)

	err, ok := ErrorFromEnvelope(env)
	if ok {
		t.Error("ErrorFromEnvelope should return false for non-error envelope")
	}
	if err != nil {
		t.Error("ErrorFromEnvelope should return nil for non-error envelope")
	}
}

func TestIsErrorEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		env      *Envelope
		expected bool
	}{
		{
			name:     "nil envelope",
			env:      nil,
			expected: false,
		},
		{
			name:     "empty envelope",
			env:      NewEnvelope("test", 1),
			expected: false,
		},
		{
			name:     "error envelope",
			env:      NewErrorEnvelope("test", "s", "r", ErrUnknown),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsErrorEnvelope(tt.env)
			if got != tt.expected {
				t.Errorf("IsErrorEnvelope() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestToProtocolError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode pb.ErrorCode
	}{
		{
			name:         "nil error",
			err:          nil,
			expectedCode: 0, // Will return nil
		},
		{
			name:         "already protocol error",
			err:          ErrTimeout,
			expectedCode: pb.ErrorCode_TIMEOUT,
		},
		{
			name:         "auth verification failed",
			err:          auth.ErrVerificationFailed,
			expectedCode: pb.ErrorCode_AUTH_SIGNATURE_INVALID,
		},
		{
			name:         "auth signature missing",
			err:          auth.ErrSignatureMissing,
			expectedCode: pb.ErrorCode_AUTH_SIGNATURE_INVALID,
		},
		{
			name:         "auth timestamp expired",
			err:          auth.ErrTimestampExpired,
			expectedCode: pb.ErrorCode_AUTH_TIMESTAMP_EXPIRED,
		},
		{
			name:         "auth timestamp future",
			err:          auth.ErrTimestampFuture,
			expectedCode: pb.ErrorCode_AUTH_TIMESTAMP_EXPIRED,
		},
		{
			name:         "auth timestamp missing",
			err:          auth.ErrTimestampMissing,
			expectedCode: pb.ErrorCode_AUTH_TIMESTAMP_EXPIRED,
		},
		{
			name:         "auth replay detected",
			err:          auth.ErrReplay,
			expectedCode: pb.ErrorCode_AUTH_REPLAY_DETECTED,
		},
		{
			name:         "auth recipient missing",
			err:          auth.ErrRecipientMissing,
			expectedCode: pb.ErrorCode_MALFORMED_REQUEST,
		},
		{
			name:         "auth recipient mismatch",
			err:          auth.ErrRecipientMismatch,
			expectedCode: pb.ErrorCode_MALFORMED_REQUEST,
		},
		{
			name:         "auth sender missing",
			err:          auth.ErrSenderMissing,
			expectedCode: pb.ErrorCode_AUTH_DID_INVALID,
		},
		{
			name:         "auth DID not found",
			err:          auth.ErrDIDNotFound,
			expectedCode: pb.ErrorCode_AUTH_DID_INVALID,
		},
		{
			name:         "auth envelope nil",
			err:          auth.ErrEnvelopeNil,
			expectedCode: pb.ErrorCode_MALFORMED_REQUEST,
		},
		{
			name:         "unknown error",
			err:          errors.New("something unexpected"),
			expectedCode: pb.ErrorCode_UNKNOWN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToProtocolError(tt.err)
			if tt.err == nil {
				if got != nil {
					t.Errorf("ToProtocolError(nil) = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("ToProtocolError returned nil for non-nil error")
			}
			if got.Code != tt.expectedCode {
				t.Errorf("Code = %v, want %v", got.Code, tt.expectedCode)
			}
		})
	}
}

func TestFromErrorCode(t *testing.T) {
	tests := []struct {
		code     pb.ErrorCode
		expected *ProtocolError
	}{
		{pb.ErrorCode_UNKNOWN, ErrUnknown},
		{pb.ErrorCode_INVALID_CONTEXT, ErrInvalidContext},
		{pb.ErrorCode_UNAUTHORIZED, ErrUnauthorized},
		{pb.ErrorCode_TOOL_NOT_FOUND, ErrToolNotFound},
		{pb.ErrorCode_TIMEOUT, ErrTimeout},
		{pb.ErrorCode_UNSUPPORTED_VERSION, ErrUnsupportedVer},
		{pb.ErrorCode_BAD_DELTA, ErrBadDelta},
		{pb.ErrorCode_PAYLOAD_TOO_LARGE, ErrPayloadTooLarge},
		{pb.ErrorCode_MALFORMED_REQUEST, ErrMalformedRequest},
		{pb.ErrorCode_TOO_MANY_REQUESTS, ErrTooManyRequests},
		{pb.ErrorCode_AUTH_SIGNATURE_INVALID, ErrAuthSignatureInvalid},
		{pb.ErrorCode_AUTH_REPLAY_DETECTED, ErrAuthReplayDetected},
		{pb.ErrorCode_PROFILE_MISMATCH, ErrProfileMismatch},
		{pb.ErrorCode_PROFILE_UNSUPPORTED, ErrProfileUnsupported},
		{pb.ErrorCode_PROFILE_NEGOTIATION_FAILED, ErrProfileNegFailed},
		{pb.ErrorCode_MISSING_PATCH_RANGE, ErrMissingPatchRange},
		{pb.ErrorCode_AUTH_DID_INVALID, ErrAuthDIDInvalid},
		{pb.ErrorCode_AUTH_TIMESTAMP_EXPIRED, ErrAuthTimestampExpired},
		{pb.ErrorCode(9999), ErrUnknown}, // Unknown code falls back to ErrUnknown
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			got := FromErrorCode(tt.code)
			if got != tt.expected {
				t.Errorf("FromErrorCode(%v) = %v, want %v", tt.code, got, tt.expected)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"non-protocol error", errors.New("test"), false},
		{"UNKNOWN", ErrUnknown, false},
		{"TIMEOUT", ErrTimeout, false},
		{"AUTH_SIGNATURE_INVALID", ErrAuthSignatureInvalid, true},
		{"AUTH_REPLAY_DETECTED", ErrAuthReplayDetected, true},
		{"AUTH_DID_INVALID", ErrAuthDIDInvalid, true},
		{"AUTH_TIMESTAMP_EXPIRED", ErrAuthTimestampExpired, true},
		{"UNAUTHORIZED", ErrUnauthorized, true},
		{"PROFILE_MISMATCH", ErrProfileMismatch, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAuthError(tt.err)
			if got != tt.expected {
				t.Errorf("IsAuthError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsProfileError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"non-protocol error", errors.New("test"), false},
		{"UNKNOWN", ErrUnknown, false},
		{"AUTH_SIGNATURE_INVALID", ErrAuthSignatureInvalid, false},
		{"PROFILE_MISMATCH", ErrProfileMismatch, true},
		{"PROFILE_UNSUPPORTED", ErrProfileUnsupported, true},
		{"PROFILE_NEGOTIATION_FAILED", ErrProfileNegFailed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProfileError(tt.err)
			if got != tt.expected {
				t.Errorf("IsProfileError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"non-protocol error", errors.New("test"), false},
		{"UNKNOWN", ErrUnknown, false},
		{"AUTH_SIGNATURE_INVALID", ErrAuthSignatureInvalid, false},
		{"TIMEOUT", ErrTimeout, true},
		{"TOO_MANY_REQUESTS", ErrTooManyRequests, true},
		{"UNAUTHORIZED", ErrUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllErrorConstants(t *testing.T) {
	// Verify all error constants are properly initialized
	errors := []*ProtocolError{
		ErrUnknown,
		ErrInvalidContext,
		ErrUnauthorized,
		ErrToolNotFound,
		ErrTimeout,
		ErrUnsupportedVer,
		ErrBadDelta,
		ErrPayloadTooLarge,
		ErrMalformedRequest,
		ErrTooManyRequests,
		ErrMissingPatchRange,
		ErrAuthSignatureInvalid,
		ErrAuthReplayDetected,
		ErrAuthDIDInvalid,
		ErrAuthTimestampExpired,
		ErrProfileMismatch,
		ErrProfileUnsupported,
		ErrProfileNegFailed,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Found nil error constant")
			continue
		}
		// Each error should have a valid error string
		errStr := err.Error()
		if errStr == "" {
			t.Errorf("Error %v has empty string", err.Code)
		}
	}
}
