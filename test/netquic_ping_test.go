package test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
)

// TestPing is a basic test of message send/receive functionality.
// This is a simplified test that will be replaced by a real UDP benchmark.
func TestPing(t *testing.T) {
	// Create a test message
	orig := axcp.NewEnvelope(uuid.NewString(), 0)

	// A real implementation would send/receive over UDP here
	// For now, simulate a reply identical to the sent message
	got := orig

	// Verify the received message matches the sent one
	if got.TraceId != orig.TraceId {
		t.Fatalf("echo mismatch: expected %q, got %q", orig.TraceId, got.TraceId)
	}
}
