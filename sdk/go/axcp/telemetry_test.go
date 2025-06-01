package axcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tradephantom/axcp-spec/sdk/go/internal/pb"
)

func TestNewTelemetryDatagram(t *testing.T) {
	td := NewTelemetryDatagram()
	assert.NotNil(t, td)
	assert.Greater(t, td.TimestampMs, uint64(0))

	// Verifica che il timestamp sia approssimativamente corretto
	now := uint64(time.Now().UnixNano() / int64(time.Millisecond))
	assert.InDelta(t, now, td.TimestampMs, 1000) // Tolleranza di 1 secondo
}

func TestWithSystemStats(t *testing.T) {
	td := NewTelemetryDatagram()
	cpuPercent := uint32(75)
	memBytes := uint64(8 * 1024 * 1024 * 1024) // 8 GB
	tempC := uint32(45)

	result := WithSystemStats(td, cpuPercent, memBytes, tempC)

	// Verifica che il datagramma sia stato aggiornato correttamente
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

	// Verifica che il datagramma sia stato aggiornato correttamente
	tokens, ok := result.Payload.(*pb.TelemetryDatagram_Tokens)
	assert.True(t, ok, "Payload should be of type TokenUsage")
	assert.Equal(t, promptTokens, tokens.Tokens.PromptTokens)
	assert.Equal(t, completionTokens, tokens.Tokens.CompletionTokens)
}

func TestGetSystemStats(t *testing.T) {
	// Caso 1: Il datagramma contiene statistiche di sistema
	td1 := NewTelemetryDatagram()
	WithSystemStats(td1, 80, 4*1024*1024*1024, 50)

	stats := GetSystemStats(td1)
	assert.NotNil(t, stats)
	assert.Equal(t, uint32(80), stats.CpuPercent)

	// Caso 2: Il datagramma contiene altri dati
	td2 := NewTelemetryDatagram()
	WithTokenUsage(td2, 100, 200)

	stats = GetSystemStats(td2)
	assert.Nil(t, stats)
}

func TestGetTokenUsage(t *testing.T) {
	// Caso 1: Il datagramma contiene statistiche di utilizzo token
	td1 := NewTelemetryDatagram()
	WithTokenUsage(td1, 100, 200)

	tokens := GetTokenUsage(td1)
	assert.NotNil(t, tokens)
	assert.Equal(t, uint32(100), tokens.PromptTokens)
	assert.Equal(t, uint32(200), tokens.CompletionTokens)

	// Caso 2: Il datagramma contiene altri dati
	td2 := NewTelemetryDatagram()
	WithSystemStats(td2, 80, 4*1024*1024*1024, 50)

	tokens = GetTokenUsage(td2)
	assert.Nil(t, tokens)
}

// TestCombinedTelemetryOperations verifica che più operazioni di telemetria
// funzionino correttamente insieme, simulando un flusso di lavoro reale
func TestCombinedTelemetryOperations(t *testing.T) {
	// Crea un nuovo datagramma di telemetria
	td := NewTelemetryDatagram()

	// Aggiungi statistiche di sistema
	td = WithSystemStats(td, 75, 4*1024*1024*1024, 42)

	// Verifica che le statistiche di sistema siano state impostate correttamente
	system := GetSystemStats(td)
	assert.NotNil(t, system)
	assert.Equal(t, uint32(75), system.CpuPercent)

	// Il tipo di payload è stato cambiato, quindi non dovremmo più avere token usage
	tokens := GetTokenUsage(td)
	assert.Nil(t, tokens)

	// Ora cambiamo il payload in token usage
	td = WithTokenUsage(td, 150, 300)

	// Verifica che l'utilizzo dei token sia stato impostato correttamente
	tokens = GetTokenUsage(td)
	assert.NotNil(t, tokens)
	assert.Equal(t, uint32(150), tokens.PromptTokens)

	// Il tipo di payload è stato cambiato, quindi non dovremmo più avere system stats
	system = GetSystemStats(td)
	assert.Nil(t, system)
}
