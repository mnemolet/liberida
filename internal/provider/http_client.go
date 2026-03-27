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
	"time"
)

// HTTPClient wraps http.Client with common functionality
type HTTPClient struct {
	client  *http.Client
	baseURL string
	apiKey  string
	headers map[string]string
}

// NewHTTPClient creates a new HTTP client for API calls
func NewHTTPClient(baseURL, apiKey string) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
		headers: make(map[string]string),
	}
}

// SetHeader adds a custom header
func (c *HTTPClient) SetHeader(key, value string) {
	c.headers[key] = value
}

// Post sends a POST request and decodes the response
func (c *HTTPClient) Post(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	// Marshal request body
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode response
	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// PostStream sends a POST request and returns a stream of events
func (c *HTTPClient) PostStream(ctx context.Context, endpoint string, body interface{}) (<-chan string, error) {
	// Marshal request body
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	// Add custom headers
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Create channel for streaming
	chunkChan := make(chan string)

	go func() {
		defer resp.Body.Close()
		defer close(chunkChan)

		// Read stream line by line
		reader := bufio.NewReader(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					// Log error but don't stop the channel
				}
				return
			}

			// Send the line as-is (including newline)
			// The caller can parse the SSE format
			select {
			case <-ctx.Done():
				return
			case chunkChan <- line:
			}
		}
	}()

	return chunkChan, nil
}

// ParseSSEEvent extracts the data from an SSE event line
func ParseSSEEvent(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data: ") {
		return "", false
	}

	data := strings.TrimPrefix(line, "data: ")
	return data, true
}

// IsSSEDone checks if the line indicates the end of stream
func IsSSEDone(line string) bool {
	data, _ := ParseSSEEvent(line)
	return data == "[DONE]"
}
