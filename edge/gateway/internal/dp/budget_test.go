package dp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetLookup(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "dp-budget-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test config file
	configPath := filepath.Join(tempDir, "test_budget.yaml")
	testConfig := `
version: v1
budgets:
  "*":
    epsilon: 1.0
    delta: 1e-5
    clip_norm: 10.0

  "telemetry/edge":
    epsilon: 0.5
    delta: 1e-6
    clip_norm: 5.0
`
	err = os.WriteFile(configPath, []byte(testConfig), 0644)
	require.NoError(t, err)

	// Test loading the config
	lookup, err := LoadBudget(configPath)
	require.NoError(t, err)
	require.NotNil(t, lookup)

	// Test getting default budget
	budget, err := lookup.ForTopic("unknown/topic")
	require.NoError(t, err)
	assert.Equal(t, 1.0, budget.Epsilon)
	assert.Equal(t, 1e-5, budget.Delta)
	assert.Equal(t, 10.0, budget.ClipNorm)

	// Test getting specific budget
	budget, err = lookup.ForTopic("telemetry/edge/device1")
	require.NoError(t, err)
	assert.Equal(t, 0.5, budget.Epsilon)
	assert.Equal(t, 1e-6, budget.Delta)
	assert.Equal(t, 5.0, budget.ClipNorm)

	// Test exact match
	budget, err = lookup.ForTopic("telemetry/edge")
	require.NoError(t, err)
	assert.Equal(t, 0.5, budget.Epsilon)

	// Test default budget
	defaultLookup := DefaultBudget()
	budget, err = defaultLookup.ForTopic("any/topic")
	require.NoError(t, err)
	assert.Equal(t, 1.0, budget.Epsilon)
}

func TestFindConfigFile(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "dp-config-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test with local config file
	testConfig := []byte(`version: v1
budgets: {}
`)

	// Test local file
	localPath := filepath.Join(tempDir, "dp_budget.yaml")
	err = os.WriteFile(localPath, testConfig, 0644)
	require.NoError(t, err)

	// Change to the temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	found, err := FindConfigFile()
	require.NoError(t, err)
	require.Equal(t, "dp_budget.yaml", filepath.Base(found))
}
