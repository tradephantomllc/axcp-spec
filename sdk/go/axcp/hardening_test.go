package axcp

import (
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/tradephantomllc/axcp-spec/sdk/go/auth"
	"github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"
)

type testPeers struct {
	senderDID string
	senderKey ed25519.PrivateKey
	localDID  string
	localKey  ed25519.PrivateKey
	resolver  *auth.MemoryDIDResolver
}

func newTestPeers(t *testing.T) testPeers {
	t.Helper()
	senderDID, senderPub, senderKey, err := auth.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	localDID, localPub, localKey, err := auth.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	resolver := auth.NewMemoryDIDResolver()
	resolver.AddDID(senderDID, senderPub)
	resolver.AddDID(localDID, localPub)
	return testPeers{senderDID, senderKey, localDID, localKey, resolver}
}

var testSeq uint64 = 1000

// signedEnvelope builds a fresh, correctly signed envelope after applying mutate.
func (p testPeers) signedEnvelope(t *testing.T, mutate func(*Envelope)) *Envelope {
	t.Helper()
	testSeq++
	env := newTestEnvelope("trace-hardening")
	env.SenderDid = p.senderDID
	env.RecipientDid = p.localDID
	env.TimestampMs = auth.NowMs()
	env.Sequence = testSeq
	if mutate != nil {
		mutate(env)
	}
	transcript, err := BuildAuthTranscript(env)
	if err != nil {
		t.Fatal(err)
	}
	env.Signature = ed25519.Sign(p.senderKey, transcript)
	return env
}

// entryPoint is one public way to validate an incoming envelope.
type entryPoint struct {
	name     string
	validate func(env *Envelope) error
}

func (p testPeers) entryPoints(t *testing.T, cfg func(*ValidatorConfig)) []entryPoint {
	t.Helper()
	base := ValidatorConfig{LocalDID: p.localDID, Resolver: p.resolver}
	if cfg != nil {
		cfg(&base)
	}
	validator := NewValidator(base)

	handled := HandlerFunc(func(context.Context, *Envelope) (*Envelope, error) { return nil, nil })
	secureBaseline := SecureBaselineMiddleware(p.localDID, p.resolver, auth.NewDefaultReplayProtector())(handled)
	validatorMiddleware := validator.Middleware()(handled)

	server := ServerConfig{LocalDID: p.localDID, PrivateKey: p.localKey, Resolver: p.resolver}
	if base.AllowedDIDs != nil {
		server.Security.AllowedDIDs = base.AllowedDIDs
	}
	built, err := server.BuildValidator()
	if err != nil {
		t.Fatal(err)
	}

	asErr := func(perr *ProtocolError) error {
		if perr == nil {
			return nil
		}
		return perr
	}
	points := []entryPoint{
		{"ValidateAndVerify", func(env *Envelope) error { return asErr(validator.ValidateAndVerify(context.Background(), env)) }},
		{"ValidateWithResult", func(env *Envelope) error {
			res := validator.ValidateWithResult(context.Background(), env)
			if res.Valid {
				return nil
			}
			return res.Error
		}},
		{"Validator.Middleware", func(env *Envelope) error {
			_, err := validatorMiddleware.HandleEnvelope(context.Background(), env)
			return err
		}},
		{"SecureBaselineMiddleware", func(env *Envelope) error {
			_, err := secureBaseline.HandleEnvelope(context.Background(), env)
			return err
		}},
		{"ServerConfig.BuildValidator", func(env *Envelope) error { return asErr(built.ValidateAndVerify(context.Background(), env)) }},
	}
	if base.AllowedDIDs != nil {
		// SecureBaselineMiddleware has no allowlist parameter.
		filtered := points[:0]
		for _, ep := range points {
			if ep.name != "SecureBaselineMiddleware" {
				filtered = append(filtered, ep)
			}
		}
		points = filtered
	}
	return points
}

func setUnknown(m protoreflect.Message, num protowire.Number) {
	raw := protowire.AppendTag(nil, num, protowire.VarintType)
	raw = protowire.AppendVarint(raw, 7)
	m.SetUnknown(raw)
}

