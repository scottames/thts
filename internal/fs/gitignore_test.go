package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottames/thts/internal/config"
)

func TestAddToGitignore(t *testing.T) {
	t.Run("project mode creates gitignore", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		added, err := AddToGitignore(dir, "thoughts/", config.ComponentModeLocal)
		if err != nil {
			t.Fatalf("AddToGitignore() error: %v", err)
		}

		if !added {
			t.Error("expected pattern to be added")
		}

		content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		if !strings.Contains(string(content), "thoughts/") {
			t.Error("expected .gitignore to contain thoughts/")
		}
	})

	t.Run("project mode appends to existing", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		gitignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(gitignore, []byte("node_modules/\n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		added, err := AddToGitignore(dir, "thoughts/", config.ComponentModeLocal)
		if err != nil {
			t.Fatalf("AddToGitignore() error: %v", err)
		}

		if !added {
			t.Error("expected pattern to be added")
		}

		content, err := os.ReadFile(gitignore)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "node_modules/") {
			t.Error("expected .gitignore to still contain node_modules/")
		}
		if !strings.Contains(contentStr, "thoughts/") {
			t.Error("expected .gitignore to contain thoughts/")
		}
	})

	t.Run("project mode adds newline before pattern if needed", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		gitignore := filepath.Join(dir, ".gitignore")
		// No trailing newline
		if err := os.WriteFile(gitignore, []byte("node_modules/"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		_, err := AddToGitignore(dir, "thoughts/", config.ComponentModeLocal)
		if err != nil {
			t.Fatalf("AddToGitignore() error: %v", err)
		}

		content, err := os.ReadFile(gitignore)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		// Should have proper newline separation
		if !strings.Contains(string(content), "node_modules/\nthoughts/") {
			t.Errorf("expected proper newline separation, got: %q", string(content))
		}
	})

	t.Run("global mode uses XDG config", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		// Set XDG_CONFIG_HOME to temp dir
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		defer func() {
			if originalXDG != "" {
				_ = os.Setenv("XDG_CONFIG_HOME", originalXDG)
			} else {
				_ = os.Unsetenv("XDG_CONFIG_HOME")
			}
		}()

		xdgConfig := filepath.Join(dir, ".config")
		if err := os.Setenv("XDG_CONFIG_HOME", xdgConfig); err != nil {
			t.Fatalf("failed to set XDG_CONFIG_HOME: %v", err)
		}

		// Use a different directory as the "repo" since the global gitignore
		// is not in the repo
		repoDir := filepath.Join(dir, "repo")
		if err := os.Mkdir(repoDir, 0755); err != nil {
			t.Fatalf("failed to create repo directory: %v", err)
		}

		added, err := AddToGitignore(repoDir, "thoughts/", config.ComponentModeGlobal)
		if err != nil {
			t.Fatalf("AddToGitignore() error: %v", err)
		}

		if !added {
			t.Error("expected pattern to be added")
		}

		globalIgnore := filepath.Join(xdgConfig, "git", "ignore")
		content, err := os.ReadFile(globalIgnore)
		if err != nil {
			t.Fatalf("failed to read global gitignore: %v", err)
		}

		if !strings.Contains(string(content), "thoughts/") {
			t.Error("expected global gitignore to contain thoughts/")
		}
	})

	t.Run("disabled mode does nothing", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		added, err := AddToGitignore(dir, "thoughts/", config.ComponentModeDisabled)
		if err != nil {
			t.Fatalf("AddToGitignore() error: %v", err)
		}

		if added {
			t.Error("expected pattern not to be added in disabled mode")
		}

		if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
			t.Error("expected .gitignore not to be created in disabled mode")
		}
	})

	t.Run("pattern already exists", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		gitignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(gitignore, []byte("thoughts/\n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		added, err := AddToGitignore(dir, "thoughts/", config.ComponentModeLocal)
		if err != nil {
			t.Fatalf("AddToGitignore() error: %v", err)
		}

		if added {
			t.Error("expected pattern not to be added when already exists")
		}

		// Content should not be duplicated
		content, err := os.ReadFile(gitignore)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		count := strings.Count(string(content), "thoughts/")
		if count != 1 {
			t.Errorf("expected exactly 1 occurrence of pattern, got %d", count)
		}
	})

	t.Run("pattern with whitespace matches trimmed", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		gitignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(gitignore, []byte("  thoughts/  \n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		added, err := AddToGitignore(dir, "thoughts/", config.ComponentModeLocal)
		if err != nil {
			t.Fatalf("AddToGitignore() error: %v", err)
		}

		if added {
			t.Error("expected pattern not to be added when already exists (with whitespace)")
		}
	})
}

func TestRemoveFromGitignore(t *testing.T) {
	t.Run("removes pattern", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		gitignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(gitignore, []byte("node_modules/\nthoughts/\n.env\n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		removed, err := RemoveFromGitignore(dir, "thoughts/", config.ComponentModeLocal)
		if err != nil {
			t.Fatalf("RemoveFromGitignore() error: %v", err)
		}

		if !removed {
			t.Error("expected pattern to be removed")
		}

		content, err := os.ReadFile(gitignore)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		contentStr := string(content)
		if strings.Contains(contentStr, "thoughts/") {
			t.Error("expected thoughts/ to be removed")
		}
		if !strings.Contains(contentStr, "node_modules/") {
			t.Error("expected node_modules/ to remain")
		}
		if !strings.Contains(contentStr, ".env") {
			t.Error("expected .env to remain")
		}
	})

	t.Run("no-op if pattern missing", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		gitignore := filepath.Join(dir, ".gitignore")
		originalContent := "node_modules/\n.env\n"
		if err := os.WriteFile(gitignore, []byte(originalContent), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		removed, err := RemoveFromGitignore(dir, "thoughts/", config.ComponentModeLocal)
		if err != nil {
			t.Fatalf("RemoveFromGitignore() error: %v", err)
		}

		if removed {
			t.Error("expected pattern not to be removed when not present")
		}
	})

	t.Run("no-op if file missing", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		removed, err := RemoveFromGitignore(dir, "thoughts/", config.ComponentModeLocal)
		if err != nil {
			t.Fatalf("RemoveFromGitignore() error: %v", err)
		}

		if removed {
			t.Error("expected pattern not to be removed when file doesn't exist")
		}
	})

	t.Run("disabled mode does nothing", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		gitignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(gitignore, []byte("thoughts/\n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		removed, err := RemoveFromGitignore(dir, "thoughts/", config.ComponentModeDisabled)
		if err != nil {
			t.Fatalf("RemoveFromGitignore() error: %v", err)
		}

		if removed {
			t.Error("expected pattern not to be removed in disabled mode")
		}
	})

	t.Run("removes from global mode", func(t *testing.T) {
		dir, cleanup := setupTestDir(t)
		defer cleanup()

		// Set XDG_CONFIG_HOME to temp dir
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		defer func() {
			if originalXDG != "" {
				_ = os.Setenv("XDG_CONFIG_HOME", originalXDG)
			} else {
				_ = os.Unsetenv("XDG_CONFIG_HOME")
			}
		}()

		xdgConfig := filepath.Join(dir, ".config")
		if err := os.Setenv("XDG_CONFIG_HOME", xdgConfig); err != nil {
			t.Fatalf("failed to set XDG_CONFIG_HOME: %v", err)
		}

		// Create global gitignore
		globalIgnoreDir := filepath.Join(xdgConfig, "git")
		if err := os.MkdirAll(globalIgnoreDir, 0755); err != nil {
			t.Fatalf("failed to create git config dir: %v", err)
		}

		globalIgnore := filepath.Join(globalIgnoreDir, "ignore")
		if err := os.WriteFile(globalIgnore, []byte("thoughts/\n"), 0644); err != nil {
			t.Fatalf("failed to create global gitignore: %v", err)
		}

		// Use a different directory as the "repo"
		repoDir := filepath.Join(dir, "repo")
		if err := os.Mkdir(repoDir, 0755); err != nil {
			t.Fatalf("failed to create repo directory: %v", err)
		}

		removed, err := RemoveFromGitignore(repoDir, "thoughts/", config.ComponentModeGlobal)
		if err != nil {
			t.Fatalf("RemoveFromGitignore() error: %v", err)
		}

		if !removed {
			t.Error("expected pattern to be removed")
		}

		content, err := os.ReadFile(globalIgnore)
		if err != nil {
			t.Fatalf("failed to read global gitignore: %v", err)
		}

		if strings.Contains(string(content), "thoughts/") {
			t.Error("expected thoughts/ to be removed from global gitignore")
		}
	})
}
