package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/attachment"
	"github.com/mnemolet/liberida/internal/config"
	workspace "github.com/mnemolet/liberida/internal/context"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/mnemolet/liberida/internal/executor"
	"github.com/mnemolet/liberida/internal/provider"
	"github.com/mnemolet/liberida/internal/tui"
)

var autoContextFlag bool

func createProvider(cfg *config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "ollama":
		return provider.NewOllamaProvider(cfg.OllamaURL, cfg.Model), nil
	case "openrouter":
		return provider.NewProvider("openrouter", "", cfg.Model, cfg.OpenRouterAPIKey)
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

func runChatSession(prov provider.Provider, cfg *config.Config, sessionID uint, forceNew bool, attachFiles []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nInterrupted. Exiting.")
		cancel()
	}()

	dbManager, exec, err := initializeResources(cfg)
	if err != nil {
		return fmt.Errorf("[ERROR]: %w", err)
	}
	defer dbManager.Close()
	defer exec.Close()

	// Prepare the Session
	currentSession, isNew, err := prepareSession(dbManager, sessionID, forceNew)
	if err != nil {
		return err
	}

	// Ghost Session Janitor
	defer func() {
		if isNew {
			msgs, _ := dbManager.GetMessages(currentSession.ID)
			if len(msgs) == 0 {
				_ = dbManager.DeleteSession(currentSession.ID)
				// No print needed, just silent cleanup
			}
		}
	}()

	if sessionID != 0 && !forceNew {
		currentSession, err = dbManager.GetSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session %d: %w", sessionID, err)
		}
		fmt.Printf("Resumed session: %s (ID: %d)\n", currentSession.Title, currentSession.ID)

		if len(currentSession.Messages) > 0 {
			fmt.Println("\n--- Previous conversation ---")
			for _, msg := range currentSession.Messages {
				if msg.Role == "user" {
					fmt.Printf("You: %s\n", msg.Message)
				} else {
					// cleanedMsg := cleanAIResponse(msg.Message)
					fmt.Printf("AI: %s\n", msg.Message)
				}
			}
			fmt.Println("--- Continuing ---")
		}
	}

	// Collect workspace context if enabled
	contextStr := getWorkspaceContext(cfg, exec)

	// Load system prompt
	systemMsgContent := cfg.GetSystemPrompt()

	if contextStr != "" {
		systemMsgContent = fmt.Sprintf("%s\n\n%s", systemMsgContent, contextStr)
	}

	// Process attached files
	if len(attachFiles) > 0 {
		handler := attachment.NewHandler()
		components, errs := handler.ProcessPaths(attachFiles)
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
		if len(components) > 0 {
			attachStr := handler.FormatComponents(components)
			systemMsgContent = fmt.Sprintf("%s\n\n%s", systemMsgContent, attachStr)
		}
	}

	// Build initial messages from existing session (excluding system message)
	var initialMessages []provider.Message
	if !isNew && currentSession != nil {
		for _, msg := range currentSession.Messages {
			initialMessages = append(initialMessages, provider.Message{
				Role:    msg.Role,
				Content: msg.Message,
			})
		}
	}

	var sessID uint
	if currentSession != nil {
		sessID = currentSession.ID
	}

	// Create TUI model
	model := tui.NewChatModel(cfg, prov, dbManager, exec, sessID, systemMsgContent, initialMessages, ctx, cancel)
	program := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(program)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	msgs, err := dbManager.GetMessages(currentSession.ID)
	if err != nil {
		return fmt.Errorf("failed to check session messages: %w", err)
	}

	// Only print the summary if there are messages.
	// If len(msgs) == 0, the deferred janitor will delete it, and we stay silent.
	if len(msgs) > 0 {
		finalSession, err := dbManager.GetSession(currentSession.ID)
		if err != nil {
			return fmt.Errorf("failed to reload session: %w", err)
		}
		fmt.Printf("%s\n", finalSession.Title)
		fmt.Printf("liberida --session %d\n", finalSession.ID)
	}

	return nil
}

// initializeResources handles the setup of the DB and local executor.
func initializeResources(cfg *config.Config) (*db.Manager, executor.Executor, error) {
	dbManager, err := db.NewManager(cfg.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		dbManager.Close()
		return nil, nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	exec, err := executor.NewLocal(cwd)
	if err != nil {
		dbManager.Close()
		return nil, nil, fmt.Errorf("failed to initialize local executor: %w", err)
	}

	return dbManager, exec, nil
}

func getWorkspaceContext(cfg *config.Config, exec executor.Executor) string {
	if !cfg.AutoContext || exec == nil {
		return ""
	}

	fmt.Print("Scanning workspace for context...")
	scanner := workspace.NewWorkspaceScanner()
	cwd, _ := os.Getwd()

	contextStr, err := scanner.CollectContext(exec, cwd)
	if err != nil {
		fmt.Printf("Could not collect workspace context: %v\n", err)
		return ""
	}

	fmt.Println("Done")
	const maxContextLength = 2000 // Increased for better LLM utility
	if len(contextStr) > maxContextLength {
		return contextStr[:maxContextLength] + "\n... (truncated)"
	}
	return contextStr
}

// prepareSession returns the session and a boolean indicating if it's a new, empty session.
func prepareSession(dbManager *db.Manager, sessionID uint, forceNew bool) (*db.ChatSession, bool, error) {
	if sessionID != 0 && !forceNew {
		sess, err := dbManager.GetSession(sessionID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to load session %d: %w", sessionID, err)
		}
		return sess, false, nil
	}

	// Create a placeholder session
	sess, err := dbManager.CreateSession("New Chat")
	if err != nil {
		return nil, false, fmt.Errorf("failed to create new session: %w", err)
	}
	return sess, true, nil
}

// getLastNMessages returns the last N messages, or all if len(messages) < N.
func getLastNMessages(messages []provider.Message, n int) []provider.Message {
	if n <= 0 || len(messages) <= n {
		return messages
	}
	return messages[len(messages)-n:]
}
