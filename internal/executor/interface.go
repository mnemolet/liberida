package executor

import "context"

// Executor defines the operations that can be performed in an execution environment.
type Executor interface {
	// WriteFile writes data to a file at the given path.
	WriteFile(path string, data []byte) error
	// ReadFile returns the content of a file.
	ReadFile(path string) ([]byte, error)
	// DeleteFile removes a file.
	DeleteFile(path string) error
	// ListFiles returns all files in the workspace (relative paths).
	ListFiles() ([]string, error)
	// RunCommand executes a command and returns its combined output.
	RunCommand(ctx context.Context, command []string) (string, error)
	// Close releases any resources (e.g., stops a container).
	Close() error
}
