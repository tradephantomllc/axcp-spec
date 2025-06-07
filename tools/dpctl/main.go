package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	configFile string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "axcp-dpctl",
		Short: "CLI tool for managing differential privacy budgets",
		Long: `A command-line tool to manage differential privacy budgets for topics.
Use this tool to get, set, or list budget configurations.`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is $HOME/.axcp/dp_budget.yaml)")
	rootCmd.PersistentFlags().String("epsilon", "", "epsilon value for differential privacy")
	rootCmd.PersistentFlags().String("delta", "", "delta value for differential privacy")
	rootCmd.PersistentFlags().String("clip-norm", "", "clip norm value for differential privacy")

	// Add commands
	rootCmd.AddCommand(newGetCommand())
	rootCmd.AddCommand(newSetCommand())
	rootCmd.AddCommand(newListCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getConfigPath() string {
	if configFile != "" {
		return configFile
	}

	// Default config location
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".axcp", "dp_budget.yaml")
}

func loadConfig() (*BudgetLookup, error) {
	configPath := getConfigPath()
	
	// If config file doesn't exist, create a default one
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}

		// Create default config
		defaultConfig := BudgetConfig{
			Version: "v1",
			Budgets: map[string]Budget{
				"*": {
					Epsilon:  1.0,
					Delta:    1e-5,
					ClipNorm: 10.0,
				},
			},
		}

		lookup := NewBudgetLookup(defaultConfig)
		if err := lookup.Save(configPath); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}

		return lookup, nil
	}

	return Load(configPath)
}

func newGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get [topic]",
		Short: "Get budget for a topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := args[0]
			lookup, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			budget, err := lookup.ForTopic(topic)
			if err != nil {
				return fmt.Errorf("failed to get budget: %w", err)
			}

			fmt.Printf("Budget for topic '%s':\n", topic)
			fmt.Printf("  Epsilon:  %f\n", budget.Epsilon)
			fmt.Printf("  Delta:    %g\n", budget.Delta)
			fmt.Printf("  ClipNorm: %f\n", budget.ClipNorm)

			return nil
		},
	}
}

func newSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [topic]",
		Short: "Set budget for a topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := args[0]
			lookup, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get current budget or create a new one
			budget, _ := lookup.ForTopic(topic)
			if budget == nil {
				budget = &Budget{}
			}

			// Update with provided values
			if cmd.Flags().Changed("epsilon") {
				epsilon, _ := cmd.Flags().GetFloat64("epsilon")
				budget.Epsilon = epsilon
			}

			if cmd.Flags().Changed("delta") {
				delta, _ := cmd.Flags().GetFloat64("delta")
				budget.Delta = delta
			}

			if cmd.Flags().Changed("clip-norm") {
				clipNorm, _ := cmd.Flags().GetFloat64("clip-norm")
				budget.ClipNorm = clipNorm
			}

			// Update the config
			lookup.config.Budgets[topic] = *budget

			// Save the config
			if err := lookup.Save(getConfigPath()); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Updated budget for topic '%s'\n", topic)
			return nil
		},
	}

	// Add flags
	cmd.Flags().Float64("epsilon", 0, "Epsilon value for differential privacy")
	cmd.Flags().Float64("delta", 0, "Delta value for differential privacy")
	cmd.Flags().Float64("clip-norm", 0, "Clip norm value for differential privacy")

	return cmd
}

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured budgets",
		RunE: func(cmd *cobra.Command, args []string) error {
			lookup, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(lookup.config.Budgets) == 0 {
				fmt.Println("No budgets configured")
				return nil
			}

			fmt.Println("Configured budgets:")
			for topic, budget := range lookup.config.Budgets {
				fmt.Printf("\nTopic: %s\n", topic)
				fmt.Printf("  Epsilon:  %f\n", budget.Epsilon)
				fmt.Printf("  Delta:    %g\n", budget.Delta)
				fmt.Printf("  ClipNorm: %f\n", budget.ClipNorm)
			}

			return nil
		},
	}
}
