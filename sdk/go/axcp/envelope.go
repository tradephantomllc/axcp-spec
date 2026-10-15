package axcp

import (
	"google.golang.org/protobuf/proto"

	"github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"
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

// CloneEnvelope returns a deep copy of a protobuf envelope wrapped in an
// Envelope. Use it instead of dereferencing proto.Clone results, which would
// copy internal protobuf state by value.
func CloneEnvelope(src *pb.AxcpEnvelope) *Envelope {
	env := &Envelope{}
	if src != nil {
		proto.Merge(&env.AxcpEnvelope, src)
	}
	return env
}
