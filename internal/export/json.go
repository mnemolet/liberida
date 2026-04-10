package export

import (
	"encoding/json"
	"fmt"
)

// JSONSession represents a session in JSON format
type JSONSession struct {
	ID        uint          `json:"id"`
	Title     string        `json:"title"`
	Provider  string        `json:"provider"`
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
	Messages  []JSONMessage `json:"messages"`
}

// JSONMessage represents a message in JSON format
type JSONMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// exportJSON converts a session to JSON format
func (e *Exporter) exportJSON(session *Session) (string, error) {
	jsonSession := JSONSession{
		ID:        session.ID,
		Title:     session.Title,
		Provider:  session.Provider,
		Model:     session.Model,
		CreatedAt: session.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: session.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Messages:  make([]JSONMessage, len(session.Messages)),
	}

	for i, msg := range session.Messages {
		jsonSession.Messages[i] = JSONMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	data, err := json.MarshalIndent(jsonSession, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}
