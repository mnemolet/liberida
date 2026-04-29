package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/mnemolet/liberida/internal/export"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export chat sessions",
	Long:  "Export chat sessions to various formats (markdown, json)",
}

var exportSessionCmd = &cobra.Command{
	Use:   "session [session-id]",
	Short: "Export a specific session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid session ID: %w", err)
		}

		format, _ := cmd.Flags().GetString("format")
		output, _ := cmd.Flags().GetString("output")

		return exportSession(uint(sessionID), format, output)
	},
}

var exportCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Export the current/last session",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		output, _ := cmd.Flags().GetString("output")

		// Get the most recent session
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

		sessions, err := dbManager.ListSessions(1)
		if err != nil {
			return fmt.Errorf("failed to get sessions: %w", err)
		}

		if len(sessions) == 0 {
			return fmt.Errorf("no sessions found")
		}

		return exportSession(sessions[0].ID, format, output)
	},
}

var exportAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Export all sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		output, _ := cmd.Flags().GetString("output")

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

		sessions, err := dbManager.ListSessions(0) // 0 = all
		if err != nil {
			return fmt.Errorf("failed to get sessions: %w", err)
		}

		if len(sessions) == 0 {
			return fmt.Errorf("no sessions found")
		}

		// For MVP, export each session to separate files
		for _, session := range sessions {
			// Construct filename without extension
			filename := fmt.Sprintf("%s-session-%d", output, session.ID)
			err := exportSession(session.ID, format, filename)
			if err != nil {
				fmt.Printf("Warning: failed to export session %d: %v\n", session.ID, err)
			}
		}

		fmt.Printf("Exported %d sessions\n", len(sessions))
		return nil
	},
}

func exportSession(sessionID uint, formatStr, outputFile string) error {
	// Validate format
	var format export.Format
	switch formatStr {
	case "markdown", "md":
		format = export.FormatMarkdown
	case "json":
		format = export.FormatJSON
	default:
		return fmt.Errorf("unsupported format: %s (use 'markdown' or 'json')", formatStr)
	}

	// Load config and database
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

	// Get session with messages
	session, err := dbManager.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Convert to export format
	exportSession := &export.Session{
		ID:        session.ID,
		Title:     session.Title,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
		Provider:  cfg.Provider,
		Model:     cfg.Model,
		Messages:  make([]export.Message, len(session.Messages)),
	}

	for i, msg := range session.Messages {
		exportSession.Messages[i] = export.Message{
			Role:      msg.Role,
			Content:   msg.Message,
			CreatedAt: msg.CreatedAt,
		}
	}

	// Export
	exporter := export.NewExporter(export.ExportOptions{
		Format: format,
	})

	content, err := exporter.Export(exportSession)
	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}

	// Write output
	if outputFile == "" {
		// Print to stdout
		fmt.Println(content)
	} else {
		// Auto-add extension if not present (use format string directly)
		if !strings.HasSuffix(outputFile, "."+string(format)) {
			outputFile = outputFile + "." + string(format)
		}

		// Write to file
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Exported session %d to %s\n", sessionID, outputFile)
	}

	return nil
}

func init() {
	// Session subcommand
	exportSessionCmd.Flags().StringP("format", "f", "markdown", "Export format (markdown, json)")
	exportSessionCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	exportCmd.AddCommand(exportSessionCmd)

	// Current subcommand
	exportCurrentCmd.Flags().StringP("format", "f", "markdown", "Export format (markdown, json)")
	exportCurrentCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	exportCmd.AddCommand(exportCurrentCmd)

	// All subcommand
	exportAllCmd.Flags().StringP("format", "f", "markdown", "Export format (markdown, json)")
	exportAllCmd.Flags().StringP("output", "o", "export", "Output file prefix (e.g., export will create export-1.md, export-2.md)")
	exportCmd.AddCommand(exportAllCmd)

	rootCmd.AddCommand(exportCmd)
}
