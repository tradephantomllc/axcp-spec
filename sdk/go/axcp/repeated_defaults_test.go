package axcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func TestRepeatedDefaultsCrossSDKVector(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/sdk/repeated_defaults_vector.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Expected map[string]string `json:"expected"`
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(vector.Expected["envelope_b64"])
	if err != nil {
		t.Fatal(err)
	}
	env, err := FromBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	ids := env.GetCapabilityMsg().GetRequest().GetIds()
	if len(ids) != 3 || ids[0] != "" || ids[1] != "read" || ids[2] != "" {
		t.Fatalf("lost repeated defaults: %q", ids)
	}
	encoded, err := ToBytes(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, encoded) {
		t.Fatal("wire encoding differs from Python/TypeScript")
	}
	payload, err := SigningPayloadFromProto(&env.AxcpEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := base64.StdEncoding.DecodeString(vector.Expected["signing_payload_b64"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, expected) {
		t.Fatal("canonical payload differs from Python/TypeScript")
	}
}
