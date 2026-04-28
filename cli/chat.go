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
	"time"

	"github.com/mnemolet/liberida/internal/actions"
	"github.com/mnemolet/liberida/internal/config"
	workspace "github.com/mnemolet/liberida/internal/context"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/mnemolet/liberida/internal/executor"
	"github.com/mnemolet/liberida/internal/llm"
	"github.com/mnemolet/liberida/internal/provider"
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

	// Determine workspace directory (current working directory)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	workspaceDir := cwd

	// Create local executor
	exec, err := executor.NewLocal(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to initialize local executor: %w", err)
	}
	defer exec.Close()
	fmt.Printf("File operations allowed in: %s\n", workspaceDir)

	reader := bufio.NewReader(os.Stdin)

	// Build messages slice from history
	messages := make([]provider.Message, 0)

	// Collect workspace context if enabled
	// TODO: need to fix!
	var contextStr string
	if cfg.AutoContext && exec != nil {
		fmt.Print("Scanning workspace for context...")
		scanner := workspace.NewWorkspaceScanner()
		contextStr, err = scanner.CollectContext(exec, workspaceDir)
		if err != nil {
			fmt.Printf("Could not collect workspace context: %v\n", err)
		} else {
			fmt.Println("Done")
			// Truncate if too long
			const maxContextLength = 500
			if len(contextStr) > maxContextLength {
				contextStr = contextStr[:maxContextLength] + "\n... (truncated)"
			}
		}
	}

	// System message with mode-appropriate instructions
	var systemMsg provider.Message

	systemMsgContent := fmt.Sprint(`You are a helpful AI assistant. 
You can answer questions and also perform file operations when asked.
CRITICAL INSTRUCTION:
- For NORMAL questions: Respond with plain text, just like a normal conversation
- Never output JSON for normal questions
- Never prefix your responses with "Assistant:" or "AI:". Just respond directly.

NORMAL CONVERSATION EXAMPLE (what you should do 99% of the time):
User: "What is Python?"
You: Python is a high-level, interpreted programming language known for its simplicity...

User: "Explain variables"
You: A variable is a container for storing data values...
User: "who are you?"
You: I'm an AI assistant that can help you with programming and general questions...`)

	if contextStr != "" {
		systemMsgContent = fmt.Sprintf("%s\n\n%s", systemMsgContent, contextStr)
	}

	systemMsg = provider.Message{
		Role:    "system",
		Content: systemMsgContent,
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

		// Generate title from first user message
		if cfg.AutoTitle && !titleGenerated && len(currentSession.Messages) == 0 {
			title, err := llm.GenerateTitle(ctx, prov, input)
			if err != nil || title == "" {
				// Fallback to truncated first message
				title = input
				if len(title) > 30 {
					title = title[:27] + "..."
				}
				fmt.Printf("\nUsing fallback title: %s\n", title)
			} else {
				fmt.Printf("\nGenerated title: %s\n", title)
			}

			if err := dbManager.UpdateSessionTitle(currentSession.ID, title); err != nil {
				fmt.Printf("Warning: failed to save title: %v\n", err)
			}
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
		// fmt.Printf("\n[DEBUG] Sending %d messages to Ollama\n", len(reqMessages))
		// for i, m := range reqMessages {
		// 	fmt.Printf("[DEBUG] Message %d: role=%s, content preview=%s\n", i, m.Role, truncateString(m.Content, 50))
		// }
		chunkChan, usageChan, err := prov.Stream(ctx, req)
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

		// Get usage information
		var usage provider.Usage
		gotUsage := false // Track if we actually got data

		select {
		case u, ok := <-usageChan:
			if ok {
				usage = u
				gotUsage = true
			}
		case <-time.After(2 * time.Second):
			// Timeout
			fmt.Println("[DEBUG] Timeout: No usage data received after 2 seconds")
		}

		// Only do this if we actually got usage data
		if gotUsage {
			// Calculate cost based on provider and model
			if usage.TotalTokens > 0 {
				pricing := provider.GetPricing(cfg.Provider, cfg.Model)
				usage.EstimatedCost = provider.CalculateCost(usage.PromptTokens, usage.CompletionTokens, pricing)
				// Get the message ID of the assistant's response
				// We need to get the most recent message for this session
				messages, err := dbManager.GetMessages(currentSession.ID)
				if err == nil && len(messages) > 0 {
					lastMessage := messages[len(messages)-1]
					err = dbManager.SaveTokenUsage(currentSession.ID, lastMessage.ID, cfg.Provider, cfg.Model, usage)
					if err != nil {
						fmt.Printf("Warning: failed to save token usage: %v\n", err)
					}
				}
			}

			// Display usage information
			if cfg.ShowUsage {
				fmt.Printf("\n[Tokens: %d prompt, %d completion, %d total | Cost: $%.6f]\n",
					usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, usage.EstimatedCost)
			}
		}

		// Clean the AI response
		rawResponse := fullResponse.String()
		// fmt.Printf("RAW response: %s\n", rawResponse)
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
