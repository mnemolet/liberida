package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Sandbox struct {
	rootDir string
}

func New(rootDir string) (*Sandbox, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox directory: %w", err)
	}
	return &Sandbox{rootDir: abs}, nil
}

// resolvePath ensures the path is inside the sandbox and returns absolute path.
func (s *Sandbox) resolvePath(relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("empty path")
	}
	clean := filepath.Clean(relPath)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	full := filepath.Join(s.rootDir, clean)
	if !strings.HasPrefix(full, s.rootDir+string(filepath.Separator)) && full != s.rootDir {
		return "", fmt.Errorf("path escapes sandbox")
	}
	return full, nil
}

func (s *Sandbox) WriteFile(path string, data []byte) error {
	full, err := s.resolvePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0644)
}

func (s *Sandbox) ReadFile(path string) ([]byte, error) {
	full, err := s.resolvePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (s *Sandbox) DeleteFile(path string) error {
	full, err := s.resolvePath(path)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

func (s *Sandbox) ListFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(s.rootDir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
