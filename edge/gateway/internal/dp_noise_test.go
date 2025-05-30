package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyNoiseToData(t *testing.T) {
	// Crea un oggetto TelemetryData con valori noti
	td := &TelemetryData{
		TimestampMs: 1621234567890,
		SystemStats: &SystemStats{
			CPUPercent:   50,
			MemBytes:     1024 * 1024 * 1024, // 1 GB
			TemperatureC: 40,
		},
		TokenUsage: &TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 200,
		},
		DifferentialDP: true, // Abilita l'applicazione del rumore differenziale
	}

	// Conserva i valori originali per confronto
	originalCPU := td.SystemStats.CPUPercent
	originalMem := td.SystemStats.MemBytes
	originalTemp := td.SystemStats.TemperatureC
	originalPrompt := td.TokenUsage.PromptTokens
	originalCompletion := td.TokenUsage.CompletionTokens

	// Applica il rumore differenziale
	ApplyNoiseToData(td)

	// Verifica che i valori siano stati modificati (con una tolleranza per i valori di rumore)
	// Il rumore è casuale, quindi non possiamo testare valori esatti, ma solo che siano stati modificati
	// entro un range accettabile
	if td.SystemStats.CPUPercent == originalCPU {
		t.Log("CPU value unchanged, but that's possible with noise")
	}
	assert.InDelta(t, originalCPU, td.SystemStats.CPUPercent, 6, "CPU should change within ±5%")

	if td.SystemStats.MemBytes == originalMem {
		t.Log("Memory value unchanged, but that's possible with noise")
	}
	// Tolleriamo una variazione fino al 3% per la memoria
	maxMemDelta := float64(originalMem) * 0.03
	assert.InDelta(t, originalMem, td.SystemStats.MemBytes, maxMemDelta, "Memory should change within ±3%")

	if td.SystemStats.TemperatureC == originalTemp {
		t.Log("Temperature value unchanged, but that's possible with noise")
	}
	assert.InDelta(t, originalTemp, td.SystemStats.TemperatureC, 2, "Temperature should change within ±1°C")

	if td.TokenUsage.PromptTokens == originalPrompt {
		t.Log("Prompt tokens unchanged, but that's possible with noise")
	}
	assert.InDelta(t, originalPrompt, td.TokenUsage.PromptTokens, 6, "Prompt tokens should change within ±5")

	if td.TokenUsage.CompletionTokens == originalCompletion {
		t.Log("Completion tokens unchanged, but that's possible with noise")
	}
	assert.InDelta(t, originalCompletion, td.TokenUsage.CompletionTokens, 11, "Completion tokens should change within ±10")
}

func TestApplyNoiseToData_NoDpEnabled(t *testing.T) {
	// Crea un oggetto TelemetryData con DP disabilitata
	td := &TelemetryData{
		TimestampMs: 1621234567890,
		SystemStats: &SystemStats{
			CPUPercent:   50,
			MemBytes:     1024 * 1024 * 1024, // 1 GB
			TemperatureC: 40,
		},
		TokenUsage: &TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 200,
		},
		DifferentialDP: false, // Disabilita l'applicazione del rumore differenziale
	}

	// Conserva i valori originali per confronto
	originalCPU := td.SystemStats.CPUPercent
	originalMem := td.SystemStats.MemBytes
	originalTemp := td.SystemStats.TemperatureC
	originalPrompt := td.TokenUsage.PromptTokens
	originalCompletion := td.TokenUsage.CompletionTokens

	// Applica il rumore differenziale (ma dovrebbe essere ignorato)
	ApplyNoiseToData(td)

	// Verifica che i valori NON siano stati modificati
	assert.Equal(t, originalCPU, td.SystemStats.CPUPercent, "CPU should not change when DP is disabled")
	assert.Equal(t, originalMem, td.SystemStats.MemBytes, "Memory should not change when DP is disabled")
	assert.Equal(t, originalTemp, td.SystemStats.TemperatureC, "Temperature should not change when DP is disabled")
	assert.Equal(t, originalPrompt, td.TokenUsage.PromptTokens, "Prompt tokens should not change when DP is disabled")
	assert.Equal(t, originalCompletion, td.TokenUsage.CompletionTokens, "Completion tokens should not change when DP is disabled")
}
