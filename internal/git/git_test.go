package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// =============================================================================
// Phase 1.3: Pure Parsing Functions (No External Dependencies)
// =============================================================================

func TestGetRepoNameFromRemote(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "HTTPS URL with .git",
			url:  "https://github.com/user/myrepo.git",
			want: "myrepo",
		},
		{
			name: "HTTPS URL without .git",
			url:  "https://github.com/user/myrepo",
			want: "myrepo",
		},
		{
			name: "SSH URL with .git",
			url:  "git@github.com:user/myrepo.git",
			want: "myrepo",
		},
		{
			name: "SSH URL without .git",
			url:  "git@github.com:user/myrepo",
			want: "myrepo",
		},
		{
			name: "SSH protocol URL",
			url:  "ssh://git@github.com/user/myrepo.git",
			want: "myrepo",
		},
		{
			name: "GitLab style URL",
			url:  "https://gitlab.com/group/subgroup/project.git",
			want: "project",
		},
		{
			name: "empty string",
			url:  "",
			want: "",
		},
		{
			name: "just repo name",
			url:  "myrepo.git",
			want: "myrepo",
		},
		{
			name: "deep nested path",
			url:  "https://example.com/org/team/subteam/repo.git",
			want: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRepoNameFromRemote(tt.url)
			if got != tt.want {
				t.Errorf("GetRepoNameFromRemote(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "alphanumeric passthrough",
			input: "myrepo123",
			want:  "myrepo123",
		},
		{
			name:  "allows hyphen",
			input: "my-repo",
			want:  "my-repo",
		},
		{
			name:  "allows underscore",
			input: "my_repo",
			want:  "my_repo",
		},
		{
			name:  "replaces spaces",
			input: "my repo",
			want:  "my_repo",
		},
		{
			name:  "replaces dots",
			input: "my.repo",
			want:  "my_repo",
		},
		{
			name:  "replaces special characters",
			input: "my@repo!name#",
			want:  "my_repo_name_",
		},
		{
			name:  "replaces slashes",
			input: "path/to/repo",
			want:  "path_to_repo",
		},
		{
			name:  "preserves mixed case",
			input: "MyRepo",
			want:  "MyRepo",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeRepoName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeRepoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Phase 3.1: Git Operations (Require Git Repos)
// =============================================================================

// Helper function to create a temporary git repo for testing
func setupTestGitRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	dir, err := os.MkdirTemp("", "thts-git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(dir)
		t.Fatalf("failed to initialize git repo: %v", err)
	}

	// Set up git config for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	_ = cmd.Run()

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	return dir, cleanup
}

// Helper to create a git worktree
func setupTestWorktree(t *testing.T, mainRepo string) (string, func()) {
	t.Helper()

	// Create an initial commit so we can create worktrees
	filePath := filepath.Join(mainRepo, "README.md")
	if err := os.WriteFile(filePath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Create worktree directory
	worktreeDir, err := os.MkdirTemp("", "thts-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create worktree directory: %v", err)
	}

	// Remove the directory since git worktree add creates it
	_ = os.RemoveAll(worktreeDir)

	// Create worktree
	cmd = exec.Command("git", "worktree", "add", worktreeDir, "-b", "test-branch")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	cleanup := func() {
		// Remove worktree first
		cmd := exec.Command("git", "worktree", "remove", worktreeDir, "--force")
		cmd.Dir = mainRepo
		_ = cmd.Run()
		_ = os.RemoveAll(worktreeDir)
	}

	return worktreeDir, cleanup
}

func TestIsInGitRepo(t *testing.T) {
	t.Run("inside git repo", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Change to repo directory
		oldDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldDir) }()

		if err := os.Chdir(repoDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		if !IsInGitRepo() {
			t.Error("expected IsInGitRepo() to return true inside a git repo")
		}
	})

	t.Run("outside git repo", func(t *testing.T) {
		// Create a non-git temp directory
		dir, err := os.MkdirTemp("", "thts-non-git-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		// Change to non-git directory
		oldDir, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldDir) }()

		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		if IsInGitRepo() {
			t.Error("expected IsInGitRepo() to return false outside a git repo")
		}
	})
}

func TestIsInGitRepoAt(t *testing.T) {
	t.Run("inside git repo", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		if !IsInGitRepoAt(repoDir) {
			t.Error("expected IsInGitRepoAt() to return true for git repo")
		}
	})

	t.Run("outside git repo", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "thts-non-git-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		if IsInGitRepoAt(dir) {
			t.Error("expected IsInGitRepoAt() to return false for non-git directory")
		}
	})
}

func TestGetGitDirAt(t *testing.T) {
	t.Run("normal repo", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		gitDir, err := GetGitDirAt(repoDir)
		if err != nil {
			t.Fatalf("GetGitDirAt() error: %v", err)
		}

		expected := filepath.Join(repoDir, ".git")
		if gitDir != expected {
			t.Errorf("GetGitDirAt() = %q, want %q", gitDir, expected)
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "thts-non-git-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		_, err = GetGitDirAt(dir)
		if err != ErrNotInGitRepo {
			t.Errorf("GetGitDirAt() error = %v, want ErrNotInGitRepo", err)
		}
	})
}

