package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mnemolet/liberida/internal/config"
)

const (
	// Default values
	defaultOllamaURL = "http://localhost:11434"
	defaultModel     = "llama2"
	defaultWorkspace = "liberida-workspace"

	stepWelcome = iota
	stepProvider
	stepProviderConfirm
	stepOllamaURL
	stepModel
	stepExecMode
	stepDirectory
	stepContainerImage
	stepComplete
	totalSteps = 8

	title = "LiberIda Setup Wizard\n"
)

var (
	modelChoices = []string{"llama2", "mistral", "codellama", "neural-chat", "phi", "tinyllama"}

	urlChoices = []string{
		fmt.Sprintf("Use default (%s)", defaultOllamaURL),
		"Enter custom URL",
	}

	modeChoices = []string{
		"Chat only (no file access)",
		"Local directory (restricted access)",
		"Docker container",
		"Podman container",
	}

	// Provider choices
	providerChoices = []string{
		"Ollama (local)",
		"OpenAI (cloud, paid, requires API key)",
		"Anthropic Claude (cloud, paid, requires API key)",
		"Google Gemini (cloud, paid, requires API key)",
	}

	// Welcome screen choices
	welcomeChoices = []string{
		"Start setup",
		"Exit",
	}
)

type Model struct {
	choices          []string
	cursor           int
	question         string
	step             int
	completed        bool
	provider         string
	selectedProvider string
	ollamaURL        string
	model            string
	execMode         string
	allowedDir       string
	containerImage   string
	configMgr        *config.Manager
	reader           *bufio.Reader
}

func InitialModel(cm *config.Manager) Model {
	// Load existing config if any
	cm.Load()
	existing := cm.Get()

	return Model{
		step:             stepWelcome,
		cursor:           0,
		completed:        false,
		provider:         existing.Provider,
		selectedProvider: "",
		ollamaURL:        existing.OllamaURL,
		model:            existing.Model,
		execMode:         string(existing.ExecutionMode),
		allowedDir:       existing.AllowedDir,
		containerImage:   existing.ContainerImage,
		configMgr:        cm,
		question:         "Welcome to LiberIda Setup!",
		choices:          welcomeChoices,
		reader:           bufio.NewReader(os.Stdin),
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
					m.step = stepProvider
					m.cursor = 0
					m.question = "Select AI Provider:"
					m.choices = providerChoices
				} else if m.cursor == 1 { // Exit
					return m, tea.Quit
				}
			case stepProvider:
				// Provider selection
				switch m.cursor {
				case 0: // Ollama (local)
					m.provider = "ollama"
					m.step = stepOllamaURL
					m.cursor = 0
					m.question = "Ollama URL:"
					m.choices = urlChoices

				case 1: // OpenAI
					m.provider = "openai"
					m.selectedProvider = "openai"
					m.step = stepProviderConfirm
					m.cursor = 0
					m.question = "OpenAI requires an API key and incurs usage costs.\nYou can add your API key later in ~/.liberida/config.toml"
					m.choices = []string{
						"Yes, continue with OpenAI",
						"No, go back to provider selection",
					}

				case 2: // Anthropic Claude
					m.provider = "anthropic"
					m.selectedProvider = "anthropic"
					m.step = stepProviderConfirm
					m.cursor = 0
					m.question = "Anthropic Claude requires an API key and incurs usage costs.\nYou can add your API key later in ~/.liberida/config.toml"
					m.choices = []string{
						"Yes, continue with Anthropic",
						"No, go back to provider selection",
					}

				case 3: // Google Gemini
					m.provider = "gemini"
					m.selectedProvider = "gemini"
					m.step = stepProviderConfirm
					m.cursor = 0
					m.question = "Google Gemini requires an API key and incurs usage costs.\nYou can add your API key later in ~/.liberida/config.toml"
					m.choices = []string{
						"Yes, continue with Gemini",
						"No, go back to provider selection",
					}
				}
			case stepProviderConfirm:
				if m.cursor == 0 {
					// User confirmed
					m.step = stepModel
					m.cursor = 0

					// Set appropriate model choices based on provider
					switch m.provider {
					case "openai":
						m.question = "Select OpenAI Model:"
						m.choices = []string{
							"gpt-5.4-pro",
							"gpt-5.4",
							"gpt-5.4-mini",
							"gpt-5.3-codex",
						}
					case "anthropic":
						m.question = "Select Claude Model:"
						m.choices = []string{
							"claude-opus-4.5",
							"claude-sonnet-4.5",
							"claude-haiku-4.5",
						}
					case "gemini":
						m.question = "Select Gemini Model:"
						m.choices = []string{
							"gemini-pro-2.5",
							"gemini-flash-2.5",
						}
					}
				} else {
					// User cancelled, go back to provider selection
					m.step = stepProvider
					m.cursor = 0
					m.question = "Select AI Provider:"
					m.choices = providerChoices
				}
			case stepOllamaURL:
				if m.cursor == 0 {
					m.ollamaURL = defaultOllamaURL

					// Try to fetch models from Ollama
					fmt.Print("\nConnecting to Ollama to fetch available models...")
					models, err := fetchOllamaModels(m.ollamaURL)
					if err != nil {
						// If we can't fetch, use default list
						fmt.Println("Could not connect to Ollama, using default model list.")
						log.Printf("Failed to fetch models: %v", err)
						m.choices = modelChoices
					} else {
						fmt.Println("Found", len(models), "models!")
						m.choices = models
					}

					m.step = stepModel
					m.cursor = 0
					m.question = "Select Model:"
				} else {
					m.step = 2
					m.cursor = 0
				}
			case stepModel:
				m.model = m.choices[m.cursor] //modelChoices[m.cursor]
				m.step = stepExecMode
				m.cursor = 0
				m.question = "Execution Mode:"
				m.choices = modeChoices
			case stepExecMode:
				modes := []string{
					string(config.ModeChatOnly),
					string(config.ModeLocal),
					string(config.ModeDocker),
					string(config.ModePodman),
				}
				m.execMode = modes[m.cursor]

				if m.execMode == string(config.ModeChatOnly) {
					m.saveConfig()
					m.step = stepComplete
					m.completed = true
				} else {
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
				}
			case stepDirectory:
				home, _ := os.UserHomeDir()
				if m.cursor == 0 {
					m.allowedDir = filepath.Join(home, defaultWorkspace)
				} else {
					m.allowedDir = filepath.Join(home, defaultWorkspace) // Default for now
				}

				m.saveConfig()
				m.step = stepComplete
				m.completed = true
			}
		}
	}
	return m, nil
}

