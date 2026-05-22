package export

import (
	"strconv"
	"strings"
	"time"
)

// exportMarkdown converts a session to markdown format
func (e *Exporter) exportMarkdown(session *Session) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# " + session.Title + "\n\n")
	sb.WriteString("**Session ID:** " + strconv.FormatUint(uint64(session.ID), 10) + "  \n")
	sb.WriteString("**Provider:** " + session.Provider + "  \n")
	sb.WriteString("**Model:** " + session.Model + "  \n")
	sb.WriteString("**Created:** " + session.CreatedAt.Format("2006-01-02 15:04:05") + " \n")
	sb.WriteString("**Updated:** " + session.UpdatedAt.Format("2006-01-02 15:04:05") + "\n\n")
	sb.WriteString("---\n\n")

	// Messages
	for _, msg := range session.Messages {
		// Role header
		role := "**User:**"
		if msg.Role == "assistant" {
			role = "**Assistant:**"
		}
		sb.WriteString(role + "\n\n")

		// Message content with proper markdown escaping
		content := msg.Content
		// Ensure content ends with newline
		if !strings.HasSuffix(content, "\n") {
			content = content + "\n"
		}
		sb.WriteString(content)
		sb.WriteString("\n")

		// Timestamp
		sb.WriteString("*" + msg.CreatedAt.Format("15:04:05") + "*\n\n")
		sb.WriteString("---\n\n")
	}

	// Footer
	sb.WriteString("\n*Exported on *" + time.Now().Format("2006-01-02 15:04:05") + "*\n")

	return sb.String()
}
