package provider

import "context"

// Chat Message
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// Request holds parameters for a completion.
type Request struct {
	Model       string
	Messages    []Message
	Stream      bool
	Temperature float32
	MaxTokens   int
}

// Response from the LLM.
type Response struct {
	Content string
}

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	// Name returns the provider name (e.g., "ollama", "openai").
	Name() string
	// Complete sends a request and returns the full response.
	Complete(ctx context.Context, req Request) (*Response, error)
	// Stream sends a request and returns a channel of response chunks.
	Stream(ctx context.Context, req Request) (<-chan string, error)
	// ListModels returns available models (if supported).
	ListModels(ctx context.Context) ([]string, error)
}
