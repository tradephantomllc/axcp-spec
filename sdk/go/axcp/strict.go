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

// maxStrictDepth bounds recursion when scanning nested messages.
const maxStrictDepth = 32

// FromBytesStrict decodes an envelope for secure verification.
//
// In addition to normal protobuf decoding (which rejects invalid UTF-8), it
// rejects envelopes that carry two or more distinct members of any oneof
// (the payload or a nested oneof) on the wire, unknown fields at any nesting
// level and undefined enum values. These constructs are handled differently
// by protobuf libraries, so accepting them would let the same signed bytes
// mean different things to different SDKs. Repeated occurrences of the same
// oneof member are merged as usual.
func FromBytesStrict(raw []byte) (*Envelope, error) {
	md := (&pb.AxcpEnvelope{}).ProtoReflect().Descriptor()
	if err := checkOneofMembers(raw, md, 0); err != nil {
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

// checkOneofMembers scans raw message bytes and rejects two distinct members
// of the same oneof, recursing into known message fields. Occurrences of a
// singular message field are merged by protobuf decoders, so they are
// concatenated before the nested message is checked.
func checkOneofMembers(raw []byte, md protoreflect.MessageDescriptor, depth int) error {
	if depth > maxStrictDepth {
		return ErrMalformedRequest.WithReason("envelope nesting too deep")
	}
	members := map[protoreflect.FullName]protowire.Number{}
	merged := map[protowire.Number][]byte{}
	var mergedOrder []protoreflect.FieldDescriptor
	fields := md.Fields()
	for len(raw) > 0 {
		num, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			return ErrMalformedRequest.WithReason("envelope is not valid protobuf")
		}
		raw = raw[n:]
		m := protowire.ConsumeFieldValue(num, typ, raw)
		if m < 0 {
			return ErrMalformedRequest.WithReason("envelope is not valid protobuf")
		}
		value := raw[:m]
		raw = raw[m:]

		fd := fields.ByNumber(num)
		if fd == nil {
			continue // unknown fields are rejected after decoding
		}
		if od := fd.ContainingOneof(); od != nil && !od.IsSynthetic() {
			if prev, seen := members[od.FullName()]; seen && prev != num {
				return ErrMalformedRequest.WithReason("envelope carries more than one member of a oneof")
			}
			members[od.FullName()] = num
		}
		if fd.Message() == nil || fd.IsMap() || typ != protowire.BytesType {
			continue
		}
		inner, k := protowire.ConsumeBytes(value)
		if k < 0 {
			return ErrMalformedRequest.WithReason("envelope is not valid protobuf")
		}
		if fd.IsList() {
			if err := checkOneofMembers(inner, fd.Message(), depth+1); err != nil {
				return err
			}
			continue
		}
		if _, seen := merged[num]; !seen {
			mergedOrder = append(mergedOrder, fd)
		}
		merged[num] = append(merged[num], inner...)
	}
	for _, fd := range mergedOrder {
		if err := checkOneofMembers(merged[fd.Number()], fd.Message(), depth+1); err != nil {
			return err
		}
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
