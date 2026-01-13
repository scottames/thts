//go:build integration

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottames/thts/internal/config"
	"gopkg.in/yaml.v3"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build binary once for all tests
	tmp, err := os.MkdirTemp("", "thts-test-bin-*")
	if err != nil {
		os.Exit(1)
	}

	binaryPath = filepath.Join(tmp, "thts")

	// Find project root by looking for go.mod
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		os.Stderr.WriteString("Failed to find project root (go.mod)\n")
		os.Exit(1)
	}

	// Build from the project root
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/thts")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.WriteString("Failed to build thts binary: " + err.Error() + "\n")
		os.Stderr.Write(output)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func findProjectRoot() string {
	dir := mustGetwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}

// testEnv holds the test environment paths
type testEnv struct {
	// Root temp directory for this test
	root string
	// XDG_CONFIG_HOME for this test
	configHome string
	// Path to a git repo that simulates "current project"
	projectRepo string
	// Path to a git repo that simulates "central thoughts repo"
	thoughtsRepo string
	// Cleanup function
	cleanup func()
}

// setupTestEnv creates a complete test environment with:
// - A "project" git repo (simulating user's project)
// - A "thoughts" git repo (simulating central thoughts repo)
// - A config file pointing to the thoughts repo
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	root, err := os.MkdirTemp("", "thts-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp root: %v", err)
	}

	env := &testEnv{
		root:         root,
		configHome:   filepath.Join(root, "config"),
		projectRepo:  filepath.Join(root, "project"),
		thoughtsRepo: filepath.Join(root, "thoughts-repo"),
	}

	env.cleanup = func() {
		_ = os.RemoveAll(root)
	}

	// Create config directory
	if err := os.MkdirAll(filepath.Join(env.configHome, "thts"), 0755); err != nil {
		env.cleanup()
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create and initialize project repo
	if err := os.MkdirAll(env.projectRepo, 0755); err != nil {
		env.cleanup()
		t.Fatalf("failed to create project dir: %v", err)
	}
	if err := initGitRepo(env.projectRepo); err != nil {
		env.cleanup()
		t.Fatalf("failed to init project repo: %v", err)
	}

	// Create and initialize thoughts repo
	if err := os.MkdirAll(env.thoughtsRepo, 0755); err != nil {
		env.cleanup()
		t.Fatalf("failed to create thoughts dir: %v", err)
	}
	if err := initGitRepo(env.thoughtsRepo); err != nil {
		env.cleanup()
		t.Fatalf("failed to init thoughts repo: %v", err)
	}

	// Create initial config
	cfg := &config.Config{
		User:                "testuser",
		Gitignore:           config.ComponentModeLocal,
		AutoSyncInWorktrees: true,
		RepoMappings:        make(map[string]*config.RepoMapping),
		Profiles: map[string]*config.ProfileConfig{
			"default": {
				ThoughtsRepo: env.thoughtsRepo,
				ReposDir:     "repos",
				GlobalDir:    "global",
				Default:      true,
			},
		},
	}

	configPath := filepath.Join(env.configHome, "thts", "config.yaml")
	configData, err := yaml.Marshal(cfg)
	if err != nil {
		env.cleanup()
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		env.cleanup()
		t.Fatalf("failed to write config: %v", err)
	}

	return env
}

// initGitRepo initializes a git repo with basic config
func initGitRepo(dir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	return cmd.Run()
}

// runThts runs the thts binary with the given args and environment
func (e *testEnv) runThts(args ...string) (string, error) {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = e.projectRepo
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+e.configHome,
		"HOME="+e.root, // Ensure ~ expansion works correctly
	)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// runThtsInDir runs the thts binary in a specific directory
