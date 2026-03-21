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
		return nil, fmt.Errorf("OpenAI support requires API key. Add it to config and try again")
	case "anthropic":
		return nil, fmt.Errorf("Anthropic support coming soon - API key required")
	case "gemini":
		return nil, fmt.Errorf("Gemini support coming soon - API key required")
	case "mock":
		return NewMockProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}
