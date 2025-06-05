package dp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
)

func createTestBudgetLookup(budgets map[string]Budget) *BudgetLookup {
	config := BudgetConfig{
		Version: "v1",
		Budgets: budgets,
	}
	return NewBudgetLookup(config)
}

func createTestSystemTelemetry(cpuPercent uint32) *pb.TelemetryDatagram {
	return &pb.TelemetryDatagram{
		Payload: &pb.TelemetryDatagram_System{
			System: &pb.SystemStats{
				CpuPercent: cpuPercent,
			},
		},
	}
}

func createTestTokenTelemetry(prompt, completion uint32) *pb.TelemetryDatagram {
	return &pb.TelemetryDatagram{
		Payload: &pb.TelemetryDatagram_Tokens{
			Tokens: &pb.TokenUsage{
				PromptTokens:     prompt,
				CompletionTokens: completion,
			},
		},
	}
}

func TestApplyNoise_SystemStats(t *testing.T) {
	td := createTestSystemTelemetry(50) // 50% CPU

	lookup := createTestBudgetLookup(map[string]Budget{
		"*": {Epsilon: 1.0, Delta: 1e-5, ClipNorm: 100.0}, // 100% max for CPU
	})

	originalCPU := td.GetSystem().CpuPercent
	err := ApplyNoise(td, "test", lookup)
	assert.NoError(t, err)
	
	// CPU should be between 0 and 100
	cpuAfter := td.GetSystem().CpuPercent
	assert.True(t, cpuAfter >= 0 && cpuAfter <= 100, "CPU should be between 0 and 100")
	assert.NotEqual(t, originalCPU, cpuAfter, "CPU value should be modified")
}

func TestApplyNoise_TokenUsage(t *testing.T) {
	td := createTestTokenTelemetry(100, 50) // 100 prompt tokens, 50 completion tokens

	lookup := createTestBudgetLookup(map[string]Budget{
		"*": {Epsilon: 1.0, Delta: 1e-5, ClipNorm: 1000.0},
	})

	originalPrompt := td.GetTokens().PromptTokens
	originalCompletion := td.GetTokens().CompletionTokens

	err := ApplyNoise(td, "test", lookup)
	assert.NoError(t, err)
	
	promptAfter := td.GetTokens().PromptTokens
	completionAfter := td.GetTokens().CompletionTokens

	assert.NotEqual(t, originalPrompt, promptAfter, "Prompt tokens should be modified")
	assert.NotEqual(t, originalCompletion, completionAfter, "Completion tokens should be modified")
}

func TestApplyNoise_NoBudget(t *testing.T) {
	td := createTestSystemTelemetry(50)

	lookup := createTestBudgetLookup(map[string]Budget{})

	err := ApplyNoise(td, "test", lookup)
	assert.NoError(t, err)
	assert.Equal(t, uint32(50), td.GetSystem().CpuPercent) // No budget, no noise
}

func TestApplyNoise_WithBudget(t *testing.T) {
	td := createTestSystemTelemetry(50)

	lookup := createTestBudgetLookup(map[string]Budget{
		"*": {Epsilon: 1.0, Delta: 1e-5, ClipNorm: 100.0},
	})

	originalCPU := td.GetSystem().CpuPercent
	err := ApplyNoise(td, "test", lookup)
	assert.NoError(t, err)
	
	// CPU should be between 0 and 100
	cpuAfter := td.GetSystem().CpuPercent
	assert.True(t, cpuAfter >= 0 && cpuAfter <= 100, "CPU should be between 0 and 100")
	assert.NotEqual(t, originalCPU, cpuAfter, "CPU value should be modified")
}

func TestApplyNoise_WithSpecificBudget(t *testing.T) {
	td := createTestTokenTelemetry(100, 50)

	lookup := createTestBudgetLookup(map[string]Budget{
		"specific": {Epsilon: 2.0, Delta: 1e-6, ClipNorm: 1000.0},
	})

	originalPrompt := td.GetTokens().PromptTokens
	err := ApplyNoise(td, "specific", lookup)
	assert.NoError(t, err)
	assert.NotEqual(t, originalPrompt, td.GetTokens().PromptTokens, "Token count should be modified")
}

func TestApplyNoise_WithClipping(t *testing.T) {
	td := createTestSystemTelemetry(150) // Above 100%

	lookup := createTestBudgetLookup(map[string]Budget{
		"*": {Epsilon: 1.0, Delta: 1e-5, ClipNorm: 100.0}, // Clip to 100%
	})

	err := ApplyNoise(td, "test", lookup)
	assert.NoError(t, err)
	assert.True(t, td.GetSystem().CpuPercent <= 100, "CPU should be clipped to 100%")
}

func TestApplyNoise_WithTokensClipping(t *testing.T) {
	td := createTestTokenTelemetry(5000, 1000) // Large token counts

	lookup := createTestBudgetLookup(map[string]Budget{
		"*": {Epsilon: 1.0, Delta: 1e-5, ClipNorm: 1000.0}, // Clip to 1000
	})

	err := ApplyNoise(td, "test", lookup)
	assert.NoError(t, err)
	assert.True(t, td.GetTokens().PromptTokens <= 1000, "Prompt tokens should be clipped to 1000")
	assert.True(t, td.GetTokens().CompletionTokens <= 1000, "Completion tokens should be clipped to 1000")
}
