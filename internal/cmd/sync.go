package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/scottames/tpd/internal/config"
	"github.com/scottames/tpd/internal/fs"
	"github.com/scottames/tpd/internal/tpd"
	"github.com/scottames/tpd/internal/ui"
	"github.com/spf13/cobra"
)

var syncMessage string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync thoughts to the central repository",
	Long: `Synchronize thoughts with the central thoughts repository.

This command:
  1. Updates symlinks for any new users in the shared repository
  2. Rebuilds the searchable/ directory with hard links
  3. Commits and pushes changes to the central thoughts repository

If there are merge conflicts during rebase, the command will provide
instructions for manual resolution.`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().StringVarP(&syncMessage, "message", "m", "", "Commit message (default: auto-generated)")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	// Check if config exists
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(ui.Error("tpd not configured."))
		fmt.Printf("Run %s first to set up.\n", ui.Accent("tpd setup"))
		return nil
	}

	// Get current repo path
	currentRepo, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if thoughts directory exists
	thoughtsDir := filepath.Join(currentRepo, "thoughts")
	if !fs.Exists(thoughtsDir) {
		fmt.Println(ui.Error("Thoughts not initialized for this repository."))
		fmt.Printf("Run %s to set up thoughts.\n", ui.Accent("tpd init"))
		return nil
	}

	// Get mapping for current repo
	mapping := cfg.RepoMappings[currentRepo]
	if mapping == nil {
		fmt.Println(ui.Error("This repository is not registered."))
		fmt.Printf("Run %s to set up thoughts.\n", ui.Accent("tpd init"))
		return nil
	}

	projectName := mapping.GetRepoName()
	if projectName == "" {
		fmt.Println(ui.Error("Invalid repository mapping."))
		return nil
	}

	// Resolve profile
	profileConfig := cfg.ResolveProfileForRepo(currentRepo)
	expandedRepo := config.ExpandPath(profileConfig.ThoughtsRepo)

	// 1. Update symlinks for any new users
	newUsers := updateSymlinksForNewUsers(thoughtsDir, profileConfig, projectName, cfg.User)
	if len(newUsers) > 0 {
		fmt.Println(ui.SuccessF("Added symlinks for new users: %s", strings.Join(newUsers, ", ")))
	}

	// 2. Create searchable directory with hard links
	fmt.Println(ui.Info("Creating searchable index..."))
	result, err := tpd.CreateSearchableDir(thoughtsDir)
	if err != nil {
		fmt.Println(ui.WarningF("Could not create searchable directory: %v", err))
	} else {
		if result.CrossFilesystem {
			fmt.Println(ui.Warning("Some files skipped (cross-filesystem - hard links not supported)"))
		}
		fmt.Println(ui.Bullet(fmt.Sprintf("Created %d hard links in searchable directory", result.LinkedCount)))
	}

	// 3. Sync the thoughts repository
	fmt.Println(ui.Info("Syncing thoughts..."))
	if err := syncThoughtsRepo(expandedRepo, syncMessage); err != nil {
		return err
	}

	return nil
}

// syncThoughtsRepo performs git operations to sync the thoughts repository.
func syncThoughtsRepo(repoPath, message string) error {
	// Stage all changes
	if err := runGitCommand(repoPath, "add", "-A"); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Check if there are changes to commit
	hasChanges, err := hasUncommittedChanges(repoPath)
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}

	if hasChanges {
		// Commit changes
		commitMessage := message
		if commitMessage == "" {
			commitMessage = fmt.Sprintf("Sync thoughts - %s", time.Now().Format(time.RFC3339))
		}

		if err := runGitCommand(repoPath, "commit", "-m", commitMessage); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
		fmt.Println(ui.Success("Thoughts committed"))
	} else {
		fmt.Println(ui.Muted("No changes to commit"))
	}

	// Pull latest changes (after committing to avoid conflicts with staged changes)
	if err := pullWithRebase(repoPath); err != nil {
		return err
	}

	// Push if remote exists
	if err := pushToRemote(repoPath); err != nil {
		// Push errors are warnings, not fatal
		fmt.Println(ui.WarningF("%v", err))
	}

	return nil
}