// saveConfig persists the configuration to disk
func (m *Model) saveConfig() {
	cfg := m.configMgr.Get()
	cfg.Provider = m.provider
	cfg.OllamaURL = m.ollamaURL
	cfg.Model = m.model
	cfg.ExecutionMode = config.ExecutionMode(m.execMode)
	cfg.ContainerImage = m.containerImage

	// Only set AllowedDir if not in chat-only mode
	if m.execMode == string(config.ModeChatOnly) {
		cfg.AllowedDir = "" // Clear any existing directory for chat-only mode
	} else {
		cfg.AllowedDir = m.allowedDir
	}

	// Set container name for Docker/Podman modes
	if m.execMode == string(config.ModeDocker) || m.execMode == string(config.ModePodman) {
		if cfg.ContainerName == "" {
			cfg.ContainerName = "liberida-workspace"
		}
	}

	if err := m.configMgr.Save(); err != nil {
		// In a TUI, we can't easily show errors, so we'll log to file
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
	}
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
	s.WriteString(fmt.Sprintf("- Provider: %s\n", m.provider))
	s.WriteString(fmt.Sprintf("- Ollama URL: %s\n", m.ollamaURL))
	s.WriteString(fmt.Sprintf("- Model: %s\n", m.model))
	s.WriteString(fmt.Sprintf("- Execution mode: %s\n", m.execMode))

	if m.execMode != string(config.ModeChatOnly) {
		s.WriteString(fmt.Sprintf("- Workspace: %s\n", m.allowedDir))
	} else {
		s.WriteString("- Workspace: None (chat only)\n")
	}

	s.WriteString("\nPress q to exit.\n")
	return s.String()
}

func (m Model) Completed() bool {
	return m.completed
}

// OllamaModelsResponse represents the response from Ollama's /api/tags endpoint
type OllamaModelsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// fetchOllamaModels attempts to get the list of models from a running Ollama instance
func fetchOllamaModels(ollamaURL string) ([]string, error) {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(strings.TrimRight(ollamaURL, "/") + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var modelsResp OllamaModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var models []string
	for _, m := range modelsResp.Models {
		models = append(models, m.Name)
	}

	return models, nil
}
