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
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/provider"
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

	// Handle Ctrl+C gracefully
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

	reader := bufio.NewReader(os.Stdin)
	var messages []provider.Message

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

		// Add user message to history
		messages = append(messages, provider.Message{Role: "user", Content: input})

		// Build request with last N messages based on context size
		reqMessages := getLastNMessages(messages, cfg.ContextSize)
		req := provider.Request{
			Model:    cfg.Model, // provider will use its default if empty
			Messages: reqMessages,
			Stream:   true,
		}

		fmt.Print("AI: ")
		chunkChan, err := prov.Stream(ctx, req)
		if err != nil {
			fmt.Printf("\nError: %v\n", err)
			// Remove the last user message?
			// For simplicity, we'll keep it but you might want to handle differently.
			continue
		}

		var fullResponse strings.Builder
		for chunk := range chunkChan {
			fmt.Print(chunk)
			fullResponse.WriteString(chunk)
		}
		fmt.Println()

		// Add assistant response to history
		messages = append(messages, provider.Message{Role: "assistant", Content: fullResponse.String()})
	}

	return nil
}

// getLastNMessages returns the last N messages, or all if len(messages) < N.
func getLastNMessages(messages []provider.Message, n int) []provider.Message {
	if n <= 0 || len(messages) <= n {
		return messages
	}
	return messages[len(messages)-n:]
}