// hasUncommittedChanges checks if there are uncommitted changes.
func hasUncommittedChanges(repoPath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// runGitCommand runs a git command in the specified directory.
func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = nil // Suppress output
	cmd.Stderr = nil
	return cmd.Run()
}

// pullWithRebase attempts to pull with rebase, handling conflicts.
func pullWithRebase(repoPath string) error {
	// Check if remote exists first
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		// No remote configured
		return nil
	}

	// Try to pull
	cmd = exec.Command("git", "pull", "--rebase")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)

		// Check for merge conflict indicators
		if isConflictError(outputStr) {
			fmt.Println()
			fmt.Println(ui.Error("Sync conflict detected in thoughts repository."))
			fmt.Println()
			fmt.Println("To resolve:")
			fmt.Printf("  %s\n", ui.Accent(fmt.Sprintf("cd %s", repoPath)))
			fmt.Printf("  %s        # See conflicting files\n", ui.Accent("git status"))
			fmt.Println("  # Fix conflicts manually")
			fmt.Printf("  %s\n", ui.Accent("git rebase --continue"))
			fmt.Printf("  %s          # Retry sync\n", ui.Accent("tpd sync"))
			fmt.Println()
			return fmt.Errorf("merge conflict - manual resolution required")
		}

		// Other pull errors are warnings
		fmt.Println(ui.WarningF("Could not pull latest changes: %s", strings.TrimSpace(outputStr)))
	}

	return nil
}

// isConflictError checks if the git output indicates a merge conflict.
func isConflictError(output string) bool {
	conflictIndicators := []string{
		"CONFLICT (",
		"Automatic merge failed",
		"Patch failed at",
		"When you have resolved this problem, run \"git rebase --continue\"",
	}

	for _, indicator := range conflictIndicators {
		if strings.Contains(output, indicator) {
			return true
		}
	}
	return false
}

// pushToRemote attempts to push to the remote.
func pushToRemote(repoPath string) error {
	// Check if remote exists
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		fmt.Println(ui.Muted("No remote configured for thoughts repository"))
		return nil
	}

	// Try to push
	fmt.Println(ui.Muted("Pushing to remote..."))
	cmd = exec.Command("git", "push")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not push to remote - you may need to push manually")
	}

	fmt.Println(ui.Success("Pushed to remote"))
	return nil
}

// updateSymlinksForNewUsers checks for other users' directories and creates symlinks.
func updateSymlinksForNewUsers(thoughtsDir string, profile *config.ResolvedProfile, projectName, currentUser string) []string {
	expandedRepo := config.ExpandPath(profile.ThoughtsRepo)
	repoThoughtsPath := filepath.Join(expandedRepo, profile.ReposDir, projectName)

	var addedUsers []string

	// Check if repo thoughts path exists
	if !fs.Exists(repoThoughtsPath) {
		return addedUsers
	}

	// Get all directories in the repo thoughts path
	entries, err := os.ReadDir(repoThoughtsPath)
	if err != nil {
		return addedUsers
	}

	for _, entry := range entries {
		name := entry.Name()

		// Skip non-directories, shared, and dotfiles
		if !entry.IsDir() || name == "shared" || strings.HasPrefix(name, ".") {
			continue
		}

		// Skip current user (already has symlink from init)
		if name == currentUser {
			continue
		}

		// Check if symlink already exists
		symlinkPath := filepath.Join(thoughtsDir, name)
		if fs.ExistsNoFollow(symlinkPath) {
			continue
		}

		// Create symlink for this user
		targetPath := filepath.Join(repoThoughtsPath, name)
		if err := os.Symlink(targetPath, symlinkPath); err != nil {
			continue // Skip on error
		}

		addedUsers = append(addedUsers, name)
	}

	return addedUsers
}
