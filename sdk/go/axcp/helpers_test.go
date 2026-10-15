package axcp

import "github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"

// buildEnvelopeTranscript builds the canonical auth transcript for tests and
// panics on error, so a broken envelope fails the test loudly.
func buildEnvelopeTranscript(env *Envelope) []byte {
	transcript, err := BuildAuthTranscript(env)
	if err != nil {
		panic(err)
	}
	return transcript
}

// newTestEnvelope returns a Secure Baseline envelope with a context patch
// payload and sequence 1, ready for auth fields to be filled in.
func newTestEnvelope(traceID string) *Envelope {
	env := NewEnvelope(traceID, 1)
	env.Payload = &pb.AxcpEnvelope_ContextPatch{ContextPatch: &pb.ContextPatch{ContextId: "ctx", BaseVersion: 1}}
	env.Sequence = 1
	return env
}