func TestHardening_EveryEntryPointRejects(t *testing.T) {
	p := newTestPeers(t)
	other, _, _, _ := auth.GenerateIdentity()

	cases := []struct {
		name   string
		build  func(t *testing.T) *Envelope
		want   *ProtocolError
		config func(*ValidatorConfig)
	}{
		{"version 2", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.Version = 2 })
		}, ErrUnsupportedVer, nil},
		{"profile 0", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.Profile = 0 })
		}, ErrProfileUnsupported, nil},
		{"profile 3", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.Profile = 3 })
		}, ErrProfileUnsupported, nil},
		{"no payload", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.Payload = nil })
		}, ErrMalformedRequest, nil},
		{"unknown top-level field", func(t *testing.T) *Envelope {
			env := p.signedEnvelope(t, nil)
			setUnknown(env.AxcpEnvelope.ProtoReflect(), 24) // not covered by the signature
			return env
		}, ErrMalformedRequest, nil},
		{"unknown nested field", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { setUnknown(e.GetContextPatch().ProtoReflect(), 99) })
		}, ErrMalformedRequest, nil},
		{"undefined enum", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) {
				e.GetContextPatch().Ops = []*pb.DeltaOp{{Op: pb.DeltaOp_OpType(7), Path: "/x"}}
			})
		}, ErrMalformedRequest, nil},
		{"zero sequence", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.Sequence = 0 })
		}, ErrMalformedRequest, nil},
		{"missing recipient", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.RecipientDid = "" })
		}, ErrAuthDIDInvalid, nil},
		{"other recipient", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.RecipientDid = other })
		}, ErrAuthDIDInvalid, nil},
		{"stale timestamp", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.TimestampMs = auth.NowMs() - uint64((10 * time.Minute).Milliseconds()) })
		}, ErrAuthTimestampExpired, nil},
		{"future timestamp", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.TimestampMs = auth.NowMs() + uint64((5 * time.Minute).Milliseconds()) })
		}, ErrAuthTimestampExpired, nil},
		{"short signature", func(t *testing.T) *Envelope {
			env := p.signedEnvelope(t, nil)
			env.Signature = env.Signature[:63]
			return env
		}, ErrMalformedRequest, nil},
		{"tampered trace", func(t *testing.T) *Envelope {
			env := p.signedEnvelope(t, nil)
			env.TraceId = "changed"
			return env
		}, ErrAuthSignatureInvalid, nil},
		{"tampered sequence", func(t *testing.T) *Envelope {
			env := p.signedEnvelope(t, nil)
			env.Sequence++
			return env
		}, ErrAuthSignatureInvalid, nil},
		{"unknown sender", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, func(e *Envelope) { e.SenderDid = other })
		}, ErrAuthDIDInvalid, nil},
		{"sender not allowed", func(t *testing.T) *Envelope {
			return p.signedEnvelope(t, nil)
		}, ErrUnauthorized, func(c *ValidatorConfig) { c.AllowedDIDs = []string{p.localDID} }},
	}

	for _, tc := range cases {
		for _, ep := range p.entryPoints(t, tc.config) {
			t.Run(tc.name+"/"+ep.name, func(t *testing.T) {
				err := ep.validate(tc.build(t))
				if !errors.Is(err, tc.want) {
					t.Fatalf("got %v, want %v", err, tc.want)
				}
			})
		}
	}
}

func TestHardening_EveryEntryPointAcceptsValidAndRejectsReplay(t *testing.T) {
	p := newTestPeers(t)
	for _, ep := range p.entryPoints(t, nil) {
		t.Run(ep.name, func(t *testing.T) {
			env := p.signedEnvelope(t, nil)
			if err := ep.validate(env); err != nil {
				t.Fatalf("valid envelope rejected: %v", err)
			}
			if err := ep.validate(env); !errors.Is(err, ErrAuthReplayDetected) {
				t.Fatalf("replay: got %v, want ErrAuthReplayDetected", err)
			}
		})
	}
}

func TestHardening_InvalidSignatureDoesNotConsumeSequence(t *testing.T) {
	p := newTestPeers(t)
	for _, ep := range p.entryPoints(t, nil) {
		t.Run(ep.name, func(t *testing.T) {
			env := p.signedEnvelope(t, nil)
			forged := proto.Clone(&env.AxcpEnvelope).(*pb.AxcpEnvelope)
			forged.Signature = append([]byte(nil), env.Signature...)
			forged.Signature[0] ^= 0xff
			forgedEnv := &Envelope{}
			proto.Merge(&forgedEnv.AxcpEnvelope, forged)
			if err := ep.validate(forgedEnv); !errors.Is(err, ErrAuthSignatureInvalid) {
				t.Fatalf("forged: got %v", err)
			}
			if err := ep.validate(env); err != nil {
				t.Fatalf("genuine envelope rejected after forgery attempt: %v", err)
			}
		})
	}
}

