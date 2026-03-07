package config

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
)

type ExecutionMode string

const (
	ModeLocal  ExecutionMode = "local"
	ModeDocker ExecutionMode = "docker"
	ModePodman ExecutionMode = "podman"
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
		ExecutionMode: ModeLocal,
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

	return nil
}
