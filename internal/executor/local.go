package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type LocalExecutor struct {
	rootDir string
}

func NewLocal(rootDir string) (*LocalExecutor, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox directory: %w", err)
	}
	return &LocalExecutor{rootDir: abs}, nil
}

// resolvePath ensures the path is inside the sandbox and returns absolute path.
func (l *LocalExecutor) resolvePath(relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("empty path")
	}
	clean := filepath.Clean(relPath)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	full := filepath.Join(l.rootDir, clean)
	if !strings.HasPrefix(full, l.rootDir+string(filepath.Separator)) && full != l.rootDir {
		return "", fmt.Errorf("path escapes workspace")
	}
	return full, nil
}

func (l *LocalExecutor) WriteFile(path string, data []byte) error {
	full, err := l.resolvePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0644)
}

func (l *LocalExecutor) ReadFile(path string) ([]byte, error) {
	full, err := l.resolvePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (l *LocalExecutor) DeleteFile(path string) error {
	full, err := l.resolvePath(path)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

func (l *LocalExecutor) ListFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(l.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(l.rootDir, path)
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

func (l *LocalExecutor) RunCommand(ctx context.Context, command []string) (string, error) {
	if len(command) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Create the command
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = l.rootDir // Run in workspace directory

	// Run and get output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

func (l *LocalExecutor) Close() error {
	return nil // nothing to close
}
