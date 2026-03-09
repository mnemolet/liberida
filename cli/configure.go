package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/tui"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure LiberIda",
	Long:  "Run the interactive TUI to configure LiberIda AI agent preferences",
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

func init() {
	rootCmd.AddCommand(configureCmd)
}
