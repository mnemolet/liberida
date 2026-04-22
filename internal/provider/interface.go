package provider

import "context"

// Usage represents token usage and cost information
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
}

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
	Usage   Usage
}

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	// Name returns the provider name (e.g., "ollama", "openai").
	Name() string
	// Complete sends a request and returns the full response.
	Complete(ctx context.Context, req Request) (*Response, error)
	// Stream sends a request and returns a channel of response chunks.
	Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, error)
	// ListModels returns available models (if supported).
	ListModels(ctx context.Context) ([]string, error)
}
