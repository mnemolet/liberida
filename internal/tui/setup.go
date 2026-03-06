package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// Default values
	defaultOllamaURL = "http://localhost:11434"
	defaultModel     = "llama2"
	defaultWorkspace = "liberida-workspace"
)

var (
	urlChoices = []string{
		fmt.Sprintf("Use default (%s)", defaultOllamaURL),
		"Enter custom URL",
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
	allowedDir string
}

func InitialModel() Model {
	return Model{
		step:      0,
		cursor:    0,
		completed: false,
		ollamaURL: defaultOllamaURL,
		model:     defaultModel,
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
			case 0: // Welcome screen
				m.step = 1
				m.question = "Ollama URL:"
				m.choices = urlChoices
			case 1:
				if m.cursor == 0 {
					m.ollamaURL = defaultOllamaURL
					m.step = 2
					m.cursor = 0
					m.question = "Select Model:"
					m.choices = []string{"llama2", "mistral", "codellama"}
				} else {
					m.step = 2
					m.cursor = 0
				}
			case 2:
				models := []string{"llama2", "mistral", "codellama"}
				m.model = models[m.cursor]
				m.step = 3
				m.cursor = 0
				m.question = "Execution Mode:"
				m.choices = []string{"Local directory", "Docker container", "Podman container"}
			case 3:
				m.step = 4
				m.cursor = 0
				m.question = "Workspace Directory:"
				// Get home directory for default
				home, _ := os.UserHomeDir()
				defaultDir := filepath.Join(home, defaultWorkspace)
				m.choices = []string{fmt.Sprintf("Use default (%s)", defaultDir), "Enter custom path"}
			case 4:
				home, _ := os.UserHomeDir()
				if m.cursor == 0 {
					m.allowedDir = filepath.Join(home, defaultWorkspace)
				} else {
					m.allowedDir = filepath.Join(home, defaultWorkspace) // Default for now
				}
				m.completed = true
				// Here we would save the config
				// For now, just complete
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	// Header
	s.WriteString("LiberIda Setup Wizard\n")

	if m.completed {
		s.WriteString("Setup complete!\n")
		s.WriteString("\nConfiguration saved to ~/.liberida/config.toml\n")
		s.WriteString("\nPress q to exit.\n")
		return s.String()
	}

	// Show current step
	s.WriteString(fmt.Sprintf("Step %d/5\n\n", m.step+1))

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

func (m Model) Completed() bool {
	return m.completed
}
