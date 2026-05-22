package executor

import (
	"os"
	"path/filepath"
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
