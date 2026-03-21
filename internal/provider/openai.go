package provider

import (
	"context"
	"fmt"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	apiKey string
	model  string
	// client will be added when implementing
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey: apiKey,
		model:  model,
	}
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Complete sends a non-streaming request to OpenAI
func (p *OpenAIProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	// TODO: Implement OpenAI API call
	// For now, return helpful error
	return nil, fmt.Errorf("OpenAI provider not yet implemented - coming soon")
}

// Stream sends a streaming request to OpenAI
func (p *OpenAIProvider) Stream(ctx context.Context, req Request) (<-chan string, error) {
	// TODO: Implement streaming OpenAI API call
	return nil, fmt.Errorf("OpenAI streaming not yet implemented - coming soon")
}

// ListModels returns available OpenAI models
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	// TODO: Implement OpenAI models list
	return nil, fmt.Errorf("OpenAI models listing not yet implemented - coming soon")
}
