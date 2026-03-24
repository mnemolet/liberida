package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient("https://api.example.com", "test-key-123")

	if client.baseURL != "https://api.example.com" {
		t.Errorf("Expected baseURL 'https://api.example.com', got '%s'", client.baseURL)
	}

	if client.apiKey != "test-key-123" {
		t.Errorf("Expected apiKey 'test-key-123', got '%s'", client.apiKey)
	}

	if client.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if client.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", client.client.Timeout)
	}
}

func TestHTTPClient_SetHeader(t *testing.T) {
	client := NewHTTPClient("https://api.example.com", "test-key")

	client.SetHeader("X-Custom-Header", "custom-value")
	client.SetHeader("Another-Header", "another-value")

	if client.headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("Expected header 'custom-value', got '%s'", client.headers["X-Custom-Header"])
	}

	if client.headers["Another-Header"] != "another-value" {
		t.Errorf("Expected header 'another-value', got '%s'", client.headers["Another-Header"])
	}
}

func TestHTTPClient_Post_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Verify path
		if r.URL.Path != "/test/endpoint" {
			t.Errorf("Expected path '/test/endpoint', got %s", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("Expected Authorization 'Bearer test-api-key', got '%s'", auth)
		}

		// Verify content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
		}

		// Read and verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read body: %v", err)
		}

		var reqBody map[string]string
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Errorf("Failed to unmarshal body: %v", err)
		}

		if reqBody["message"] != "hello" {
			t.Errorf("Expected message 'hello', got '%s'", reqBody["message"])
		}

		// Send response
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"result":  "success",
			"message": "Hello from server",
		})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-api-key")

	// Test with custom headers
	client.SetHeader("X-Test-Header", "test-value")

	request := map[string]string{
		"message": "hello",
	}

	var result map[string]string
	err := client.Post(context.Background(), "/test/endpoint", request, &result)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}

	if result["result"] != "success" {
		t.Errorf("Expected result 'success', got '%s'", result["result"])
	}

	if result["message"] != "Hello from server" {
		t.Errorf("Expected message 'Hello from server', got '%s'", result["message"])
	}
}

func TestHTTPClient_Post_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid request",
			"code":  "400",
		})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-key")

	var result map[string]string
	err := client.Post(context.Background(), "/error", map[string]string{}, &result)

	if err == nil {
		t.Error("Expected error for bad request, got nil")
	}

	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("Expected error to mention status 400, got: %v", err)
	}
}

func TestHTTPClient_Post_NetworkError(t *testing.T) {
	// Create a server that will be closed immediately
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do nothing
	}))
	server.Close() // Close to simulate network error

	client := NewHTTPClient(server.URL, "test-key")

	var result map[string]string
	err := client.Post(context.Background(), "/test", map[string]string{}, &result)

	if err == nil {
		t.Error("Expected error for network failure, got nil")
	}
}

func TestHTTPClient_Post_WithTimeout(t *testing.T) {
	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with shorter timeout
	client := NewHTTPClient(server.URL, "test-key")
	client.client.Timeout = 1 * time.Second

	var result map[string]string
	err := client.Post(context.Background(), "/slow", map[string]string{}, &result)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
		t.Logf("Got expected timeout-like error: %v", err)
	}
}

func TestHTTPClient_Post_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-key")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var result map[string]string
	err := client.Post(ctx, "/slow", map[string]string{}, &result)

	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}
}

func TestHTTPClient_Post_InvalidJSONRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-key")

	// Send a request that cannot be marshaled (e.g., channel)
	badRequest := make(chan int)
	var result map[string]string
	err := client.Post(context.Background(), "/test", badRequest, &result)

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestHTTPClient_Post_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		// Send invalid JSON
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-key")

	var result map[string]string
	err := client.Post(context.Background(), "/test", map[string]string{}, &result)

	if err == nil {
		t.Error("Expected error for invalid JSON response, got nil")
	}
}

func TestHTTPClient_PostStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("Expected http.Flusher")
			return
		}

		// Send SSE format chunks
		chunks := []string{
			"data: {\"text\":\"Hello \"}\n\n",
			"data: {\"text\":\"world!\"}\n\n",
			"data: [DONE]\n\n",
		}

		for _, chunk := range chunks {
			w.Write([]byte(chunk))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-key")

	chunkChan, err := client.PostStream(context.Background(), "/stream", map[string]string{})
	if err != nil {
		t.Fatalf("PostStream failed: %v", err)
	}

	var received []string
	for chunk := range chunkChan {
		// Parse SSE events
		data, ok := ParseSSEEvent(chunk)
		if !ok {
			continue
		}

		if IsSSEDone(chunk) {
			break
		}

		received = append(received, data)
	}

	if len(received) != 2 {
		t.Errorf("Expected 2 data chunks, got %d", len(received))
	}
}

func TestHTTPClient_PostStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "invalid-key")

	chunkChan, err := client.PostStream(context.Background(), "/stream", map[string]string{})

	if err == nil {
		t.Error("Expected error for unauthorized request, got nil")
	}

	if chunkChan != nil {
		t.Error("Expected nil channel on error")
	}

	if !strings.Contains(err.Error(), "status 401") {
		t.Errorf("Expected status 401 in error, got: %v", err)
	}
}

func TestHTTPClient_PostStream_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send infinite stream
		for i := 0; i < 100; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
				w.Write([]byte(fmt.Sprintf("data: chunk%d\n\n", i)))
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-key")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // Automatically called when test finishes

	chunkChan, err := client.PostStream(ctx, "/stream", map[string]string{})
	if err != nil {
		t.Fatalf("PostStream failed: %v", err)
	}

	// Read a few chunks
	count := 0
	for range chunkChan {
		count++
		if count >= 3 {
			cancel()
			break
		}
	}

	// Wait a bit to ensure channel closes
	time.Sleep(100 * time.Millisecond)

	// Verify channel is closed
	_, ok := <-chunkChan
	if ok {
		t.Error("Channel should be closed after cancellation")
	}
}

func TestHTTPClient_PostStream_InvalidChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send SSE events
		events := []string{
			"data: {\"text\":\"valid\"}\n\n",
			"invalid line\n\n", // Invalid - doesn't start with "data: "
			"data: [DONE]\n\n",
		}

		for _, event := range events {
			w.Write([]byte(event))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-key")

	chunkChan, err := client.PostStream(context.Background(), "/stream", map[string]string{})
	if err != nil {
		t.Fatalf("PostStream failed: %v", err)
	}

	// Parse SSE events
	var dataChunks []string
	var rawLines []string

	for line := range chunkChan {
		rawLines = append(rawLines, line)

		// Parse SSE events
		if strings.HasPrefix(strings.TrimSpace(line), "data: ") {
			data := strings.TrimPrefix(strings.TrimSpace(line), "data: ")
			if data != "" && data != "[DONE]" {
				dataChunks = append(dataChunks, data)
			}
		}
	}

	// We should have 6 raw lines (3 events * 2 lines each)
	if len(rawLines) != 6 {
		t.Errorf("Expected 6 raw lines, got %d", len(rawLines))
	}

	// We should have 1 valid data chunk (the invalid line is ignored)
	if len(dataChunks) != 1 {
		t.Errorf("Expected 1 valid data chunk, got %d", len(dataChunks))
	}

	if len(dataChunks) > 0 && dataChunks[0] != "{\"text\":\"valid\"}" {
		t.Errorf("Unexpected data chunk: %s", dataChunks[0])
	}
}
