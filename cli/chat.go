package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/actions"
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
	case "openai":
		return provider.NewProvider("openai", "", cfg.Model, cfg.OpenAIAPIKey)
	case "anthropic":
		return provider.NewProvider("anthropic", "", cfg.Model, cfg.AnthropicAPIKey)
	case "gemini":
		return provider.NewProvider("gemini", "", cfg.Model, cfg.GeminiAPIKey)
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

func runChatSession(prov provider.Provider, cfg *config.Config, sessionID uint, forceNew bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nInterrupted. Exiting.")
		cancel()
	}()

	fmt.Printf("Starting chat session with %s (model: %s)\n", prov.Name(), cfg.Model)
	fmt.Println("Type '/exit' or '/quit' to end the session.")
	fmt.Println("------------------------------------------------")

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
	program := tea.NewProgram(model)
	model.SetProgram(program)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
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
		fmt.Printf("Resumed session: %s (ID: %d)\n", sess.Title, sess.ID)
		return sess, false, nil
	}

	// Create a placeholder session
	sess, err := dbManager.CreateSession("New Chat")
	if err != nil {
		return nil, false, fmt.Errorf("failed to create new session: %w", err)
	}
	return sess, true, nil
}

// executeAction performs a single operation using the executor.
func executeAction(exec executor.Executor, act actions.Action) {
	ctx := context.Background()

	switch act.Type {
	case actions.TypeWrite:
		err := exec.WriteFile(act.Path, []byte(act.Content))
		if err != nil {
			fmt.Printf("Error: Write %s: %v\n", act.Path, err)
		} else {
			fmt.Printf("Ok: Written to %s\n", act.Path)
		}

	case actions.TypeRead:
		data, err := exec.ReadFile(act.Path)
		if err != nil {
			fmt.Printf("Error: Read %s: %v\n", act.Path, err)
		} else {
			fmt.Printf("Ok: %s:\n%s\n", act.Path, string(data))
		}

	case actions.TypeDelete:
		err := exec.DeleteFile(act.Path)
		if err != nil {
			fmt.Printf("Error: Delete %s: %v\n", act.Path, err)
		} else {
			fmt.Printf("Ok: Deleted %s\n", act.Path)
		}

	case actions.TypeList:
		files, err := exec.ListFiles()
		if err != nil {
			fmt.Printf("Error: List files: %v\n", err)
		} else {
			fmt.Println("Ok: Files in workspace:")
			for _, f := range files {
				fmt.Printf("  - %s\n", f)
			}
		}

	case actions.TypeExec:
		output, err := exec.RunCommand(ctx, act.Command)
		if err != nil {
			fmt.Printf("Error: Command execution failed: %v\n", err)
			if output != "" {
				fmt.Printf("Output: %s\n", output)
			}
		} else {
			fmt.Printf("Ok: Command executed successfully:\n%s\n", output)
		}

	default:
		fmt.Printf("Error: Unknown action type: %s\n", act.Type)
	}
}

// getLastNMessages returns the last N messages, or all if len(messages) < N.
func getLastNMessages(messages []provider.Message, n int) []provider.Message {
	if n <= 0 || len(messages) <= n {
		return messages
	}
	return messages[len(messages)-n:]
}
