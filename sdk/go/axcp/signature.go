package axcp

import (
	"errors"
	"fmt"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

var errNilEnvelopeForSignature = errors.New("axcp: envelope is nil")

// SigningPayload returns the canonical protobuf payload covered by the AXCP
// DID authentication transcript. Sender, recipient, and timestamp are supplied
// as transcript bindings, and the detached signature is excluded from the
// signed bytes. Sequence remains covered because replay protection consumes it.
func SigningPayload(env *Envelope) ([]byte, error) {
	if env == nil {
		return nil, errNilEnvelopeForSignature
	}
	return SigningPayloadFromProto(&env.AxcpEnvelope)
}

// SigningPayloadFromProto is the protobuf-envelope variant of SigningPayload.
func SigningPayloadFromProto(env *pb.AxcpEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errNilEnvelopeForSignature
	}
	clone, ok := proto.Clone(env).(*pb.AxcpEnvelope)
	if !ok {
		return nil, errors.New("axcp: failed to clone envelope")
	}

	return marshalCanonicalSigningPayload(clone)
}

func marshalCanonicalSigningPayload(env *pb.AxcpEnvelope) ([]byte, error) {
	var out []byte
	out = appendVarintField(out, 1, uint64(env.GetVersion()))
	out = appendStringField(out, 2, env.GetTraceId())
	out = appendVarintField(out, 3, uint64(env.GetProfile()))

	out, err := appendEnvelopePayload(out, env.GetPayload())
	if err != nil {
		return nil, err
	}

	out = appendVarintField(out, 22, env.GetSequence())
	return out, nil
}

func appendEnvelopePayload(out []byte, payload any) ([]byte, error) {
	var err error
	switch payload := payload.(type) {
	case nil:
	case *pb.AxcpEnvelope_ContextPatch:
		out, err = appendMessageField(out, 4, payload.ContextPatch)
	case *pb.AxcpEnvelope_CapabilityMsg:
		out, err = appendMessageField(out, 5, payload.CapabilityMsg)
	case *pb.AxcpEnvelope_RouteMsg:
		out, err = appendMessageField(out, 6, payload.RouteMsg)
	case *pb.AxcpEnvelope_Error:
		out, err = appendMessageField(out, 7, payload.Error)
	case *pb.AxcpEnvelope_ProfileNeg:
		out, err = appendMessageField(out, 8, payload.ProfileNeg)
	case *pb.AxcpEnvelope_ProfileAck:
		out, err = appendMessageField(out, 9, payload.ProfileAck)
	case *pb.AxcpEnvelope_RetryEnv:
		out, err = appendMessageField(out, 10, payload.RetryEnv)
	case *pb.AxcpEnvelope_Telemetry:
		out, err = appendMessageField(out, 11, payload.Telemetry)
	default:
		return nil, fmt.Errorf("axcp: unsupported payload type %T", payload)
	}
	return out, err
}

func appendVarintField(out []byte, number protowire.Number, value uint64) []byte {
	if value == 0 {
		return out
	}
	out = protowire.AppendTag(out, number, protowire.VarintType)
	return protowire.AppendVarint(out, value)
}

func appendStringField(out []byte, number protowire.Number, value string) []byte {
	if value == "" {
		return out
	}
	out = protowire.AppendTag(out, number, protowire.BytesType)
	return protowire.AppendString(out, value)
}

func appendBytesField(out []byte, number protowire.Number, value []byte) []byte {
	if len(value) == 0 {
		return out
	}
	out = protowire.AppendTag(out, number, protowire.BytesType)
	return protowire.AppendBytes(out, value)
}

func appendMessageField(out []byte, number protowire.Number, msg proto.Message) ([]byte, error) {
	var payload []byte
	if msg != nil {
		var err error
		payload, err = proto.MarshalOptions{Deterministic: true}.Marshal(msg)
		if err != nil {
			return nil, err
		}
	}
	out = protowire.AppendTag(out, number, protowire.BytesType)
	return protowire.AppendBytes(out, payload), nil
}

// BuildAuthTranscript builds the canonical DID authentication transcript for
// an AXCP envelope.
func BuildAuthTranscript(env *Envelope) ([]byte, error) {
	if env == nil {
		return nil, errNilEnvelopeForSignature
	}

	payload, err := SigningPayload(env)
	if err != nil {
		return nil, err
	}

	timestamp := time.UnixMilli(int64(env.GetTimestampMs()))
	return auth.BuildDIDAuthTranscript(
		env.GetSenderDid(),
		env.GetRecipientDid(),
		payload,
		timestamp,
	), nil
}
