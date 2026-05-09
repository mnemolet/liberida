package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OpenRouterProvider struct {
	BaseProvider
	apiKey  string
	model   string
	baseURL string
}

func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	// Map common aliases to full OpenRouter model IDs
	model = mapModelAlias(model)
	return &OpenRouterProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://openrouter.ai/api/v1",
		BaseProvider: BaseProvider{
			httpClient: &http.Client{},
		},
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

func (p *OpenRouterProvider) Stream(
	ctx context.Context,
	req Request,
) (<-chan string, <-chan Usage, <-chan []ToolCall, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	payload := map[string]interface{}{
		"model":    model,
		"messages": req.Messages,
		"stream":   true,
	}

	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, nil, err
	}

	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/mnemolet/liberida")
	httpReq.Header.Set("X-Title", "Liberida")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, nil, fmt.Errorf("openrouter error: %s", resp.Status)
	}

	return GenericStream(ctx, resp.Body, func(line []byte) (string, []ToolCall, *Usage, bool) {
		lineStr := string(line)
		if !strings.HasPrefix(lineStr, "data: ") {
			return "", nil, nil, false
		}

		data := strings.TrimPrefix(lineStr, "data: ")
		if data == "[DONE]" {
			return "", nil, nil, true
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *Usage `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", nil, nil, false
		}

		// If usage is present but choices are empty (common in final chunk)
		if len(chunk.Choices) == 0 {
			// If we have usage, this is likely the end or a meta-chunk
			return "", nil, chunk.Usage, chunk.Usage != nil
		}

		choice := chunk.Choices[0]
		// Return the content/tools, the usage (if present in this chunk),
		// and true if finish_reason is set
		return choice.Delta.Content, choice.Delta.ToolCalls, chunk.Usage, choice.FinishReason != ""
	})
}

func (p *OpenRouterProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	chunkChan, usageChan, toolChan, err := p.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	return AggregateStream(chunkChan, usageChan, toolChan)
}

func (p *OpenRouterProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{
		"openrouter/free",
		"openai/gpt-4o",
		"openai/gpt-4o-mini",
		"openai/gpt-3.5-turbo",
		"anthropic/claude-3.5-sonnet",
		"anthropic/claude-3-haiku",
		"google/gemini-2.0-flash",
		"google/gemini-1.5-pro",
		"meta-llama/llama-3.2-3b-instruct",
		"meta-llama/llama-3.1-8b-instruct",
		"mistralai/mistral-7b-instruct",
	}, nil
}
