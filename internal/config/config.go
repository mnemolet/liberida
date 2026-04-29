package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed prompts/default.md
var defaultPromptEmbed embed.FS

type Config struct {
	Provider         string `mapstructure:"provider"`
	OllamaURL        string `mapstructure:"ollama_url"`
	Model            string `mapstructure:"model"`
	ContextSize      int    `mapstructure:"context_size"`
	DBPath           string `mapstructure:"db_path"`
	AutoContext      bool   `mapstructure:"auto_context"`
	AutoTitle        bool   `mapstructure:"auto_title"`
	ShowUsage        bool   `mapstructure:"show_usage"`
	OpenRouterAPIKey string `mapstructure:"openrouter_api_key"`
	PromptFile       string `mapstructure:"prompt_file"`
}

func DefaultConfig(hp HomeDirProvider) *Config {
	home := hp.GetHomeDir()
	defaultDBPath := filepath.Join(home, ".liberida", "chat.db")

	return &Config{
		Provider:         "ollama",
		OllamaURL:        "http://localhost:11434",
		Model:            "llama3.2",
		ContextSize:      10,
		DBPath:           defaultDBPath,
		AutoContext:      false,
		AutoTitle:        false,
		ShowUsage:        true,
		OpenRouterAPIKey: "",
		PromptFile:       "prompts/default.md",
	}
}

// default and a user-defined file.
func (c *Config) GetSystemPrompt() string {
	// Try to read from User's PromptFile if it exists
	if c.PromptFile != "" {
		content, err := os.ReadFile(c.PromptFile)
		if err == nil {
			return string(content)
		}
	}

	// Fallback to embedded default
	data, err := defaultPromptEmbed.ReadFile("prompts/default.md")
	if err != nil {
		return "You are a helpful AI assistant." // Hard fallback
	}
	return string(data)
}

func ExpandPath(path string, hp HomeDirProvider) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home := hp.GetHomeDir()

	if path == "~" {
		return home
	}

	return filepath.Join(home, path[2:])
}

func (c *Config) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	if c.OllamaURL == "" && c.Provider == "ollama" {
		return fmt.Errorf("ollama URL is required for ollama provider")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}

	return nil
}

// GetWorkspaceDir returns the effective workspace directory.
// If AllowedDir is empty, returns the current working directory.
func (c *Config) GetWorkspaceDir() (string, error) {
	return os.Getwd()
}

// String returns a string representation of the config
func (c *Config) String() string {
	return fmt.Sprintf(`Configuration:
  Provider: %s
  Ollama URL: %s
  Model: %s
  Context Size: %d
  Auto Context: %v
  Auto Title: %v
  Show Usage: %v`,
		c.Provider,
		c.OllamaURL,
		c.Model,
		c.ContextSize,
		c.AutoContext,
		c.AutoTitle,
		c.ShowUsage,
	)
}