func TestHardening_ReplayProtectionIsOnByDefault(t *testing.T) {
	p := newTestPeers(t)
	v := NewValidator(ValidatorConfig{LocalDID: p.localDID, Resolver: p.resolver})
	if v.ReplayProtector() == nil {
		t.Fatal("NewValidator must enable replay protection by default")
	}
	env := p.signedEnvelope(t, nil)
	if perr := v.ValidateAndVerify(context.Background(), env); perr != nil {
		t.Fatal(perr)
	}
	if perr := v.ValidateAndVerify(context.Background(), env); !errors.Is(perr, ErrAuthReplayDetected) {
		t.Fatalf("got %v", perr)
	}

	// Replay state spans one-off ValidateRequest calls.
	env = p.signedEnvelope(t, nil)
	if perr := ValidateRequest(context.Background(), env, p.localDID, p.resolver); perr != nil {
		t.Fatal(perr)
	}
	if perr := ValidateRequest(context.Background(), env, p.localDID, p.resolver); !errors.Is(perr, ErrAuthReplayDetected) {
		t.Fatalf("ValidateRequest replay: got %v", perr)
	}

	// Only an explicit opt-out disables it.
	off := NewValidator(ValidatorConfig{LocalDID: p.localDID, Resolver: p.resolver, DisableReplayProtection: true})
	if off.ReplayProtector() != nil {
		t.Fatal("DisableReplayProtection ignored")
	}
}

func TestHardening_ReplayTTLCoversTimestampWindow(t *testing.T) {
	p := newTestPeers(t)
	short, _ := auth.NewReplayProtector(time.Minute, 100, nil)
	NewValidator(ValidatorConfig{
		LocalDID:           p.localDID,
		Resolver:           p.resolver,
		TimestampValidator: auth.NewTimestampValidator(5*time.Minute, 30*time.Second),
		ReplayProtector:    short,
	})
	if short.TTL() < 5*time.Minute+30*time.Second {
		t.Fatalf("replay TTL %v does not cover max age + skew", short.TTL())
	}
}

func TestHardening_SkipSignatureStillAppliesConfiguredPolicy(t *testing.T) {
	p := newTestPeers(t)
	v := NewValidator(ValidatorConfig{
		LocalDID:                  p.localDID,
		Resolver:                  p.resolver,
		TimestampValidator:        auth.NewTimestampValidator(time.Minute, time.Second),
		SkipSignatureVerification: true,
	})
	env := p.signedEnvelope(t, func(e *Envelope) { e.TimestampMs = auth.NowMs() - uint64((3 * time.Minute).Milliseconds()) })
	if perr := v.ValidateAndVerify(context.Background(), env); !errors.Is(perr, ErrAuthTimestampExpired) {
		t.Fatalf("got %v, want timestamp rejection with configured policy", perr)
	}
	if res := v.ValidateWithResult(context.Background(), env); res.Valid {
		t.Fatal("ValidateWithResult ignored the configured timestamp policy")
	}
}

func TestFromBytesStrict(t *testing.T) {
	p := newTestPeers(t)
	env := p.signedEnvelope(t, nil)
	raw, err := proto.Marshal(&env.AxcpEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FromBytesStrict(raw); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}

	errMsg, _ := proto.Marshal(&pb.ErrorMessage{Code: 1, Reason: "x"})
	twoMembers := protowire.AppendTag(append([]byte(nil), raw...), 7, protowire.BytesType)
	twoMembers = protowire.AppendBytes(twoMembers, errMsg)
	if _, err := FromBytesStrict(twoMembers); !errors.Is(err, ErrMalformedRequest) {
		t.Fatalf("two payload members: got %v", err)
	}

	patch, _ := proto.Marshal(&pb.ContextPatch{ContextId: "ctx2"})
	sameMember := protowire.AppendTag(append([]byte(nil), raw...), 4, protowire.BytesType)
	sameMember = protowire.AppendBytes(sameMember, patch)
	if _, err := FromBytesStrict(sameMember); err != nil {
		t.Fatalf("repeated same member must merge, got %v", err)
	}

	unknown := protowire.AppendTag(append([]byte(nil), raw...), 24, protowire.VarintType)
	unknown = protowire.AppendVarint(unknown, 1)
	if _, err := FromBytesStrict(unknown); !errors.Is(err, ErrMalformedRequest) {
		t.Fatalf("unknown field: got %v", err)
	}

	badUTF8 := protowire.AppendTag(append([]byte(nil), raw...), 2, protowire.BytesType)
	badUTF8 = protowire.AppendBytes(badUTF8, []byte{0xff, 0xfe})
	if _, err := FromBytesStrict(badUTF8); !errors.Is(err, ErrMalformedRequest) {
		t.Fatalf("invalid UTF-8: got %v", err)
	}

	if _, err := FromBytesStrict([]byte{0xff}); !errors.Is(err, ErrMalformedRequest) {
		t.Fatalf("garbage: got %v", err)
	}
}

