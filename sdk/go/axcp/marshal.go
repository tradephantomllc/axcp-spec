package axcp

import (
	"encoding/json"
	"errors"

	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var errNilEnvelopeForMarshal = errors.New("axcp: envelope is nil")

// ToBytes encodes an Envelope into canonical protobuf binary. The explicit
// field ordering keeps Go wire bytes aligned with the Python and TypeScript
// SDKs for release vectors and detached-signature interoperability.
func ToBytes(env *Envelope) ([]byte, error) {
	if env == nil {
		return nil, errNilEnvelopeForMarshal
	}
	return marshalCanonicalEnvelope(&env.AxcpEnvelope)
}

func marshalCanonicalEnvelope(env *pb.AxcpEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errNilEnvelopeForMarshal
	}

	var out []byte
	out = appendVarintField(out, 1, uint64(env.GetVersion()))
	out = appendStringField(out, 2, env.GetTraceId())
	out = appendVarintField(out, 3, uint64(env.GetProfile()))

	out, err := appendEnvelopePayload(out, env.GetPayload())
	if err != nil {
		return nil, err
	}

	out = appendStringField(out, 20, env.GetSenderDid())
	out = appendVarintField(out, 21, env.GetTimestampMs())
	out = appendVarintField(out, 22, env.GetSequence())
	out = appendStringField(out, 23, env.GetRecipientDid())
	out = appendBytesField(out, 100, env.GetSignature())
	out = appendBytesField(out, 101, env.GetAttestationProof())
	return out, nil
}

// FromBytes decodes protobuf binary into Envelope.
func FromBytes(raw []byte) (*Envelope, error) {
	var pbEnv Envelope
	if err := proto.Unmarshal(raw, &pbEnv.AxcpEnvelope); err != nil {
		return nil, err
	}
	return &pbEnv, nil
}

// ToJSON renders any protobuf message as pretty JSON (debug / logs).
func ToJSON(msg proto.Message) ([]byte, error) {
	return protojson.MarshalOptions{
		Multiline:       true,
		UseProtoNames:   true,
		EmitUnpopulated: false,
	}.Marshal(msg)
}

// FromJSON parses JSON into the given protobuf message.
func FromJSON(data []byte, msg proto.Message) error {
	return protojson.Unmarshal(data, msg)
}

// MarshalIndent is a helper to pretty-print arbitrary structs.
func MarshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
