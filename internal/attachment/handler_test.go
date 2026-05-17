package attachment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandler_ParsePaths(t *testing.T) {
	h := NewHandler()

	tests := []struct {
		name     string
		paths    []string
		expected []string
	}{
		{
			name:     "empty input",
			paths:    []string{},
			expected: []string{},
		},
		{
			name:     "single path",
			paths:    []string{"file.txt"},
			expected: []string{"file.txt"},
		},
		{
			name:     "multiple paths",
			paths:    []string{"file1.txt", "file2.go"},
			expected: []string{"file1.txt", "file2.go"},
		},
		{
			name:     "comma-separated",
			paths:    []string{"file1.txt,file2.go"},
			expected: []string{"file1.txt", "file2.go"},
		},
		{
			name:     "mixed comma-separated and separate",
			paths:    []string{"file1.txt,file2.go", "file3.md"},
			expected: []string{"file1.txt", "file2.go", "file3.md"},
		},
		{
			name:     "empty strings filtered",
			paths:    []string{"", "file.txt", ""},
			expected: []string{"file.txt"},
		},
		{
			name:     "whitespace trimmed",
			paths:    []string{"  file1.txt  ,  file2.go  "},
			expected: []string{"file1.txt", "file2.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.ParsePaths(tt.paths)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d paths, got %d", len(tt.expected), len(result))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected %q, got %q", tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestHandler_ValidateAndRead(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("Hello World"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := NewHandler()

	tests := []struct {
		name      string
		path      string
		wantErr   bool
		errType   error
		checkFunc func(*testing.T, *ContextComponent)
	}{
		{
			name:    "valid file",
			path:    testFile,
			wantErr: false,
			checkFunc: func(t *testing.T, c *ContextComponent) {
				if c.Content != "Hello World" {
					t.Errorf("expected content 'Hello World', got %q", c.Content)
				}
				if c.Name != testFile {
					t.Errorf("expected name %q, got %q", testFile, c.Name)
				}
			},
		},
		{
			name:    "file not found",
			path:    filepath.Join(tmpDir, "nonexistent.txt"),
			wantErr: true,
		},
		{
			name:    "directory",
			path:    tmpDir,
			wantErr: true,
		},
		{
			name:    "unsupported extension",
			path:    testFile,
			wantErr: false,
			checkFunc: func(t *testing.T, c *ContextComponent) {
				badFile := filepath.Join(tmpDir, "test.bin")
				os.WriteFile(badFile, []byte("data"), 0644)
				_, err := h.ValidateAndRead(badFile)
				if err == nil {
					t.Error("expected error for unsupported extension")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := h.ValidateAndRead(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errType != nil && err != tt.errType {
					t.Fatalf("expected error type %v, got %v", tt.errType, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, comp)
			}
		})
	}
}

func TestHandler_SymlinkHandling(t *testing.T) {
	tmpDir := t.TempDir()

	realFile := filepath.Join(tmpDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("real content"), 0644); err != nil {
		t.Fatalf("failed to create real file: %v", err)
	}

	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	h := NewHandler()

	comp, err := h.ValidateAndRead(symlink)
	if err != nil {
		t.Fatalf("unexpected error reading symlink: %v", err)
	}
	if comp.Content != "real content" {
		t.Errorf("expected 'real content', got %q", comp.Content)
	}

	brokenLink := filepath.Join(tmpDir, "broken.txt")
	os.Symlink("/nonexistent", brokenLink)
	_, err = h.ValidateAndRead(brokenLink)
	if err == nil {
		t.Error("expected error for broken symlink")
	}
}

func TestHandler_ProcessPaths(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)

	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file2, []byte("content2"), 0644)

	h := NewHandler()

	components, errors := h.ProcessPaths([]string{file1, file2, "nonexistent.txt"})

	if len(components) != 2 {
		t.Errorf("expected 2 components, got %d", len(components))
	}
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}

	if components[0].Content != "content1" {
		t.Errorf("expected 'content1', got %q", components[0].Content)
	}
	if components[1].Content != "content2" {
		t.Errorf("expected 'content2', got %q", components[1].Content)
	}
}

func TestHandler_FormatComponents(t *testing.T) {
	h := NewHandler()

	tests := []struct {
		name       string
		components []ContextComponent
		expected   string
	}{
		{
			name:       "empty",
			components: []ContextComponent{},
			expected:   "",
		},
		{
			name: "single component",
			components: []ContextComponent{
				{Name: "/path/to/file.txt", Content: "hello"},
			},
			expected: "Attached Files:\n================\n\n=== file.txt ===\nhello\n",
		},
		{
			name: "multiple components",
			components: []ContextComponent{
				{Name: "/path/a.txt", Content: "content A"},
				{Name: "/path/b.txt", Content: "content B"},
			},
			expected: "Attached Files:\n================\n\n=== a.txt ===\ncontent A\n=== b.txt ===\ncontent B\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.FormatComponents(tt.components)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestHandler_SetMaxFileSize(t *testing.T) {
	h := NewHandler()

	h.SetMaxFileSize(1024)
	if h.maxFileSize != 1024 {
		t.Errorf("expected 1024, got %d", h.maxFileSize)
	}

	h.SetMaxFileSize(5 * 1024 * 1024)
	if h.maxFileSize != 5*1024*1024 {
		t.Errorf("expected 5MB, got %d", h.maxFileSize)
	}
}

func TestHandler_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	smallFile := filepath.Join(tmpDir, "small.txt")
	os.WriteFile(smallFile, []byte("small"), 0644)

	largeFile := filepath.Join(tmpDir, "large.txt")
	os.WriteFile(largeFile, make([]byte, 3*1024*1024), 0644)

	h := NewHandler()

	_, err := h.ValidateAndRead(smallFile)
	if err != nil {
		t.Errorf("unexpected error for small file: %v", err)
	}

	h.SetMaxFileSize(1024)
	_, err = h.ValidateAndRead(largeFile)
	if err == nil {
		t.Error("expected error for large file")
	}
}