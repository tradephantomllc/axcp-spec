package axcp

import (
	"errors"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
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

	clone.SenderDid = ""
	clone.RecipientDid = ""
	clone.TimestampMs = 0
	clone.Signature = nil
	clone.AttestationProof = nil

	return proto.MarshalOptions{Deterministic: true}.Marshal(clone)
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
