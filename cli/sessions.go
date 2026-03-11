package main

import (
	"fmt"
	"strconv"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage chat sessions",
	Long:  "List, view, and manage saved chat sessions.",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		dbManager, err := initSessionCommand()
		if err != nil {
			return err
		}
		defer dbManager.Close()

		sessions, err := dbManager.ListSessions(20) // last 20
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			fmt.Println("No saved sessions found.")
			return nil
		}

		fmt.Println("Saved Sessions:")
		for _, s := range sessions {
			fmt.Printf("  [%d] %s (%d messages) - %s\n",
				s.ID,
				truncateString(s.Title, 40),
				len(s.Messages),
				s.UpdatedAt.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show [session-id]",
	Short: "Show a specific session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid session ID: %w", err)
		}

		dbManager, err := initSessionCommand()
		if err != nil {
			return err
		}
		defer dbManager.Close()

		session, err := dbManager.GetSession(uint(id))
		if err != nil {
			return err
		}

		fmt.Printf("Session: %s\n", session.Title)
		fmt.Printf("Created: %s\n", session.CreatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("Messages: %d\n", len(session.Messages))
		fmt.Println("\n--- Conversation ---")

		for _, msg := range session.Messages {
			prefix := "User:"
			if msg.Role == "assistant" {
				prefix = "AI:"
			}
			fmt.Printf("%s [%s]: %s\n",
				prefix,
				msg.CreatedAt.Format("15:04"),
				truncateString(msg.Message, 80))
		}
		return nil
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete [session-id]",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid session ID: %w", err)
		}

		dbManager, err := initSessionCommand()
		if err != nil {
			return err
		}
		defer dbManager.Close()

		if err := dbManager.DeleteSession(uint(id)); err != nil {
			return err
		}

		fmt.Printf("Session %d deleted.\n", id)
		return nil
	},
}

// Helper to initialize config and db for session commands
func initSessionCommand() (*db.Manager, error) {
	cfgManager := config.NewManager()
	if err := cfgManager.Load(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.Get()

	dbManager, err := db.NewManager(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return dbManager, nil
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	rootCmd.AddCommand(sessionsCmd)
}
