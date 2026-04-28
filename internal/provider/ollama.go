package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
// baseURL should be like "http://localhost:11434".
// model is the default model to use (can be overridden per request).
func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	return &OllamaProvider{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		httpClient: &http.Client{},
	}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// ---------- Request/Response Structs for Ollama API ----------
// replace ollamaGenerateRequest and ollamaGenerateResponse with:
type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaChatMessage    `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

type ollamaChatStreamResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done            bool `json:"done"`
	PromptEvalCount int  `json:"prompt_eval_count"`
	EvalCount       int  `json:"eval_count"`
}

// Complete sends a non-streaming request to Ollama using chat API.
func (p *OllamaProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Convert internal messages to Ollama chat messages
	chatMessages := make([]ollamaChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		chatMessages[i] = ollamaChatMessage{Role: msg.Role, Content: msg.Content}
	}

	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: chatMessages,
		Stream:   false,
	}
	if req.Temperature != 0 {
		ollamaReq.Options = map[string]interface{}{
			"temperature": req.Temperature,
		}
	}

	data, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + "/api/chat" // changed from /api/generate
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API returned status %d", resp.StatusCode)
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &Response{
		Content: chatResp.Message.Content,
		Usage: Usage{
			// Ollama chat API doesn't return token counts in non-streaming mode easily
			// You can ignore or try to get from another endpoint
			EstimatedCost: 0.0,
		},
	}, nil
}

// Stream sends a streaming request using chat API.
func (p *OllamaProvider) Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Convert internal messages to Ollama chat messages
	chatMessages := make([]ollamaChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		chatMessages[i] = ollamaChatMessage{Role: msg.Role, Content: msg.Content}
	}

	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: chatMessages,
		Stream:   true,
	}
	if req.Temperature != 0 {
		ollamaReq.Options = map[string]interface{}{
			"temperature": req.Temperature,
		}
	}

	data, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	// fmt.Printf("[DEBUG] Ollama request: %s\n", string(data))

	url := p.baseURL + "/api/chat" // changed
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// bodyBytes, _ := io.ReadAll(resp.Body)
		// fmt.Printf("[DEBUG] Ollama error response: %s\n", string(bodyBytes))
		resp.Body.Close()
		return nil, nil, fmt.Errorf("ollama API returned status %d", resp.StatusCode)
	}

	chunkChan := make(chan string)
	usageChan := make(chan Usage, 1)

	go func() {
		defer resp.Body.Close()
		defer close(chunkChan)
		defer close(usageChan)

		var finalUsage *Usage

		scanner := bufio.NewScanner(resp.Body)
		// For streaming, Ollama sends one JSON object per line, each with a "message" field.
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			// fmt.Printf("[DEBUG] Raw line: %s\n", line)
			var streamResp ollamaChatStreamResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				// fmt.Printf("[DEBUG] JSON parse error: %v\n", err)
				continue
			}
			if streamResp.Message.Content != "" {
				select {
				case <-ctx.Done():
					return
				case chunkChan <- streamResp.Message.Content:
				}
			}
			if streamResp.Done {
				finalUsage = &Usage{
					PromptTokens:     streamResp.PromptEvalCount,
					CompletionTokens: streamResp.EvalCount,
					TotalTokens:      streamResp.PromptEvalCount + streamResp.EvalCount,
					EstimatedCost:    0.0,
				}
				break
			}
		}
		if finalUsage != nil {
			usageChan <- *finalUsage
		}
	}()

	return chunkChan, usageChan, nil
}

// ListModels returns the list of available models from Ollama.
func (p *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	url := p.baseURL + "/api/tags"
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