func (e *testEnv) runThtsInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+e.configHome,
		"HOME="+e.root,
	)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestInitCommand(t *testing.T) {
	t.Run("initializes thoughts in project repo", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Run: thts init --name myproject --force
		output, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("thts init failed: %v\nOutput: %s", err, output)
		}

		// Verify success message
		if !strings.Contains(output, "Thoughts setup complete") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify thoughts directory was created
		thoughtsDir := filepath.Join(env.projectRepo, "thoughts")
		if _, err := os.Stat(thoughtsDir); err != nil {
			t.Errorf("thoughts directory not created: %v", err)
		}

		// Verify symlinks were created
		userSymlink := filepath.Join(thoughtsDir, "testuser")
		sharedSymlink := filepath.Join(thoughtsDir, "shared")
		globalSymlink := filepath.Join(thoughtsDir, "global")

		for _, symlink := range []string{userSymlink, sharedSymlink, globalSymlink} {
			info, err := os.Lstat(symlink)
			if err != nil {
				t.Errorf("symlink not created: %s - %v", symlink, err)
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s is not a symlink", symlink)
			}
		}

		// Verify structure was created in thoughts repo
		repoDir := filepath.Join(env.thoughtsRepo, "repos", "myproject")
		if _, err := os.Stat(repoDir); err != nil {
			t.Errorf("repo directory not created in thoughts repo: %v", err)
		}

		userDir := filepath.Join(repoDir, "testuser")
		if _, err := os.Stat(userDir); err != nil {
			t.Errorf("user directory not created: %v", err)
		}

		// Verify .gitignore was updated
		gitignore := filepath.Join(env.projectRepo, ".gitignore")
		content, err := os.ReadFile(gitignore)
		if err != nil {
			t.Errorf("gitignore not created: %v", err)
		} else if !strings.Contains(string(content), "thoughts/") {
			t.Errorf("gitignore doesn't contain thoughts/: %s", content)
		}

		// Verify hooks were installed
		preCommit := filepath.Join(env.projectRepo, ".git", "hooks", "pre-commit")
		if _, err := os.Stat(preCommit); err != nil {
			t.Errorf("pre-commit hook not installed: %v", err)
		}

		postCommit := filepath.Join(env.projectRepo, ".git", "hooks", "post-commit")
		if _, err := os.Stat(postCommit); err != nil {
			t.Errorf("post-commit hook not installed: %v", err)
		}

		// Verify config was updated with mapping
		configPath := filepath.Join(env.configHome, "thts", "config.yaml")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}

		var cfg config.Config
		if err := yaml.Unmarshal(configData, &cfg); err != nil {
			t.Fatalf("failed to parse config: %v", err)
		}

		mapping := cfg.RepoMappings[env.projectRepo]
		if mapping == nil {
			t.Errorf("repo mapping not added to config")
		} else {
			if mapping.Repo != "myproject" {
				t.Errorf("repo mapping name = %q, want %q", mapping.Repo, "myproject")
			}
			// Verify profile is explicitly set even without --profile flag
			if mapping.Profile != "default" {
				t.Errorf("repo mapping profile = %q, want %q", mapping.Profile, "default")
			}
		}
	})

	t.Run("fails outside git repo", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Create a non-git directory
		nonGitDir := filepath.Join(env.root, "non-git")
		if err := os.MkdirAll(nonGitDir, 0755); err != nil {
			t.Fatalf("failed to create non-git dir: %v", err)
		}

		output, err := env.runThtsInDir(nonGitDir, "init", "--name", "test", "--force")
		if err == nil {
			t.Error("expected error when running outside git repo")
		}
		if !strings.Contains(output, "not in a git repository") {
			t.Errorf("expected 'not in a git repository' error, got: %s", output)
		}
	})

	t.Run("reinit with --force works", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// First init
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("first init failed: %v", err)
		}

		// Second init with --force should succeed
		output, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("second init failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "Thoughts setup complete") {
			t.Errorf("expected success message on reinit, got: %s", output)
		}
	})
}

func TestStatusCommand(t *testing.T) {
	t.Run("shows status after init", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize first
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Run status
		output, err := env.runThts("status")
		if err != nil {
			t.Fatalf("status failed: %v\nOutput: %s", err, output)
		}

		// Verify expected content
		expectedStrings := []string{
			"Thoughts Repository Status",
			"Configuration",
			"testuser",
			"Initialized",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(output, expected) {
				t.Errorf("status output missing %q\nOutput: %s", expected, output)
			}
		}
	})

	t.Run("shows error when not configured", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Remove config
		configPath := filepath.Join(env.configHome, "thts", "config.yaml")
		if err := os.Remove(configPath); err != nil {
			t.Fatalf("failed to remove config: %v", err)
		}

		output, _ := env.runThts("status")
		if !strings.Contains(output, "not configured") {
			t.Errorf("expected 'not configured' message, got: %s", output)
		}
	})
}

func TestSyncCommand(t *testing.T) {
	t.Run("syncs thoughts with commit message", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Create a note in the thoughts directory
		userDir := filepath.Join(env.thoughtsRepo, "repos", "myproject", "testuser")
		notePath := filepath.Join(userDir, "test-note.md")
		if err := os.WriteFile(notePath, []byte("# Test Note\n\nSome content."), 0644); err != nil {
			t.Fatalf("failed to create note: %v", err)
		}

		// Run sync with commit message
		output, err := env.runThts("sync", "-m", "Test sync commit")
		if err != nil {
			t.Fatalf("sync failed: %v\nOutput: %s", err, output)
		}

		// Verify searchable directory was created
		searchableDir := filepath.Join(env.projectRepo, "thoughts", "searchable")
		if _, err := os.Stat(searchableDir); err != nil {
			t.Errorf("searchable directory not created: %v", err)
		}

		// Verify commit was made in thoughts repo
		cmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
		cmd.Dir = env.thoughtsRepo
		commitMsg, err := cmd.Output()
		if err != nil {
			t.Errorf("failed to get commit log: %v", err)
		} else if !strings.Contains(string(commitMsg), "Test sync commit") {
			t.Errorf("commit message not found, got: %s", commitMsg)
		}
	})

	t.Run("sync with no changes", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// First sync to commit everything
		_, err = env.runThts("sync", "-m", "Initial sync")
		if err != nil {
			t.Fatalf("first sync failed: %v", err)
		}

		// Second sync with no changes
		output, err := env.runThts("sync", "-m", "No changes")
		if err != nil {
			t.Fatalf("second sync failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "No changes to commit") {
			t.Errorf("expected 'No changes to commit' message, got: %s", output)
		}
	})
}

