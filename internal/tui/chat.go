package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	ready          bool
	lastUsage      provider.Usage
}

const (
	headerHeight = 1
	footerHeight = 3
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	viewportBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(0, 1)

	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(0, 1)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	aiMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	sidebarBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))
)

const (
	sidebarWidth = 30
)

type usageUpdateMsg provider.Usage

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

	// Prepend system message to history
	history := make([]provider.Message, 0, len(initialMessages)+1)
	if systemMessage != "" {
		history = append(history, provider.Message{Role: "system", Content: systemMessage})
	}
	history = append(history, initialMessages...)

	// Populate initial messages for display
	displayMessages := make([]string, 0, len(initialMessages))
	for _, msg := range initialMessages {
		if msg.Role == "user" {
			displayMessages = append(displayMessages, fmt.Sprintf("You: %s", msg.Content))
		} else if msg.Role == "assistant" {
			displayMessages = append(displayMessages, fmt.Sprintf("AI: %s", msg.Content))
		}
	}

	return &ChatModel{
		input:          input,
		messages:       displayMessages,
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
		terminalWidth:  0,
		terminalHeight: 0,
		viewport:       viewport.New(0, 0),
		titleGenerated: false,
		ready:          false,
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
				m.viewport.GotoBottom()
				return m, nil
			}
		case "up":
			m.viewport.ScrollUp(1)
			return m, nil
		case "down":
			m.viewport.ScrollDown(1)
			return m, nil
		case "pgup":
			m.viewport.HalfPageUp()
			return m, nil
		case "pgdown":
			m.viewport.HalfPageDown()
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
		m.viewport.GotoBottom()
		return m, nil
	case error:
		if m.waiting {
			m.messages[len(m.messages)-1] = "AI: [Error] " + msg.Error()
			m.waiting = false
		}
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height

		chatWidth := msg.Width - sidebarWidth

		// FrameSize includes Borders + Padding + Margin.
		// We subtract the frame size so the INTERNAL content leaves room for the border.
		vpWidth := chatWidth - viewportBorderStyle.GetHorizontalFrameSize()
		vpHeight := msg.Height - headerHeight - footerHeight - viewportBorderStyle.GetVerticalFrameSize()

		if !m.ready {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}

		// We must subtract the frame size plus the prompt length (e.g., "> ")
		m.input.Width = chatWidth - inputBorderStyle.GetHorizontalFrameSize() - 3
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil
	case usageUpdateMsg:
		m.lastUsage = provider.Usage(msg)
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *ChatModel) updateViewportContent() {
	var b strings.Builder
	// Subtract frame size to ensure text doesn't touch the borders
	availableWidth := m.viewport.Width - 2

	for _, msg := range m.messages {
		// Cleanly wrap the text to the viewport width
		wrapped := lipgloss.NewStyle().Width(availableWidth).Render(msg)
		b.WriteString(wrapped + "\n")
	}
	m.viewport.SetContent(b.String())
}

func (m *ChatModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	chatWidth := m.terminalWidth - sidebarWidth

	// Render Columns as usual
	header := headerStyle.Width(chatWidth).Render("Liberida Chat")
	vpView := viewportBorderStyle.Width(chatWidth - viewportBorderStyle.GetHorizontalFrameSize()).Render(m.viewport.View())
	inputView := inputBorderStyle.Width(chatWidth - inputBorderStyle.GetHorizontalFrameSize()).Render(m.input.View())
	mainCol := lipgloss.JoinVertical(lipgloss.Left, header, vpView, inputView)

	// Prepare Sidebar with explicit height to match chat
	sideCol := sidebarBorderStyle.
		Width(sidebarWidth - sidebarBorderStyle.GetHorizontalFrameSize()).
		Height(m.terminalHeight - sidebarBorderStyle.GetVerticalFrameSize()).
		Render(m.renderSidebarContent())

	// The main column is in a fixed-width container.
	// This ensures mainCol CANNOT push the sidebar, even if content is buggy.
	chatContainer := lipgloss.NewStyle().
		Width(chatWidth).
		MaxWidth(chatWidth).
		Render(mainCol)

	return lipgloss.JoinHorizontal(lipgloss.Top, chatContainer, sideCol)
}

func (m *ChatModel) renderSidebarContent() string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("SESSION INFO"))
	sb.WriteString("\n\n")

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Model:"))
	sb.WriteString(fmt.Sprintf("\n%s\n\n", m.cfg.Model))

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Tokens:"))
	sb.WriteString(fmt.Sprintf("\nPrompt: %d", m.lastUsage.PromptTokens))
	sb.WriteString(fmt.Sprintf("\nCompl:  %d", m.lastUsage.CompletionTokens))
	sb.WriteString(fmt.Sprintf("\nTotal:  %d\n\n", m.lastUsage.TotalTokens))

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Cost:"))
	sb.WriteString(fmt.Sprintf("\n$%.6f", m.lastUsage.EstimatedCost))

	return sb.String()
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
	m.prog.Send(usageUpdateMsg(usage))

	m.waiting = false
}
