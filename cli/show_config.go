package main

import (
	"fmt"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/spf13/cobra"
)

var showConfigCmd = &cobra.Command{
	Use:   "show-config",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		manager := config.NewManager()
		manager.Load()
		cfg := manager.Get()

		fmt.Printf("Provider: %s\n", cfg.Provider)
		fmt.Printf("Ollama URL: %s\n", cfg.OllamaURL)
		fmt.Printf("Model: %s\n", cfg.Model)
		fmt.Printf("Execution mode: %s\n", cfg.ExecutionMode)
		fmt.Printf("Allowed Dir: %s\n", cfg.AllowedDir)
		fmt.Printf("Context size: %v\n", cfg.ContextSize)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(showConfigCmd)
}
