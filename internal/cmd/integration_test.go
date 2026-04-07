//go:build integration

package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottames/thts/internal/config"
	"go.yaml.in/yaml/v3"
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
	// XDG_STATE_HOME for this test
	stateHome string
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
		stateHome:    filepath.Join(root, "state"),
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

	// Create initial config (RepoMappings now in state file, not config)
	cfg := &config.Config{
		User:                "testuser",
		Gitignore:           config.ComponentModeLocal,
		AutoSyncInWorktrees: true,
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

func ensureInitialCommit(dir string) error {
	check := exec.Command("git", "rev-parse", "--verify", "HEAD")
	check.Dir = dir
	if err := check.Run(); err == nil {
		return nil
	}

	seedFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(seedFile, []byte("# test\n"), 0644); err != nil {
		return err
	}

	add := exec.Command("git", "add", ".")
	add.Dir = dir
	if err := add.Run(); err != nil {
		return err
	}

	commit := exec.Command("git", "commit", "-m", "initial")
	commit.Dir = dir
	return commit.Run()
}

func createWorktree(mainRepo, worktreePath, branch string) error {
	if err := ensureInitialCommit(mainRepo); err != nil {
		return err
	}

	_ = os.RemoveAll(worktreePath)
	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branch)
	cmd.Dir = mainRepo
	return cmd.Run()
}

