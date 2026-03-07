package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/tui"
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
			fmt.Printf("Provider: %s\n", cfg.Provider)
			fmt.Printf("Using model: %s\n", cfg.Model)
			fmt.Printf("Working dir: %s\n", cfg.AllowedDir)
		},
	}

	var configureCmd = &cobra.Command{
		Use:   "configure",
		Short: "Configure LiberIda",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := config.NewManager()
			p := tea.NewProgram(tui.InitialModel(manager))

			finalModel, err := p.Run()
			if err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}

			if m, ok := finalModel.(tui.Model); ok && m.Completed() {
				fmt.Println("Setup completed successfully!")
				fmt.Printf("Configuration saved to: %s\n", manager.GetConfigPath())
			} else {
				fmt.Println("Setup cancelled.")
			}

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

			fmt.Printf("Provider: %s\n", cfg.Provider)
			fmt.Printf("Ollama URL: %s\n", cfg.OllamaURL)
			fmt.Printf("Model: %s\n", cfg.Model)
			fmt.Printf("Execution mode: %s\n", cfg.ExecutionMode)
			fmt.Printf("Allowed Dir: %s\n", cfg.AllowedDir)
			fmt.Printf("Context size: %v\n", cfg.ContextSize)
			return nil
		},
	}

	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(showConfigCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
