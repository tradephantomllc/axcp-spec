package axcp

import "github.com/tradephantom/axcp-spec/sdk/go/internal/pb"

// Re-export all protobuf types through the axcp module
// This allows external modules to import axcp instead of internal/pb

// Core envelope types
type AxcpEnvelope = pb.AxcpEnvelope

// Telemetry types
type TelemetryDatagram = pb.TelemetryDatagram
type SystemStats = pb.SystemStats
type TokenUsage = pb.TokenUsage
type LatencyStats = pb.LatencyStats

// Telemetry oneof types
type TelemetryDatagram_System = pb.TelemetryDatagram_System
type TelemetryDatagram_Tokens = pb.TelemetryDatagram_Tokens
type TelemetryDatagram_Latency = pb.TelemetryDatagram_Latency

// Error types
type ErrorCode = pb.ErrorCode
type ErrorMessage = pb.ErrorMessage

// Differential privacy types
type DpMechanism = pb.DpMechanism
type DpParams = pb.DpParams

// Context and delta types
type ContextPatch = pb.ContextPatch
type DeltaOp = pb.DeltaOp

// Export commonly used constants
const (
	// Error codes
	ErrorCode_UNKNOWN                    = pb.ErrorCode_UNKNOWN
	ErrorCode_INVALID_CONTEXT            = pb.ErrorCode_INVALID_CONTEXT
	ErrorCode_UNAUTHORIZED               = pb.ErrorCode_UNAUTHORIZED
	ErrorCode_TOOL_NOT_FOUND             = pb.ErrorCode_TOOL_NOT_FOUND
	ErrorCode_TIMEOUT                    = pb.ErrorCode_TIMEOUT
	ErrorCode_UNSUPPORTED_VERSION        = pb.ErrorCode_UNSUPPORTED_VERSION
	ErrorCode_BAD_DELTA                  = pb.ErrorCode_BAD_DELTA
	ErrorCode_PAYLOAD_TOO_LARGE          = pb.ErrorCode_PAYLOAD_TOO_LARGE
	ErrorCode_MALFORMED_REQUEST          = pb.ErrorCode_MALFORMED_REQUEST
	ErrorCode_TOO_MANY_REQUESTS          = pb.ErrorCode_TOO_MANY_REQUESTS
	ErrorCode_PROFILE_MISMATCH           = pb.ErrorCode_PROFILE_MISMATCH
	ErrorCode_PROFILE_UNSUPPORTED        = pb.ErrorCode_PROFILE_UNSUPPORTED
	ErrorCode_PROFILE_NEGOTIATION_FAILED = pb.ErrorCode_PROFILE_NEGOTIATION_FAILED
	ErrorCode_MISSING_PATCH_RANGE        = pb.ErrorCode_MISSING_PATCH_RANGE
	ErrorCode_DP_POLICY_CONFLICT         = pb.ErrorCode_DP_POLICY_CONFLICT

	// DP mechanisms
	DpMechanism_LAPLACE  = pb.DpMechanism_LAPLACE
	DpMechanism_GAUSSIAN = pb.DpMechanism_GAUSSIAN
)