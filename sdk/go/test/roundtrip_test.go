package test

import (
  "testing"

  "github.com/google/uuid"
  "github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

func TestRoundTrip(t *testing.T) {
  orig := axcp.NewEnvelope(uuid.NewString(), 1) // Profile-1

  raw, err := axcp.ToBytes(orig)
  if err != nil {
    t.Fatalf("marshal: %v", err)
  }
  got, err := axcp.FromBytes(raw)
  if err != nil {
    t.Fatalf("unmarshal: %v", err)
  }

  if got.TraceId != orig.TraceId || got.Profile != orig.Profile {
    t.Fatal("round-trip mismatch")
  }
}