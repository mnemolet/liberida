package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/db"
	"github.com/mnemolet/liberida/internal/executor"
	"github.com/mnemolet/liberida/internal/llm"
	"github.com/mnemolet/liberida/internal/provider"
	"github.com/mnemolet/liberida/internal/version"
)

type ChatModel struct {
	input          textarea.Model
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

	toolCallDepth    int
	maxToolCallDepth int
}

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	viewportBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(0, 1)
	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	inputFocusedStyle = inputBorderStyle.
				BorderForeground(lipgloss.Color("205"))

	sidebarBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))
)

const (
	sidebarWidth            = 30
	defaultMaxToolCallDepth = 25
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
	input := textarea.New()
	input.Placeholder = "Type your message..."
	input.ShowLineNumbers = false
	input.FocusedStyle.CursorLine = lipgloss.NewStyle() // Fixes the double-spacing look
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
		switch msg.Role {
		case "user":
			displayMessages = append(displayMessages, "You: "+msg.Content)
		case "assistant":
			displayMessages = append(displayMessages, "AI: "+msg.Content)
		}
	}

	return &ChatModel{
		input:            input,
		messages:         displayMessages,
		sessionID:        sessionID,
		cfg:              cfg,
		prov:             prov,
		dbManager:        dbManager,
		exec:             exec,
		ctx:              ctx,
		cancel:           cancel,
		waiting:          false,
		prog:             nil,
		fullResponse:     strings.Builder{},
		messageHistory:   history,
		terminalWidth:    0,
		terminalHeight:   0,
		viewport:         viewport.New(0, 0),
		titleGenerated:   false,
		ready:            false,
		maxToolCallDepth: defaultMaxToolCallDepth,
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

		// Textarea needs to know its physical limit to wrap text
		// The internal width is total width minus the 2 border lines
		m.input.SetWidth(chatWidth - 2)
		m.input.SetHeight(3)
		m.input.MaxHeight = 3

		// We subtract the frame size so the INTERNAL content leaves room for the border.
		vpWidth := chatWidth - viewportBorderStyle.GetHorizontalFrameSize()

		// VpHeight math: Total - Header(1) - Input(3) - InputBorders(2) - VpBorders(2)
		vpHeight := msg.Height - 3 - 2 - viewportBorderStyle.GetVerticalFrameSize()

		if !m.ready {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}

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
	vpView := viewportBorderStyle.Render(m.viewport.View())

	// Select style based on focus
	style := inputBorderStyle
	if m.input.Focused() {
		style = inputFocusedStyle
	}

	inputView := style.
		Width(chatWidth - 2).
		Height(3).
		Render(m.input.View())

	mainCol := lipgloss.JoinVertical(
		lipgloss.Left,
		vpView,
		inputView,
	)

	// Sidebar
	sideCol := sidebarBorderStyle.
		Width(sidebarWidth - 2).
		Height(m.terminalHeight - 2).
		Render(m.renderSidebarContent())
	return lipgloss.JoinHorizontal(lipgloss.Top, mainCol, sideCol)
}

func (m *ChatModel) renderSidebarContent() string {
	var sb strings.Builder

	// Build the dynamic version string
	displayVersion := version.Version
	if displayVersion == "dev" && version.Commit != "none" {
		// If running a dev build, append the short commit hash for precision
		shortCommit := version.Commit
		if len(shortCommit) > 7 {
			shortCommit = shortCommit[:7]
		}
		displayVersion = fmt.Sprintf("dev-%s", shortCommit)
	} else if !strings.HasPrefix(displayVersion, "v") && displayVersion != "dev" {
		displayVersion = "v" + displayVersion
	}

	// App name
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
	titleText := fmt.Sprintf("%s %s", headerStyle.Render("Liberida"), versionStyle.Render(displayVersion))
	sb.WriteString(titleText)
	sb.WriteString("\n" + strings.Repeat("─", sidebarWidth-4) + "\n\n")

	sb.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("Session"))
	sb.WriteString("\n\n")

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Model:"))
	sb.WriteString("\n" + m.cfg.Model + "\n\n")

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Tokens:"))
	sb.WriteString("\nPrompt: " + strconv.Itoa(m.lastUsage.PromptTokens))
	sb.WriteString("\nCompl: " + strconv.Itoa(m.lastUsage.CompletionTokens))
	sb.WriteString("\nTotal: \n\n" + strconv.Itoa(m.lastUsage.TotalTokens))

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Cost:"))

	costStr := strconv.FormatFloat(m.lastUsage.EstimatedCost, 'f', 6, 64)
	sb.WriteString("\n$" + costStr)

	sb.WriteString("\n\n") // add whitespace

	// Environment Workspace Stats (Directory & Git Branch)
	sb.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("Workspace"))
	sb.WriteString("\n\n")

	// Get Working Directory
	dir, err := os.Getwd()
	if err != nil {
		dir = "Unknown"
	} else {
		parts := strings.Split(dir, string(os.PathSeparator))
		if len(parts) > 2 {
			dir = strings.Join(parts[len(parts)-2:], string(os.PathSeparator))
		}
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Directory:"))
	sb.WriteString("\n" + dir + "\n\n")

	// Get Git Branch Name
	branch := "Not a repository"
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	if output, err := cmd.Output(); err == nil {
		branch = strings.TrimSpace(string(output))
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Branch:"))
	sb.WriteString("\n" + branch + "\n")

	return sb.String()
}

func (m *ChatModel) startAIResponse(userInput string) {
	if strings.TrimSpace(userInput) != "" {
		m.toolCallDepth = 0
	} else {
		m.toolCallDepth++
	}
	if m.toolCallDepth > m.maxToolCallDepth {
		m.prog.Send(fmt.Errorf("tool call recursion depth exceeded (%d)", m.maxToolCallDepth))
		m.waiting = false
		return
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
