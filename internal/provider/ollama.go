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

type ollamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Context []int                  `json:"context,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Context  []int  `json:"context"`
	Done     bool   `json:"done"`
}

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// messagesToPrompt converts a list of messages into a simple prompt.
// For Ollama, we can format as "role: content\n" or use chat templates.
// This is a simplistic approach; for better results, consider using
// chat templates based on the model.
func messagesToPrompt(messages []Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return sb.String()
}

// Complete sends a non-streaming request to Ollama.
func (p *OllamaProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	// Use request model if specified, otherwise fallback to provider default
	model := req.Model
	if model == "" {
		model = p.model
	}

	prompt := messagesToPrompt(req.Messages)

	ollamaReq := ollamaGenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
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

	url := p.baseURL + "/api/generate"
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

	var ollamaResp ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &Response{
		Content: ollamaResp.Response,
	}, nil
}

// Stream sends a streaming request and returns a channel of strings.
func (p *OllamaProvider) Stream(ctx context.Context, req Request) (<-chan string, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	prompt := messagesToPrompt(req.Messages)

	ollamaReq := ollamaGenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: true,
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

	url := p.baseURL + "/api/generate"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama API returned status %d", resp.StatusCode)
	}

	// Create a channel to stream responses
	chunkChan := make(chan string)

	go func() {
		defer resp.Body.Close()
		defer close(chunkChan)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var ollamaResp ollamaGenerateResponse
			if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
				// If we can't parse, send the raw line? Better to break.
				// For robustness, we could send error but it's complex.
				continue
			}
			select {
			case <-ctx.Done():
				return
			case chunkChan <- ollamaResp.Response:
			}
			if ollamaResp.Done {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			// Optionally log error
		}
	}()

	return chunkChan, nil
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