// runThts runs the thts binary with the given args and environment
func (e *testEnv) runThts(args ...string) (string, error) {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = e.projectRepo
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+e.configHome,
		"XDG_STATE_HOME="+e.stateHome,
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
		"XDG_STATE_HOME="+e.stateHome,
		"HOME="+e.root,
	)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (e *testEnv) defaultStatePath() string {
	configPath := filepath.Join(e.configHome, "thts", "config.yaml")
	hash := sha256.Sum256([]byte(configPath))
	return filepath.Join(e.stateHome, "thts", "state-"+hex.EncodeToString(hash[:])+".yaml")
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

		// Verify state was updated with mapping (RepoMappings are in state, not config)
		statePath := env.defaultStatePath()
		stateData, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("failed to read state: %v", err)
		}

		var state config.State
		if err := yaml.Unmarshal(stateData, &state); err != nil {
			t.Fatalf("failed to parse state: %v", err)
		}

		mapping := state.RepoMappings[env.projectRepo]
		if mapping == nil {
			t.Errorf("repo mapping not added to state")
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

	t.Run("summary keeps arrow column aligned for long usernames", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		configPath := filepath.Join(env.configHome, "thts", "config.yaml")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}

		var cfg config.Config
		if err := yaml.Unmarshal(configData, &cfg); err != nil {
			t.Fatalf("failed to parse config: %v", err)
		}

		cfg.User = "very-long-username-for-alignment"

		updatedConfigData, err := yaml.Marshal(&cfg)
		if err != nil {
			t.Fatalf("failed to marshal config: %v", err)
		}
		if err := os.WriteFile(configPath, updatedConfigData, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		output, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("thts init failed: %v\nOutput: %s", err, output)
		}

		var arrowColumns []int
		for _, line := range strings.Split(output, "\n") {
			if !strings.Contains(line, "→") {
				continue
			}
			arrowColumns = append(arrowColumns, strings.Index(line, "→"))
		}

		if len(arrowColumns) != 3 {
			t.Fatalf("expected 3 summary lines with arrows, got %d\nOutput: %s", len(arrowColumns), output)
		}

		for i := 1; i < len(arrowColumns); i++ {
			if arrowColumns[i] != arrowColumns[0] {
				t.Fatalf("expected aligned arrows, got columns %v\nOutput: %s", arrowColumns, output)
			}
		}
	})

	t.Run("worktree init reuses existing repository mapping", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init in main repo failed: %v", err)
		}

		worktreeDir := filepath.Join(env.root, "feature-worktree")
		if err := createWorktree(env.projectRepo, worktreeDir, "feature-worktree"); err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		output, err := env.runThtsInDir(worktreeDir, "init", "--force")
		if err != nil {
			t.Fatalf("init in worktree failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "Reusing mapping from") {
			t.Errorf("expected mapping reuse message, got: %s", output)
		}

		worktreeThoughts := filepath.Join(worktreeDir, "thoughts")
		if _, err := os.Stat(worktreeThoughts); err != nil {
			t.Fatalf("expected thoughts directory in worktree: %v", err)
		}

		statePath := env.defaultStatePath()
		stateData, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("failed to read state: %v", err)
		}

		var state config.State
		if err := yaml.Unmarshal(stateData, &state); err != nil {
			t.Fatalf("failed to parse state: %v", err)
		}

		if len(state.RepoMappings) != 1 {
			t.Fatalf("expected a single mapping, got %d", len(state.RepoMappings))
		}

		mapping := state.RepoMappings[env.projectRepo]
		if mapping == nil {
			t.Fatalf("expected mapping to remain keyed by main repo path")
		}
		if mapping.Repo != "myproject" {
			t.Errorf("mapping repo = %q, want %q", mapping.Repo, "myproject")
		}
		if mapping.RepoIdentity == "" {
			t.Error("expected repoIdentity to be recorded")
		}
	})

	t.Run("accepts no-agents flag for automation", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		output, err := env.runThts("init", "--name", "myproject", "--force", "--no-agents")
		if err != nil {
			t.Fatalf("thts init with --no-agents failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "Thoughts setup complete") {
			t.Errorf("expected success message, got: %s", output)
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

	t.Run("warns when thoughts dir exists but mapping is in different state namespace", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize repo with default config namespace.
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Create alternate thoughts repo and config (different namespace).
		altThoughtsRepo := filepath.Join(env.root, "alt-thoughts-repo")
		if err := os.MkdirAll(altThoughtsRepo, 0755); err != nil {
			t.Fatalf("failed to create alt thoughts repo: %v", err)
		}
		if err := initGitRepo(altThoughtsRepo); err != nil {
			t.Fatalf("failed to init alt thoughts repo: %v", err)
		}

		altCfg := &config.Config{
			User:                "testuser",
			Gitignore:           config.ComponentModeLocal,
			AutoSyncInWorktrees: true,
			Profiles: map[string]*config.ProfileConfig{
				"alt": {
					ThoughtsRepo: altThoughtsRepo,
					ReposDir:     "repos",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		altConfigPath := filepath.Join(env.root, "alt-config.yaml")
		altConfigData, err := yaml.Marshal(altCfg)
		if err != nil {
			t.Fatalf("failed to marshal alt config: %v", err)
		}
		if err := os.WriteFile(altConfigPath, altConfigData, 0644); err != nil {
			t.Fatalf("failed to write alt config: %v", err)
		}

		cmd := exec.Command(binaryPath, "sync", "--mode", "local")
		cmd.Dir = env.projectRepo
		cmd.Env = append(os.Environ(),
			"XDG_CONFIG_HOME="+env.configHome,
			"XDG_STATE_HOME="+env.stateHome,
			"HOME="+env.root,
			"THTS_CONFIG_PATH="+altConfigPath,
		)

		outputBytes, err := cmd.CombinedOutput()
		output := string(outputBytes)
		if err != nil {
			t.Fatalf("sync failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "Current repository has thoughts/ but no mapping in the active state file") {
			t.Errorf("expected namespace mismatch warning, got: %s", output)
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
		if !strings.Contains(output, "Local thoughts setup removed from repository") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify thoughts directory was removed
		if _, err := os.Stat(thoughtsDir); !os.IsNotExist(err) {
			t.Errorf("thoughts directory should be removed")
		}

		// Verify state mapping was removed (RepoMappings are in state, not config)
		statePath := env.defaultStatePath()
		stateData, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("failed to read state: %v", err)
		}

		var state config.State
		if err := yaml.Unmarshal(stateData, &state); err != nil {
			t.Fatalf("failed to parse state: %v", err)
		}

		if state.RepoMappings[env.projectRepo] == nil {
			t.Errorf("repo mapping should be retained in state")
		}
	})

	t.Run("uninit --all removes thoughts mapping", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		output, err := env.runThts("uninit", "--all", "--force")
		if err != nil {
			t.Fatalf("uninit --all failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "Thoughts mapping removed from repository") {
			t.Errorf("expected --all success message, got: %s", output)
		}

		statePath := env.defaultStatePath()
		stateData, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("failed to read state: %v", err)
		}

		var state config.State
		if err := yaml.Unmarshal(stateData, &state); err != nil {
			t.Fatalf("failed to parse state: %v", err)
		}

		if state.RepoMappings[env.projectRepo] != nil {
			t.Errorf("repo mapping should be removed from state")
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

func TestInitCheck(t *testing.T) {
	t.Run("returns exit code 1 when not initialized", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Run: thts init --check (without init first)
		_, err := env.runThts("init", "--check")
		if err == nil {
			t.Error("expected error when repo not initialized")
		}

		// Check exit code is 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
			}
		} else {
			t.Errorf("expected ExitError, got %T", err)
		}
	})

	t.Run("returns exit code 0 when initialized", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize first
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Run: thts init --check (should succeed)
		_, err = env.runThts("init", "--check")
		if err != nil {
			t.Errorf("expected no error when repo is initialized, got: %v", err)
		}
	})

	t.Run("returns exit code 1 with partial setup", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Create thoughts directory but not the symlinks
		thoughtsDir := filepath.Join(env.projectRepo, "thoughts")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts dir: %v", err)
		}
		// Create only one symlink (incomplete setup)
		userDir := filepath.Join(thoughtsDir, "testuser")
		if err := os.Symlink("/tmp/fake", userDir); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Run: thts init --check (should fail)
		_, err := env.runThts("init", "--check")
		if err == nil {
			t.Error("expected error with incomplete setup")
		}

		// Check exit code is 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
			}
		}
	})

	t.Run("works from subdirectory", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize first
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Create a subdirectory
		subDir := filepath.Join(env.projectRepo, "some", "nested", "dir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdir: %v", err)
		}

		// Run: thts init --check from subdirectory (should succeed)
		_, err = env.runThtsInDir(subDir, "init", "--check")
		if err != nil {
			t.Errorf("expected no error when checking from subdir, got: %v", err)
		}
	})

	t.Run("returns exit code 1 outside git repo", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Create a non-git directory
		nonGitDir := filepath.Join(env.root, "non-git")
		if err := os.MkdirAll(nonGitDir, 0755); err != nil {
			t.Fatalf("failed to create non-git dir: %v", err)
		}

		// Run: thts init --check (should fail)
		_, err := env.runThtsInDir(nonGitDir, "init", "--check")
		if err == nil {
			t.Error("expected error when not in git repo")
		}

		// Check exit code is 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
			}
		}
	})

	t.Run("silent output for hook consumption", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize first
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Run: thts init --check (should produce no output)
		output, err := env.runThts("init", "--check")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if len(strings.TrimSpace(output)) > 0 {
			t.Errorf("expected silent output, got: %q", output)
		}
	})
}

