package axcp

import (
	"github.com/tradephantom/axcp-spec/sdk/go/axcp/internal/pb"
)

// Envelope is a minimal façade around the generated AxcpEnvelope.
type Envelope struct {
	pb.AxcpEnvelope
}

// NewEnvelope returns an Envelope with TraceID + Version pre-filled.
func NewEnvelope(traceID string, profile uint32) *Envelope {
	return &Envelope{
		pb.AxcpEnvelope{
			Version: 1,
			TraceId: traceID,
			Profile: profile,
		},
	}
}
