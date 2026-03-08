package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/actions"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/provider"
	"github.com/mnemolet/liberida/internal/sandbox"
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

	var chatCmd = &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session",
		Long:  "Start an interactive chat session with the configured AI provider.",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := config.NewManager()
			if err := manager.Load(); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			cfg := manager.Get()

			// Validate config has necessary fields
			if cfg.Provider != "ollama" {
				return fmt.Errorf("unsupported provider: %s (only ollama is currently supported)", cfg.Provider)
			}
			if cfg.OllamaURL == "" {
				return fmt.Errorf("ollama URL not configured (run 'ai-agent configure')")
			}
			if cfg.Model == "" {
				return fmt.Errorf("model not configured (run 'ai-agent configure')")
			}

			// Create provider
			var prov provider.Provider
			switch cfg.Provider {
			case "ollama":
				prov = provider.NewOllamaProvider(cfg.OllamaURL, cfg.Model)
			default:
				return fmt.Errorf("provider %s not implemented", cfg.Provider)
			}

			// Start chat session
			return runChatSession(prov, cfg)
		},
	}

	rootCmd.AddCommand(configureCmd, showConfigCmd, chatCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runChatSession(prov provider.Provider, cfg *config.Config) error {
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

	// Create sandbox if file operations are allowed
	var sb *sandbox.Sandbox
	if cfg.IsFileOperationAllowed() {
		var err error
		sb, err = sandbox.New(cfg.AllowedDir)
		if err != nil {
			return fmt.Errorf("failed to initialize sandbox: %w", err)
		}
		fmt.Printf("File operations allowed in: %s\n", cfg.AllowedDir)
	} else {
		fmt.Println("File operations are disabled (chat-only mode).")
	}

	reader := bufio.NewReader(os.Stdin)
	var messages []provider.Message

	// System message with instructions for file operations if allowed
	if sb != nil {
		systemMsg := provider.Message{
			Role: "system",
			Content: `You are an AI assistant that can perform file operations when requested.
To perform a file operation, output a JSON object on its own line with the following format:
{"type":"write","path":"filename.txt","content":"file content"}
{"type":"read","path":"filename.txt"}
{"type":"delete","path":"filename.txt"}
{"type":"list"}  // lists all files in workspace
Only use relative paths. Do not use absolute paths. Do not include any other text with the JSON.`,
		}
		messages = append(messages, systemMsg)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}
		input = strings.TrimSpace(input)

		if input == "/exit" || input == "/quit" {
			fmt.Println("Goodbye!")
			break
		}
		if input == "" {
			continue
		}

		messages = append(messages, provider.Message{Role: "user", Content: input})
		reqMessages := getLastNMessages(messages, cfg.ContextSize)
		req := provider.Request{
			Model:    cfg.Model,
			Messages: reqMessages,
			Stream:   true,
		}

		fmt.Print("AI: ")
		chunkChan, err := prov.Stream(ctx, req)
		if err != nil {
			fmt.Printf("\nError: %v\n", err)
			continue
		}

		var fullResponse strings.Builder
		for chunk := range chunkChan {
			fmt.Print(chunk)
			fullResponse.WriteString(chunk)
		}
		fmt.Println()

		messages = append(messages, provider.Message{Role: "assistant", Content: fullResponse.String()})

		// Execute any file operations requested in the response
		if sb != nil {
			actList, err := actions.Parse(fullResponse.String())
			if err == nil && len(actList) > 0 {
				fmt.Println()
				for _, act := range actList {
					executeAction(sb, act)
				}
			}
		}
	}
	return nil
}

// executeAction performs a single file operation using the sandbox.
func executeAction(sb *sandbox.Sandbox, act actions.Action) {
	switch act.Type {
	case actions.TypeWrite:
		err := sb.WriteFile(act.Path, []byte(act.Content))
		if err != nil {
			fmt.Printf("Error: Write %s: %v\n", act.Path, err)
		} else {
			fmt.Printf("Ok: Written to %s\n", act.Path)
		}
	case actions.TypeRead:
		data, err := sb.ReadFile(act.Path)
		if err != nil {
			fmt.Printf("Error: Read %s: %v\n", act.Path, err)
		} else {
			fmt.Printf("Ok: %s:\n%s\n", act.Path, string(data))
		}
	case actions.TypeDelete:
		err := sb.DeleteFile(act.Path)
		if err != nil {
			fmt.Printf("Error: Delete %s: %v\n", act.Path, err)
		} else {
			fmt.Printf("Ok: Deleted %s\n", act.Path)
		}
	case actions.TypeList:
		files, err := sb.ListFiles()
		if err != nil {
			fmt.Printf("Error: List files: %v\n", err)
		} else {
			fmt.Println("Ok: Files in workspace:")
			for _, f := range files {
				fmt.Printf("  - %s\n", f)
			}
		}
	}
}

// getLastNMessages returns the last N messages, or all if len(messages) < N.
func getLastNMessages(messages []provider.Message, n int) []provider.Message {
	if n <= 0 || len(messages) <= n {
		return messages
	}
	return messages[len(messages)-n:]
}
