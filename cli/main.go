package main

import (
	"fmt"
	"os"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "liberida",
		Short: "LiberIda - local AI Agent",
		Run: func(cmd *cobra.Command, args []string) {
			manager := config.NewManager()
			manager.Load()
			cfg := manager.Get()

			fmt.Println("LiberIda is ready!")
			fmt.Printf("Using model: %s\n", cfg.Model)
			fmt.Printf("Working dir: %s\n", cfg.AllowedDir)
		},
	}

	var configureCmd = &cobra.Command{
		Use:   "configure",
		Short: "Configure LiberIda",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Setup wizard will go here")
			return nil
		},
	}

	var showConfigCmd = &cobra.Command{
		Use:   "show-config",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := config.NewManager()
			manager.Load()
			cfg := manager.Get()

			fmt.Printf("Ollama URL: %s\n", cfg.OllamaURL)
			fmt.Printf("Model: %s\n", cfg.Model)
			fmt.Printf("Allowed Dir: %s\n", cfg.AllowedDir)
			return nil
		},
	}

	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(showConfigCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(os.Stderr, err)
		os.Exit(1)
	}
}
