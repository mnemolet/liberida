package attachment

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsBlockedPath_SystemPaths(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("system path tests only apply to Unix systems")
	}

	tests := []struct {
		name    string
		path    string
		blocked bool
	}{
		{"/etc/passwd", "/etc/passwd", true},
		{"/etc/shadow", "/etc/shadow", true},
		{"/etc/hosts", "/etc/hosts", true},
		{"/etc/", "/etc/", true},
		{"/proc/self/environ", "/proc/self/environ", true},
		{"/proc/cpuinfo", "/proc/cpuinfo", true},
		{"/sys/devices", "/sys/devices", true},
		{"/dev/sda", "/dev/sda", true},
		{"/boot/grub.cfg", "/boot/grub.cfg", true},
		{"/root/.bashrc", "/root/.bashrc", true},
		{"/run/secrets", "/run/secrets", true},
		{"/sbin/init", "/sbin/init", true},
		{"/bin/bash", "/bin/bash", true},
		{"/lib/systemd", "/lib/systemd", true},
		{"/lib64/ld-linux", "/lib64/ld-linux", true},
		{"/usr/bin/python", "/usr/bin/python", true},
		{"/var/log/syslog", "/var/log/syslog", true},
		{"/opt/secrets", "/opt/secrets", true},
		{"/snap/bin", "/snap/bin", true},
		{"/lost+found", "/lost+found", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, _ := isBlockedPath(tt.path)
			if blocked != tt.blocked {
				t.Errorf("isBlockedPath(%q) = %v, want %v", tt.path, blocked, tt.blocked)
			}
		})
	}
}

func TestIsBlockedPath_AllowedPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"/tmp/test.txt", "/tmp/test.txt"},
		{"/home/user/file.go", "/home/user/file.go"},
		{"/home/user/project/.env", "/home/user/project/.env"},
		{"/tmp/data.txt", "/tmp/data.txt"},
		{"relative/path", "relative/path"},
		{"/mnt/data", "/mnt/data"},
		{"/media/usb", "/media/usb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, _ := isBlockedPath(tt.path)
			if blocked {
				t.Errorf("isBlockedPath(%q) = true, want false", tt.path)
			}
		})
	}
}

func TestIsBlockedPath_HomeSensitiveDirs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{".ssh/id_rsa", filepath.Join(home, ".ssh/id_rsa")},
		{".ssh/config", filepath.Join(home, ".ssh/config")},
		{".ssh", filepath.Join(home, ".ssh")},
		{".gnupg/secring.gpg", filepath.Join(home, ".gnupg/secring.gpg")},
		{".aws/credentials", filepath.Join(home, ".aws/credentials")},
		{".kube/config", filepath.Join(home, ".kube/config")},
		{".docker/config.json", filepath.Join(home, ".docker/config.json")},
		{".password-store", filepath.Join(home, ".password-store")},
		{".gcloud/access.json", filepath.Join(home, ".gcloud/access.json")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, _ := isBlockedPath(tt.path)
			if !blocked {
				t.Errorf("isBlockedPath(%q) = false, want true", tt.path)
			}
		})
	}
}

func TestIsBlockedPath_HomeAllowedDirs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{".bashrc", filepath.Join(home, ".bashrc")},
		{".vimrc", filepath.Join(home, ".vimrc")},
		{".gitconfig", filepath.Join(home, ".gitconfig")},
		{".local/share", filepath.Join(home, ".local/share")},
		{".config/foo", filepath.Join(home, ".config/foo")},
		{"Documents/file.txt", filepath.Join(home, "Documents/file.txt")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, _ := isBlockedPath(tt.path)
			if blocked {
				t.Errorf("isBlockedPath(%q) = true, want false", tt.path)
			}
		})
	}
}

func TestValidateAndRead_SystemFile(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("requires Unix system files")
	}

	h := NewHandler()

	_, err := h.ValidateAndRead("/etc/hosts")
	if err == nil {
		t.Fatal("expected error for system file, got nil")
	}
}

func TestValidateAndRead_SystemFileResult(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("requires Unix system files")
	}

	h := NewHandler()

	_, err := h.ValidateAndRead("/etc/hosts")

	if !errors.Is(err, ErrSystemFile) {
		t.Errorf("expected ErrSystemFile, got %v", err)
	}
}

func TestValidateAndRead_SymlinkToSystemFile(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("requires Unix system files")
	}

	tmpDir := t.TempDir()
	symlink := filepath.Join(tmpDir, "link.txt")

	err := os.Symlink("/etc/hosts", symlink)
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	h := NewHandler()

	_, err = h.ValidateAndRead(symlink)
	if err == nil {
		t.Fatal("expected error for symlink to system file, got nil")
	}
}

func TestValidateAndRead_SymlinkToSystemFileResult(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("requires Unix system files")
	}

	tmpDir := t.TempDir()
	symlink := filepath.Join(tmpDir, "link.txt")

	err := os.Symlink("/etc/hosts", symlink)
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	h := NewHandler()

	_, err = h.ValidateAndRead(symlink)

	if !errors.Is(err, ErrSystemFile) {
		t.Errorf("expected ErrSystemFile, got %v", err)
	}
}

func TestProcessPaths_MixedAllowedAndBlocked(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("requires Unix system files")
	}

	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(validFile, []byte("content"), 0644)

	h := NewHandler()

	components, errs := h.ProcessPaths([]string{validFile, "/etc/hosts", validFile})

	if len(components) != 2 {
		t.Errorf("expected 2 components, got %d", len(components))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestInitWindowsBlocked_NoopOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test only applies to non-Windows systems")
	}

	windowsBlockedOnce = false
	windowsBlockedPrefixes = nil

	initWindowsBlocked()

	if len(windowsBlockedPrefixes) != 0 {
		t.Errorf("expected 0 Windows blocked prefixes on %s, got %d", runtime.GOOS, len(windowsBlockedPrefixes))
	}
	if !windowsBlockedOnce {
		t.Error("windowsBlockedOnce should be true after init")
	}
}

func TestInitWindowsBlocked_Idempotent(t *testing.T) {
	windowsBlockedOnce = false
	windowsBlockedPrefixes = nil

	initWindowsBlocked()
	firstLen := len(windowsBlockedPrefixes)

	windowsBlockedOnce = false

	initWindowsBlocked()
	secondLen := len(windowsBlockedPrefixes)

	if firstLen != secondLen {
		t.Errorf("expected same length after second init (%d vs %d)", firstLen, secondLen)
	}
}
