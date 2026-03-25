package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GeminiProvider implements the Provider interface for Google Gemini
type GeminiProvider struct {
	client *HTTPClient
	model  string
}

// NewGeminiProvider creates a new Gemini provider
func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	client := NewHTTPClient("https://generativelanguage.googleapis.com/v1", apiKey)
	return &GeminiProvider{
		client: client,
		model:  model,
	}
}

// Name returns the provider name
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Gemini request structures based on official API docs
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

// Gemini response structures
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

// Gemini streaming response structure
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

	return &Response{
		Content: resp.Candidates[0].Content.Parts[0].Text,
	}, nil
}

// Stream sends a streaming request to Gemini
func (p *GeminiProvider) Stream(ctx context.Context, req Request) (<-chan string, error) {
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
		return nil, err
	}

	contentChan := make(chan string)

	go func() {
		defer close(contentChan)

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
		}
	}()

	return contentChan, nil
}

// ListModels returns available Gemini models
func (p *GeminiProvider) ListModels(ctx context.Context) ([]string, error) {
	// Gemini models are well-known
	return []string{
		"gemini-pro-2.5",
		"gemini-flash-2.5",
	}, nil
}
