package main

import (
	"fmt"
	"strconv"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/spf13/cobra"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show token usage statistics",
	Long:  "Display token usage and cost statistics for sessions",
}

var usageSessionCmd = &cobra.Command{
	Use:   "session [session-id]",
	Short: "Show usage for a specific session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid session ID: %w", err)
		}

		cfgManager := config.NewManager()
		if err := cfgManager.Load(); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg := cfgManager.Get()

		dbManager, err := db.NewManager(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer dbManager.Close()

		// Get session info
		session, err := dbManager.GetSession(uint(id))
		if err != nil {
			return fmt.Errorf("failed to get session: %w", err)
		}

		// Get usage stats
		usage, err := dbManager.GetSessionUsage(uint(id))
		if err != nil {
			return fmt.Errorf("failed to get usage: %w", err)
		}

		fmt.Printf("Session %d Usage Statistics:\n", id)
		fmt.Printf("  Title:             %s\n", session.Title)
		fmt.Printf("  Created:           %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Prompt tokens:     %d\n", usage.PromptTokens)
		fmt.Printf("  Completion tokens: %d\n", usage.CompletionTokens)
		fmt.Printf("  Total tokens:      %d\n", usage.TotalTokens)
		fmt.Printf("  Estimated cost:    $%.6f\n", usage.EstimatedCost)

		return nil
	},
}

var usageTotalCmd = &cobra.Command{
	Use:   "total",
	Short: "Show total usage across all sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgManager := config.NewManager()
		if err := cfgManager.Load(); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg := cfgManager.Get()

		dbManager, err := db.NewManager(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer dbManager.Close()

		// Get total usage stats
		usage, err := dbManager.GetTotalUsage()
		if err != nil {
			return fmt.Errorf("failed to get total usage: %w", err)
		}

		// Get session count
		sessions, err := dbManager.ListSessions(0)
		if err != nil {
			return fmt.Errorf("failed to get sessions: %w", err)
		}

		fmt.Println("Total Usage Statistics (All Sessions):")
		fmt.Printf("  Total sessions:    %d\n", len(sessions))
		fmt.Printf("  Prompt tokens:     %d\n", usage.PromptTokens)
		fmt.Printf("  Completion tokens: %d\n", usage.CompletionTokens)
		fmt.Printf("  Total tokens:      %d\n", usage.TotalTokens)
		fmt.Printf("  Estimated cost:    $%.6f\n", usage.EstimatedCost)

		return nil
	},
}

func init() {
	usageCmd.AddCommand(usageSessionCmd)
	usageCmd.AddCommand(usageTotalCmd)
	rootCmd.AddCommand(usageCmd)
}
