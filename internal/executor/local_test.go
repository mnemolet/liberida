package executor

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNewLocal(t *testing.T) {
	t.Run("creates sandbox directory", func(t *testing.T) {
		tmp := t.TempDir()
		sandbox := filepath.Join(tmp, "sandbox")

		exec, err := NewLocal(sandbox)
		if err != nil {
			t.Fatalf("NewLocal failed: %v", err)
		}
		if exec.rootDir != sandbox {
			t.Errorf("expected rootDir %q, got %q", sandbox, exec.rootDir)
		}
		if _, err := os.Stat(sandbox); os.IsNotExist(err) {
			t.Error("sandbox directory was not created")
		}
	})

	t.Run("stores absolute path", func(t *testing.T) {
		tmp := t.TempDir()
		sandbox := filepath.Join(tmp, "subdir")

		exec, err := NewLocal(sandbox)
		if err != nil {
			t.Fatalf("NewLocal failed: %v", err)
		}
		if !filepath.IsAbs(exec.rootDir) {
			t.Error("rootDir should be absolute")
		}
	})
}

func TestResolvePath(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		exec := &LocalExecutor{rootDir: "/tmp/test"}
		_, err := exec.resolvePath("")
		if err == nil {
			t.Fatal("expected error for empty path")
		}
	})

	t.Run("absolute path rejected", func(t *testing.T) {
		exec := &LocalExecutor{rootDir: "/tmp/test"}
		_, err := exec.resolvePath("/etc/passwd")
		if err == nil {
			t.Fatal("expected error for absolute path")
		}
	})

	t.Run("simple relative path", func(t *testing.T) {
		tmp := t.TempDir()
		exec := &LocalExecutor{rootDir: tmp}

		got, err := exec.resolvePath("foo/bar.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(tmp, "foo/bar.txt")
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("dot path resolves to root", func(t *testing.T) {
		tmp := t.TempDir()
		exec := &LocalExecutor{rootDir: tmp}

		got, err := exec.resolvePath(".")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != tmp {
			t.Errorf("expected %q, got %q", tmp, got)
		}
	})

	t.Run("naive traversal blocked", func(t *testing.T) {
		tmp := t.TempDir()
		exec := &LocalExecutor{rootDir: tmp}

		_, err := exec.resolvePath("../../etc/passwd")
		if err == nil {
			t.Fatal("expected error for path traversal")
		}
	})

	t.Run("traversal within workspace name blocked", func(t *testing.T) {
		tmp := t.TempDir()
		// rootDir is /tmp/TestFooNNN/proj
		// a path like ../proj-extra/file should be blocked
		exec := &LocalExecutor{rootDir: tmp}

		// Try to escape to a sibling of rootDir
		sibling := filepath.Dir(tmp)
		rel, _ := filepath.Rel(tmp, sibling)
		_, err := exec.resolvePath(filepath.Join(rel, "foo"))
		if err == nil {
			t.Fatal("expected error for traversal to sibling directory")
		}
	})

	t.Run("symlink inside workspace is allowed (existing file)", func(t *testing.T) {
		tmp := t.TempDir()
		targetDir := filepath.Join(tmp, "target")
		linkDir := filepath.Join(tmp, "link")
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(targetDir, linkDir); err != nil {
			t.Fatal(err)
		}
		existingFile := filepath.Join(targetDir, "file.txt")
		if err := os.WriteFile(existingFile, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}

		exec := &LocalExecutor{rootDir: tmp}

		got, err := exec.resolvePath("link/file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// For existing files, symlinks are resolved
		want := existingFile
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("symlink inside workspace is allowed (new file)", func(t *testing.T) {
		tmp := t.TempDir()
		targetDir := filepath.Join(tmp, "target")
		linkDir := filepath.Join(tmp, "link")
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(targetDir, linkDir); err != nil {
			t.Fatal(err)
		}

		exec := &LocalExecutor{rootDir: tmp}

		got, err := exec.resolvePath("link/newfile.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// For non-existent files, the unresolved symlink path is returned;
		// the OS will follow the symlink at IO time.
		want := filepath.Join(tmp, "link/newfile.txt")
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("symlink pointing outside workspace is blocked (existing file)", func(t *testing.T) {
		tmp := t.TempDir()
		outsideDir := filepath.Join(tmp, "..", "outside")
		if err := os.MkdirAll(outsideDir, 0755); err != nil {
			t.Fatal(err)
		}
		linkDir := filepath.Join(tmp, "evil-link")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Fatal(err)
		}
		outsideFile := filepath.Join(outsideDir, "secret.txt")
		if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
			t.Fatal(err)
		}

		exec := &LocalExecutor{rootDir: tmp}

		_, err := exec.resolvePath("evil-link/secret.txt")
		if err == nil {
			t.Fatal("expected error for symlink pointing outside workspace")
		}
	})

	t.Run("symlink pointing outside workspace is blocked (new file)", func(t *testing.T) {
		tmp := t.TempDir()
		outsideDir := filepath.Join(tmp, "..", "outside-write")
		if err := os.MkdirAll(outsideDir, 0755); err != nil {
			t.Fatal(err)
		}
		linkDir := filepath.Join(tmp, "evil-link-w")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Fatal(err)
		}

		exec := &LocalExecutor{rootDir: tmp}

		_, err := exec.resolvePath("evil-link-w/newfile.txt")
		if err == nil {
			t.Fatal("expected error for new file via symlink outside workspace")
		}
	})

	t.Run("non-existent path within workspace is allowed", func(t *testing.T) {
		tmp := t.TempDir()
		exec := &LocalExecutor{rootDir: tmp}

		got, err := exec.resolvePath("newdir/newfile.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(tmp, "newdir/newfile.txt")
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("rootDir symlink resolves correctly (existing file)", func(t *testing.T) {
		tmp := t.TempDir()
		realDir := filepath.Join(tmp, "real")
		linkDir := filepath.Join(tmp, "project")
		if err := os.MkdirAll(realDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(realDir, linkDir); err != nil {
			t.Fatal(err)
		}
		existingFile := filepath.Join(realDir, "foo.txt")
		if err := os.WriteFile(existingFile, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}

		exec := &LocalExecutor{rootDir: linkDir}

		got, err := exec.resolvePath("foo.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := existingFile
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("rootDir symlink (new file)", func(t *testing.T) {
		tmp := t.TempDir()
		realDir := filepath.Join(tmp, "real")
		linkDir := filepath.Join(tmp, "project")
		if err := os.MkdirAll(realDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(realDir, linkDir); err != nil {
			t.Fatal(err)
		}

		exec := &LocalExecutor{rootDir: linkDir}

		got, err := exec.resolvePath("newfile.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Non-existent file through symlink rootDir: returns unresolved path
		want := filepath.Join(linkDir, "newfile.txt")
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("WriteFile through valid path succeeds", func(t *testing.T) {
		tmp := t.TempDir()
		exec := &LocalExecutor{rootDir: tmp}

		if err := exec.WriteFile("test.txt", []byte("hello")); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmp, "test.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "hello" {
			t.Errorf("expected 'hello', got %q", string(data))
		}
	})

	t.Run("WriteFile with traversal is blocked", func(t *testing.T) {
		tmp := t.TempDir()
		exec := &LocalExecutor{rootDir: tmp}

		err := exec.WriteFile("../../etc/evil", []byte("hack"))
		if err == nil {
			t.Fatal("expected error for path traversal in WriteFile")
		}
	})

	t.Run("ReadFile through valid path succeeds", func(t *testing.T) {
		tmp := t.TempDir()
		exec := &LocalExecutor{rootDir: tmp}
		os.WriteFile(filepath.Join(tmp, "data.txt"), []byte("content"), 0644)

		data, err := exec.ReadFile("data.txt")
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		if string(data) != "content" {
			t.Errorf("expected 'content', got %q", string(data))
		}
	})

	t.Run("DeleteFile through valid path succeeds", func(t *testing.T) {
		tmp := t.TempDir()
		exec := &LocalExecutor{rootDir: tmp}
		os.WriteFile(filepath.Join(tmp, "del.txt"), []byte("x"), 0644)

		if err := exec.DeleteFile("del.txt"); err != nil {
			t.Fatalf("DeleteFile failed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(tmp, "del.txt")); !os.IsNotExist(err) {
			t.Error("file should have been deleted")
		}
	})
}

func TestIsWithinRoot(t *testing.T) {
	t.Run("path inside root", func(t *testing.T) {
		if !isWithinRoot("/home/user/proj", "/home/user/proj/file.txt") {
			t.Error("expected true")
		}
	})

	t.Run("path equal to root", func(t *testing.T) {
		if !isWithinRoot("/home/user/proj", "/home/user/proj") {
			t.Error("expected true")
		}
	})

	t.Run("path with common prefix escapes", func(t *testing.T) {
		if isWithinRoot("/home/user/proj", "/home/user/projextra") {
			t.Error("expected false: projextra is not inside proj")
		}
	})

	t.Run("path outside root", func(t *testing.T) {
		if isWithinRoot("/home/user/proj", "/etc/passwd") {
			t.Error("expected false")
		}
	})
}

func TestDefaultBlocklistBlocksDangerousCommands(t *testing.T) {
	tests := []struct {
		cmd string
	}{
		{"dd"}, {"mkfs"}, {"fdisk"}, {"parted"}, {"gdisk"},
		{"shutdown"}, {"reboot"}, {"halt"}, {"poweroff"}, {"init"},
		{"sudo"}, {"doas"}, {"su"},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			exec := &LocalExecutor{
				rootDir: t.TempDir(),
				policy:  CommandPolicy{Blocklist: slices.Clone(defaultBlocklist)},
			}
			_, err := exec.RunCommand(context.Background(), []string{tt.cmd})
			if err == nil {
				t.Fatal("expected error for blocked command")
			}
			if !strings.Contains(err.Error(), "is blocked") {
				t.Errorf("expected 'is blocked' error, got: %v", err)
			}
		})
	}
}

func TestRunCommand_Empty(t *testing.T) {
	exec := &LocalExecutor{rootDir: t.TempDir()}
	_, err := exec.RunCommand(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestRunCommand_AllowedCommandSucceeds(t *testing.T) {
	exec := &LocalExecutor{rootDir: t.TempDir()}
	out, err := exec.RunCommand(context.Background(), []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected output containing 'hello', got %q", out)
	}
}

func TestRunCommand_WithPathPrefix(t *testing.T) {
	exec := &LocalExecutor{
		rootDir: t.TempDir(),
		policy:  CommandPolicy{Blocklist: []string{"rm"}},
	}
	_, err := exec.RunCommand(context.Background(), []string{"/usr/bin/rm", "file"})
	if err == nil {
		t.Fatal("expected error: /usr/bin/rm should match 'rm' in blocklist")
	}
	if !strings.Contains(err.Error(), "is blocked") {
		t.Errorf("expected 'is blocked' error, got: %v", err)
	}
}

func TestRunCommand_AllowlistEnforced(t *testing.T) {
	exec := &LocalExecutor{
		rootDir: t.TempDir(),
		policy:  CommandPolicy{Allowlist: []string{"ls"}},
	}
	_, err := exec.RunCommand(context.Background(), []string{"echo", "test"})
	if err == nil {
		t.Fatal("expected error: echo is not in allowlist")
	}
	if !strings.Contains(err.Error(), "not in the allowlist") {
		t.Errorf("expected 'not in the allowlist' error, got: %v", err)
	}
}

func TestRunCommand_AllowlistHonored(t *testing.T) {
	exec := &LocalExecutor{
		rootDir: t.TempDir(),
		policy:  CommandPolicy{Allowlist: []string{"echo"}},
	}
	out, err := exec.RunCommand(context.Background(), []string{"echo", "allowed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "allowed") {
		t.Errorf("expected output containing 'allowed', got %q", out)
	}
}

func TestRunCommand_BlocklistOverridesAllowlist(t *testing.T) {
	exec := &LocalExecutor{
		rootDir: t.TempDir(),
		policy: CommandPolicy{
			Allowlist: []string{"sudo", "echo"},
			Blocklist: []string{"sudo"},
		},
	}
	// sudo is in both allowlist and blocklist — blocklist wins
	_, err := exec.RunCommand(context.Background(), []string{"sudo", "whoami"})
	if err == nil {
		t.Fatal("expected error: sudo is in blocklist despite being in allowlist")
	}
	// echo is only in allowlist — allowed
	out, err := exec.RunCommand(context.Background(), []string{"echo", "ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("expected output containing 'ok', got %q", out)
	}
}

func TestSetPolicy(t *testing.T) {
	exec := &LocalExecutor{rootDir: t.TempDir()}

	exec.SetPolicy(CommandPolicy{
		Allowlist: []string{"ls", "echo", "cat"},
		Blocklist: []string{"rm", "cat"},
	})

	// rm is not in allowlist → blocked by allowlist
	_, err := exec.RunCommand(context.Background(), []string{"rm", "file"})
	if err == nil {
		t.Fatal("expected error: rm is not in allowlist")
	}
	if !strings.Contains(err.Error(), "not in the allowlist") {
		t.Errorf("expected 'not in the allowlist' error, got: %v", err)
	}

	// cat is in allowlist but also in blocklist → blocklist wins
	_, err = exec.RunCommand(context.Background(), []string{"cat", "file"})
	if err == nil {
		t.Fatal("expected error: cat should be blocked")
	}
	if !strings.Contains(err.Error(), "is blocked") {
		t.Errorf("expected 'is blocked' error, got: %v", err)
	}

	// echo is in allowlist and not in blocklist → allowed
	out, err := exec.RunCommand(context.Background(), []string{"echo", "ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("expected output containing 'ok', got %q", out)
	}
}

func TestNewLocalInitializesFileBlocklist(t *testing.T) {
	exec, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}
	if len(exec.filePolicy.Blocklist) == 0 {
		t.Fatal("expected file blocklist to be initialized")
	}
}

func TestCheckFilePolicy(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "blocked .env file", path: ".env", wantErr: true},
		{name: "blocked .env.prod", path: ".env.production", wantErr: true},
		{name: "blocked .env.local", path: ".env.local", wantErr: true},
		{name: "blocked pem file", path: "cert.pem", wantErr: true},
		{name: "blocked key file", path: "private.key", wantErr: true},
		{name: "blocked id_rsa", path: "id_rsa", wantErr: true},
		{name: "blocked id_ed25519", path: "id_ed25519", wantErr: true},
		{name: "blocked id_ecdsa", path: "id_ecdsa", wantErr: true},
		{name: "blocked .ssh file", path: ".ssh/authorized_keys", wantErr: true},
		{name: "blocked .netrc", path: ".netrc", wantErr: true},
		{name: "blocked .git-credentials", path: ".git-credentials", wantErr: true},
		{name: "blocked key in subdir", path: "config/keys/secret.pem", wantErr: true},
		{name: "blocked .env in subdir", path: "project/.env", wantErr: true},
		{name: "blocked .ssh nested deep", path: "config/infra/.ssh/authorized_keys", wantErr: true},
		{name: "blocked .gnupg nested", path: "users/alice/.gnupg/pubring.kbx", wantErr: true},
		{name: "blocked id_rsa nested", path: "backup/ssh/id_rsa.old", wantErr: true},
		{name: "allowed regular file", path: "main.go", wantErr: false},
		{name: "allowed .txt", path: "notes.txt", wantErr: false},
		{name: "allowed nested file", path: "src/main.go", wantErr: false},
	}

	exec := &LocalExecutor{
		rootDir: tmp,
		filePolicy: FilePolicy{
			Blocklist: slices.Clone(defaultFileBlocklist),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath := filepath.Join(tmp, tt.path)
			err := exec.checkFilePolicy(absPath)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for path %q", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for path %q: %v", tt.path, err)
			}
		})
	}
}

func TestWriteFile_BlockedPaths(t *testing.T) {
	tmp := t.TempDir()
	exec, err := NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}

	blockedPaths := []string{
		".env",
		".env.production",
		"config/keys/cert.pem",
		".ssh/id_rsa.pub",
		"config/infra/.ssh/authorized_keys",
		"id_ed25519",
		".netrc",
		".git-credentials",
	}

	for _, p := range blockedPaths {
		t.Run(p, func(t *testing.T) {
			err := exec.WriteFile(p, []byte("content"))
			if err == nil {
				t.Fatal("expected error for blocked path")
			}
			if !strings.Contains(err.Error(), "matches blocked pattern") {
				t.Errorf("expected 'matches blocked pattern' error, got: %v", err)
			}
		})
	}
}

func TestWriteFile_AllowedPaths(t *testing.T) {
	tmp := t.TempDir()
	exec, err := NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}

	allowedPaths := []string{
		"main.go",
		"src/app.go",
		"README.md",
		"config/settings.json",
		"scripts/build.sh",
	}

	for _, p := range allowedPaths {
		t.Run(p, func(t *testing.T) {
			err := exec.WriteFile(p, []byte("content"))
			if err != nil {
				t.Fatalf("unexpected error for path %q: %v", p, err)
			}
		})
	}
}

func TestReadFile_BlockedPath(t *testing.T) {
	tmp := t.TempDir()
	exec, err := NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}

	_, err = exec.ReadFile(".env")
	if err == nil {
		t.Fatal("expected error for blocked path")
	}
	if !strings.Contains(err.Error(), "matches blocked pattern") {
		t.Errorf("expected 'matches blocked pattern' error, got: %v", err)
	}
}

func TestDeleteFile_BlockedPath(t *testing.T) {
	tmp := t.TempDir()
	exec, err := NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}

	err = exec.DeleteFile(".env")
	if err == nil {
		t.Fatal("expected error for blocked path")
	}
	if !strings.Contains(err.Error(), "matches blocked pattern") {
		t.Errorf("expected 'matches blocked pattern' error, got: %v", err)
	}
}

func TestSetFilePolicy(t *testing.T) {
	tmp := t.TempDir()
	exec := &LocalExecutor{
		rootDir: tmp,
		filePolicy: FilePolicy{
			Blocklist: []string{"*.env"},
		},
	}

	// Replace blocklist entirely
	exec.SetFilePolicy(FilePolicy{
		Blocklist: []string{"*.pem"},
	})

	if err := exec.checkFilePolicy(filepath.Join(tmp, "secret.pem")); err == nil {
		t.Fatal("expected .pem files to be blocked after policy update")
	}
	// Old blocklist is replaced, so .env should no longer be blocked
	if err := exec.checkFilePolicy(filepath.Join(tmp, ".env")); err != nil {
		t.Fatal("expected .env to not be blocked after blocklist replacement")
	}
}

func TestWriteFile_TraversalBypassBlocked(t *testing.T) {
	tmp := t.TempDir()
	exec, err := NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}

	// Traversal is caught by resolvePath before checkFilePolicy even runs
	err = exec.WriteFile("subdir/../../.env", []byte("hacked"))
	if err == nil {
		t.Fatal("expected error for traversal path")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Errorf("expected sandbox escape error, got: %v", err)
	}
}

func TestWriteFile_SymlinkToBlockedFile(t *testing.T) {
	tmp := t.TempDir()
	exec, err := NewLocal(tmp)
	if err != nil {
		t.Fatal(err)
	}

	// Create a .env file and a symlink pointing to it
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../.env", filepath.Join(tmp, "config/link.env")); err != nil {
		t.Fatal(err)
	}

	// Writing through the symlink must be blocked:
	// resolvePath resolves the symlink -> full path to .env
	// checkFilePolicy then matches the resolved path against blocklist
	err = exec.WriteFile("config/link.env", []byte("hacked"))
	if err == nil {
		t.Fatal("expected error for writing to .env through symlink")
	}
	if !strings.Contains(err.Error(), "matches blocked pattern") {
		t.Errorf("expected 'matches blocked pattern' error, got: %v", err)
	}
}

func TestSetFilePolicy_NilPreservesBlocklist(t *testing.T) {
	tmp := t.TempDir()
	exec := &LocalExecutor{
		rootDir: tmp,
		filePolicy: FilePolicy{
			Blocklist: []string{"*.env"},
		},
	}

	// Nil blocklist should preserve existing
	exec.SetFilePolicy(FilePolicy{})

	if err := exec.checkFilePolicy(filepath.Join(tmp, ".env")); err == nil {
		t.Fatal("expected .env to remain blocked after nil update")
	}
}

func TestSetPolicy_NilFieldsLeftUnchanged(t *testing.T) {
	exec := &LocalExecutor{
		rootDir: t.TempDir(),
		policy: CommandPolicy{
			Allowlist: []string{"echo", "rm"},
			Blocklist: []string{"rm"},
		},
	}

	// Only update allowlist, blocklist should stay (rm still blocked)
	exec.SetPolicy(CommandPolicy{
		Allowlist: []string{"echo", "rm"},
	})

	// rm is in both allowlist and blocklist → blocklist wins
	_, err := exec.RunCommand(context.Background(), []string{"rm", "file"})
	if err == nil {
		t.Fatal("expected error: rm should still be blocked")
	}
	if !strings.Contains(err.Error(), "is blocked") {
		t.Errorf("expected 'is blocked' error, got: %v", err)
	}
}

func TestRunCommand_CancelledContext(t *testing.T) {
	exec := &LocalExecutor{rootDir: t.TempDir()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := exec.RunCommand(ctx, []string{"sleep", "10"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