func TestUninitCommand(t *testing.T) {
	t.Run("removes thoughts setup", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize first
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Verify thoughts exists
		thoughtsDir := filepath.Join(env.projectRepo, "thoughts")
		if _, err := os.Stat(thoughtsDir); err != nil {
			t.Fatalf("thoughts not created: %v", err)
		}

		// Run uninit with --force
		output, err := env.runThts("uninit", "--force")
		if err != nil {
			t.Fatalf("uninit failed: %v\nOutput: %s", err, output)
		}

		// Verify success message
		if !strings.Contains(output, "Thoughts removed from repository") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify thoughts directory was removed
		if _, err := os.Stat(thoughtsDir); !os.IsNotExist(err) {
			t.Errorf("thoughts directory should be removed")
		}

		// Verify config mapping was removed
		configPath := filepath.Join(env.configHome, "thts", "config.yaml")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}

		var cfg config.Config
		if err := yaml.Unmarshal(configData, &cfg); err != nil {
			t.Fatalf("failed to parse config: %v", err)
		}

		if cfg.RepoMappings[env.projectRepo] != nil {
			t.Errorf("repo mapping should be removed from config")
		}
	})

	t.Run("preserves thoughts content in central repo", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Create a note
		userDir := filepath.Join(env.thoughtsRepo, "repos", "myproject", "testuser")
		notePath := filepath.Join(userDir, "important-note.md")
		if err := os.WriteFile(notePath, []byte("# Important Note"), 0644); err != nil {
			t.Fatalf("failed to create note: %v", err)
		}

		// Uninit
		_, err = env.runThts("uninit", "--force")
		if err != nil {
			t.Fatalf("uninit failed: %v", err)
		}

		// Verify note still exists in central repo
		if _, err := os.Stat(notePath); err != nil {
			t.Errorf("note should still exist in central repo: %v", err)
		}
	})
}

func TestFullWorkflow(t *testing.T) {
	t.Run("init -> create note -> sync -> uninit", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// 1. Initialize
		output, err := env.runThts("init", "--name", "workflow-test", "--force")
		if err != nil {
			t.Fatalf("init failed: %v\nOutput: %s", err, output)
		}

		// 2. Create a note via the symlink
		notePath := filepath.Join(env.projectRepo, "thoughts", "testuser", "workflow-note.md")
		noteContent := "# Workflow Test\n\nThis is a test note created during the workflow."
		if err := os.WriteFile(notePath, []byte(noteContent), 0644); err != nil {
			t.Fatalf("failed to create note: %v", err)
		}

		// 3. Sync
		output, err = env.runThts("sync", "-m", "Workflow test sync")
		if err != nil {
			t.Fatalf("sync failed: %v\nOutput: %s", err, output)
		}

		// Verify searchable has the note
		searchablePath := filepath.Join(env.projectRepo, "thoughts", "searchable", "testuser", "workflow-note.md")
		if _, err := os.Stat(searchablePath); err != nil {
			t.Errorf("note not in searchable directory: %v", err)
		}

		// 4. Check status
		output, err = env.runThts("status")
		if err != nil {
			t.Fatalf("status failed: %v\nOutput: %s", err, output)
		}
		if !strings.Contains(output, "Initialized") {
			t.Errorf("status should show initialized")
		}

		// 5. Uninit
		output, err = env.runThts("uninit", "--force")
		if err != nil {
			t.Fatalf("uninit failed: %v\nOutput: %s", err, output)
		}

		// 6. Verify note persists in central repo
		centralNotePath := filepath.Join(env.thoughtsRepo, "repos", "workflow-test", "testuser", "workflow-note.md")
		content, err := os.ReadFile(centralNotePath)
		if err != nil {
			t.Errorf("note should persist in central repo: %v", err)
		} else if string(content) != noteContent {
			t.Errorf("note content mismatch: got %q, want %q", content, noteContent)
		}
	})
}
