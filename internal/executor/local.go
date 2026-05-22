package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// defaultFileBlocklist contains sensitive file path patterns blocked by default.
// Patterns use filepath.Match syntax and are matched against paths relative to the workspace root.
var defaultFileBlocklist = []string{
	".env*",
	"*.pem",
	"*.key",
	"id_*",
	".ssh/*",
	".gnupg/*",
	".netrc",
	".git-credentials",
}

// defaultBlocklist contains dangerous system commands blocked by default.
var defaultBlocklist = []string{
	"dd",
	"mkfs",
	"fdisk",
	"parted",
	"gdisk",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"init",
	"sudo",
	"doas",
	"su",
}

// CommandPolicy defines allow/block rules for command execution.
// If Allowlist is non-empty, only commands in it may run.
// Blocklist always overrides Allowlist.
type CommandPolicy struct {
	Allowlist []string
	Blocklist []string
}

// FilePolicy defines block rules for file operations.
// Blocklist entries are glob patterns matched against paths relative to the workspace root.
type FilePolicy struct {
	Blocklist []string
}

type LocalExecutor struct {
	mu         sync.RWMutex
	rootDir    string
	policy     CommandPolicy
	filePolicy FilePolicy
}

func NewLocal(rootDir string) (*LocalExecutor, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox directory: %w", err)
	}
	return &LocalExecutor{
		rootDir: abs,
		policy: CommandPolicy{
			Blocklist: slices.Clone(defaultBlocklist),
		},
		filePolicy: FilePolicy{
			Blocklist: slices.Clone(defaultFileBlocklist),
		},
	}, nil
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

	// First line of defense: string-based prefix check.
	// This catches naive ../ traversal before any symlink resolution.
	if !strings.HasPrefix(full, l.rootDir+string(filepath.Separator)) && full != l.rootDir {
		return "", fmt.Errorf("path escapes workspace")
	}

	// Second line of defense: resolve symlinks to prevent symlink-based escape.
	// For non-existent files (e.g. WriteFile to a new path), resolve the parent.
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if parent, e2 := filepath.EvalSymlinks(filepath.Dir(full)); e2 == nil {
				if !isWithinRoot(l.rootDir, parent) {
					return "", fmt.Errorf("path escapes workspace via symlink")
				}
			}
			return full, nil
		}
		return "", fmt.Errorf("path resolution error: %w", err)
	}

	if !isWithinRoot(l.rootDir, resolved) {
		return "", fmt.Errorf("path escapes workspace via symlink")
	}
	return resolved, nil
}

// isWithinRoot checks whether resolvedPath is inside rootDir,
// accounting for the possibility that rootDir itself is a symlink.
func isWithinRoot(rootDir, resolvedPath string) bool {
	rootResolved, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		rootResolved = rootDir
	}
	prefix := rootResolved + string(filepath.Separator)
	return strings.HasPrefix(resolvedPath, prefix) || resolvedPath == rootResolved
}

// checkFilePolicy enforces the file blocklist for a given resolved absolute path.
func (l *LocalExecutor) checkFilePolicy(absPath string) error {
	rel, err := filepath.Rel(l.rootDir, absPath)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}
	clean := filepath.ToSlash(rel)

	l.mu.RLock()
	blocklist := l.filePolicy.Blocklist
	l.mu.RUnlock()

	for _, pattern := range blocklist {
		if matched, _ := filepath.Match(pattern, clean); matched {
			return fmt.Errorf("file path %q matches blocked pattern %q", clean, pattern)
		}

		// Check each path suffix to catch directory-bound patterns
		// at any nesting level, e.g. ".ssh/*" must match:
		//   ".ssh/authorized_keys"                 (suffix starts at .ssh)
		//   "config/infra/.ssh/authorized_keys"    (suffix starts at .ssh)
		// while "*.pem" must still match "secret.pem" at any depth.
		parts := strings.Split(clean, "/")
		for i := 1; i < len(parts); i++ {
			suffix := strings.Join(parts[i:], "/")
			if matched, _ := filepath.Match(pattern, suffix); matched {
				return fmt.Errorf("file path %q matches blocked pattern %q", clean, pattern)
			}
		}
	}
	return nil
}

func (l *LocalExecutor) WriteFile(path string, data []byte) error {
	full, err := l.resolvePath(path)
	if err != nil {
		return err
	}
	if err := l.checkFilePolicy(full); err != nil {
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
	if err := l.checkFilePolicy(full); err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (l *LocalExecutor) DeleteFile(path string) error {
	full, err := l.resolvePath(path)
	if err != nil {
		return err
	}
	if err := l.checkFilePolicy(full); err != nil {
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

	bin := filepath.Base(command[0])

	if err := l.checkCommandPolicy(bin); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = l.rootDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

// checkCommandPolicy enforces the allowlist/blocklist for a command binary name.
func (l *LocalExecutor) checkCommandPolicy(bin string) error {
	l.mu.RLock()
	allowlist := l.policy.Allowlist
	blocklist := l.policy.Blocklist
	l.mu.RUnlock()

	if len(allowlist) > 0 {
		if !slices.Contains(allowlist, bin) {
			return fmt.Errorf("command %q is not in the allowlist", bin)
		}
	}
	if slices.Contains(blocklist, bin) {
		return fmt.Errorf("command %q is blocked", bin)
	}
	return nil
}

// SetPolicy replaces the current command policy.
// Passing nil fields preserves existing values.
func (l *LocalExecutor) SetPolicy(p CommandPolicy) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if p.Allowlist != nil {
		l.policy.Allowlist = slices.Clone(p.Allowlist)
	}
	if p.Blocklist != nil {
		l.policy.Blocklist = slices.Clone(p.Blocklist)
	}
}

// SetFilePolicy replaces the current file policy.
// Passing a nil Blocklist preserves the existing value.
func (l *LocalExecutor) SetFilePolicy(p FilePolicy) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if p.Blocklist != nil {
		l.filePolicy.Blocklist = slices.Clone(p.Blocklist)
	}
}

func (l *LocalExecutor) Close() error {
	return nil // nothing to close
}

// ExecuteTool takes a tool call from the LLM, parses arguments, and runs the local command
func (l *LocalExecutor) ExecuteTool(ctx context.Context, name string, argsJSON string) (string, error) {
	switch name {
	case "write_file":
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments for write_file: %w", err)
		}
		err := l.WriteFile(args.Path, []byte(args.Content))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Successfully wrote to %s", args.Path), nil

	case "run_command":
		var args struct {
			Command []string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments for run_command: %w", err)
		}
		return l.RunCommand(ctx, args.Command)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
