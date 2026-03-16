package config

import (
	"fmt"
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
	Provider       string        `mapstructure:"provider"`
	OllamaURL      string        `mapstructure:"ollama_url"`
	Model          string        `mapstructure:"model"`
	ExecutionMode  ExecutionMode `mapstructure:"execution_mode"`
	AllowedDir     string        `mapstructure:"allowed_dir"`
	ContextSize    int           `mapstructure:"context_size"`
	DBPath         string        `mapstructure:"db_path"`
	ContainerName  string        `mapstructure:"container_name"`
	ContainerImage string        `mapstructure:"container_image"`
}

func DefaultConfig(hp HomeDirProvider) *Config {
	home := hp.GetHomeDir()
	defaultWorkspace := filepath.Join(home, "liberida-workspace")
	defaultDBPath := filepath.Join(home, ".liberida", "chat.db")

	return &Config{
		Provider:       "ollama",
		OllamaURL:      "http://localhost:11434",
		Model:          "llama2",
		ExecutionMode:  ModeChatOnly,
		AllowedDir:     defaultWorkspace,
		ContextSize:    10,
		DBPath:         defaultDBPath,
		ContainerName:  "liberida-workspace",
		ContainerImage: "alpine:latest",
	}
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

	// Validate container-specific fields for Docker/Podman modes
	if c.ExecutionMode == ModeDocker || c.ExecutionMode == ModePodman {
		if c.ContainerName == "" {
			return fmt.Errorf("container name is required for %s mode", c.ExecutionMode)
		}
		if c.ContainerImage == "" {
			return fmt.Errorf("container image is required for %s mode", c.ExecutionMode)
		}
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
  Provider: %s
  Ollama URL: %s
  Model: %s
  Execution Mode: %s
  Context Size: %d
  File Operations: %v`,
		c.Provider,
		c.OllamaURL,
		c.Model,
		modeStr,
		c.ContextSize,
		c.IsFileOperationAllowed())
}
