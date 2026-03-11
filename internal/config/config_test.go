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

	if cfg.Model != "llama2" {
		t.Errorf("Expected default Model to be 'llama2', got %s", cfg.Model)
	}

	if cfg.ExecutionMode != ModeChatOnly {
		t.Errorf("Expected default ExecutionMode to be 'chat-only', got %s", cfg.ExecutionMode)
	}

	if cfg.ContextSize != 10 {
		t.Errorf("Expected default ContextSize to be 10, got %d", cfg.ContextSize)
	}

	expectedWorkspace := "/test/home/liberida-workspace"
	if cfg.AllowedDir != expectedWorkspace {
		t.Errorf("Expected default AllowedDir to be '%s', got '%s'", expectedWorkspace, cfg.AllowedDir)
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
			name: "Valid chat-only config",
			config: &Config{
				Provider:      "ollama",
				OllamaURL:     "http://localhost:11434",
				Model:         "llama2",
				ExecutionMode: ModeChatOnly,
				AllowedDir:    "",
			},
			wantErr: false,
		},
		{
			name: "Valid local config with dir",
			config: &Config{
				Provider:      "ollama",
				OllamaURL:     "http://localhost:11434",
				Model:         "llama2",
				ExecutionMode: ModeLocal,
				AllowedDir:    "/home/test/workspace",
			},
			wantErr: false,
		},
		{
			name: "Invalid - missing Ollama URL",
			config: &Config{
				Provider:      "ollama",
				OllamaURL:     "",
				Model:         "llama2",
				ExecutionMode: ModeChatOnly,
			},
			wantErr: true,
		},
		{
			name: "Invalid - missing model",
			config: &Config{
				Provider:      "ollama",
				OllamaURL:     "http://localhost:11434",
				Model:         "",
				ExecutionMode: ModeChatOnly,
			},
			wantErr: true,
		},
		{
			name: "Invalid - local mode missing dir",
			config: &Config{
				Provider:      "ollama",
				OllamaURL:     "http://localhost:11434",
				Model:         "llama2",
				ExecutionMode: ModeLocal,
				AllowedDir:    "",
			},
			wantErr: true,
		},
		{
			name: "Invalid - unknown execution mode",
			config: &Config{
				Provider:      "ollama",
				OllamaURL:     "http://localhost:11434",
				Model:         "llama2",
				ExecutionMode: "invalid",
			},
			wantErr: true,
		},
		{
			name: "Invalid - missing provider",
			config: &Config{
				OllamaURL:     "http://localhost:11434",
				Model:         "llama2",
				ExecutionMode: ModeChatOnly,
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

func TestIsFileOperationAllowed(t *testing.T) {
	tests := []struct {
		name     string
		mode     ExecutionMode
		expected bool
	}{
		{"Chat-only mode", ModeChatOnly, false},
		{"Local mode", ModeLocal, true},
		{"Docker mode", ModeDocker, true},
		{"Podman mode", ModePodman, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ExecutionMode: tt.mode}
			result := cfg.IsFileOperationAllowed()
			if result != tt.expected {
				t.Errorf("IsFileOperationAllowed() for %s = %v, want %v", tt.mode, result, tt.expected)
			}
		})
	}
}

func TestConfigString(t *testing.T) {
	cfg := &Config{
		OllamaURL:     "http://localhost:11434",
		Model:         "llama2",
		ExecutionMode: ModeChatOnly,
		ContextSize:   10,
	}

	str := cfg.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Check if it contains key information
	if !strings.Contains(str, "http://localhost:11434") {
		t.Error("String representation missing Ollama URL")
	}
	if !strings.Contains(str, "llama2") {
		t.Error("String representation missing model")
	}
	if !strings.Contains(str, "chat-only") {
		t.Error("String representation missing execution mode")
	}
}
