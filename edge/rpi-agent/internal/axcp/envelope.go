package axcp

import (
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

// Envelope is a minimal façade around the generated AxcpEnvelope.
type Envelope struct {
	axcp.AxcpEnvelope
}

// NewEnvelope returns an Envelope with TraceID + Version pre-filled.
func NewEnvelope(traceID string, profile uint32) *Envelope {
	return &Envelope{
		axcp.AxcpEnvelope{
			Version: 1,
			TraceId: traceID,
			Profile: profile,
		},
	}
}
