package axcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tradephantomllc/axcp-spec/sdk/go/internal/pb"
)

func TestNewTelemetryDatagram(t *testing.T) {
	td := NewTelemetryDatagram()
	assert.NotNil(t, td)
	assert.Greater(t, td.TimestampMs, uint64(0))

	// Verify the timestamp is approximately correct
	now := uint64(time.Now().UnixNano() / int64(time.Millisecond))
	assert.InDelta(t, now, td.TimestampMs, 1000) // 1 second tolerance
}

func TestWithSystemStats(t *testing.T) {
	td := NewTelemetryDatagram()
	cpuPercent := uint32(75)
	memBytes := uint64(8 * 1024 * 1024 * 1024) // 8 GB
	tempC := uint32(45)

	result := WithSystemStats(td, cpuPercent, memBytes, tempC)

	// Verify the datagram was updated correctly
	system, ok := result.Payload.(*pb.TelemetryDatagram_System)
	assert.True(t, ok, "Payload should be of type SystemStats")
	assert.Equal(t, cpuPercent, system.System.CpuPercent)
	assert.Equal(t, memBytes, system.System.MemBytes)
	assert.Equal(t, tempC, system.System.TemperatureC)
}

func TestWithTokenUsage(t *testing.T) {
	td := NewTelemetryDatagram()
	promptTokens := uint32(123)
	completionTokens := uint32(456)

	result := WithTokenUsage(td, promptTokens, completionTokens)

	// Verify the datagram was updated correctly
	tokens, ok := result.Payload.(*pb.TelemetryDatagram_Tokens)
	assert.True(t, ok, "Payload should be of type TokenUsage")
	assert.Equal(t, promptTokens, tokens.Tokens.PromptTokens)
	assert.Equal(t, completionTokens, tokens.Tokens.CompletionTokens)
}

func TestGetSystemStats(t *testing.T) {
	// Case 1: the datagram contains system stats
	td1 := NewTelemetryDatagram()
	WithSystemStats(td1, 80, 4*1024*1024*1024, 50)

	stats := GetSystemStats(td1)
	assert.NotNil(t, stats)
	assert.Equal(t, uint32(80), stats.CpuPercent)

	// Case 2: the datagram contains other data
	td2 := NewTelemetryDatagram()
	WithTokenUsage(td2, 100, 200)

	stats = GetSystemStats(td2)
	assert.Nil(t, stats)
}

func TestGetTokenUsage(t *testing.T) {
	// Case 1: the datagram contains token usage stats
	td1 := NewTelemetryDatagram()
	WithTokenUsage(td1, 100, 200)

	tokens := GetTokenUsage(td1)
	assert.NotNil(t, tokens)
	assert.Equal(t, uint32(100), tokens.PromptTokens)
	assert.Equal(t, uint32(200), tokens.CompletionTokens)

	// Case 2: the datagram contains other data
	td2 := NewTelemetryDatagram()
	WithSystemStats(td2, 80, 4*1024*1024*1024, 50)

	tokens = GetTokenUsage(td2)
	assert.Nil(t, tokens)
}

// TestCombinedTelemetryOperations verifies that multiple telemetry operations
// work correctly together, simulating a real workflow
func TestCombinedTelemetryOperations(t *testing.T) {
	// Create a new telemetry datagram
	td := NewTelemetryDatagram()

	// Add system stats
	td = WithSystemStats(td, 75, 4*1024*1024*1024, 42)

	// Verify the system stats were set correctly
	system := GetSystemStats(td)
	assert.NotNil(t, system)
	assert.Equal(t, uint32(75), system.CpuPercent)

	// The payload type changed, so token usage should no longer be present
	tokens := GetTokenUsage(td)
	assert.Nil(t, tokens)

	// Now switch the payload to token usage
	td = WithTokenUsage(td, 150, 300)

	// Verify the token usage was set correctly
	tokens = GetTokenUsage(td)
	assert.NotNil(t, tokens)
	assert.Equal(t, uint32(150), tokens.PromptTokens)

	// The payload type changed, so system stats should no longer be present
	system = GetSystemStats(td)
	assert.Nil(t, system)
}
