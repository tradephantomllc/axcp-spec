package main

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

// Load loads the budget configuration from a YAML file
func Load(path string) (*BudgetLookup, error) {
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

// Save saves the budget configuration to a YAML file
func (b *BudgetLookup) Save(path string) error {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := yaml.Marshal(b.config)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
