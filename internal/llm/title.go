package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/mnemolet/liberida/internal/provider"
)

// GenerateTitle creates a concise title from the first user message
func GenerateTitle(ctx context.Context, prov provider.Provider, userMessage string) (string, error) {
	prompt := fmt.Sprintf(`Generate a very short, descriptive title (3-8 words, max 50 characters) for a conversation starting with: "%s"
STRICT RULES:
1. Return ONLY the raw title text, nothing else
2. Do NOT include prefixes like "Title:", "Gen...", or any leading labels
3. Do NOT add quotes, explanations, or examples
4. Use title case, be specific
Examples of correct output:
- Input: "What's the weather like in Paris?" → Weather in Paris
- Input: "Help me debug this Go function" → Go Debug Help
Your response:`, userMessage)
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
