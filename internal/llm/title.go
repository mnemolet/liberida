package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/mnemolet/liberida/internal/provider"
)

// GenerateTitle creates a concise title from the first user message
func GenerateTitle(ctx context.Context, prov provider.Provider, userMessage string) (string, error) {
	prompt := fmt.Sprintf(`Generate a very short, descriptive title (3-8 words) 
for a conversation that starts with this message: "%s"

Rules:
- Return ONLY the title, no quotes, no explanation
- Maximum 50 characters
- Be specific and meaningful
- Use title case

Examples:
- "What's the weather like in Paris?" -> "Weather in Paris"
- "Help me debug this Go function" -> "Go Debug Help"
- "Write a haiku about programming" -> "Programming Haiku"

Title:`, userMessage)

	req := provider.Request{
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3, // Low temperature for consistent output
		MaxTokens:   50,  // Short response
	}

	resp, err := prov.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	title := strings.TrimSpace(resp.Content)

	// Clean up the title
	title = strings.Trim(title, "\"'")
	if len(title) > 50 {
		title = title[:47] + "..."
	}

	return title, nil
}
