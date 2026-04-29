package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type GeminiProvider struct {
	client *HTTPClient
	model  string
}

func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	client := NewHTTPClient("https://generativelanguage.googleapis.com/v1", apiKey)
	return &GeminiProvider{
		client: client,
		model:  model,
	}
}

func (p *GeminiProvider) Name() string {
	return "gemini"
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig *geminiConfig   `json:"generationConfig,omitempty"`
}

type geminiConfig struct {
	Temperature     float32 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type geminiStreamResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

// messagesToGeminiFormat converts internal messages to Gemini format
func messagesToGeminiFormat(messages []Message) []geminiContent {
	var contents []geminiContent

	for _, msg := range messages {
		content := geminiContent{
			Parts: []geminiPart{
				{Text: msg.Content},
			},
		}

		// Gemini uses "user" and "model" roles
		if msg.Role == "assistant" {
			content.Role = "model"
		} else {
			content.Role = "user"
		}

		contents = append(contents, content)
	}

	return contents
}

// Complete sends a non-streaming request to Gemini
func (p *GeminiProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	geminiReq := geminiRequest{
		Contents: messagesToGeminiFormat(req.Messages),
	}

	// Add generation config if provided
	if req.Temperature > 0 || req.MaxTokens > 0 {
		geminiReq.GenerationConfig = &geminiConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		}
	}

	endpoint := fmt.Sprintf("/models/%s:generateContent?key=%s", model, p.client.apiKey)
	var resp geminiResponse
	err := p.client.Post(ctx, endpoint, geminiReq, &resp)
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	if len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	usage := Usage{
		PromptTokens:     resp.UsageMetadata.PromptTokenCount,
		CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		EstimatedCost:    0, // Will be calculated by caller
	}

	return &Response{
		Content: resp.Candidates[0].Content.Parts[0].Text,
		Usage:   usage,
	}, nil
}

// Stream sends a streaming request to Gemini
func (p *GeminiProvider) Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, <-chan []ToolCall, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	geminiReq := geminiRequest{
		Contents: messagesToGeminiFormat(req.Messages),
	}

	// Add generation config if provided
	if req.Temperature > 0 || req.MaxTokens > 0 {
		geminiReq.GenerationConfig = &geminiConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		}
	}

	endpoint := fmt.Sprintf("/models/%s:streamGenerateContent?key=%s", model, p.client.apiKey)
	chunkChan, err := p.client.PostStream(ctx, endpoint, geminiReq)
	if err != nil {
		return nil, nil, nil, err
	}

	contentChan := make(chan string)
	usageChan := make(chan Usage)
	toolChan := make(chan []ToolCall, 1)

	go func() {
		defer close(contentChan)
		defer close(usageChan)
		defer close(toolChan)

		var finalUsage *Usage

		for chunk := range chunkChan {
			// Remove "data: " prefix if present (from SSE)
			chunk = strings.TrimPrefix(chunk, "data: ")
			chunk = strings.TrimSpace(chunk)

			if chunk == "[DONE]" {
				return
			}

			var streamResp geminiStreamResponse
			if err := json.Unmarshal([]byte(chunk), &streamResp); err != nil {
				// Skip invalid chunks
				continue
			}

			if len(streamResp.Candidates) > 0 {
				if len(streamResp.Candidates[0].Content.Parts) > 0 {
					text := streamResp.Candidates[0].Content.Parts[0].Text
					if text != "" {
						select {
						case <-ctx.Done():
							return
						case contentChan <- text:
						}
					}
				}
			}
			// Note: Gemini streaming responses don't include usage metadata in each chunk
			// Usage is typically only available in the final non-streaming response
			// For now, we'll send usage as zero and let the caller calculate
		}

		if finalUsage != nil {
			select {
			case <-ctx.Done():
				return
			case usageChan <- *finalUsage:
			}
		}
	}()

	return contentChan, usageChan, toolChan, nil
}

func (p *GeminiProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{
		"gemini-pro-2.5",
		"gemini-flash-2.5",
	}, nil
}
