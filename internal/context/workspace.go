package context

import (
	"path/filepath"
	"strings"
)

// FileInfo holds information about a file in the workspace
type FileInfo struct {
	Path    string
	Size    int64
	IsText  bool
	Content string
}

// WorkspaceScanner scans and collects context from workspace files
type WorkspaceScanner struct {
	maxFileSize int64           // maximum size for text files to include (bytes)
	extensions  map[string]bool // allowed file extensions
}

// NewWorkspaceScanner creates a new workspace scanner with defaults
func NewWorkspaceScanner() *WorkspaceScanner {
	return &WorkspaceScanner{
		maxFileSize: 5 * 1024, // 5KB default
		extensions: map[string]bool{
			".txt":          true,
			".md":           true,
			".go":           true,
			".py":           true,
			".js":           true,
			".ts":           true,
			".json":         true,
			".yaml":         true,
			".yml":          true,
			".toml":         true,
			".sh":           true,
			".bash":         true,
			".zsh":          true,
			".env":          true,
			".gitignore":    true,
			".dockerignore": true,
		},
	}
}

// SetMaxFileSize sets the maximum file size to include
func (s *WorkspaceScanner) SetMaxFileSize(size int64) {
	s.maxFileSize = size
}

// AddExtension adds a file extension to include
func (s *WorkspaceScanner) AddExtension(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	s.extensions[ext] = true
}

// isTextFile checks if file extension is allowed
func (s *WorkspaceScanner) isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return s.extensions[ext]
}

// CollectContext collects context from workspace files
func CollectContext(workspaceDir string, executor interface{}) (string, error) {
	// TODO: need to implement
	// Need to use the executor to list and read files
	return "", nil
}
