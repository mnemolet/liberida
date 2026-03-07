package config

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
)

type ExecutionMode string

const (
	ModeChatOnly ExecutionMode = "chat-only"
	ModeLocal    ExecutionMode = "local"
	ModeDocker   ExecutionMode = "docker"
	ModePodman   ExecutionMode = "podman"
)

type Config struct {
	OllamaURL     string        `mapstructure:"ollama_url"`
	Model         string        `mapstructure:"model"`
	ExecutionMode ExecutionMode `mapstructure:"execution_mode"`
	AllowedDir    string        `mapstructure:"allowed_dir"`
	ContainerName string        `mapstructure:"container_name"`
	ContextSize   int           `mapstructure:"context_size"`
}

func DefaultConfig() *Config {
	home := getHomeDir()
	defaultWorkspace := filepath.Join(home, "ai-agent-workspace")

	return &Config{
		OllamaURL:     "http://localhost:11434",
		Model:         "llama2",
		ExecutionMode: ModeChatOnly,
		AllowedDir:    defaultWorkspace,
		ContainerName: "",
		ContextSize:   10,
	}
}

func getHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		return "."
	}
	return usr.HomeDir
}

func ExpandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home := getHomeDir()

	if path == "~" {
		return home
	}

	return filepath.Join(home, path[2:])
}

func (c *Config) Validate() error {
	if c.OllamaURL == "" {
		return fmt.Errorf("ollama URL is required")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}

	// Only validate AllowedDir if not in chat-only mode
	if c.ExecutionMode != ModeChatOnly && c.AllowedDir == "" {
		return fmt.Errorf("allowed directory is required for %s mode", c.ExecutionMode)
	}

	// Validate execution mode
	switch c.ExecutionMode {
	case ModeLocal, ModeDocker, ModePodman, ModeChatOnly:
		// valid
	default:
		return fmt.Errorf("invalid execution mode: %s", c.ExecutionMode)
	}

	return nil
}

// IsFileOperationAllowed returns true if the agent can perform file operations
func (c *Config) IsFileOperationAllowed() bool {
	return c.ExecutionMode != ModeChatOnly
}

// String returns a string representation of the config
func (c *Config) String() string {
	modeStr := string(c.ExecutionMode)
	if c.ExecutionMode == ModeChatOnly {
		modeStr = "chat-only (no file access)"
	}

	return fmt.Sprintf(`Configuration:
  Ollama URL: %s
  Model: %s
  Execution Mode: %s
  Context Size: %d
  File Operations: %v`,
		c.OllamaURL,
		c.Model,
		modeStr,
		c.ContextSize,
		c.IsFileOperationAllowed())
}
