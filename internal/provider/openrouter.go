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
	Tools    []Tool    `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
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
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
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
	httpReq, err := p.newOpenRouterRequest(ctx, useModel, req.Messages, req.Tools, false)
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
		Content:   orResp.Choices[0].Message.Content,
		ToolCalls: orResp.Choices[0].Message.ToolCalls,
		Usage: Usage{
			PromptTokens:     orResp.Usage.PromptTokens,
			CompletionTokens: orResp.Usage.CompletionTokens,
			TotalTokens:      orResp.Usage.TotalTokens,
			EstimatedCost:    orResp.Usage.Cost,
		},
	}, nil
}

func (p *OpenRouterProvider) Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, <-chan []ToolCall, error) {
	useModel := req.Model
	if useModel == "" {
		useModel = p.model
	}
	httpReq, err := p.newOpenRouterRequest(ctx, useModel, req.Messages, req.Tools, true)
	if err != nil {
		return nil, nil, nil, err
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, nil, nil, fmt.Errorf("OpenRouter API error %d: %s", resp.StatusCode, string(body))
	}

	chunkChan := make(chan string)
	usageChan := make(chan Usage, 1)
	toolChan := make(chan []ToolCall, 1)

	go func() {
		defer resp.Body.Close()
		defer close(chunkChan)
		defer close(usageChan)
		defer close(toolChan)

		scanner := bufio.NewScanner(resp.Body)
		var finalUsage *Usage
		var accumulatedTools []ToolCall

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

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta

				// Stream text output to the UI
				if delta.Content != "" {
					select {
					case <-ctx.Done():
						return
					case chunkChan <- delta.Content:
					}
				}

				// Stitch together tool call fragments
				if len(delta.ToolCalls) > 0 {
					for _, tc := range delta.ToolCalls {
						// OpenRouter provides an 'Index' for streaming tool calls
						// This tells us which tool call in the array we are updating
						idx := 0 // default for safety if index is missing

						if len(accumulatedTools) <= idx {
							accumulatedTools = append(accumulatedTools, tc)
						} else {
							// Append the fragments of arguments/name as they arrive
							accumulatedTools[idx].Function.Arguments += tc.Function.Arguments
							if tc.Function.Name != "" {
								accumulatedTools[idx].Function.Name = tc.Function.Name
							}
						}
					}
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

		// Finalize: Send the complete tool calls and usage
		if len(accumulatedTools) > 0 {
			toolChan <- accumulatedTools
		}

		if finalUsage != nil {
			usageChan <- *finalUsage
		} else {
			usageChan <- Usage{}
		}
	}()
	return chunkChan, usageChan, toolChan, nil
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
func (p *OpenRouterProvider) newOpenRouterRequest(ctx context.Context, model string, messages []Message, tools []Tool, stream bool) (*http.Request, error) {
	model = mapModelAlias(model)
	orReq := openRouterRequest{
		Model:    model,
		Messages: sanitizeMessages(messages),
		Tools:    tools,
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
