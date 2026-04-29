package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/mnemolet/liberida/internal/executor"
	"github.com/mnemolet/liberida/internal/llm"
	"github.com/mnemolet/liberida/internal/provider"
)

type ChatModel struct {
	input          textinput.Model
	messages       []string // UI display lines
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
				return m, nil
			}
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
		return m, nil
	case error:
		if m.waiting {
			m.messages[len(m.messages)-1] = "AI: [Error] " + msg.Error()
			m.waiting = false
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.input.Width = msg.Width - 4
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *ChatModel) View() string {
	var b strings.Builder
	width := m.terminalWidth
	if width < 40 { // min width for wrapping
		width = 40
	}
	for _, msg := range m.messages {
		wrapped := wrapText(msg, width)
		b.WriteString(wrapped)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n(ctrl+c to quit)")
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
	// Session creation (if new)
	if m.sessionID == 0 {
		session, err := m.dbManager.CreateSession("")
		if err != nil {
			m.prog.Send(fmt.Errorf("failed to create session: %w", err))
			m.waiting = false
			return
		}
		m.sessionID = session.ID
	}

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

	// Call AI
	req := provider.Request{
		Model:    m.cfg.Model,
		Messages: m.messageHistory,
		Stream:   true,
	}
	chunkChan, usageChan, err := m.prov.Stream(m.ctx, req)
	if err != nil {
		m.prog.Send(err)
		m.waiting = false
		return
	}

	// Stream AI response and accumulate
	m.fullResponse.Reset()
	for chunk := range chunkChan {
		m.fullResponse.WriteString(chunk)
		m.prog.Send(chunk)
	}
	fullText := m.fullResponse.String()

	// Save assistant message synchronously
	if _, err := m.dbManager.AddMessage(m.sessionID, "assistant", fullText); err != nil {
		m.prog.Send(fmt.Errorf("failed to save assistant message: %w", err))
		m.waiting = false
		return
	}

	// Handle token usage (if available)
	var usage provider.Usage
	select {
	case u, ok := <-usageChan:
		if ok {
			usage = u
		}
	case <-time.After(2 * time.Second):
		// timeout, ignore
	}
	if usage.TotalTokens > 0 && m.cfg.ShowUsage {
		pricing := provider.GetPricing(m.cfg.Provider, m.cfg.Model)
		usage.EstimatedCost = provider.CalculateCost(usage.PromptTokens, usage.CompletionTokens, pricing)
		// Save token usage
		msgs, _ := m.dbManager.GetMessages(m.sessionID)
		if len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			_ = m.dbManager.SaveTokenUsage(m.sessionID, lastMsg.ID, m.cfg.Provider, m.cfg.Model, usage)
		}
		// Display usage in the UI (as a message)
		m.prog.Send(fmt.Sprintf("\n[Tokens: %d prompt, %d completion, %d total | Cost: $%.6f]\n",
			usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, usage.EstimatedCost))
	}

	// Update message history for future turns
	assistantMsg := provider.Message{Role: "assistant", Content: fullText}
	m.messageHistory = append(m.messageHistory, assistantMsg)

	m.waiting = false
}
