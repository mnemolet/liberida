package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OllamaProvider struct {
	BaseProvider
	url   string
	model string
}

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func NewOllamaProvider(url, model string) *OllamaProvider {
	return &OllamaProvider{
		url:   url,
		model: model,
		BaseProvider: BaseProvider{
			httpClient: &http.Client{
				Timeout: DefaultHTTPTimeout,
			},
		},
	}
}

func (p *OllamaProvider) Name() string { return "ollama" }

func (p *OllamaProvider) Stream(
	ctx context.Context,
	req Request,
) (<-chan string, <-chan Usage, <-chan []ToolCall, error) {
	// Prepare Ollama Request
	payload := map[string]interface{}{
		"model":    p.model,
		"messages": req.Messages,
		"stream":   true,
	}

	// Only add tools if it's NOT deepseek
	isDeepSeek := strings.Contains(strings.ToLower(p.model), "deepseek")
	if !isDeepSeek && len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}

	body, _ := json.Marshal(payload)

	// fmt.Printf("\n[DEBUG] Sending to Ollama: %s\n", p.url+"/api/chat")
	// fmt.Printf("[DEBUG] Payload: %s\n\n", string(body))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.url+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, err
	}

	resp, err := p.httpClient.Do(httpReq)
	//resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, nil, fmt.Errorf("ollama error: %s", resp.Status)
	}

	// Use GenericStream with Ollama-specific logic
	return GenericStream(ctx, resp.Body, func(line []byte) (string, []ToolCall, *Usage, bool) {
		var chunk struct {
			Message struct {
				Role      string     `json:"role"`
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
			Response        string `json:"response"`
			Done            bool   `json:"done"`
			PromptEvalCount int    `json:"prompt_eval_count"`
			EvalCount       int    `json:"eval_count"`
		}

		if err := json.Unmarshal(line, &chunk); err != nil {
			return "", nil, nil, false
		}

		var usage *Usage
		if chunk.Done {
			usage = &Usage{
				PromptTokens:     chunk.PromptEvalCount,
				CompletionTokens: chunk.EvalCount,
				TotalTokens:      chunk.PromptEvalCount + chunk.EvalCount,
			}
		}

		return chunk.Message.Content, chunk.Message.ToolCalls, usage, chunk.Done
	})
}

func (p *OllamaProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	chunkChan, usageChan, toolChan, err := p.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	return AggregateStream(chunkChan, usageChan, toolChan)
}

func (p *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	url := p.url + "/api/tags"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API returned status %d", resp.StatusCode)
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		models[i] = m.Name
	}

	return models, nil
}
