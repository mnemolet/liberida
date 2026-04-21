package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	client *HTTPClient
	model  string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	client := NewHTTPClient("https://api.openai.com/v1", apiKey)
	return &OpenAIProvider{
		client: client,
		model:  model,
	}
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// openaiRequest represents the request body for OpenAI chat completion
type openaiRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// openaiResponse represents the response from OpenAI
type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// openaiStreamResponse represents a streaming response chunk
type openaiStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// Complete sends a non-streaming request to OpenAI
func (p *OpenAIProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	openaiReq := openaiRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	var resp openaiResponse
	err := p.client.Post(ctx, "/chat/completions", openaiReq, &resp)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	usage := Usage{
		PromptTokens:     0,
		CompletionTokens: 0,
		TotalTokens:      0,
		EstimatedCost:    0,
	}

	if resp.Usage != nil {
		usage = Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
			EstimatedCost:    0, // Will be calculated by caller
		}
	}

	return &Response{
		Content: resp.Choices[0].Message.Content,
		Usage:   usage,
	}, nil
}

// Stream sends a streaming request to OpenAI
func (p *OpenAIProvider) Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	openaiReq := openaiRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	chunkChan, err := p.client.PostStream(ctx, "/chat/completions", openaiReq)
	if err != nil {
		return nil, nil, err
	}

	// Transform SSE chunks into content strings
	contentChan := make(chan string)
	usageChan := make(chan Usage)

	go func() {
		defer close(contentChan)
		defer close(usageChan)

		var finalUsage *Usage

		for chunk := range chunkChan {
			// Remove "data: " prefix
			chunk = strings.TrimPrefix(chunk, "data: ")
			chunk = strings.TrimSpace(chunk)

			if chunk == "[DONE]" {
				return
			}

			var streamResp openaiStreamResponse
			if err := json.Unmarshal([]byte(chunk), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				delta := streamResp.Choices[0].Delta.Content
				if delta != "" {
					select {
					case <-ctx.Done():
						return
					case contentChan <- delta:
					}
				}
			}

			// Capture usage from the response (usually in the last chunk)
			if streamResp.Usage != nil {
				finalUsage = &Usage{
					PromptTokens:     streamResp.Usage.PromptTokens,
					CompletionTokens: streamResp.Usage.CompletionTokens,
					TotalTokens:      streamResp.Usage.TotalTokens,
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

// ListModels returns available OpenAI models
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	err := p.client.Post(ctx, "/models", nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	var models []string
	for _, model := range resp.Data {
		// Filter for GPT models
		if strings.Contains(model.ID, "gpt") {
			models = append(models, model.ID)
		}
	}

	return models, nil
}
