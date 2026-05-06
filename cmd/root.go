package main

import (
	"fmt"
	"os"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "liberida",
	Short:   "LiberIda - local AI Agent",
	Long:    `CLI LiberIda AI agent that runs locally using Ollama.`,
	Version: version.Short(),
	RunE: func(cmd *cobra.Command, args []string) error {
		manager := config.NewManager()
		if err := manager.Load(); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg := manager.Get()

		prov, err := createProvider(cfg)
		if err != nil {
			return err
		}

		sessionID, _ := cmd.Flags().GetUint("session")
		forceNew, _ := cmd.Flags().GetBool("new")
		noContext, _ := cmd.Flags().GetBool("no-context")
		if noContext {
			cfg.AutoContext = false
		}

		return runChatSession(prov, cfg, sessionID, forceNew)
	},
}

// Execute adds all child commands to the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().UintP("session", "s", 0, "Resume existing session by ID")
	rootCmd.PersistentFlags().Bool("new", false, "Force create new session (ignore --session)")
	rootCmd.PersistentFlags().Bool("no-context", false, "Disable automatic workspace context")
}
