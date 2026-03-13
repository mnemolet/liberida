package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/mnemolet/liberida/internal/actions"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/mnemolet/liberida/internal/executor"
	"github.com/mnemolet/liberida/internal/provider"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long:  "Start an interactive chat session with the configured AI provider.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get session ID from flag
		sessionID, _ := cmd.Flags().GetUint("session")
		newSession, _ := cmd.Flags().GetBool("new")

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
		return runChatSession(prov, cfg, sessionID, newSession)
	},
}

func init() {
	chatCmd.Flags().Uint("session", 0, "Resume existing session by ID")
	chatCmd.Flags().Bool("new", false, "Force create new session (ignore --session)")
	rootCmd.AddCommand(chatCmd)
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

	// Init DB
	dbManager, err := db.NewManager(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dbManager.Close()

	// Handle session
	var currentSession *db.ChatSession
	var isNewSession bool

	if sessionID != 0 && !forceNew {
		// Load existing session
		currentSession, err = dbManager.GetSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session %d: %w", sessionID, err)
		}
		fmt.Printf("Resumed session: %s (ID: %d)\n", currentSession.Title, currentSession.ID)

		// Display previous messages
		if len(currentSession.Messages) > 0 {
			fmt.Println("\n--- Previous conversation ---")
			for _, msg := range currentSession.Messages {
				if msg.Role == "user" {
					fmt.Printf("You: %s\n", msg.Message)
				} else {
					cleanedMsg := cleanAIResponse(msg.Message)
					fmt.Printf("AI: %s\n", cleanedMsg)
				}
			}
			fmt.Println("--- Continuing ---")
		}
	} else {
		// New session will be created on first message
		isNewSession = true
		fmt.Println("New session will be created when you send your first message.")
	}

	// Create executor based on execution mode
	var exec executor.Executor
	if cfg.IsFileOperationAllowed() {
		switch cfg.ExecutionMode {
		case config.ModeLocal:
			exec, err = executor.NewLocal(cfg.AllowedDir)
			if err != nil {
				return fmt.Errorf("failed to initialize local executor: %w", err)
			}
			fmt.Printf("File operations allowed in: %s\n", cfg.AllowedDir)

		case config.ModePodman:
			exec, err = executor.NewPodman(cfg.ContainerName, cfg.ContainerImage, cfg.AllowedDir)
			if err != nil {
				return fmt.Errorf("failed to initialize Podman executor: %w", err)
			}
			fmt.Printf("Podman container '%s' ready with image %s\n", cfg.ContainerName, cfg.ContainerImage)
			fmt.Printf("Workspace mounted at: %s\n", cfg.AllowedDir)

		case config.ModeDocker:
			// For Docker, we'll use the same Podman executor with Docker socket
			// For now, return error until Docker executor is implemented
			return fmt.Errorf("Docker mode not yet implemented, please use Podman")

		default:
			return fmt.Errorf("unsupported execution mode: %s", cfg.ExecutionMode)
		}
		defer exec.Close()
	} else {
		fmt.Println("File operations are disabled (chat-only mode).")
	}

	reader := bufio.NewReader(os.Stdin)

	// Build messages slice from history
	messages := make([]provider.Message, 0)

	// System message with mode-appropriate instructions
	var systemMsg provider.Message
	if exec != nil {
		// Check if executor supports command execution (for exec action)
		supportsExec := cfg.ExecutionMode == config.ModePodman || cfg.ExecutionMode == config.ModeDocker

		execInstructions := ""
		if supportsExec {
			execInstructions = `
{"type":"exec","command":["ls","-la"]}  // runs a command in the container
{"type":"exec","command":["echo","hello"]}`
		}

		systemMsg = provider.Message{
			Role: "system",
			Content: fmt.Sprintf(`You are an AI assistant that can perform file operations when requested.
IMPORTANT: Never prefix your responses with "Assistant:" or "AI:". Just respond directly.

To perform a file operation, output a JSON object on its own line with the following format:
{"type":"write","path":"filename.txt","content":"file content"}
{"type":"read","path":"filename.txt"}
{"type":"delete","path":"filename.txt"}
{"type":"list"}  // lists all files in workspace
Only use relative paths. Do not use absolute paths. Do not include any other text with the JSON.`,
				execInstructions),
		}
	} else {
		systemMsg = provider.Message{
			Role: "system",
			Content: `You are an AI assistant in chat-only mode. 
IMPORTANT: Never prefix your responses with "Assistant:" or "AI:". Just respond directly.

You cannot create, read, modify, or delete files. Do not suggest file operations or pretend to execute them. Simply answer questions and chat with the user.`,
		}
	}
	messages = append(messages, systemMsg)

	// Add historical messages only if we have an existing session
	if !isNewSession && currentSession != nil {
		for _, msg := range currentSession.Messages {
			messages = append(messages, provider.Message{
				Role:    msg.Role,
				Content: msg.Message,
			})
		}
	}

	titleGenerated := false

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

		// Create new session if needed
		if isNewSession {
			currentSession, err = dbManager.CreateSession("")
			if err != nil {
				return err
			}
			fmt.Printf("New session created (ID: %d)\n", currentSession.ID)
			isNewSession = false
		}

		// Save user message to database
		_, err = dbManager.AddMessage(currentSession.ID, "user", input)
		if err != nil {
			fmt.Printf("Warning: failed to save message: %v\n", err)
		}

		// Generate title from first user message if not already set
		if !titleGenerated && len(currentSession.Messages) == 0 {
			newTitle := input
			if len(newTitle) > 30 {
				newTitle = newTitle[:27] + "..."
			}
			dbManager.UpdateSessionTitle(currentSession.ID, newTitle)
			titleGenerated = true
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

		// Clean the AI response
		rawResponse := fullResponse.String()
		cleanedResponse := cleanAIResponse(rawResponse)

		// Save cleaned AI response to database
		_, err = dbManager.AddMessage(currentSession.ID, "assistant", cleanedResponse)
		if err != nil {
			fmt.Printf("Warning: failed to save message: %v\n", err)
		}

		// Add cleaned response to messages slice
		messages = append(messages, provider.Message{Role: "assistant", Content: cleanedResponse})

		// Execute any actions requested in the response
		if exec != nil {
			actList, err := actions.Parse(fullResponse.String())
			if err == nil && len(actList) > 0 {
				fmt.Println()
				fmt.Println("The AI requested the following operations:")
				for _, act := range actList {
					fmt.Printf("- %s\n", act.String())
				}
				fmt.Print("Do you want to execute these? (y/n): ")
				confirm, _ := reader.ReadString('\n')
				confirm = strings.TrimSpace(strings.ToLower(confirm))
				if confirm == "y" || confirm == "yes" {
					for _, act := range actList {
						executeAction(exec, act)
					}
				} else {
					fmt.Println("Operations cancelled.")
				}
			}
		}
	}
	return nil
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

// cleanAIResponse aggressively removes role prefixes from AI responses
// Different models behave differently - this ensures consistent
// output regardless of the underlying model's quirks.
// It is idempotent - applying it multiple times is safe
func cleanAIResponse(response string) string {
	// Remove common role prefixes at the start of the string
	re := regexp.MustCompile(`^(?i)(assistant|ai)\s*[:.-]?\s*`)
	cleaned := re.ReplaceAllString(response, "")

	// Remove any leading spaces or newlines
	cleaned = strings.TrimSpace(cleaned)

	// If the cleaned string is empty, return the original
	if cleaned == "" {
		return response
	}

	return cleaned
}
