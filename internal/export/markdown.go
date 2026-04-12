package export

import (
	"fmt"
	"strings"
	"time"
)

// exportMarkdown converts a session to markdown format
func (e *Exporter) exportMarkdown(session *Session) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", session.Title))
	sb.WriteString(fmt.Sprintf("**Session ID:** %d  \n", session.ID))
	sb.WriteString(fmt.Sprintf("**Provider:** %s  \n", session.Provider))
	sb.WriteString(fmt.Sprintf("**Model:** %s  \n", session.Model))
	sb.WriteString(fmt.Sprintf("**Created:** %s  \n", session.CreatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Updated:** %s  \n\n", session.UpdatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString("---\n\n")

	// Messages
	for _, msg := range session.Messages {
		// Role header
		role := "**User:**"
		if msg.Role == "assistant" {
			role = "**Assistant:**"
		}
		sb.WriteString(fmt.Sprintf("%s\n\n", role))

		// Message content with proper markdown escaping
		content := msg.Content
		// Ensure content ends with newline
		if !strings.HasSuffix(content, "\n") {
			content = content + "\n"
		}
		sb.WriteString(content)
		sb.WriteString("\n")

		// Timestamp
		sb.WriteString(fmt.Sprintf("*%s*\n\n", msg.CreatedAt.Format("15:04:05")))
		sb.WriteString("---\n\n")
	}

	// Footer
	sb.WriteString(fmt.Sprintf("\n*Exported on %s*\n", time.Now().Format("2006-01-02 15:04:05")))

	return sb.String()
}
