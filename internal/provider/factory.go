package provider

import (
	"fmt"
)

// NewProvider creates a provider based on configuration
func NewProvider(providerType, ollamaURL, model, apiKey string) (Provider, error) {
	switch providerType {
	case "ollama":
		return NewOllamaProvider(ollamaURL, model), nil

	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required. Get one from https://platform.openai.com/api-keys")
		}
		if model == "" {
			model = "gpt-5-4" // Default to latest model
		}
		return NewOpenAIProvider(apiKey, model), nil

	case "anthropic":
		if apiKey == "" {
			return nil, fmt.Errorf("Anthropic API key is required. Get one from https://console.anthropic.com")
		}
		if model == "" {
			model = "claude-sonnet-4-5" // Default to latest
		}
		return NewAnthropicProvider(apiKey, model), nil

	case "gemini":
		if apiKey == "" {
			return nil, fmt.Errorf("Gemini API key is required. Get one from https://makersuite.google.com/app/apikey")
		}
		if model == "" {
			model = "gemini-flash-2-5" // Default to latest
		}
		return NewGeminiProvider(apiKey, model), nil

	case "mock":
		return NewMockProvider(), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}
