package thts

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Helper function to create a temp directory for testing
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "thts-searchable-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	return dir, func() {
		_ = os.RemoveAll(dir)
	}
}

// getInode returns the inode number of a file
func getInode(t *testing.T, path string) uint64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat %s: %v", path, err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("failed to get syscall.Stat_t for %s", path)
	}

	return stat.Ino
}

func TestCreateSearchableDir(t *testing.T) {
	t.Run("creates hard links for files", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		// Create thoughts directory structure
		thoughtsDir := filepath.Join(dir, "thoughts")
		userDir := filepath.Join(thoughtsDir, "testuser")
		if err := os.MkdirAll(userDir, 0755); err != nil {
			t.Fatalf("failed to create user directory: %v", err)
		}

		// Create a test file directly (not via symlink for this basic test)
		testFile := filepath.Join(userDir, "note.md")
		if err := os.WriteFile(testFile, []byte("# Test Note"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("CreateSearchableDir() error: %v", err)
		}

		if result.LinkedCount != 1 {
			t.Errorf("LinkedCount = %d, want 1", result.LinkedCount)
		}

		// Verify searchable directory was created
		searchDir := filepath.Join(thoughtsDir, "searchable")
		if _, err := os.Stat(searchDir); err != nil {
			t.Errorf("searchable directory not created: %v", err)
		}

		// Verify hard link was created
		linkedFile := filepath.Join(searchDir, "testuser", "note.md")
		if _, err := os.Stat(linkedFile); err != nil {
			t.Errorf("hard link not created: %v", err)
		}

		// Verify it's actually a hard link (same inode)
		originalInode := getInode(t, testFile)
		linkedInode := getInode(t, linkedFile)

		if originalInode != linkedInode {
			t.Errorf("inodes differ: original=%d, linked=%d", originalInode, linkedInode)
		}
	})

	t.Run("skips dotfiles", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		thoughtsDir := filepath.Join(dir, "thoughts")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts directory: %v", err)
		}

		// Create a dotfile
		dotfile := filepath.Join(thoughtsDir, ".hidden")
		if err := os.WriteFile(dotfile, []byte("hidden"), 0644); err != nil {
			t.Fatalf("failed to create dotfile: %v", err)
		}

		// Create a visible file
		visibleFile := filepath.Join(thoughtsDir, "visible.md")
		if err := os.WriteFile(visibleFile, []byte("visible"), 0644); err != nil {
			t.Fatalf("failed to create visible file: %v", err)
		}

		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("CreateSearchableDir() error: %v", err)
		}

		if result.LinkedCount != 1 {
			t.Errorf("LinkedCount = %d, want 1 (should skip dotfile)", result.LinkedCount)
		}

		// Verify dotfile was not linked
		searchDir := filepath.Join(thoughtsDir, "searchable")
		if _, err := os.Stat(filepath.Join(searchDir, ".hidden")); !os.IsNotExist(err) {
			t.Error("dotfile should not be linked")
		}
	})

	t.Run("skips CLAUDE.md", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		thoughtsDir := filepath.Join(dir, "thoughts")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts directory: %v", err)
		}

		// Create CLAUDE.md
		claudeMd := filepath.Join(thoughtsDir, "CLAUDE.md")
		if err := os.WriteFile(claudeMd, []byte("# Claude instructions"), 0644); err != nil {
			t.Fatalf("failed to create CLAUDE.md: %v", err)
		}

		// Create a regular file
		regularFile := filepath.Join(thoughtsDir, "notes.md")
		if err := os.WriteFile(regularFile, []byte("notes"), 0644); err != nil {
			t.Fatalf("failed to create regular file: %v", err)
		}

		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("CreateSearchableDir() error: %v", err)
		}

		if result.LinkedCount != 1 {
			t.Errorf("LinkedCount = %d, want 1 (should skip CLAUDE.md)", result.LinkedCount)
		}

		// Verify CLAUDE.md was not linked
		searchDir := filepath.Join(thoughtsDir, "searchable")
		if _, err := os.Stat(filepath.Join(searchDir, "CLAUDE.md")); !os.IsNotExist(err) {
			t.Error("CLAUDE.md should not be linked")
		}
	})

	t.Run("follows symlinks", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		// Create external directory (simulating thoughts repo)
		externalDir := filepath.Join(dir, "external")
		if err := os.MkdirAll(externalDir, 0755); err != nil {
			t.Fatalf("failed to create external directory: %v", err)
		}

		// Create a file in external directory
		externalFile := filepath.Join(externalDir, "external-note.md")
		if err := os.WriteFile(externalFile, []byte("external"), 0644); err != nil {
			t.Fatalf("failed to create external file: %v", err)
		}

		// Create thoughts directory with symlink to external
		thoughtsDir := filepath.Join(dir, "thoughts")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts directory: %v", err)
		}

		// Create symlink: thoughts/user -> external
		symlink := filepath.Join(thoughtsDir, "user")
		if err := os.Symlink(externalDir, symlink); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("CreateSearchableDir() error: %v", err)
		}

		if result.LinkedCount != 1 {
			t.Errorf("LinkedCount = %d, want 1", result.LinkedCount)
		}

		// Verify hard link points to external file (same inode)
		searchDir := filepath.Join(thoughtsDir, "searchable")
		linkedFile := filepath.Join(searchDir, "user", "external-note.md")

		if _, err := os.Stat(linkedFile); err != nil {
			t.Fatalf("hard link not created: %v", err)
		}

		externalInode := getInode(t, externalFile)
		linkedInode := getInode(t, linkedFile)

		if externalInode != linkedInode {
			t.Errorf("inodes differ: external=%d, linked=%d", externalInode, linkedInode)
		}
	})

	t.Run("skips searchable directory itself", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		thoughtsDir := filepath.Join(dir, "thoughts")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts directory: %v", err)
		}

		// Create a file
		testFile := filepath.Join(thoughtsDir, "note.md")
		if err := os.WriteFile(testFile, []byte("note"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Run CreateSearchableDir twice - should work and not recurse
		_, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("first CreateSearchableDir() error: %v", err)
		}

		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("second CreateSearchableDir() error: %v", err)
		}

		// Should still only link one file, not files from searchable/
		if result.LinkedCount != 1 {
			t.Errorf("LinkedCount = %d, want 1", result.LinkedCount)
		}
	})

	t.Run("handles nested directories", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		thoughtsDir := filepath.Join(dir, "thoughts")
		nestedDir := filepath.Join(thoughtsDir, "user", "projects", "myproject")
		if err := os.MkdirAll(nestedDir, 0755); err != nil {
			t.Fatalf("failed to create nested directory: %v", err)
		}

		// Create files at different levels
		if err := os.WriteFile(filepath.Join(thoughtsDir, "user", "root.md"), []byte("root"), 0644); err != nil {
			t.Fatalf("failed to create root file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(nestedDir, "nested.md"), []byte("nested"), 0644); err != nil {
			t.Fatalf("failed to create nested file: %v", err)
		}

		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("CreateSearchableDir() error: %v", err)
		}

		if result.LinkedCount != 2 {
			t.Errorf("LinkedCount = %d, want 2", result.LinkedCount)
		}

		// Verify nested structure in searchable
		searchDir := filepath.Join(thoughtsDir, "searchable")
		if _, err := os.Stat(filepath.Join(searchDir, "user", "root.md")); err != nil {
			t.Error("root file not linked")
		}
		if _, err := os.Stat(filepath.Join(searchDir, "user", "projects", "myproject", "nested.md")); err != nil {
			t.Error("nested file not linked")
		}
	})

	t.Run("cleans existing searchable directory", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		thoughtsDir := filepath.Join(dir, "thoughts")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts directory: %v", err)
		}

		// Create initial file
		testFile := filepath.Join(thoughtsDir, "note.md")
		if err := os.WriteFile(testFile, []byte("note"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// First run
		_, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("first CreateSearchableDir() error: %v", err)
		}

		// Add a file manually to searchable that shouldn't be there
		searchDir := filepath.Join(thoughtsDir, "searchable")
		staleFile := filepath.Join(searchDir, "stale.md")
		if err := os.WriteFile(staleFile, []byte("stale"), 0644); err != nil {
			t.Fatalf("failed to create stale file: %v", err)
		}

		// Second run should clean up
		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("second CreateSearchableDir() error: %v", err)
		}

		if result.LinkedCount != 1 {
			t.Errorf("LinkedCount = %d, want 1", result.LinkedCount)
		}

		// Stale file should be gone
		if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
			t.Error("stale file should have been removed")
		}
	})

	t.Run("avoids symlink cycles", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		thoughtsDir := filepath.Join(dir, "thoughts")
		cycleDir := filepath.Join(thoughtsDir, "cycle")
		if err := os.MkdirAll(cycleDir, 0755); err != nil {
			t.Fatalf("failed to create cycle directory: %v", err)
		}

		// Create a file
		testFile := filepath.Join(cycleDir, "file.md")
		if err := os.WriteFile(testFile, []byte("file"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Create a symlink that points back to parent (cycle)
		loopSymlink := filepath.Join(cycleDir, "loop")
		if err := os.Symlink(cycleDir, loopSymlink); err != nil {
			t.Fatalf("failed to create cycle symlink: %v", err)
		}

		// Should not hang or error - should complete successfully
		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("CreateSearchableDir() error: %v", err)
		}

		// Should have linked the file but avoided the cycle
		if result.LinkedCount != 1 {
			t.Errorf("LinkedCount = %d, want 1", result.LinkedCount)
		}
	})

	t.Run("handles empty directory", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		thoughtsDir := filepath.Join(dir, "thoughts")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts directory: %v", err)
		}

		result, err := CreateSearchableDir(thoughtsDir)
		if err != nil {
			t.Fatalf("CreateSearchableDir() error: %v", err)
		}

		if result.LinkedCount != 0 {
			t.Errorf("LinkedCount = %d, want 0", result.LinkedCount)
		}

		// Searchable directory should still be created
		searchDir := filepath.Join(thoughtsDir, "searchable")
		if _, err := os.Stat(searchDir); err != nil {
			t.Error("searchable directory should be created even if empty")
		}
	})
}

func TestIsCrossFilesystemError(t *testing.T) {
	// Test the isCrossFilesystemError function
	// Note: Actually triggering EXDEV requires cross-filesystem hard links,
	// which we can't easily do in tests. We'll test the detection logic instead.

	t.Run("syscall.EXDEV is detected", func(t *testing.T) {
		err := syscall.EXDEV
		if !isCrossFilesystemError(err) {
			t.Error("expected EXDEV to be detected as cross-filesystem error")
		}
	})

	t.Run("other errors are not detected", func(t *testing.T) {
		err := syscall.ENOENT
		if isCrossFilesystemError(err) {
			t.Error("ENOENT should not be detected as cross-filesystem error")
		}
	})
}
