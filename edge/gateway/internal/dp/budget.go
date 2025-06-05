package dp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Budget represents the differential privacy budget for a topic
type Budget struct {
	Epsilon  float64 `yaml:"epsilon"`
	Delta    float64 `yaml:"delta"`
	ClipNorm float64 `yaml:"clip_norm"`
}

// BudgetConfig represents the YAML configuration file structure
type BudgetConfig struct {
	Version string             `yaml:"version"`
	Budgets map[string]Budget `yaml:"budgets"`
}

// BudgetLookup provides methods to find budgets for topics
type BudgetLookup struct {
	config BudgetConfig
}

// NewBudgetLookup creates a new BudgetLookup from a BudgetConfig
func NewBudgetLookup(config BudgetConfig) *BudgetLookup {
	return &BudgetLookup{config: config}
}

// ForTopic finds the most specific budget for the given topic
func (b *BudgetLookup) ForTopic(topic string) (*Budget, error) {
	// Try to find the most specific match
	var bestMatch string
	var bestBudget Budget
	found := false

	for pattern, budget := range b.config.Budgets {
		// Check if the topic matches the pattern or if it's a prefix match
		if pattern == "*" || strings.HasPrefix(topic, pattern) {
			// If we haven't found a match yet, or if this pattern is more specific
			if !found || len(pattern) > len(bestMatch) {
				bestMatch = pattern
				bestBudget = budget
				found = true
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("no matching budget found for topic: %s", topic)
	}

	// Return a copy to prevent modification of the original
	result := bestBudget
	return &result, nil
}

// LoadBudget loads the budget configuration from a YAML file
func LoadBudget(path string) (*BudgetLookup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config BudgetConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return NewBudgetLookup(config), nil
}

// DefaultBudget returns a default budget configuration
func DefaultBudget() *BudgetLookup {
	return NewBudgetLookup(BudgetConfig{
		Version: "v1",
		Budgets: map[string]Budget{
			"*": {
				Epsilon:  1.0,
				Delta:    1e-5,
				ClipNorm: 10.0,
			},
		},
	})
}

// FindConfigFile searches for the budget configuration file in standard locations
func FindConfigFile() (string, error) {
	// Check current directory
	localConfig := "dp_budget.yaml"
	if _, err := os.Stat(localConfig); err == nil {
		return localConfig, nil
	}

	// Check config directory
	configDir := filepath.Join("config", "dp_budget.yaml")
	if _, err := os.Stat(configDir); err == nil {
		return configDir, nil
	}

	// Check home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	homeConfig := filepath.Join(home, ".axcp", "dp_budget.yaml")
	if _, err := os.Stat(homeConfig); err == nil {
		return homeConfig, nil
	}

	// Check /etc/axcp/
	systemConfig := "/etc/axcp/dp_budget.yaml"
	if _, err := os.Stat(systemConfig); err == nil {
		return systemConfig, nil
	}

	return "", fmt.Errorf("no configuration file found in standard locations")
}
