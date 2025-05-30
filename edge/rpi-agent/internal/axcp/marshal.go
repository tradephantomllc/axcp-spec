package axcp

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ToBytes encodes an Envelope into protobuf binary.
func ToBytes(env *Envelope) ([]byte, error) {
	return proto.Marshal(&env.AxcpEnvelope)
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
