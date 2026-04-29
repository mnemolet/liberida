package provider

import (
	"fmt"
)

// NewProvider creates a provider based on configuration
func NewProvider(providerType, ollamaURL, model, apiKey string) (Provider, error) {
	switch providerType {
	case "ollama":
		return NewOllamaProvider(ollamaURL, model), nil

	case "openrouter":
		if apiKey == "" {
			return nil, fmt.Errorf("OpenRouter API key is required. Get one from https://openrouter.ai/keys")
		}
		if model == "" {
			model = "qwen/qwen3-coder:free"
		}
		return NewOpenRouterProvider(apiKey, model), nil

	case "mock":
		return NewMockProvider(), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}
