package git

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// HookVersion is incremented when hooks need updating.
// v1: Initial implementation
const HookVersion = "1"

// HookResult contains information about hook installation.
type HookResult struct {
	Updated []string // List of hooks that were installed/updated
}

// HookOptions configures hook behavior.
type HookOptions struct {
	// AutoSyncInWorktrees enables auto-sync in git worktrees.
	// If false, post-commit hook skips sync in worktrees.
	AutoSyncInWorktrees bool
}

// SetupHooks installs git hooks for thoughts protection and auto-sync.
// Hooks are installed in the git common directory (shared across worktrees).
func SetupHooks(repoPath string, opts HookOptions) (*HookResult, error) {
	result := &HookResult{
		Updated: make([]string, 0),
	}

	// Get git common directory (handles worktrees)
	gitCommonDir, err := GetGitCommonDirAt(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find git common directory: %w", err)
	}

	hooksDir := filepath.Join(gitCommonDir, "hooks")

	// Ensure hooks directory exists
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Install pre-commit hook
	if updated, err := installPreCommitHook(hooksDir); err != nil {
		return nil, err
	} else if updated {
		result.Updated = append(result.Updated, "pre-commit")
	}

	// Install post-commit hook
	if updated, err := installPostCommitHook(hooksDir, opts); err != nil {
		return nil, err
	} else if updated {
		result.Updated = append(result.Updated, "post-commit")
	}

	return result, nil
}

// installPreCommitHook installs the pre-commit hook that prevents committing thoughts/.
func installPreCommitHook(hooksDir string) (bool, error) {
	hookPath := filepath.Join(hooksDir, "pre-commit")
	oldHookPath := hookPath + ".old"

	content := fmt.Sprintf(`#!/bin/bash
# thts thoughts protection - prevent committing thoughts directory
# Version: %s

if git diff --cached --name-only | grep -q "^thoughts/"; then
    echo "Error: Cannot commit thoughts/ to code repository"
    echo "The thoughts directory should only exist in your separate thoughts repository."
    git reset HEAD -- thoughts/
    exit 1
fi

# Call any existing pre-commit hook
if [ -f "%s" ]; then
    "%s" "$@"
fi
`, HookVersion, oldHookPath, oldHookPath)

	return installHook(hookPath, oldHookPath, content, "thts thoughts")
}

// installPostCommitHook installs the post-commit hook for auto-sync.
func installPostCommitHook(hooksDir string, opts HookOptions) (bool, error) {
	hookPath := filepath.Join(hooksDir, "post-commit")
	oldHookPath := hookPath + ".old"

	// Build worktree check based on options
	worktreeCheck := ""
	if !opts.AutoSyncInWorktrees {
		worktreeCheck = `
# Check if we're in a worktree (skip auto-sync if so)
if [ -f .git ]; then
    exit 0
fi
`
	}

	content := fmt.Sprintf(`#!/bin/bash
# thts thoughts auto-sync
# Version: %s
%s
# Get the commit message
COMMIT_MSG=$(git log -1 --pretty=%%B)

# Auto-sync thoughts after each commit
thts sync --message "Auto-sync with commit: $COMMIT_MSG" >/dev/null 2>&1 &

# Call any existing post-commit hook
if [ -f "%s" ]; then
    "%s" "$@"
fi
`, HookVersion, worktreeCheck, oldHookPath, oldHookPath)

	return installHook(hookPath, oldHookPath, content, "thts thoughts")
}

// installHook installs a hook, backing up any existing non-thts hook.
// Returns true if the hook was installed/updated.
func installHook(hookPath, oldHookPath, content, marker string) (bool, error) {
	// Check if hook needs updating
	if !hookNeedsUpdate(hookPath, marker) {
		return false, nil
	}

	// Backup existing hook if it's not ours
	if fileExists(hookPath) {
		existingContent, err := os.ReadFile(hookPath)
		if err != nil {
			return false, fmt.Errorf("failed to read existing hook: %w", err)
		}

		if !strings.Contains(string(existingContent), marker) {
			// It's not our hook, back it up
			if err := os.Rename(hookPath, oldHookPath); err != nil {
				return false, fmt.Errorf("failed to backup existing hook: %w", err)
			}
		} else {
			// It's our outdated hook, just remove it
			if err := os.Remove(hookPath); err != nil {
				return false, fmt.Errorf("failed to remove outdated hook: %w", err)
			}
		}
	}

	// Write new hook
	if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
		return false, fmt.Errorf("failed to write hook: %w", err)
	}

	return true, nil
}

// hookNeedsUpdate checks if a hook needs to be installed or updated.
func hookNeedsUpdate(hookPath, marker string) bool {
	if !fileExists(hookPath) {
		return true
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		return true
	}

	contentStr := string(content)

	// If it's not our hook, don't touch it
	if !strings.Contains(contentStr, marker) {
		return true // We need to install ours (will backup the existing one)
	}

	// Check version
	versionPattern := regexp.MustCompile(`# Version: (\d+)`)
	matches := versionPattern.FindStringSubmatch(contentStr)
	if len(matches) < 2 {
		return true // Old hook without version
	}

	// Compare versions (simple string comparison works for single digits)
	return matches[1] < HookVersion
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// RemoveHooks removes thts hooks and restores any backed up hooks.
func RemoveHooks(repoPath string) error {
	gitCommonDir, err := GetGitCommonDirAt(repoPath)
	if err != nil {
		return fmt.Errorf("failed to find git common directory: %w", err)
	}

	hooksDir := filepath.Join(gitCommonDir, "hooks")

	for _, hookName := range []string{"pre-commit", "post-commit"} {
		hookPath := filepath.Join(hooksDir, hookName)
		oldHookPath := hookPath + ".old"

		if fileExists(hookPath) {
			content, err := os.ReadFile(hookPath)
			if err == nil && strings.Contains(string(content), "thts thoughts") {
				// It's our hook, remove it
				if err := os.Remove(hookPath); err != nil {
					return fmt.Errorf("failed to remove %s hook: %w", hookName, err)
				}

				// Restore backup if it exists
				if fileExists(oldHookPath) {
					if err := os.Rename(oldHookPath, hookPath); err != nil {
						return fmt.Errorf("failed to restore %s hook: %w", hookName, err)
					}
				}
			}
		}
	}

	return nil
}
