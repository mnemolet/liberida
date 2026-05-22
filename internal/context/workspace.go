package context

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mnemolet/liberida/internal/executor"
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
	excludes    []string
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
		excludes: []string{".git", "node_modules", ".DS_Store", "*.log"},
	}
}

// AddExclude adds a pattern to skip (e.g., "tmp", "*.bin")
func (s *WorkspaceScanner) AddExclude(pattern string) {
	s.excludes = append(s.excludes, pattern)
}

// shouldExclude checks if a file path matches any exclude pattern
func (s *WorkspaceScanner) shouldExclude(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range s.excludes {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
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

// CollectContext collects context from workspace files using the executor
func (s *WorkspaceScanner) CollectContext(exec executor.Executor, workspaceDir string) (string, error) {
	// List all files in workspace
	files, err := exec.ListFiles()
	if err != nil {
		return "", fmt.Errorf("failed to list workspace files: %w", err)
	}

	if len(files) == 0 {
		return "No files found in workspace.\n", nil
	}

	var result strings.Builder
	result.WriteString("Workspace Context:\n")
	result.WriteString("==================\n\n")

	// Collect file info
	var textFiles []FileInfo
	var binaryFiles []string
	var largeFiles []string

	for _, file := range files {
		if s.shouldExclude(file) {
			continue
		}

		// Skip directories and hidden files
		if strings.HasPrefix(filepath.Base(file), ".") {
			continue
		}

		// Check if it's a text file
		if !s.isTextFile(file) {
			binaryFiles = append(binaryFiles, file)
			continue
		}

		// Read file content
		content, err := exec.ReadFile(file)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		// Check file size
		fileSize := int64(len(content))
		if fileSize > s.maxFileSize {
			largeFiles = append(largeFiles, fmt.Sprintf("%s (%d KB)", file, fileSize/1024))
			continue
		}

		textFiles = append(textFiles, FileInfo{
			Path:    file,
			Size:    fileSize,
			IsText:  true,
			Content: string(content),
		})
	}

	// Write text files with content
	if len(textFiles) > 0 {
		result.WriteString("Text Files:\n")
		result.WriteString("-------------\n")
		for _, f := range textFiles {
			result.WriteString("\n=== " + f.Path + " ===\n")
			result.WriteString(f.Content)
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	// List binary files
	if len(binaryFiles) > 0 {
		result.WriteString("Binary Files (content not shown):\n")
		for _, f := range binaryFiles {
			result.WriteString("  - " + f + "\n")
		}
		result.WriteString("\n")
	}

	// List large files
	if len(largeFiles) > 0 {
		result.WriteString("Large Files (exceeded size limit, content not shown):\n")
		for _, f := range largeFiles {
			result.WriteString("  - " + f + "\n")
		}
		result.WriteString("\n")
	}

	result.WriteString("==================\n")
	result.WriteString("You can ask questions about these files. Use actions to modify them.\n")

	return result.String(), nil
}
