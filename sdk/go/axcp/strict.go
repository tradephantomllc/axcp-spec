package axcp

import (
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
)

// ProtocolVersion is the only envelope version accepted by secure verification.
const ProtocolVersion = 1

// ProfileSecureBaselineNumber is the envelope profile number of secure-baseline-v1.
const ProfileSecureBaselineNumber = 1

// payloadFieldNumbers are the wire numbers of the AxcpEnvelope payload oneof.
var payloadFieldNumbers = map[protowire.Number]bool{4: true, 5: true, 6: true, 7: true, 8: true, 9: true, 10: true, 11: true}

// FromBytesStrict decodes an envelope for secure verification.
//
// In addition to normal protobuf decoding (which rejects invalid UTF-8), it
// rejects envelopes that carry two or more distinct payload members on the
// wire, unknown fields at any nesting level and undefined enum values. These
// constructs are handled differently by protobuf libraries, so accepting them
// would let the same signed bytes mean different things to different SDKs.
// Repeated occurrences of the same payload member are merged as usual.
func FromBytesStrict(raw []byte) (*Envelope, error) {
	if err := checkSinglePayloadMember(raw); err != nil {
		return nil, err
	}
	env, err := FromBytes(raw)
	if err != nil {
		return nil, ErrMalformedRequest.WithReason("envelope is not valid protobuf")
	}
	if err := ValidateEnvelopeStructure(env); err != nil {
		return nil, err
	}
	return env, nil
}

func checkSinglePayloadMember(raw []byte) error {
	var member protowire.Number
	for len(raw) > 0 {
		num, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			return ErrMalformedRequest.WithReason("envelope is not valid protobuf")
		}
		raw = raw[n:]
		if payloadFieldNumbers[num] {
			if member != 0 && member != num {
				return ErrMalformedRequest.WithReason("envelope carries more than one payload member")
			}
			member = num
		}
		m := protowire.ConsumeFieldValue(num, typ, raw)
		if m < 0 {
			return ErrMalformedRequest.WithReason("envelope is not valid protobuf")
		}
		raw = raw[m:]
	}
	return nil
}

// ValidateEnvelopeStructure checks the envelope fields that are not covered by
// signature verification but must be fixed for Secure Baseline: protocol
// version, exactly one payload member, no unknown fields and defined enum values.
// The profile is checked by the Validator against its allowed profiles.
func ValidateEnvelopeStructure(env *Envelope) error {
	if env == nil {
		return ErrMalformedRequest.WithReason("envelope is nil")
	}
	if env.GetVersion() != ProtocolVersion {
		return ErrUnsupportedVer.WithReason("unsupported envelope version")
	}
	if env.GetPayload() == nil {
		return ErrMalformedRequest.WithReason("envelope has no payload")
	}
	if err := checkMessageStrict(env.AxcpEnvelope.ProtoReflect()); err != nil {
		return err
	}
	return nil
}

func checkMessageStrict(m protoreflect.Message) error {
	if len(m.GetUnknown()) > 0 {
		return ErrMalformedRequest.WithReason("envelope contains unknown fields")
	}
	var err error
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		err = checkValueStrict(fd, v)
		return err == nil
	})
	return err
}

func checkValueStrict(fd protoreflect.FieldDescriptor, v protoreflect.Value) error {
	switch {
	case fd.IsList():
		list := v.List()
		for i := 0; i < list.Len(); i++ {
			if err := checkScalarOrMessage(fd, list.Get(i)); err != nil {
				return err
			}
		}
		return nil
	case fd.IsMap():
		var err error
		v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool {
			err = checkScalarOrMessage(fd.MapValue(), mv)
			return err == nil
		})
		return err
	default:
		return checkScalarOrMessage(fd, v)
	}
}

func checkScalarOrMessage(fd protoreflect.FieldDescriptor, v protoreflect.Value) error {
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return checkMessageStrict(v.Message())
	case protoreflect.EnumKind:
		if fd.Enum().Values().ByNumber(v.Enum()) == nil {
			return ErrMalformedRequest.WithReason("envelope contains an undefined enum value")
		}
	}
	return nil
}

// strictMarshalCheck reports whether the envelope can be marshalled (for
// example, all strings are valid UTF-8).
func strictMarshalCheck(env *pb.AxcpEnvelope) error {
	if _, err := proto.Marshal(env); err != nil {
		return ErrMalformedRequest.WithReason("envelope cannot be encoded")
	}
	return nil
}
