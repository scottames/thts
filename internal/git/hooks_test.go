package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupHooks(t *testing.T) {
	t.Run("installs both hooks in new repo", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		opts := HookOptions{AutoSyncInWorktrees: true}
		result, err := SetupHooks(repoDir, opts)
		if err != nil {
			t.Fatalf("SetupHooks() error: %v", err)
		}

		// Both hooks should be updated (installed)
		if len(result.Updated) != 2 {
			t.Errorf("expected 2 hooks to be updated, got %d", len(result.Updated))
		}

		// Verify pre-commit hook exists and is executable
		preCommit := filepath.Join(repoDir, ".git", "hooks", "pre-commit")
		info, err := os.Stat(preCommit)
		if err != nil {
			t.Fatalf("pre-commit hook not created: %v", err)
		}
		if info.Mode()&0111 == 0 {
			t.Error("pre-commit hook should be executable")
		}

		// Verify content
		content, _ := os.ReadFile(preCommit)
		if !strings.Contains(string(content), "tpd thoughts protection") {
			t.Error("pre-commit hook should contain tpd thoughts protection marker")
		}
		if !strings.Contains(string(content), "Version: "+HookVersion) {
			t.Error("pre-commit hook should contain version marker")
		}

		// Verify post-commit hook exists and is executable
		postCommit := filepath.Join(repoDir, ".git", "hooks", "post-commit")
		info, err = os.Stat(postCommit)
		if err != nil {
			t.Fatalf("post-commit hook not created: %v", err)
		}
		if info.Mode()&0111 == 0 {
			t.Error("post-commit hook should be executable")
		}

		// Verify content
		content, _ = os.ReadFile(postCommit)
		if !strings.Contains(string(content), "tpd thoughts auto-sync") {
			t.Error("post-commit hook should contain tpd thoughts auto-sync marker")
		}
	})

	t.Run("backs up existing non-tpd hooks", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Create an existing pre-commit hook
		hooksDir := filepath.Join(repoDir, ".git", "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks dir: %v", err)
		}

		existingHook := filepath.Join(hooksDir, "pre-commit")
		existingContent := "#!/bin/bash\necho 'Custom hook'\n"
		if err := os.WriteFile(existingHook, []byte(existingContent), 0755); err != nil {
			t.Fatalf("failed to create existing hook: %v", err)
		}

		opts := HookOptions{AutoSyncInWorktrees: true}
		_, err := SetupHooks(repoDir, opts)
		if err != nil {
			t.Fatalf("SetupHooks() error: %v", err)
		}

		// Original hook should be backed up
		backupHook := filepath.Join(hooksDir, "pre-commit.old")
		backupContent, err := os.ReadFile(backupHook)
		if err != nil {
			t.Fatalf("backup hook not created: %v", err)
		}

		if string(backupContent) != existingContent {
			t.Errorf("backup content = %q, want %q", string(backupContent), existingContent)
		}

		// New hook should reference the backup
		newContent, _ := os.ReadFile(existingHook)
		if !strings.Contains(string(newContent), "pre-commit.old") {
			t.Error("new hook should reference backup hook")
		}
	})

	t.Run("updates outdated tpd hooks", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Create an old version tpd hook
		hooksDir := filepath.Join(repoDir, ".git", "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks dir: %v", err)
		}

		oldHook := filepath.Join(hooksDir, "pre-commit")
		oldContent := "#!/bin/bash\n# tpd thoughts protection\n# Version: 0\necho 'old'\n"
		if err := os.WriteFile(oldHook, []byte(oldContent), 0755); err != nil {
			t.Fatalf("failed to create old hook: %v", err)
		}

		opts := HookOptions{AutoSyncInWorktrees: true}
		result, err := SetupHooks(repoDir, opts)
		if err != nil {
			t.Fatalf("SetupHooks() error: %v", err)
		}

		// Should be updated
		updated := false
		for _, hook := range result.Updated {
			if hook == "pre-commit" {
				updated = true
				break
			}
		}
		if !updated {
			t.Error("expected pre-commit to be in updated list")
		}

		// Verify new version
		newContent, _ := os.ReadFile(oldHook)
		if !strings.Contains(string(newContent), "Version: "+HookVersion) {
			t.Error("hook should have been updated to new version")
		}

		// No backup should be created for our own hooks
		backupHook := filepath.Join(hooksDir, "pre-commit.old")
		if _, err := os.Stat(backupHook); !os.IsNotExist(err) {
			t.Error("should not backup our own outdated hooks")
		}
	})

	t.Run("skips current version hooks", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// First install
		opts := HookOptions{AutoSyncInWorktrees: true}
		_, err := SetupHooks(repoDir, opts)
		if err != nil {
			t.Fatalf("first SetupHooks() error: %v", err)
		}

		// Second install should skip
		result, err := SetupHooks(repoDir, opts)
		if err != nil {
			t.Fatalf("second SetupHooks() error: %v", err)
		}

		if len(result.Updated) != 0 {
			t.Errorf("expected 0 hooks to be updated on second run, got %d", len(result.Updated))
		}
	})

	t.Run("respects AutoSyncInWorktrees option", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Install with AutoSyncInWorktrees = false
		opts := HookOptions{AutoSyncInWorktrees: false}
		_, err := SetupHooks(repoDir, opts)
		if err != nil {
			t.Fatalf("SetupHooks() error: %v", err)
		}

		postCommit := filepath.Join(repoDir, ".git", "hooks", "post-commit")
		content, _ := os.ReadFile(postCommit)

		// Should contain worktree check
		if !strings.Contains(string(content), "if [ -f .git ]") {
			t.Error("post-commit hook should contain worktree check when AutoSyncInWorktrees is false")
		}
	})

	t.Run("installs to common dir for worktrees", func(t *testing.T) {
		mainRepo, cleanup := setupTestGitRepo(t)
		defer cleanup()

		worktree, worktreeCleanup := setupTestWorktree(t, mainRepo)
		defer worktreeCleanup()

		// Install hooks from worktree
		opts := HookOptions{AutoSyncInWorktrees: true}
		_, err := SetupHooks(worktree, opts)
		if err != nil {
			t.Fatalf("SetupHooks() error: %v", err)
		}

		// Hooks should be in main repo's hooks dir
		mainHooksDir := filepath.Join(mainRepo, ".git", "hooks")
		if _, err := os.Stat(filepath.Join(mainHooksDir, "pre-commit")); err != nil {
			t.Error("pre-commit should be installed in main repo hooks dir")
		}
		if _, err := os.Stat(filepath.Join(mainHooksDir, "post-commit")); err != nil {
			t.Error("post-commit should be installed in main repo hooks dir")
		}
	})
}