func TestGetGitCommonDirAt(t *testing.T) {
	t.Run("normal repo", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		commonDir, err := GetGitCommonDirAt(repoDir)
		if err != nil {
			t.Fatalf("GetGitCommonDirAt() error: %v", err)
		}

		// For a normal repo, common dir should be the same as git dir
		expected := filepath.Join(repoDir, ".git")
		if commonDir != expected {
			t.Errorf("GetGitCommonDirAt() = %q, want %q", commonDir, expected)
		}
	})

	t.Run("worktree", func(t *testing.T) {
		mainRepo, cleanup := setupTestGitRepo(t)
		defer cleanup()

		worktree, worktreeCleanup := setupTestWorktree(t, mainRepo)
		defer worktreeCleanup()

		commonDir, err := GetGitCommonDirAt(worktree)
		if err != nil {
			t.Fatalf("GetGitCommonDirAt() error: %v", err)
		}

		// For a worktree, common dir should be the main repo's .git
		expected := filepath.Join(mainRepo, ".git")
		if commonDir != expected {
			t.Errorf("GetGitCommonDirAt() = %q, want %q", commonDir, expected)
		}
	})
}

func TestIsWorktreeAt(t *testing.T) {
	t.Run("normal repo is not worktree", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		if IsWorktreeAt(repoDir) {
			t.Error("expected IsWorktreeAt() to return false for normal repo")
		}
	})

	t.Run("worktree is detected", func(t *testing.T) {
		mainRepo, cleanup := setupTestGitRepo(t)
		defer cleanup()

		worktree, worktreeCleanup := setupTestWorktree(t, mainRepo)
		defer worktreeCleanup()

		if !IsWorktreeAt(worktree) {
			t.Error("expected IsWorktreeAt() to return true for worktree")
		}
	})
}

func TestGetRemoteURLAt(t *testing.T) {
	t.Run("with remote", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Add a remote
		cmd := exec.Command("git", "remote", "add", "origin", "https://github.com/test/repo.git")
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to add remote: %v", err)
		}

		url, err := GetRemoteURLAt(repoDir)
		if err != nil {
			t.Fatalf("GetRemoteURLAt() error: %v", err)
		}

		if url != "https://github.com/test/repo.git" {
			t.Errorf("GetRemoteURLAt() = %q, want https://github.com/test/repo.git", url)
		}
	})

	t.Run("without remote", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		_, err := GetRemoteURLAt(repoDir)
		if err == nil {
			t.Error("expected error for repo without remote")
		}
	})
}

func TestGetRepoTopLevelAt(t *testing.T) {
	t.Run("at repo root", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		topLevel, err := GetRepoTopLevelAt(repoDir)
		if err != nil {
			t.Fatalf("GetRepoTopLevelAt() error: %v", err)
		}

		if topLevel != repoDir {
			t.Errorf("GetRepoTopLevelAt() = %q, want %q", topLevel, repoDir)
		}
	})

	t.Run("in subdirectory", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Create subdirectory
		subDir := filepath.Join(repoDir, "sub", "dir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		topLevel, err := GetRepoTopLevelAt(subDir)
		if err != nil {
			t.Fatalf("GetRepoTopLevelAt() error: %v", err)
		}

		if topLevel != repoDir {
			t.Errorf("GetRepoTopLevelAt() = %q, want %q", topLevel, repoDir)
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "thts-non-git-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		_, err = GetRepoTopLevelAt(dir)
		if err != ErrNotInGitRepo {
			t.Errorf("GetRepoTopLevelAt() error = %v, want ErrNotInGitRepo", err)
		}
	})
}

func TestGetRepoIdentityAt(t *testing.T) {
	t.Run("normal repo uses common-dir identity", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		identity, err := GetRepoIdentityAt(repoDir)
		if err != nil {
			t.Fatalf("GetRepoIdentityAt() error: %v", err)
		}

		expected := "git-common-dir:" + filepath.Join(repoDir, ".git")
		if identity != expected {
			t.Errorf("GetRepoIdentityAt() = %q, want %q", identity, expected)
		}
	})

	t.Run("main repo and worktree share identity", func(t *testing.T) {
		mainRepo, cleanup := setupTestGitRepo(t)
		defer cleanup()

		worktree, worktreeCleanup := setupTestWorktree(t, mainRepo)
		defer worktreeCleanup()

		mainID, err := GetRepoIdentityAt(mainRepo)
		if err != nil {
			t.Fatalf("GetRepoIdentityAt(main) error: %v", err)
		}

		worktreeID, err := GetRepoIdentityAt(worktree)
		if err != nil {
			t.Fatalf("GetRepoIdentityAt(worktree) error: %v", err)
		}

		if mainID != worktreeID {
			t.Errorf("expected shared identity, got main=%q worktree=%q", mainID, worktreeID)
		}
	})
}
