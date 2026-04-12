package export

import (
	"strings"
	"testing"
	"time"
)

func TestExportMarkdown(t *testing.T) {
	// Create a test session
	createdAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 1, 15, 11, 45, 0, 0, time.UTC)
	msgTime1 := time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC)
	msgTime2 := time.Date(2024, 1, 15, 10, 31, 0, 0, time.UTC)

	session := &Session{
		ID:        42,
		Title:     "Test Conversation",
		Provider:  "ollama",
		Model:     "llama3",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Messages: []Message{
			{
				Role:      "user",
				Content:   "Hello, how are you?",
				CreatedAt: msgTime1,
			},
			{
				Role:      "assistant",
				Content:   "I'm doing great! How can I help you today?",
				CreatedAt: msgTime2,
			},
		},
	}

	exporter := NewExporter(ExportOptions{
		Format: FormatMarkdown,
	})

	result, err := exporter.Export(session)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Check for required content
	requiredStrings := []string{
		"# Test Conversation",
		"**Session ID:** 42",
		"**Provider:** ollama",
		"**Model:** llama3",
		"**Created:** 2024-01-15 10:30:00",
		"**Updated:** 2024-01-15 11:45:00",
		"**User:**",
		"Hello, how are you?",
		"**Assistant:**",
		"I'm doing great! How can I help you today?",
		"10:30:05",
		"10:31:00",
		"Exported on",
	}

	for _, str := range requiredStrings {
		if !strings.Contains(result, str) {
			t.Errorf("Expected markdown to contain '%s'", str)
		}
	}

	// Check for proper markdown formatting
	if !strings.Contains(result, "---") {
		t.Error("Expected markdown separators '---'")
	}
}

func TestExportJSON(t *testing.T) {
	createdAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 1, 15, 11, 45, 0, 0, time.UTC)
	msgTime1 := time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC)
	msgTime2 := time.Date(2024, 1, 15, 10, 31, 0, 0, time.UTC)

	session := &Session{
		ID:        42,
		Title:     "Test Conversation",
		Provider:  "ollama",
		Model:     "llama3",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Messages: []Message{
			{
				Role:      "user",
				Content:   "Hello, how are you?",
				CreatedAt: msgTime1,
			},
			{
				Role:      "assistant",
				Content:   "I'm doing great!",
				CreatedAt: msgTime2,
			},
		},
	}

	exporter := NewExporter(ExportOptions{
		Format: FormatJSON,
	})

	result, err := exporter.Export(session)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Check for JSON structure
	requiredStrings := []string{
		`"id": 42`,
		`"title": "Test Conversation"`,
		`"provider": "ollama"`,
		`"model": "llama3"`,
		`"created_at": "2024-01-15T10:30:00Z"`,
		`"updated_at": "2024-01-15T11:45:00Z"`,
		`"role": "user"`,
		`"content": "Hello, how are you?"`,
		`"role": "assistant"`,
		`"content": "I'm doing great!"`,
	}

	for _, str := range requiredStrings {
		if !strings.Contains(result, str) {
			t.Errorf("Expected JSON to contain '%s'", str)
		}
	}

	// Verify it's valid JSON
	if !strings.HasPrefix(strings.TrimSpace(result), "{") {
		t.Error("Expected JSON object to start with '{'")
	}
	if !strings.HasSuffix(strings.TrimSpace(result), "}") {
		t.Error("Expected JSON object to end with '}'")
	}
}

func TestExportEmptySession(t *testing.T) {
	createdAt := time.Now()
	updatedAt := time.Now()

	session := &Session{
		ID:        1,
		Title:     "Empty Session",
		Provider:  "ollama",
		Model:     "llama3",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Messages:  []Message{},
	}

	exporter := NewExporter(ExportOptions{
		Format: FormatMarkdown,
	})

	result, err := exporter.Export(session)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Should still have metadata but no messages
	if !strings.Contains(result, "# Empty Session") {
		t.Error("Expected title in empty session export")
	}
	if strings.Contains(result, "**User:**") {
		t.Error("Should not have user message in empty session")
	}
}

func TestExportUnsupportedFormat(t *testing.T) {
	session := &Session{
		ID:    1,
		Title: "Test",
	}

	exporter := NewExporter(ExportOptions{
		Format: "unsupported",
	})

	_, err := exporter.Export(session)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("Expected unsupported format error, got: %v", err)
	}
}

func TestExportLongContent(t *testing.T) {
	longContent := strings.Repeat("This is a long message. ", 100)
	createdAt := time.Now()

	session := &Session{
		ID:        1,
		Title:     "Long Content Test",
		Provider:  "ollama",
		Model:     "llama3",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Messages: []Message{
			{
				Role:      "user",
				Content:   longContent,
				CreatedAt: createdAt,
			},
		},
	}

	exporter := NewExporter(ExportOptions{
		Format: FormatMarkdown,
	})

	result, err := exporter.Export(session)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if !strings.Contains(result, longContent) {
		t.Error("Expected long content to be present in export")
	}
}

func TestExportSpecialCharacters(t *testing.T) {
	createdAt := time.Now()

	session := &Session{
		ID:        1,
		Title:     "Special Characters: !@#$%^&*()",
		Provider:  "ollama",
		Model:     "llama3",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Messages: []Message{
			{
				Role:      "user",
				Content:   "Hello\nWorld\tTab\r\nLine breaks & symbols: < > [ ] { }",
				CreatedAt: createdAt,
			},
		},
	}

	exporter := NewExporter(ExportOptions{
		Format: FormatMarkdown,
	})

	result, err := exporter.Export(session)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if !strings.Contains(result, "Hello") {
		t.Error("Expected content with special characters")
	}
	if !strings.Contains(result, "Line breaks") {
		t.Error("Expected content with line breaks")
	}
}

func TestExportMultipleMessages(t *testing.T) {
	createdAt := time.Now()
	numMessages := 10

	messages := make([]Message, numMessages)
	for i := 0; i < numMessages; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = Message{
			Role:      role,
			Content:   "Message " + string(rune('A'+i)),
			CreatedAt: createdAt,
		}
	}

	session := &Session{
		ID:        1,
		Title:     "Multiple Messages",
		Provider:  "ollama",
		Model:     "llama3",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Messages:  messages,
	}

	exporter := NewExporter(ExportOptions{
		Format: FormatMarkdown,
	})

	result, err := exporter.Export(session)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Count message separators
	separatorCount := strings.Count(result, "---")
	// Each message pair (user+assistant) should have separators
	// For 10 messages, we expect 10 separators (after each message)
	if separatorCount < 9 {
		t.Errorf("Expected many separators, got %d", separatorCount)
	}
}
