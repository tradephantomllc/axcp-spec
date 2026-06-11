package axcp

import (
	"bytes"
	"testing"
	"time"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"
	"google.golang.org/protobuf/proto"
)

func TestSigningPayloadExcludesDetachedAuthFields(t *testing.T) {
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = "did:key:sender"
	env.RecipientDid = "did:key:recipient"
	env.TimestampMs = 1700000000000
	env.Sequence = 42
	env.Signature = []byte("signature")
	env.AttestationProof = []byte("attestation")
	env.Payload = &pb.AxcpEnvelope_ContextPatch{
		ContextPatch: &pb.ContextPatch{
			ContextId:   "ctx",
			BaseVersion: 7,
		},
	}

	payload, err := SigningPayload(env)
	if err != nil {
		t.Fatalf("SigningPayload failed: %v", err)
	}

	var decoded pb.AxcpEnvelope
	if err := proto.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal signing payload: %v", err)
	}

	if decoded.GetSenderDid() != "" {
		t.Fatalf("SenderDid = %q, want empty", decoded.GetSenderDid())
	}
	if decoded.GetRecipientDid() != "" {
		t.Fatalf("RecipientDid = %q, want empty", decoded.GetRecipientDid())
	}
	if decoded.GetTimestampMs() != 0 {
		t.Fatalf("TimestampMs = %d, want 0", decoded.GetTimestampMs())
	}
	if decoded.GetSequence() != 42 {
		t.Fatalf("Sequence = %d, want 42", decoded.GetSequence())
	}
	if len(decoded.GetSignature()) != 0 {
		t.Fatalf("Signature length = %d, want 0", len(decoded.GetSignature()))
	}
	if decoded.GetContextPatch().GetContextId() != "ctx" {
		t.Fatalf("payload was not preserved")
	}
}

func TestSigningPayloadStableWhenAuthFieldsChange(t *testing.T) {
	left := NewEnvelope("trace-123", 1)
	left.SenderDid = "did:key:left"
	left.RecipientDid = "did:key:recipient"
	left.TimestampMs = 1700000000000
	left.Sequence = 1
	left.Signature = []byte("left")

	right := NewEnvelope("trace-123", 1)
	right.SenderDid = "did:key:right"
	right.RecipientDid = "did:key:other"
	right.TimestampMs = 1700000001000
	right.Sequence = 1
	right.Signature = []byte("right")

	leftPayload, err := SigningPayload(left)
	if err != nil {
		t.Fatalf("SigningPayload(left) failed: %v", err)
	}
	rightPayload, err := SigningPayload(right)
	if err != nil {
		t.Fatalf("SigningPayload(right) failed: %v", err)
	}
	if !bytes.Equal(leftPayload, rightPayload) {
		t.Fatal("signing payload changed when only auth fields changed")
	}
}

func TestSigningPayloadChangesWhenSequenceChanges(t *testing.T) {
	left := NewEnvelope("trace-123", 1)
	left.Sequence = 1

	right := NewEnvelope("trace-123", 1)
	right.Sequence = 2

	leftPayload, err := SigningPayload(left)
	if err != nil {
		t.Fatalf("SigningPayload(left) failed: %v", err)
	}
	rightPayload, err := SigningPayload(right)
	if err != nil {
		t.Fatalf("SigningPayload(right) failed: %v", err)
	}
	if bytes.Equal(leftPayload, rightPayload) {
		t.Fatal("signing payload did not change when sequence changed")
	}
}

func TestBuildAuthTranscriptMatchesDIDAuthTranscript(t *testing.T) {
	env := NewEnvelope("trace-123", 1)
	env.SenderDid = "did:key:sender"
	env.RecipientDid = "did:key:recipient"
	env.TimestampMs = 1700000000000

	payload, err := SigningPayload(env)
	if err != nil {
		t.Fatalf("SigningPayload failed: %v", err)
	}
	want := auth.BuildDIDAuthTranscript(
		env.GetSenderDid(),
		env.GetRecipientDid(),
		payload,
		time.UnixMilli(int64(env.GetTimestampMs())),
	)

	got, err := BuildAuthTranscript(env)
	if err != nil {
		t.Fatalf("BuildAuthTranscript failed: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("transcript mismatch\ngot:  %q\nwant: %q", got, want)
	}
}