func TestRemoveHooks(t *testing.T) {
	t.Run("removes tpd hooks", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Install hooks first
		opts := HookOptions{AutoSyncInWorktrees: true}
		_, err := SetupHooks(repoDir, opts)
		if err != nil {
			t.Fatalf("SetupHooks() error: %v", err)
		}

		// Remove hooks
		if err := RemoveHooks(repoDir); err != nil {
			t.Fatalf("RemoveHooks() error: %v", err)
		}

		// Verify hooks are gone
		hooksDir := filepath.Join(repoDir, ".git", "hooks")
		if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); !os.IsNotExist(err) {
			t.Error("pre-commit hook should be removed")
		}
		if _, err := os.Stat(filepath.Join(hooksDir, "post-commit")); !os.IsNotExist(err) {
			t.Error("post-commit hook should be removed")
		}
	})

	t.Run("restores backed up hooks", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Create existing non-tpd hook
		hooksDir := filepath.Join(repoDir, ".git", "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks dir: %v", err)
		}

		existingHook := filepath.Join(hooksDir, "pre-commit")
		existingContent := "#!/bin/bash\necho 'Custom hook'\n"
		if err := os.WriteFile(existingHook, []byte(existingContent), 0755); err != nil {
			t.Fatalf("failed to create existing hook: %v", err)
		}

		// Install tpd hooks (will backup existing)
		opts := HookOptions{AutoSyncInWorktrees: true}
		_, err := SetupHooks(repoDir, opts)
		if err != nil {
			t.Fatalf("SetupHooks() error: %v", err)
		}

		// Remove tpd hooks
		if err := RemoveHooks(repoDir); err != nil {
			t.Fatalf("RemoveHooks() error: %v", err)
		}

		// Original hook should be restored
		restoredContent, err := os.ReadFile(existingHook)
		if err != nil {
			t.Fatalf("restored hook not found: %v", err)
		}

		if string(restoredContent) != existingContent {
			t.Errorf("restored content = %q, want %q", string(restoredContent), existingContent)
		}

		// Backup should be gone
		backupHook := filepath.Join(hooksDir, "pre-commit.old")
		if _, err := os.Stat(backupHook); !os.IsNotExist(err) {
			t.Error("backup hook should be removed after restore")
		}
	})

	t.Run("no-op for non-tpd hooks", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Create a non-tpd hook
		hooksDir := filepath.Join(repoDir, ".git", "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks dir: %v", err)
		}

		customHook := filepath.Join(hooksDir, "pre-commit")
		customContent := "#!/bin/bash\necho 'Custom hook'\n"
		if err := os.WriteFile(customHook, []byte(customContent), 0755); err != nil {
			t.Fatalf("failed to create custom hook: %v", err)
		}

		// Remove should not affect custom hooks
		if err := RemoveHooks(repoDir); err != nil {
			t.Fatalf("RemoveHooks() error: %v", err)
		}

		// Custom hook should still exist
		content, err := os.ReadFile(customHook)
		if err != nil {
			t.Fatalf("custom hook was removed: %v", err)
		}

		if string(content) != customContent {
			t.Error("custom hook content should be unchanged")
		}
	})

	t.Run("handles missing hooks directory", func(t *testing.T) {
		repoDir, cleanup := setupTestGitRepo(t)
		defer cleanup()

		// Remove hooks directory
		hooksDir := filepath.Join(repoDir, ".git", "hooks")
		_ = os.RemoveAll(hooksDir)

		// Should not error
		if err := RemoveHooks(repoDir); err != nil {
			t.Errorf("RemoveHooks() should not error with missing hooks dir: %v", err)
		}
	})
}

