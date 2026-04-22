package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude
type AnthropicProvider struct {
	client *HTTPClient
	model  string
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	client := NewHTTPClient("https://api.anthropic.com/v1", apiKey)
	client.SetHeader("anthropic-version", "2023-06-01")
	return &AnthropicProvider{
		client: client,
		model:  model,
	}
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// anthropicRequest represents the request body for Anthropic messages API
type anthropicRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
	Stream    bool      `json:"stream,omitempty"`
	System    string    `json:"system,omitempty"`
}

// anthropicResponse represents the response from Anthropic
type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

// anthropicStreamResponse represents a streaming response chunk
type anthropicStreamResponse struct {
	Type  string `json:"type"`
	Delta struct {
		Text string `json:"text"`
	} `json:"delta"`
	Message *struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message,omitempty"`
}

// Complete sends a non-streaming request to Anthropic
func (p *AnthropicProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Set default max tokens if not specified
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000
	}

	// Extract system message if present (first message with role "system")
	var systemMsg string
	var messages []Message
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
		} else {
			messages = append(messages, msg)
		}
	}

	anthropicReq := anthropicRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    false,
		System:    systemMsg,
	}

	var resp anthropicResponse
	err := p.client.Post(ctx, "/messages", anthropicReq, &resp)
	if err != nil {
		return nil, fmt.Errorf("Anthropic API error: %w", err)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("no response from Anthropic")
	}

	usage := Usage{
		PromptTokens:     0,
		CompletionTokens: 0,
		TotalTokens:      0,
		EstimatedCost:    0,
	}

	if resp.Usage != nil {
		usage = Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
			EstimatedCost:    0, // Will be calculated by caller
		}
	}

	return &Response{
		Content: resp.Content[0].Text,
		Usage:   usage,
	}, nil
}

// Stream sends a streaming request to Anthropic
func (p *AnthropicProvider) Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000
	}

	// Extract system message if present
	var systemMsg string
	var messages []Message
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
		} else {
			messages = append(messages, msg)
		}
	}

	anthropicReq := anthropicRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    true,
		System:    systemMsg,
	}

	chunkChan, err := p.client.PostStream(ctx, "/messages", anthropicReq)
	if err != nil {
		return nil, nil, err
	}

	contentChan := make(chan string)
	usageChan := make(chan Usage)

	go func() {
		defer close(contentChan)
		defer close(usageChan)

		var finalUsage *Usage

		for chunk := range chunkChan {
			// Parse SSE format
			if !strings.HasPrefix(strings.TrimSpace(chunk), "data: ") {
				continue
			}

			data := strings.TrimPrefix(strings.TrimSpace(chunk), "data: ")
			if data == "[DONE]" {
				return
			}

			var streamResp anthropicStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if streamResp.Type == "content_block_delta" && streamResp.Delta.Text != "" {
				select {
				case <-ctx.Done():
					return
				case contentChan <- streamResp.Delta.Text:
				}
			}
			// Capture usage from message_stop event
			if streamResp.Type == "message_stop" && streamResp.Message != nil {
				finalUsage = &Usage{
					PromptTokens:     streamResp.Message.Usage.InputTokens,
					CompletionTokens: streamResp.Message.Usage.OutputTokens,
					TotalTokens:      streamResp.Message.Usage.InputTokens + streamResp.Message.Usage.OutputTokens,
					EstimatedCost:    0, // Will be calculated by caller
				}
			}
		}

		// Send usage if we have it
		if finalUsage != nil {
			select {
			case <-ctx.Done():
				return
			case usageChan <- *finalUsage:
			}
		}
	}()

	return contentChan, usageChan, nil
}

// ListModels returns available Anthropic models using the /v1/models endpoint
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]string, error) {
	// Create a new client for the models endpoint
	modelsClient := NewHTTPClient("https://api.anthropic.com/v1", p.client.apiKey)
	modelsClient.SetHeader("anthropic-version", "2023-06-01")

	var resp struct {
		Data []struct {
			ID          string `json:"id"`
			Type        string `json:"type"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}

	err := modelsClient.Post(ctx, "/models", nil, &resp)
	if err != nil {
		// Fallback to hardcoded list if API call fails
		return p.getFallbackModels(), nil
	}

	var models []string
	for _, model := range resp.Data {
		// Only include Claude models
		if model.Type == "claude" {
			models = append(models, model.ID)
		}
	}

	if len(models) == 0 {
		return p.getFallbackModels(), nil
	}

	return models, nil
}

// getFallbackModels returns a hardcoded list of current Claude models
func (p *AnthropicProvider) getFallbackModels() []string {
	return []string{
		"claude-opus-4-5",
		"claude-sonnet-4-5",
		"claude-haiku4-5",
	}
}
