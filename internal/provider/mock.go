package provider

import (
	"context"
	"fmt"
	"strings"
)

// MockProvider implements the Provider interface for testing
type MockProvider struct {
	name           string
	completeFunc   func(ctx context.Context, req Request) (*Response, error)
	streamFunc     func(ctx context.Context, req Request) (<-chan string, <-chan Usage, error)
	listModelsFunc func(ctx context.Context) ([]string, error)
}

type CompleteFunc func(ctx context.Context, req Request) (*Response, error)
type StreamFunc func(ctx context.Context, req Request) (<-chan string, error)

// NewMockProvider creates a new mock provider with default behaviors
func NewMockProvider() *MockProvider {
	return &MockProvider{
		name: "mock",
		completeFunc: func(ctx context.Context, req Request) (*Response, error) {
			// Simulate token usage
			totalTokens := 0
			for _, msg := range req.Messages {
				totalTokens += len(strings.Fields(msg.Content))
			}

			return &Response{
				Content: fmt.Sprintf("Mock response to: %v", req.Messages),
				Usage: Usage{
					PromptTokens:     totalTokens,
					CompletionTokens: 20,
					TotalTokens:      totalTokens + 20,
					EstimatedCost:    0.0,
				},
			}, nil
		},
		streamFunc: func(ctx context.Context, req Request) (<-chan string, <-chan Usage, error) {
			chunkChan := make(chan string)
			usageChan := make(chan Usage)

			go func() {
				defer close(chunkChan)
				defer close(usageChan)

				// Simulate streaming chunks
				chunks := []string{"Mock ", "streaming ", "response"}
				totalTokens := 0

				for _, chunk := range chunks {
					select {
					case <-ctx.Done():
						return
					case chunkChan <- chunk:
					}
					totalTokens += len(strings.Fields(chunk))
				}

				// Send usage information
				usage := Usage{
					PromptTokens:     totalTokens,
					CompletionTokens: 15,
					TotalTokens:      totalTokens + 15,
					EstimatedCost:    0.0,
				}
				select {
				case <-ctx.Done():
					return
				case usageChan <- usage:
				}
			}()

			return chunkChan, usageChan, nil
		},
		listModelsFunc: func(ctx context.Context) ([]string, error) {
			return []string{"mock-model-1", "mock-model-2"}, nil
		},
	}
}

// WithCompleteFunc allows customizing the Complete behavior for testing
func (m *MockProvider) WithCompleteFunc(fn CompleteFunc) *MockProvider {
	m.completeFunc = fn
	return m
}

// WithStreamFunc allows customizing the Stream behavior for testing
func (m *MockProvider) WithStreamFunc(fn func(ctx context.Context, req Request) (<-chan string, <-chan Usage, error)) *MockProvider {
	m.streamFunc = fn
	return m
}

// WithListModelsFunc allows customizing the ListModels behavior for testing
func (m *MockProvider) WithListModelsFunc(fn func(ctx context.Context) ([]string, error)) *MockProvider {
	m.listModelsFunc = fn
	return m
}

// Name returns the provider name
func (m *MockProvider) Name() string {
	return m.name
}

// Complete sends a non-streaming request
func (m *MockProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	return m.completeFunc(ctx, req)
}

// Stream sends a streaming request
func (m *MockProvider) Stream(ctx context.Context, req Request) (<-chan string, <-chan Usage, error) {
	return m.streamFunc(ctx, req)
}

// ListModels returns available models
func (m *MockProvider) ListModels(ctx context.Context) ([]string, error) {
	return m.listModelsFunc(ctx)
}
