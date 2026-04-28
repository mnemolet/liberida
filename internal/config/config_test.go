package config

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	// Use a mock provider with a predictable home directory
	provider := mockHomeDirProvider{dir: "/test/home"}
	cfg := DefaultConfig(provider)

	if cfg.OllamaURL != "http://localhost:11434" {
		t.Errorf("Expected default OllamaURL to be 'http://localhost:11434', got %s", cfg.OllamaURL)
	}

	if cfg.Model != "llama3.2" {
		t.Errorf("Expected default Model to be 'llama3.2', got %s", cfg.Model)
	}

	if cfg.ContextSize != 10 {
		t.Errorf("Expected default ContextSize to be 10, got %d", cfg.ContextSize)
	}
}

func TestGetHomeDir(t *testing.T) {
	// Test the OS provider
	provider := OSHomeDirProvider{}
	home := provider.GetHomeDir()
	if home == "" {
		t.Error("Expected home directory to not be empty")
	}

	// Check if directory exists (optional, might fail on some CI environments)
	if _, err := os.Stat(home); err != nil {
		t.Logf("Note: Home directory %s stat failed: %v", home, err)
	}
}

func TestExpandPath(t *testing.T) {
	provider := mockHomeDirProvider{dir: "/test/home"}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No tilde",
			input:    "/usr/local/bin",
			expected: "/usr/local/bin",
		},
		{
			name:     "Tilde only",
			input:    "~",
			expected: "/test/home",
		},
		{
			name:     "Tilde with path",
			input:    "~/Documents",
			expected: "/test/home/Documents",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPath(tt.input, provider)
			if result != tt.expected {
				t.Errorf("ExpandPath(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid Ollama config",
			config: &Config{
				Provider:  "ollama",
				OllamaURL: "http://localhost:11434",
				Model:     "llama3.2",
			},
			wantErr: false,
		},
		{
			name: "Valid OpenAI config (API key can be empty, validation only checks provider existence)",
			config: &Config{
				Provider:     "openai",
				Model:        "gpt-4o",
				OpenAIAPIKey: "",
			},
			wantErr: false, // We don't validate API key presence here
		},
		{
			name: "Invalid - missing Ollama URL",
			config: &Config{
				Provider:  "ollama",
				OllamaURL: "",
				Model:     "llama3.2",
			},
			wantErr: true,
		},
		{
			name: "Invalid - missing model",
			config: &Config{
				Provider:  "ollama",
				OllamaURL: "http://localhost:11434",
				Model:     "",
			},
			wantErr: true,
		},
		{
			name: "Invalid - missing provider",
			config: &Config{
				OllamaURL: "http://localhost:11434",
				Model:     "llama3.2",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigString(t *testing.T) {
	cfg := &Config{
		Provider:    "ollama",
		OllamaURL:   "http://localhost:11434",
		Model:       "llama3.2",
		ContextSize: 10,
		AutoContext: true,
		AutoTitle:   true,
		ShowUsage:   true,
	}

	str := cfg.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Check for key information that should be present
	checks := []string{
		"Provider: ollama",
		"Ollama URL: http://localhost:11434",
		"Model: llama3.2",
		"Context Size: 10",
		"Auto Context: true",
		"Auto Title: true",
		"Show Usage: true",
	}

	for _, check := range checks {
		if !strings.Contains(str, check) {
			t.Errorf("String representation missing '%s'", check)
		}
	}
}
