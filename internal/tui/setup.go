package tui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/config"
)

const (
	// Default values
	defaultOllamaURL = "http://localhost:11434"
	defaultModel     = "llama2"
	defaultWorkspace = "liberida-workspace"

	stepWelcome = iota
	stepOllamaURL
	stepModel
	stepExecMode
	stepDirectory
	stepComplete
	totalSteps = 5

	title = "LiberIda Setup Wizard\n"
)

var (
	modelChoices = []string{"llama2", "mistral", "codellama", "neural-chat", "phi", "tinyllama"}

	urlChoices = []string{
		fmt.Sprintf("Use default (%s)", defaultOllamaURL),
		"Enter custom URL",
	}

	modeChoices = []string{
		"Local directory (restricted access)",
		"Docker container",
		"Podman container",
	}

	// Welcome screen choices
	welcomeChoices = []string{
		"Start setup",
		"Exit",
	}
)

type Model struct {
	choices    []string
	cursor     int
	question   string
	step       int
	completed  bool
	ollamaURL  string
	model      string
	execMode   string
	allowedDir string
	configMgr  *config.Manager
}

func InitialModel(cm *config.Manager) Model {
	// Load existing config if any
	cm.Load()
	existing := cm.Get()

	return Model{
		step:       stepWelcome,
		cursor:     0,
		completed:  false,
		ollamaURL:  existing.OllamaURL,
		model:      existing.Model,
		execMode:   string(existing.ExecutionMode),
		allowedDir: existing.AllowedDir,
		configMgr:  cm,
		question:   "Welcome to LiberIda Setup!",
		choices:    welcomeChoices,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", " ":
			switch m.step {
			case stepWelcome: // Welcome screen
				if m.cursor == 0 { // Start setup
					log.Println("Starting setup...")
					m.step = stepOllamaURL
					m.cursor = 0
					m.question = "Ollama URL:"
					m.choices = urlChoices
				} else if m.cursor == 1 { // Exit
					log.Println("Exiting...")
					return m, tea.Quit
				}
			case stepOllamaURL:
				if m.cursor == 0 {
					m.ollamaURL = defaultOllamaURL
					m.step = stepModel
					m.cursor = 0
					m.question = "Select Model:"
					m.choices = modelChoices
				} else {
					m.step = 2
					m.cursor = 0
				}
			case stepModel:
				m.model = modelChoices[m.cursor]
				m.step = stepExecMode
				m.cursor = 0
				m.question = "Execution Mode:"
				m.choices = modelChoices
			case stepExecMode:
				modes := []string{
					string(config.ModeLocal),
					string(config.ModeDocker),
					string(config.ModePodman),
				}
				m.execMode = modes[m.cursor]
				m.step = stepDirectory
				m.cursor = 0
				m.question = "Workspace Directory:"
				// Get home directory for default
				home, _ := os.UserHomeDir()
				defaultDir := filepath.Join(home, defaultWorkspace)
				m.choices = []string{
					fmt.Sprintf("Use default (%s)", defaultDir),
					"Enter custom path",
				}
			case stepDirectory:
				home, _ := os.UserHomeDir()
				if m.cursor == 0 {
					m.allowedDir = filepath.Join(home, defaultWorkspace)
				} else {
					m.allowedDir = filepath.Join(home, defaultWorkspace) // Default for now
				}

				// Save the configuration
				cfg := m.configMgr.Get()
				cfg.OllamaURL = m.ollamaURL
				cfg.Model = m.model
				cfg.AllowedDir = m.allowedDir

				if err := m.configMgr.Save(); err != nil {
					// We can't show error nicely in TUI, but we'll log it
					fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				}

				m.completed = true
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	// Header
	s.WriteString(title + "\n")

	if m.completed {
		return m.renderComplete()
	}

	// Don't show step counter for welcome screen
	if m.step != stepWelcome {
		s.WriteString(fmt.Sprintf("Step %d/%d\n\n", m.step, totalSteps))
	}

	// Show current question
	if m.question != "" {
		s.WriteString(m.question + "\n\n")
	}

	// Show choices
	for i, choice := range m.choices {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, choice))
	}

	// Footer
	s.WriteString("\n(up/down to move, Enter to select, q to quit)\n")

	return s.String()
}

func (m Model) renderComplete() string {
	var s strings.Builder
	s.WriteString("Setup complete!\n")
	s.WriteString("\nConfig saved to ~/.liberida/config.toml\n")
	s.WriteString("\nConfiguration:\n")
	s.WriteString(fmt.Sprintf("- Ollama URL: %s\n", m.ollamaURL))
	s.WriteString(fmt.Sprintf("- Model: %s\n", m.model))
	s.WriteString(fmt.Sprintf("- Execution mode: %s\n", m.execMode))
	s.WriteString(fmt.Sprintf("- Workspace: %s\n", m.allowedDir))
	s.WriteString("\nPress q to exit.\n")
	return s.String()
}

func (m Model) Completed() bool {
	return m.completed
}
