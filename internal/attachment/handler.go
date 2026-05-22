package attachment

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DefaultMaxFileSize = 2 * 1024 * 1024 // 2MB default
)

var (
	ErrFileNotFound    = errors.New("file not found")
	ErrNotRegularFile  = errors.New("not a regular file")
	ErrFileTooLarge    = errors.New("file exceeds maximum size limit")
	ErrSymlinkLoop     = errors.New("symlink loop detected")
	ErrSystemFile      = errors.New("system file access denied")
)

var blockedSystemPrefixes = []string{
	"/etc/",
	"/proc/",
	"/sys/",
	"/dev/",
	"/boot/",
	"/root/",
	"/run/",
	"/sbin/",
	"/bin/",
	"/lib/",
	"/lib64/",
	"/usr/",
	"/var/",
	"/opt/",
	"/snap/",
	"/lost+found",
}

var blockedHomeDirs = []string{
	".ssh",
	".gnupg",
	".aws",
	".gcloud",
	".kube",
	".docker",
	".config/opencode",
	".config/gcloud",
	".password-store",
	".gnome",
}

func isBlockedPath(absPath string) (bool, string) {
	clean := filepath.Clean(absPath)

	for _, prefix := range blockedSystemPrefixes {
		if strings.HasPrefix(clean, prefix) || clean == prefix[:len(prefix)-1] {
			if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
				return true, prefix[:len(prefix)-1]
			}
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, ""
	}

	for _, dir := range blockedHomeDirs {
		blocked := filepath.Join(homeDir, dir)
		if strings.HasPrefix(clean, blocked+string(filepath.Separator)) || clean == blocked {
			return true, blocked
		}
	}

	return false, ""
}

type ContextComponent struct {
	Name    string
	Content string
}

type Handler struct {
	maxFileSize int64
	allowedExts map[string]bool
}

func NewHandler() *Handler {
	return &Handler{
		maxFileSize: DefaultMaxFileSize,
		allowedExts: map[string]bool{
			".txt":  true,
			".log":  true,
			".json": true,
			".yaml": true,
			".yml":  true,
			".md":   true,
			".go":   true,
			".mod":  true,
			".sum":  true,
			".py":   true,
			".js":   true,
			".ts":   true,
			".jsx":  true,
			".tsx":  true,
			".java": true,
			".c":    true,
			".h":    true,
			".cpp":  true,
			".hpp":  true,
			".rs":   true,
			".rb":   true,
			".php":  true,
			".sh":   true,
			".bash": true,
			".zsh":  true,
			".sql":  true,
			".xml":  true,
			".html": true,
			".css":  true,
			".scss": true,
			".toml": true,
			".ini":  true,
			".cfg":  true,
			".conf": true,
			".env":  true,
			".gitignore": true,
			".dockerignore": true,
		},
	}
}

func (h *Handler) SetMaxFileSize(size int64) {
	h.maxFileSize = size
}

func (h *Handler) ParsePaths(paths []string) ([]string, error) {
	var result []string
	for _, p := range paths {
		parts := strings.Split(p, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result, nil
}

func (h *Handler) ValidateAndRead(path string) (*ContextComponent, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	if blocked, match := isBlockedPath(absPath); blocked {
		return nil, fmt.Errorf("%s: %w", match, ErrSystemFile)
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %w", path, ErrFileNotFound)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, ErrSymlinkLoop)
		}

		if blocked, match := isBlockedPath(realPath); blocked {
			return nil, fmt.Errorf("symlink target %s: %w", match, ErrSystemFile)
		}

		info, err = os.Stat(realPath)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, ErrSymlinkLoop)
		}
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %w", path, ErrNotRegularFile)
	}

	if info.Size() > h.maxFileSize {
		return nil, fmt.Errorf("%s (%s): %w", path, formatSize(info.Size()), ErrFileTooLarge)
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != "" && !h.allowedExts[ext] {
		return nil, fmt.Errorf("%s: unsupported file extension '%s'", path, ext)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &ContextComponent{
		Name:    absPath,
		Content: string(content),
	}, nil
}

func (h *Handler) ProcessPaths(paths []string) ([]ContextComponent, []error) {
	var components []ContextComponent
	var errors []error

	parsedPaths, err := h.ParsePaths(paths)
	if err != nil {
		return nil, []error{err}
	}

	for _, p := range parsedPaths {
		comp, err := h.ValidateAndRead(p)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		components = append(components, *comp)
	}

	return components, errors
}

func (h *Handler) FormatComponents(components []ContextComponent) string {
	if len(components) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Attached Files:\n")
	sb.WriteString("================\n\n")

	for _, c := range components {
		sb.WriteString(fmt.Sprintf("=== %s ===\n", filepath.Base(c.Name)))
		sb.WriteString(c.Content)
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}