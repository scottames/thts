package fs

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper function to create a temp directory for testing
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "tpd-fs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	return dir, func() {
		_ = os.RemoveAll(dir)
	}
}

func TestCreateSymlink(t *testing.T) {
	t.Run("creates symlink", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create target file: %v", err)
		}

		link := filepath.Join(dir, "link")
		if err := CreateSymlink(target, link); err != nil {
			t.Fatalf("CreateSymlink() error: %v", err)
		}

		// Verify symlink was created
		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("failed to stat link: %v", err)
		}

		if info.Mode()&os.ModeSymlink == 0 {
			t.Error("expected link to be a symlink")
		}

		// Verify symlink target
		gotTarget, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("failed to read link: %v", err)
		}

		if gotTarget != target {
			t.Errorf("symlink target = %q, want %q", gotTarget, target)
		}
	})

	t.Run("error if exists", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create target file: %v", err)
		}

		link := filepath.Join(dir, "link")
		// Create initial symlink
		if err := CreateSymlink(target, link); err != nil {
			t.Fatalf("first CreateSymlink() error: %v", err)
		}

		// Try to create again - should fail
		err := CreateSymlink(target, link)
		if err == nil {
			t.Error("expected error when creating symlink that already exists")
		}
	})
}

func TestIsSymlink(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create a regular file
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create a directory
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create a symlink
	link := filepath.Join(dir, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "true for symlink",
			path: link,
			want: true,
		},
		{
			name: "false for file",
			path: file,
			want: false,
		},
		{
			name: "false for directory",
			path: subdir,
			want: false,
		},
		{
			name: "false for missing path",
			path: filepath.Join(dir, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSymlink(tt.path)
			if got != tt.want {
				t.Errorf("IsSymlink(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSymlinkTarget(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	got, err := SymlinkTarget(link)
	if err != nil {
		t.Fatalf("SymlinkTarget() error: %v", err)
	}

	if got != target {
		t.Errorf("SymlinkTarget() = %q, want %q", got, target)
	}
}

func TestResolveSymlink(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create target file
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create chain of symlinks: link3 -> link2 -> link1 -> target
	link1 := filepath.Join(dir, "link1")
	link2 := filepath.Join(dir, "link2")
	link3 := filepath.Join(dir, "link3")

	if err := os.Symlink(target, link1); err != nil {
		t.Fatalf("failed to create link1: %v", err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatalf("failed to create link2: %v", err)
	}
	if err := os.Symlink(link2, link3); err != nil {
		t.Fatalf("failed to create link3: %v", err)
	}

	got, err := ResolveSymlink(link3)
	if err != nil {
		t.Fatalf("ResolveSymlink() error: %v", err)
	}

	if got != target {
		t.Errorf("ResolveSymlink() = %q, want %q", got, target)
	}
}

func TestRemoveAll(t *testing.T) {
	t.Run("removes file", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		file := filepath.Join(dir, "file")
		if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		if err := RemoveAll(file); err != nil {
			t.Fatalf("RemoveAll() error: %v", err)
		}

		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Error("expected file to be removed")
		}
	})

	t.Run("removes directory", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		subdir := filepath.Join(dir, "subdir")
		if err := os.MkdirAll(filepath.Join(subdir, "nested"), 0755); err != nil {
			t.Fatalf("failed to create nested directory: %v", err)
		}

		// Create a file inside
		if err := os.WriteFile(filepath.Join(subdir, "nested", "file"), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		if err := RemoveAll(subdir); err != nil {
			t.Fatalf("RemoveAll() error: %v", err)
		}

		if _, err := os.Stat(subdir); !os.IsNotExist(err) {
			t.Error("expected directory to be removed")
		}
	})

	t.Run("removes directory with restricted permissions", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		restricted := filepath.Join(dir, "restricted")
		if err := os.Mkdir(restricted, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		// Create a file inside
		file := filepath.Join(restricted, "file")
		if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		// Remove write permission from directory
		if err := os.Chmod(restricted, 0555); err != nil {
			t.Fatalf("failed to chmod: %v", err)
		}

		if err := RemoveAll(restricted); err != nil {
			t.Fatalf("RemoveAll() error: %v", err)
		}

		if _, err := os.Stat(restricted); !os.IsNotExist(err) {
			t.Error("expected directory to be removed")
		}
	})

	t.Run("no-op for missing path", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		missing := filepath.Join(dir, "nonexistent")

		if err := RemoveAll(missing); err != nil {
			t.Errorf("RemoveAll() for missing path should not error, got: %v", err)
		}
	})
}

func TestEnsureDir(t *testing.T) {
	t.Run("creates nested directories", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		nested := filepath.Join(dir, "a", "b", "c")
		if err := EnsureDir(nested); err != nil {
			t.Fatalf("EnsureDir() error: %v", err)
		}

		info, err := os.Stat(nested)
		if err != nil {
			t.Fatalf("failed to stat directory: %v", err)
		}

		if !info.IsDir() {
			t.Error("expected path to be a directory")
		}
	})

	t.Run("no-op if exists", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		existing := filepath.Join(dir, "existing")
		if err := os.Mkdir(existing, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		// Should not error
		if err := EnsureDir(existing); err != nil {
			t.Errorf("EnsureDir() on existing directory should not error, got: %v", err)
		}

		info, err := os.Stat(existing)
		if err != nil {
			t.Fatalf("failed to stat directory: %v", err)
		}

		if !info.IsDir() {
			t.Error("expected path to remain a directory")
		}
	})
}

func TestExists(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create file
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create symlink to file
	goodLink := filepath.Join(dir, "goodlink")
	if err := os.Symlink(file, goodLink); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Create broken symlink
	brokenLink := filepath.Join(dir, "brokenlink")
	if err := os.Symlink(filepath.Join(dir, "nonexistent"), brokenLink); err != nil {
		t.Fatalf("failed to create broken symlink: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "file exists",
			path: file,
			want: true,
		},
		{
			name: "symlink to existing file",
			path: goodLink,
			want: true,
		},
		{
			name: "broken symlink (follows)",
			path: brokenLink,
			want: false, // Follows symlink, target doesn't exist
		},
		{
			name: "nonexistent path",
			path: filepath.Join(dir, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Exists(tt.path)
			if got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestExistsNoFollow(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create file
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create symlink to file
	goodLink := filepath.Join(dir, "goodlink")
	if err := os.Symlink(file, goodLink); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Create broken symlink
	brokenLink := filepath.Join(dir, "brokenlink")
	if err := os.Symlink(filepath.Join(dir, "nonexistent"), brokenLink); err != nil {
		t.Fatalf("failed to create broken symlink: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "file exists",
			path: file,
			want: true,
		},
		{
			name: "symlink exists",
			path: goodLink,
			want: true,
		},
		{
			name: "broken symlink exists (no follow)",
			path: brokenLink,
			want: true, // Doesn't follow, symlink itself exists
		},
		{
			name: "nonexistent path",
			path: filepath.Join(dir, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExistsNoFollow(tt.path)
			if got != tt.want {
				t.Errorf("ExistsNoFollow(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsDir(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create file
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create subdirectory
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create symlink to directory
	linkToDir := filepath.Join(dir, "linktodir")
	if err := os.Symlink(subdir, linkToDir); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "directory",
			path: subdir,
			want: true,
		},
		{
			name: "file",
			path: file,
			want: false,
		},
		{
			name: "symlink to directory (follows)",
			path: linkToDir,
			want: true,
		},
		{
			name: "nonexistent",
			path: filepath.Join(dir, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDir(tt.path)
			if got != tt.want {
				t.Errorf("IsDir(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsDirNoFollow(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create file
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create subdirectory
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create symlink to directory
	linkToDir := filepath.Join(dir, "linktodir")
	if err := os.Symlink(subdir, linkToDir); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "directory",
			path: subdir,
			want: true,
		},
		{
			name: "file",
			path: file,
			want: false,
		},
		{
			name: "symlink to directory (no follow)",
			path: linkToDir,
			want: false, // Symlink itself is not a directory
		},
		{
			name: "nonexistent",
			path: filepath.Join(dir, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDirNoFollow(tt.path)
			if got != tt.want {
				t.Errorf("IsDirNoFollow(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
