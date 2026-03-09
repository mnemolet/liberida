package main

import (
	"fmt"
	"os"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "liberida",
	Short: "LiberIda - local AI Agent",
	Long:  `CLI LiberIda AI agent that runs locally using Ollama.`,
	Run: func(cmd *cobra.Command, args []string) {
		manager := config.NewManager()
		manager.Load()
		cfg := manager.Get()

		fmt.Println("LiberIda is ready!")
		fmt.Printf("Provider: %s\n", cfg.Provider)
		fmt.Printf("Using model: %s\n", cfg.Model)
		fmt.Printf("Working dir: %s\n", cfg.AllowedDir)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// For Global flags
}
