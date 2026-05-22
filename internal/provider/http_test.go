package provider

import (
	"testing"
	"time"
)

func TestDefaultHTTPTimeout(t *testing.T) {
	if DefaultHTTPTimeout != 30*time.Second {
		t.Errorf("DefaultHTTPTimeout = %v, want 30s", DefaultHTTPTimeout)
	}
}

func TestOllamaProvider_HTTPTimeout(t *testing.T) {
	p := NewOllamaProvider("http://localhost:11434", "test-model")
	if p.httpClient.Timeout != DefaultHTTPTimeout {
		t.Errorf("Ollama HTTP timeout = %v, want %v", p.httpClient.Timeout, DefaultHTTPTimeout)
	}
}

func TestOpenRouterProvider_HTTPTimeout(t *testing.T) {
	p := NewOpenRouterProvider("test-api-key", "test-model")
	if p.httpClient.Timeout != DefaultHTTPTimeout {
		t.Errorf("OpenRouter HTTP timeout = %v, want %v", p.httpClient.Timeout, DefaultHTTPTimeout)
	}
}

func TestProvider_HTTPClientShared(t *testing.T) {
	p := NewOllamaProvider("http://localhost:11434", "test-model")
	if p.httpClient == nil {
		t.Fatal("HTTP client is nil")
	}
	if p.httpClient.Timeout <= 0 {
		t.Error("HTTP timeout should be positive")
	}
}

func TestDefaultHTTPTimeout_NotZero(t *testing.T) {
	if DefaultHTTPTimeout == 0 {
		t.Error("DefaultHTTPTimeout should not be zero")
	}
}