// TestSigningPayloadCoversEveryEnvelopeField fails when a field is added to
// AxcpEnvelope without deciding how the signature covers it.
func TestSigningPayloadCoversEveryEnvelopeField(t *testing.T) {
	signed := map[protoreflect.FieldNumber]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true, 9: true, 10: true, 11: true, 22: true}
	// Bound through the transcript (20 sender, 21 timestamp, 23 recipient) or
	// deliberately excluded (100 signature, 101 attestation proof).
	transcript := map[protoreflect.FieldNumber]bool{20: true, 21: true, 23: true, 100: true, 101: true}

	fields := (&pb.AxcpEnvelope{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if !signed[fd.Number()] && !transcript[fd.Number()] {
			t.Fatalf("AxcpEnvelope field %d (%s) is not covered by the signing payload or transcript", fd.Number(), fd.Name())
		}
	}

	base := newTestEnvelope("trace")
	basePayload, err := SigningPayload(base)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*Envelope){
		"version":  func(e *Envelope) { e.Version = 2 },
		"trace_id": func(e *Envelope) { e.TraceId = "other" },
		"profile":  func(e *Envelope) { e.Profile = 2 },
		"payload":  func(e *Envelope) { e.GetContextPatch().BaseVersion = 9 },
		"type":     func(e *Envelope) { e.Payload = &pb.AxcpEnvelope_Error{Error: &pb.ErrorMessage{}} },
		"sequence": func(e *Envelope) { e.Sequence = 77 },
	}
	for name, mutate := range mutations {
		env := newTestEnvelope("trace")
		mutate(env)
		got, err := SigningPayload(env)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) == string(basePayload) {
			t.Errorf("signing payload does not cover %s", name)
		}
	}
}

func TestToProtocolError_DoesNotLeakInternalErrors(t *testing.T) {
	perr := ToProtocolError(errors.New("db password=hunter2 at /srv/secret"))
	if strings.Contains(perr.Reason, "hunter2") || strings.Contains(perr.Reason, "/srv") {
		t.Fatalf("internal error leaked: %q", perr.Reason)
	}
	for _, err := range []error{auth.ErrTooOld, auth.ErrDIDMismatch, auth.ErrNoAcceptableKey, auth.ErrDIDTooLong, auth.ErrInvalidDIDKey, auth.ErrReplayCapacity} {
		if got := ToProtocolError(err); got.Code == ErrUnknown.Code {
			t.Errorf("%v mapped to UNKNOWN", err)
		}
	}

	var typedNil *ProtocolError
	if errors.Is(typedNil, ErrUnknown) {
		t.Fatal("nil ProtocolError matched")
	}
	_ = typedNil.Error()
}

func TestFromBytesStrict_NestedOneof(t *testing.T) {
	p := newTestPeers(t)
	env := p.signedEnvelope(t, func(e *Envelope) {
		e.Payload = &pb.AxcpEnvelope_CapabilityMsg{CapabilityMsg: &pb.CapabilityMessage{
			Kind: &pb.CapabilityMessage_Ack{Ack: &pb.CapabilityAck{}},
		}}
	})
	raw, err := proto.Marshal(&env.AxcpEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FromBytesStrict(raw); err != nil {
		t.Fatalf("valid nested oneof rejected: %v", err)
	}

	// Append a second, different member of CapabilityMessage.kind inside the
	// capability payload (field 5 is merged, so this builds the ambiguous form).
	offer, _ := proto.Marshal(&pb.CapabilityMessage{Kind: &pb.CapabilityMessage_Offer{Offer: &pb.CapabilityOffer{}}})
	ambiguous := protowire.AppendBytes(protowire.AppendTag(append([]byte(nil), raw...), 5, protowire.BytesType), offer)
	if _, err := FromBytesStrict(ambiguous); !errors.Is(err, ErrMalformedRequest) {
		t.Fatalf("two members of a nested oneof: got %v", err)
	}
}