func TestInitRefresh(t *testing.T) {
	t.Run("refresh updates templates without prompting", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// First init
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("first init failed: %v", err)
		}

		// Verify templates were created
		templatesDir := filepath.Join(env.projectRepo, "thoughts", ".templates")
		if _, err := os.Stat(templatesDir); err != nil {
			t.Errorf("templates directory not created: %v", err)
		}

		// Modify a template to verify it gets overwritten
		noteMdPath := filepath.Join(templatesDir, "note.md")
		if err := os.WriteFile(noteMdPath, []byte("modified content"), 0644); err != nil {
			t.Fatalf("failed to modify template: %v", err)
		}

		// Run refresh
		output, err := env.runThts("init", "--refresh")
		if err != nil {
			t.Fatalf("init --refresh failed: %v\nOutput: %s", err, output)
		}

		// Verify refresh completed
		if !strings.Contains(output, "Refresh complete") {
			t.Errorf("expected 'Refresh complete' message, got: %s", output)
		}

		// Verify template was restored (should not be "modified content")
		content, err := os.ReadFile(noteMdPath)
		if err != nil {
			t.Fatalf("failed to read template: %v", err)
		}
		if strings.Contains(string(content), "modified content") {
			t.Errorf("template should have been refreshed, still contains: %s", content)
		}
	})

	t.Run("refresh updates AGENTS.md and keeps CLAUDE.md symlink", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// First init
		_, err := env.runThts("init", "--name", "myproject", "--force")
		if err != nil {
			t.Fatalf("first init failed: %v", err)
		}

		// Verify AGENTS.md was created
		agentsMdPath := filepath.Join(env.projectRepo, "thoughts", "AGENTS.md")
		if _, err := os.Stat(agentsMdPath); err != nil {
			t.Errorf("AGENTS.md not created: %v", err)
		}

		// Modify AGENTS.md to verify it gets updated
		if err := os.WriteFile(agentsMdPath, []byte("# old content"), 0644); err != nil {
			t.Fatalf("failed to modify AGENTS.md: %v", err)
		}

		// Run refresh
		output, err := env.runThts("init", "--refresh")
		if err != nil {
			t.Fatalf("init --refresh failed: %v\nOutput: %s", err, output)
		}

		// Verify AGENTS.md was updated (should contain project name)
		content, err := os.ReadFile(agentsMdPath)
		if err != nil {
			t.Fatalf("failed to read AGENTS.md: %v", err)
		}
		if !strings.Contains(string(content), "myproject") {
			t.Errorf("AGENTS.md should contain project name 'myproject', got: %s", content)
		}

		claudeMdPath := filepath.Join(env.projectRepo, "thoughts", "CLAUDE.md")
		info, err := os.Lstat(claudeMdPath)
		if err != nil {
			t.Fatalf("failed to stat CLAUDE.md: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("CLAUDE.md should be a symlink")
		}
	})

	t.Run("refresh on non-initialized project fails gracefully", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Try to refresh without init
		output, err := env.runThts("init", "--refresh")
		// This might fail or show a warning - either is acceptable
		// The important thing is it doesn't panic
		if err != nil {
			// Error is expected for non-initialized project
			if !strings.Contains(output, "not configured") && !strings.Contains(output, "not found") {
				t.Logf("Output on non-init refresh: %s", output)
			}
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

func TestAddCommand(t *testing.T) {
	t.Run("add with --sync flag", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize first
		_, err := env.runThts("init", "--name", "add-sync-test", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Run add with --sync flag
		output, err := env.runThts("add", "-t", "sync-test-note", "--in", "notes", "--no-edit", "--sync")
		if err != nil {
			t.Fatalf("add --sync failed: %v\nOutput: %s", err, output)
		}

		// Verify output includes sync message
		if !strings.Contains(output, "Syncing thoughts") {
			t.Errorf("expected 'Syncing thoughts' message, got: %s", output)
		}

		// Verify the file was created (check for the success message)
		if !strings.Contains(output, "Created") {
			t.Errorf("expected 'Created' message, got: %s", output)
		}

		// Verify the file exists in the thoughts repo
		// The file should be at: repos/add-sync-test/testuser/notes/YYYY-MM-DD-sync-test-note.md
		userNotesDir := filepath.Join(env.thoughtsRepo, "repos", "add-sync-test", "testuser", "notes")
		entries, err := os.ReadDir(userNotesDir)
		if err != nil {
			t.Fatalf("failed to read notes dir: %v", err)
		}

		found := false
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "sync-test-note") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("note file not found in %s", userNotesDir)
		}

		// Verify the commit was made in the thoughts repo
		cmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
		cmd.Dir = env.thoughtsRepo
		commitMsg, err := cmd.Output()
		if err != nil {
			t.Errorf("failed to get commit log: %v", err)
		} else if !strings.Contains(string(commitMsg), "sync") {
			t.Logf("commit message: %s", commitMsg)
		}
	})

	t.Run("add with --sync and --quiet flag", func(t *testing.T) {
		env := setupTestEnv(t)
		defer env.cleanup()

		// Initialize first
		_, err := env.runThts("init", "--name", "add-quiet-test", "--force")
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Run add with --sync and --quiet flags
		output, err := env.runThts("add", "-t", "quiet-sync-note", "--in", "notes", "--no-edit", "--sync", "-q")
		if err != nil {
			t.Fatalf("add --sync -q failed: %v\nOutput: %s", err, output)
		}

		// In quiet mode, output should only be the file path (no "Syncing thoughts" message)
		if strings.Contains(output, "Syncing thoughts") {
			t.Errorf("quiet mode should not show 'Syncing thoughts' message, got: %s", output)
		}

		// Output should contain a path
		if !strings.Contains(output, "thoughts") || !strings.Contains(output, ".md") {
			t.Errorf("expected file path in output, got: %s", output)
		}
	})
}
