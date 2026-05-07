package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/mnemolet/liberida/internal/executor"
	"github.com/mnemolet/liberida/internal/llm"
	"github.com/mnemolet/liberida/internal/provider"
)

type ChatModel struct {
	input          textinput.Model
	messages       []string
	sessionID      uint
	cfg            *config.Config
	prov           provider.Provider
	dbManager      *db.Manager
	exec           executor.Executor
	ctx            context.Context
	cancel         context.CancelFunc
	waiting        bool
	prog           *tea.Program
	fullResponse   strings.Builder
	messageHistory []provider.Message // full conversation (system + user + assistant)
	terminalWidth  int
	terminalHeight int
	viewport       viewport.Model
	titleGenerated bool
}

func NewChatModel(
	cfg *config.Config,
	prov provider.Provider,
	dbManager *db.Manager,
	exec executor.Executor,
	sessionID uint,
	systemMessage string,
	initialMessages []provider.Message,
	ctx context.Context,
	cancel context.CancelFunc,
) *ChatModel {
	input := textinput.New()
	input.Placeholder = "Type your message..."
	input.Focus()
	input.CharLimit = 0
	input.Width = 80

	// Prepend system message to history
	history := make([]provider.Message, 0, len(initialMessages)+1)
	if systemMessage != "" {
		history = append(history, provider.Message{Role: "system", Content: systemMessage})
	}
	history = append(history, initialMessages...)

	vp := viewport.New(80, 20)

	return &ChatModel{
		input:          input,
		messages:       []string{}, // will be populated from initialMessages later
		sessionID:      sessionID,
		cfg:            cfg,
		prov:           prov,
		dbManager:      dbManager,
		exec:           exec,
		ctx:            ctx,
		cancel:         cancel,
		waiting:        false,
		prog:           nil,
		fullResponse:   strings.Builder{},
		messageHistory: history,
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       vp,
		titleGenerated: false,
	}
}

func (m *ChatModel) SetProgram(prog *tea.Program) {
	m.prog = prog
}

func (m *ChatModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit
		case "enter":
			if !m.waiting {
				userInput := strings.TrimSpace(m.input.Value())
				if userInput == "" {
					return m, nil
				}
				if userInput == "/exit" || userInput == "/quit" {
					return m, tea.Quit
				}
				m.input.SetValue("")
				m.messages = append(m.messages, fmt.Sprintf("You: %s", userInput))
				m.waiting = true
				m.messages = append(m.messages, "AI: ...")

				// Start AI response in background
				go m.startAIResponse(userInput)
				m.updateViewportContent()
				return m, nil
			}
		case "up":
			m.viewport.LineUp(1)
			return m, nil
		case "down":
			m.viewport.LineDown(1)
			return m, nil
		case "pgup":
			m.viewport.HalfViewUp()
			return m, nil
		case "pgdown":
			m.viewport.HalfViewDown()
			return m, nil
		}
	case string:
		// AI response chunk – append to the last AI message line
		if m.waiting && len(m.messages) > 0 {
			lastIdx := len(m.messages) - 1
			lastLine := m.messages[lastIdx]
			if lastLine == "AI: ..." {
				// Replace the placeholder with the first chunk
				m.messages[lastIdx] = "AI: " + msg
			} else if strings.HasPrefix(lastLine, "AI:") {
				// Append subsequent chunks to the same line
				m.messages[lastIdx] = lastLine + msg
			}
		} else {
			// Fallback (should not happen)
			m.messages = append(m.messages, "AI: "+msg)
		}
		m.updateViewportContent()
		return m, nil
	case error:
		if m.waiting {
			m.messages[len(m.messages)-1] = "AI: [Error] " + msg.Error()
			m.waiting = false
		}
		m.updateViewportContent()
		return m, nil
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		m.input.Width = msg.Width - 4
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4
		m.updateViewportContent()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *ChatModel) updateViewportContent() {
	var b strings.Builder
	width := m.terminalWidth
	if width < 40 {
		width = 40
	}
	for _, msg := range m.messages {
		wrapped := wrapText(msg, width)
		b.WriteString(wrapped)
		b.WriteString("\n")
	}
	m.viewport.SetContent(b.String())
}

