package provider

import "context"

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Usage represents token usage and cost information
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
}

// Chat Message
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolID    string     `json:"tool_call_id,omitempty"`
}

type Request struct {
	Model       string
	Messages    []Message
	Tools       []Tool `json:"tools,omitempty"`
	Stream      bool
	Temperature float32
	MaxTokens   int
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
}

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	// Name returns the provider name (e.g., "ollama", "openai").
	Name() string
	// Complete sends a request and returns the full response.
	Complete(ctx context.Context, req Request) (*Response, error)
	// Stream sends a request and returns a channel of response chunks.
	Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, <-chan []ToolCall, error)
	ListModels(ctx context.Context) ([]string, error)
}
