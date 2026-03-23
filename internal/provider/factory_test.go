package provider

import (
	"testing"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		expectError  bool
	}{
		{"Ollama provider", "ollama", false},
		{"OpenAI provider", "openai", true}, // Not implemented yet
		{"Anthropic provider", "anthropic", true},
		{"Gemini provider", "gemini", true},
		{"Unknown provider", "unknown", true},
		{"Mock provider", "mock", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewProvider(tt.providerType, "http://localhost:11434", "test-model", "test-key")
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && prov == nil {
				t.Error("Expected provider but got nil")
			}
		})
	}
}
