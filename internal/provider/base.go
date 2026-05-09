package provider

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
)

type BaseProvider struct {
	httpClient *http.Client
}

// StreamHandler parses a single line from the SSE stream.
// It returns content, tool calls, usage data, and a boolean indicating
// if the stream is done.
type StreamHandler func(
	line []byte,
) (
	content string,
	tools []ToolCall,
	usage *Usage,
	done bool,
)

func GenericStream(
	ctx context.Context,
	body io.ReadCloser,
	handler StreamHandler,
) (
	<-chan string,
	<-chan Usage,
	<-chan []ToolCall,
	error,
) {
	chunkChan := make(chan string)
	usageChan := make(chan Usage, 1)
	toolChan := make(chan []ToolCall, 1)

	go func() {
		defer body.Close()
		defer close(chunkChan)
		defer close(usageChan)
		defer close(toolChan)

		scanner := bufio.NewScanner(body)
		var accumulatedTools []ToolCall
		var lastUsage Usage

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Bytes()
				if len(line) == 0 {
					continue
				}

				content, tools, usage, done := handler(line)

				if content != "" {
					chunkChan <- content
				}

				if len(tools) > 0 {
					// Stitch tool fragments (Ollama usually sends them whole,
					// but this logic supports incremental fragments too)
					for _, tc := range tools {
						if tc.Index >= len(accumulatedTools) {
							accumulatedTools = append(accumulatedTools, tc)
						} else {
							accumulatedTools[tc.Index].Function.Arguments += tc.Function.Arguments
						}
					}
				}

				if usage != nil {
					lastUsage = *usage
				}

				if done {
					break
				}
			}
		}

		if len(accumulatedTools) > 0 {
			toolChan <- accumulatedTools
		} else {
			toolChan <- nil // Always send something to avoid hanging <-toolChan
		}
		usageChan <- lastUsage
	}()

	return chunkChan, usageChan, toolChan, nil
}

// AggregateStream collects all data from the stream channels into a single Response.
func AggregateStream(
	chunkChan <-chan string,
	usageChan <-chan Usage,
	toolChan <-chan []ToolCall,
) (*Response, error) {
	var fullContent strings.Builder
	for chunk := range chunkChan {
		fullContent.WriteString(chunk)
	}

	// Retrieve usage and tools (these channels are closed by GenericStream)
	usage := <-usageChan
	toolCalls := <-toolChan

	return &Response{
		Content:   fullContent.String(),
		Usage:     usage,
		ToolCalls: toolCalls,
	}, nil
}
