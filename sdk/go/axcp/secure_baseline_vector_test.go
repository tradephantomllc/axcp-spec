package axcp

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"
)

type secureBaselineVector struct {
	ProtocolVersion uint32 `json:"protocol_version"`
	Profile         uint32 `json:"profile"`
	TraceID         string `json:"trace_id"`
	RecipientDID    string `json:"recipient_did"`
	TimestampMs     uint64 `json:"timestamp_ms"`
	Sequence        uint64 `json:"sequence"`
	Identity        struct {
		PrivateSeedHex string `json:"private_seed_hex"`
		DID            string `json:"did"`
		PublicKeyB64   string `json:"public_key_b64"`
	} `json:"identity"`
	ContextPatch struct {
		ContextID   string `json:"context_id"`
		BaseVersion uint64 `json:"base_version"`
		Ops         []struct {
			Op      string `json:"op"`
			Path    string `json:"path"`
			DataB64 string `json:"data_b64"`
			Ts      uint64 `json:"ts"`
		} `json:"ops"`
	} `json:"context_patch"`
	Expected struct {
		SigningPayloadB64 string `json:"signing_payload_b64"`
		AuthTranscriptB64 string `json:"auth_transcript_b64"`
		SignatureB64      string `json:"signature_b64"`
		EnvelopeB64       string `json:"envelope_b64"`
	} `json:"expected"`
}

func TestSharedSecureBaselineVectorMatchesGoSDK(t *testing.T) {
	vector := loadSecureBaselineVector(t)
	privateKey := privateKeyFromVector(t, vector)
	env := envelopeFromVector(t, vector)

	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		t.Fatal("ed25519 private key returned non-Ed25519 public key")
	}
	if got := base64.StdEncoding.EncodeToString(publicKey); got != vector.Identity.PublicKeyB64 {
		t.Fatalf("public key mismatch\ngot:  %s\nwant: %s", got, vector.Identity.PublicKeyB64)
	}

	payload, err := SigningPayload(env)
	if err != nil {
		t.Fatalf("SigningPayload failed: %v", err)
	}
	if got := base64.StdEncoding.EncodeToString(payload); got != vector.Expected.SigningPayloadB64 {
		t.Fatalf("signing payload mismatch\ngot:  %s\nwant: %s", got, vector.Expected.SigningPayloadB64)
	}

	transcript, err := BuildAuthTranscript(env)
	if err != nil {
		t.Fatalf("BuildAuthTranscript failed: %v", err)
	}
	if got := base64.StdEncoding.EncodeToString(transcript); got != vector.Expected.AuthTranscriptB64 {
		t.Fatalf("auth transcript mismatch\ngot:  %s\nwant: %s", got, vector.Expected.AuthTranscriptB64)
	}

	env.Signature = ed25519.Sign(privateKey, transcript)
	if got := base64.StdEncoding.EncodeToString(env.Signature); got != vector.Expected.SignatureB64 {
		t.Fatalf("signature mismatch\ngot:  %s\nwant: %s", got, vector.Expected.SignatureB64)
	}

	wire, err := ToBytes(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if got := base64.StdEncoding.EncodeToString(wire); got != vector.Expected.EnvelopeB64 {
		t.Fatalf("envelope mismatch\ngot:  %s\nwant: %s", got, vector.Expected.EnvelopeB64)
	}
}

func loadSecureBaselineVector(t *testing.T) secureBaselineVector {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "sdk", "secure_baseline_vector.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vector: %v", err)
	}
	var vector secureBaselineVector
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatalf("parse vector: %v", err)
	}
	return vector
}

func privateKeyFromVector(t *testing.T, vector secureBaselineVector) ed25519.PrivateKey {
	t.Helper()
	seed, err := hex.DecodeString(vector.Identity.PrivateSeedHex)
	if err != nil {
		t.Fatalf("decode private seed: %v", err)
	}
	if len(seed) != ed25519.SeedSize {
		t.Fatalf("private seed length = %d, want %d", len(seed), ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func envelopeFromVector(t *testing.T, vector secureBaselineVector) *Envelope {
	t.Helper()
	env := NewEnvelope(vector.TraceID, vector.Profile)
	env.Version = vector.ProtocolVersion
	env.SenderDid = vector.Identity.DID
	env.RecipientDid = vector.RecipientDID
	env.TimestampMs = vector.TimestampMs
	env.Sequence = vector.Sequence

	patch := &pb.ContextPatch{
		ContextId:   vector.ContextPatch.ContextID,
		BaseVersion: vector.ContextPatch.BaseVersion,
	}
	for _, op := range vector.ContextPatch.Ops {
		data, err := base64.StdEncoding.DecodeString(op.DataB64)
		if err != nil {
			t.Fatalf("decode op data for path %q: %v", op.Path, err)
		}
		patch.Ops = append(patch.Ops, &pb.DeltaOp{
			Op:   vectorOpType(t, op.Op),
			Path: op.Path,
			Data: data,
			Ts:   op.Ts,
		})
	}
	env.Payload = &pb.AxcpEnvelope_ContextPatch{ContextPatch: patch}
	return env
}

func vectorOpType(t *testing.T, op string) pb.DeltaOp_OpType {
	t.Helper()
	switch op {
	case "ADD":
		return pb.DeltaOp_ADD
	case "REPLACE":
		return pb.DeltaOp_REPLACE
	case "REMOVE":
		return pb.DeltaOp_REMOVE
	case "MERGE":
		return pb.DeltaOp_MERGE
	default:
		t.Fatalf("unsupported vector op %q", op)
		return pb.DeltaOp_ADD
	}
}
