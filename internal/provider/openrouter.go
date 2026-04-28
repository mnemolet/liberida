package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenRouterProvider struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	// Map common aliases to full OpenRouter model IDs
	model = mapModelAlias(model)
	return &OpenRouterProvider{
		apiKey:     apiKey,
		model:      model,
		baseURL:    "https://openrouter.ai/api/v1",
		httpClient: &http.Client{},
	}
}

func mapModelAlias(model string) string {
	aliases := map[string]string{
		"openrouter":    "openrouter/free",
		"llama3.2":      "meta-llama/llama-3.2-3b-instruct",
		"llama3.1":      "meta-llama/llama-3.1-8b-instruct",
		"llama2":        "meta-llama/llama-2-7b-chat",
		"gpt-4o":        "openai/gpt-4o",
		"gpt-4o-mini":   "openai/gpt-4o-mini",
		"gpt-3.5-turbo": "openai/gpt-3.5-turbo",
		"claude-3.5":    "anthropic/claude-3.5-sonnet",
		"claude-haiku":  "anthropic/claude-3-haiku",
		"gemini-flash":  "google/gemini-2.0-flash",
		"gemini-pro":    "google/gemini-1.5-pro",
	}
	if full, ok := aliases[model]; ok {
		return full
	}
	return model
}

func (p *OpenRouterProvider) Name() string { return "openrouter" }

type openRouterRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalTokens      int     `json:"total_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
}

type openRouterStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalTokens      int     `json:"total_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage,omitempty"`
}

func (p *OpenRouterProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	useModel := req.Model
	if useModel == "" {
		useModel = p.model
	}
	httpReq, err := p.newOpenRouterRequest(ctx, useModel, req.Messages, false)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenRouter API error %d: %s", resp.StatusCode, string(body))
	}
	var orResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return nil, err
	}
	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenRouter")
	}
	return &Response{
		Content: orResp.Choices[0].Message.Content,
		Usage: Usage{
			PromptTokens:     orResp.Usage.PromptTokens,
			CompletionTokens: orResp.Usage.CompletionTokens,
			TotalTokens:      orResp.Usage.TotalTokens,
			EstimatedCost:    orResp.Usage.Cost,
		},
	}, nil
}

func (p *OpenRouterProvider) Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, error) {
	useModel := req.Model
	if useModel == "" {
		useModel = p.model
	}
	httpReq, err := p.newOpenRouterRequest(ctx, useModel, req.Messages, true)
	if err != nil {
		return nil, nil, err
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, nil, fmt.Errorf("OpenRouter API error %d: %s", resp.StatusCode, string(body))
	}
	chunkChan := make(chan string)
	usageChan := make(chan Usage, 1)
	go func() {
		defer resp.Body.Close()
		defer close(chunkChan)
		defer close(usageChan)
		scanner := bufio.NewScanner(resp.Body)
		var finalUsage *Usage
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			line = strings.TrimPrefix(line, "data: ")
			if line == "[DONE]" {
				break
			}
			var chunk openRouterStreamChunk
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				select {
				case <-ctx.Done():
					return
				case chunkChan <- chunk.Choices[0].Delta.Content:
				}
			}
			if chunk.Usage != nil {
				finalUsage = &Usage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
					EstimatedCost:    chunk.Usage.Cost,
				}
			}
		}
		if finalUsage != nil {
			usageChan <- *finalUsage
		} else {
			usageChan <- Usage{}
		}
	}()
	return chunkChan, usageChan, nil
}

func (p *OpenRouterProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{
		"openrouter/free",
		"openai/gpt-4o", "openai/gpt-4o-mini", "openai/gpt-3.5-turbo",
		"anthropic/claude-3.5-sonnet", "anthropic/claude-3-haiku",
		"google/gemini-2.0-flash", "google/gemini-1.5-pro",
		"meta-llama/llama-3.2-3b-instruct", "meta-llama/llama-3.1-8b-instruct",
		"mistralai/mistral-7b-instruct",
	}, nil
}

// Helper: sanitizeMessages converts system messages to user
func sanitizeMessages(messages []Message) []Message {
	sanitized := make([]Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == "system" {
			sanitized = append(sanitized, Message{Role: "user", Content: m.Content})
		} else {
			sanitized = append(sanitized, m)
		}
	}
	return sanitized
}

// Helper: newOpenRouterRequest creates an HTTP request with common headers
func (p *OpenRouterProvider) newOpenRouterRequest(ctx context.Context, model string, messages []Message, stream bool) (*http.Request, error) {
	model = mapModelAlias(model)
	orReq := openRouterRequest{
		Model:    model,
		Messages: sanitizeMessages(messages),
		Stream:   stream,
	}
	data, err := json.Marshal(orReq)
	if err != nil {
		return nil, err
	}
	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/mnemolet/liberida")
	req.Header.Set("X-Title", "LiberIda CLI")
	return req, nil
}
