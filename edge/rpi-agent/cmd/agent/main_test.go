package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/tradephantomllc/axcp-spec/sdk/go/axcp/pb"
	"google.golang.org/protobuf/proto"
)

func TestReadTemperature(t *testing.T) {
	dir := t.TempDir()
	write := func(content string) string {
		path := filepath.Join(dir, "temp")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	cases := map[string]uint32{
		"48312\n": 48,
		"0":       0,
		"garbage": 0,
		"-5000":   0,
		"9999999": 0,
	}
	for content, want := range cases {
		if got := readTemperature(write(content)); got != want {
			t.Errorf("readTemperature(%q) = %d, want %d", content, got, want)
		}
	}
	if got := readTemperature(filepath.Join(dir, "missing")); got != 0 {
		t.Errorf("missing file = %d, want 0", got)
	}
}

func TestBuildTelemetryEnvelope(t *testing.T) {
	now := time.UnixMilli(1700000000123)
	stats := &pb.SystemStats{CpuPercent: 12, MemBytes: 1 << 20, TemperatureC: 48}
	env := buildTelemetryEnvelope("trace-1", now, stats)

	raw, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var decoded pb.AxcpEnvelope
	if err := proto.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	tel := decoded.GetTelemetry()
	if decoded.GetVersion() != 1 || decoded.GetTraceId() != "trace-1" || tel == nil {
		t.Fatalf("unexpected envelope: %v", &decoded)
	}
	if tel.GetTimestampMs() != 1700000000123 {
		t.Fatalf("timestamp = %d", tel.GetTimestampMs())
	}
	if !proto.Equal(tel.GetSystem(), stats) {
		t.Fatalf("stats = %v, want %v", tel.GetSystem(), stats)
	}
}