func TestHookNeedsUpdate(t *testing.T) {
	dir, err := os.MkdirTemp("", "tpd-hooks-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	t.Run("missing hook needs update", func(t *testing.T) {
		hookPath := filepath.Join(dir, "missing")
		if !hookNeedsUpdate(hookPath, "tpd thoughts") {
			t.Error("missing hook should need update")
		}
	})

	t.Run("non-tpd hook needs update (to install over)", func(t *testing.T) {
		hookPath := filepath.Join(dir, "non-tpd")
		content := "#!/bin/bash\necho 'custom'\n"
		if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		if !hookNeedsUpdate(hookPath, "tpd thoughts") {
			t.Error("non-tpd hook should need update")
		}
	})

	t.Run("outdated tpd hook needs update", func(t *testing.T) {
		hookPath := filepath.Join(dir, "outdated")
		content := "#!/bin/bash\n# tpd thoughts protection\n# Version: 0\necho 'old'\n"
		if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		if !hookNeedsUpdate(hookPath, "tpd thoughts") {
			t.Error("outdated tpd hook should need update")
		}
	})

	t.Run("current tpd hook does not need update", func(t *testing.T) {
		hookPath := filepath.Join(dir, "current")
		content := "#!/bin/bash\n# tpd thoughts protection\n# Version: " + HookVersion + "\necho 'current'\n"
		if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		if hookNeedsUpdate(hookPath, "tpd thoughts") {
			t.Error("current tpd hook should not need update")
		}
	})

	t.Run("tpd hook without version needs update", func(t *testing.T) {
		hookPath := filepath.Join(dir, "no-version")
		content := "#!/bin/bash\n# tpd thoughts protection\necho 'no version'\n"
		if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		if !hookNeedsUpdate(hookPath, "tpd thoughts") {
			t.Error("tpd hook without version should need update")
		}
	})
}
