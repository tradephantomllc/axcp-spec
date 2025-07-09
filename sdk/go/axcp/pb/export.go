// Code generated shim – DO NOT EDIT.
// Public re-exports of axcp/internal/pb for external packages.

package pb

import internal "github.com/tradephantom/axcp-spec/sdk/go/axcp/internal/pb"

// ---- re-export core types we actually use ----
type (
	// Core envelope and message types
	AxcpEnvelope         = internal.AxcpEnvelope
	TelemetryDatagram    = internal.TelemetryDatagram
	SystemStats          = internal.SystemStats
	TokenUsage           = internal.TokenUsage
	
	// Capability types
	CapabilityDescriptor = internal.CapabilityDescriptor
	CapabilityOffer      = internal.CapabilityOffer
	CapabilityRequest    = internal.CapabilityRequest
	CapabilityAck        = internal.CapabilityAck
	CapabilityMessage    = internal.CapabilityMessage
	
	// Context and patch types
	ContextPatch         = internal.ContextPatch
	DeltaOp              = internal.DeltaOp
	RetryEnvelope        = internal.RetryEnvelope
	
	// Profile and routing types
	ProfileNegotiate     = internal.ProfileNegotiate
	ProfileAck           = internal.ProfileAck
	RoutePolicyMessage   = internal.RoutePolicyMessage
	
	// Error handling
	ErrorMessage         = internal.ErrorMessage
	ErrorCode            = internal.ErrorCode
	
	// Differential privacy
	DpParams             = internal.DpParams
	DpMechanism          = internal.DpMechanism
)

// Re-export constants
const (
	ErrorCode_UNKNOWN                     = internal.ErrorCode_UNKNOWN
	ErrorCode_INVALID_CONTEXT             = internal.ErrorCode_INVALID_CONTEXT
	ErrorCode_UNAUTHORIZED                = internal.ErrorCode_UNAUTHORIZED
	ErrorCode_TOOL_NOT_FOUND              = internal.ErrorCode_TOOL_NOT_FOUND
	ErrorCode_TIMEOUT                     = internal.ErrorCode_TIMEOUT
	ErrorCode_UNSUPPORTED_VERSION         = internal.ErrorCode_UNSUPPORTED_VERSION
	ErrorCode_BAD_DELTA                   = internal.ErrorCode_BAD_DELTA
	ErrorCode_PAYLOAD_TOO_LARGE           = internal.ErrorCode_PAYLOAD_TOO_LARGE
	ErrorCode_MALFORMED_REQUEST           = internal.ErrorCode_MALFORMED_REQUEST
	ErrorCode_TOO_MANY_REQUESTS           = internal.ErrorCode_TOO_MANY_REQUESTS
	ErrorCode_PROFILE_MISMATCH            = internal.ErrorCode_PROFILE_MISMATCH
	ErrorCode_PROFILE_UNSUPPORTED         = internal.ErrorCode_PROFILE_UNSUPPORTED
	ErrorCode_PROFILE_NEGOTIATION_FAILED  = internal.ErrorCode_PROFILE_NEGOTIATION_FAILED
	ErrorCode_MISSING_PATCH_RANGE         = internal.ErrorCode_MISSING_PATCH_RANGE
	ErrorCode_DP_POLICY_CONFLICT          = internal.ErrorCode_DP_POLICY_CONFLICT
	
	DpMechanism_LAPLACE                   = internal.DpMechanism_LAPLACE
	DpMechanism_GAUSSIAN                  = internal.DpMechanism_GAUSSIAN
)

// Re-export oneof wrapper types for AxcpEnvelope
type (
	AxcpEnvelope_ContextPatch   = internal.AxcpEnvelope_ContextPatch
	AxcpEnvelope_CapabilityMsg  = internal.AxcpEnvelope_CapabilityMsg
	AxcpEnvelope_RouteMsg       = internal.AxcpEnvelope_RouteMsg
	AxcpEnvelope_Error          = internal.AxcpEnvelope_Error
	AxcpEnvelope_ProfileNeg     = internal.AxcpEnvelope_ProfileNeg
	AxcpEnvelope_ProfileAck     = internal.AxcpEnvelope_ProfileAck
	AxcpEnvelope_RetryEnv       = internal.AxcpEnvelope_RetryEnv
	AxcpEnvelope_Telemetry      = internal.AxcpEnvelope_Telemetry
)

// Re-export oneof wrapper types for TelemetryDatagram
type (
	TelemetryDatagram_System = internal.TelemetryDatagram_System
	TelemetryDatagram_Tokens = internal.TelemetryDatagram_Tokens
)

// Re-export oneof wrapper types for CapabilityMessage
type (
	CapabilityMessage_Offer   = internal.CapabilityMessage_Offer
	CapabilityMessage_Request = internal.CapabilityMessage_Request
	CapabilityMessage_Ack     = internal.CapabilityMessage_Ack
)