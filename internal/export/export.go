package export

import (
	"fmt"
	"time"
)

// Format represents the export format
type Format string

const (
	FormatMarkdown Format = "md"
	FormatJSON     Format = "json"
)

// Session represents a chat session for export
type Session struct {
	ID        uint
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
	Messages  []Message
	Model     string
	Provider  string
}

// Message represents a single chat message
type Message struct {
	Role      string // "user" or "assistant"
	Content   string
	CreatedAt time.Time
}

// ExportOptions configures the export behavior
type ExportOptions struct {
	Format      Format
	Output      string // file path or empty for stdout
	IncludeMeta bool
}

// Exporter handles exporting sessions to different formats
type Exporter struct {
	options ExportOptions
}

// NewExporter creates a new exporter with given options
func NewExporter(opts ExportOptions) *Exporter {
	return &Exporter{
		options: opts,
	}
}

// Export exports a session to the specified format
func (e *Exporter) Export(session *Session) (string, error) {
	switch e.options.Format {
	case FormatMarkdown:
		return e.exportMarkdown(session), nil
	case FormatJSON:
		return e.exportJSON(session)
	default:
		return "", fmt.Errorf("unsupported format: %s", e.options.Format)
	}
}