func (m *ChatModel) View() string {
	var b strings.Builder
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n(ctrl+c to quit, ↑/↓ to scroll)")
	return b.String()
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	remaining := text
	for len(remaining) > width {
		// Find a space to break at
		split := width
		for split > 0 && split < len(remaining) && remaining[split] != ' ' {
			split--
		}
		if split == 0 {
			split = width
		}
		result.WriteString(remaining[:split])
		result.WriteString("\n")
		remaining = strings.TrimSpace(remaining[split:])
	}
	result.WriteString(remaining)
	return result.String()
}

func (m *ChatModel) startAIResponse(userInput string) {
	// Save user message synchronously
	if _, err := m.dbManager.AddMessage(m.sessionID, "user", userInput); err != nil {
		m.prog.Send(fmt.Errorf("failed to save user message: %w", err))
		m.waiting = false
		return
	}

	// Generate title from first user message
	if m.cfg.AutoTitle && !m.titleGenerated {
		// Get message count to check if this is the first message
		msgs, err := m.dbManager.GetMessages(m.sessionID)
		if err == nil && len(msgs) == 1 { // first message just saved
			title, err := llm.GenerateTitle(m.ctx, m.prov, userInput)
			if err != nil || title == "" {
				title = userInput
				if len(title) > 30 {
					title = title[:27] + "..."
				}
			}
			_ = m.dbManager.UpdateSessionTitle(m.sessionID, title)
			m.titleGenerated = true
		}
	}

	// Append user message to history
	userMsg := provider.Message{Role: "user", Content: userInput}
	m.messageHistory = append(m.messageHistory, userMsg)

	// Prepare the request with Tools
	req := provider.Request{
		Model:    m.cfg.Model,
		Messages: m.messageHistory,
		Tools:    executor.GetToolDefinitions(),
		Stream:   true,
	}

	chunkChan, usageChan, toolChan, err := m.prov.Stream(m.ctx, req)
	if err != nil {
		m.prog.Send(err)
		m.waiting = false
		return
	}

	// Process the Text Stream
	m.fullResponse.Reset()
	for chunk := range chunkChan {
		m.fullResponse.WriteString(chunk)
		// This sends the string to the Update() method to refresh the UI
		m.prog.Send(chunk)
	}

	fullText := m.fullResponse.String()

	// Check for Tool Calls after the stream closes
	// The provider sends these on toolChan after stitching fragments
	finalTools := <-toolChan
	if len(finalTools) > 0 {
		// Store the assistant's intent to call a tool in history
		m.messageHistory = append(m.messageHistory, provider.Message{
			Role:      "assistant",
			Content:   fullText,
			ToolCalls: finalTools,
		})

		for _, tc := range finalTools {
			// Parse arguments for UI display
			var displayInfo string
			switch tc.Function.Name {
			case "write_file":
				var args struct{ Path string }
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				displayInfo = fmt.Sprintf("Writing file: %s", args.Path)
			case "run_command":
				var args struct{ Command []string }
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				displayInfo = fmt.Sprintf("Running: %s", strings.Join(args.Command, " "))
			default:
				displayInfo = fmt.Sprintf("Executing: %s", tc.Function.Name)
			}

			// Send the specific action to the UI
			m.prog.Send("\n" + displayInfo)

			// Execute locally
			result, err := m.exec.ExecuteTool(m.ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				m.prog.Send(fmt.Sprintf("\nError: %v", err))
				result = fmt.Sprintf("Error: %v", err)
			} else {
				m.prog.Send(fmt.Sprintf("\n%s", result))
			}

			// Feed results back to history
			m.messageHistory = append(m.messageHistory, provider.Message{
				Role:    "tool",
				Content: result,
				ToolID:  tc.ID,
			})
		}

		// Recurse to let the AI process the tool results
		m.startAIResponse("")
		return
	}

	// Finalize turn if no tools were called
	if fullText != "" {
		// Save assistant message to DB
		if _, err := m.dbManager.AddMessage(m.sessionID, "assistant", fullText); err != nil {
			m.prog.Send(fmt.Errorf("failed to save assistant message: %w", err))
		}

		// Update history for the next turn
		m.messageHistory = append(m.messageHistory, provider.Message{
			Role:    "assistant",
			Content: fullText,
		})
	}

	// Handle Usage Display
	usage := <-usageChan
	if usage.TotalTokens > 0 && m.cfg.ShowUsage {
		m.prog.Send(fmt.Sprintf("\n[Tokens: %d prompt, %d completion, %d total | Cost: $%.6f]\n",
			usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, usage.EstimatedCost))
	}

	m.waiting = false
}
