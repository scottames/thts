package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/thts"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	syncMessage  string
	syncFromHook bool
)

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

// syncOptions holds context needed for syncing with template rendering.
type syncOptions struct {
	RepoPath        string
	Mode            config.SyncMode
	ExplicitMessage string // User-provided message via -m flag (overrides template)
	FromHook        bool   // Whether called from git hook
	TriggerMessage  string // Triggering commit message (when FromHook is true)
	ProfileName     string
	ProjectName     string
	User            string
	Config          *config.Config
	Output          io.Writer // Output writer for messages (defaults to os.Stdout)
}

// syncPrint prints to the sync output writer, defaulting to stdout.
func (o *syncOptions) print(a ...interface{}) {
	w := o.Output
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, a...)
}

// syncPrintf prints formatted output to the sync output writer.
func (o *syncOptions) printf(format string, a ...interface{}) {
	w := o.Output
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, format, a...)
}

// getOutput returns the output writer, defaulting to stdout.
func (o *syncOptions) getOutput() io.Writer {
	if o.Output == nil {
		return os.Stdout
	}
	return o.Output
}

func init() {
	syncCmd.Flags().StringVarP(&syncMessage, "message", "m", "", "Commit message (default: auto-generated)")
	syncCmd.Flags().String("mode", "", "Sync mode: full (pull+push), pull (pull only), local (no remote ops)")
	syncCmd.Flags().BoolVar(&syncFromHook, "from-hook", false, "Called from git hook (uses commitMessageHook template)")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	// Check if config exists
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(ui.Error("thts not configured."))
		fmt.Printf("Run %s first to set up.\n", ui.Accent("thts setup"))
		return nil
	}

	// Get current directory
	currentRepo, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Determine if this repo is initialized
	thoughtsDir := filepath.Join(currentRepo, "thoughts")
	isInitialized := fs.Exists(thoughtsDir) && cfg.RepoMappings[currentRepo] != nil

	var profileConfig *config.ResolvedProfile
	var projectName string

	if isInitialized {
		// Use repo-specific profile
		mapping := cfg.RepoMappings[currentRepo]
		projectName = mapping.GetRepoName()
		profileConfig = cfg.ResolveProfileForRepo(currentRepo)
	} else {
		// Use default profile directly
		profileConfig = cfg.GetDefaultProfileResolved()
		if profileConfig == nil {
			fmt.Println(ui.Error("No default profile configured."))
			fmt.Printf("Run %s to configure a default profile.\n", ui.Accent("thts setup"))
			return nil
		}
	}

	expandedRepo := config.ExpandPath(profileConfig.ThoughtsRepo)

	// Project-specific operations (only for initialized repos)
	if isInitialized {
		// 1. Update symlinks for any new users
		newUsers := updateSymlinksForNewUsers(thoughtsDir, profileConfig, projectName, cfg.User)
		if len(newUsers) > 0 {
			fmt.Println(ui.SuccessF("Added symlinks for new users: %s", strings.Join(newUsers, ", ")))
		}

		// 2. Create searchable directory with hard links
		fmt.Println(ui.Info("Creating searchable index..."))
		result, err := thts.CreateSearchableDir(thoughtsDir)
		if err != nil {
			fmt.Println(ui.WarningF("Could not create searchable directory: %v", err))
		} else {
			if result.CrossFilesystem {
				fmt.Println(ui.Warning("Some files skipped (cross-filesystem - hard links not supported)"))
			}
			fmt.Println(ui.MutedBullet(fmt.Sprintf("Created %d hard links in searchable directory", result.LinkedCount)))
		}
	}

	// Determine sync mode: CLI flag overrides config
	syncMode := cfg.GetSyncMode()
	if cmd.Flags().Changed("mode") {
		modeFlag, _ := cmd.Flags().GetString("mode")
		syncMode = config.SyncMode(modeFlag)
	}

	// 3. Sync the thoughts repository
	if isInitialized {
		fmt.Println(ui.InfoF("Syncing thoughts for %s...", ui.Accent(projectName)))
	} else {
		fmt.Println(ui.Info("Syncing thoughts repo..."))
	}

	// Build sync options with context for template rendering
	opts := syncOptions{
		RepoPath:    expandedRepo,
		Mode:        syncMode,
		FromHook:    syncFromHook,
		ProfileName: profileConfig.ProfileName,
		ProjectName: projectName,
		User:        cfg.User,
		Config:      cfg,
	}

	// Handle message: when from hook, the message is the triggering commit message
	if syncFromHook {
		opts.TriggerMessage = syncMessage
	} else if cmd.Flags().Changed("message") {
		opts.ExplicitMessage = syncMessage
	}

	if err := syncThoughtsRepo(opts); err != nil {
		return err
	}

	return nil
}

// syncThoughtsRepo performs git operations to sync the thoughts repository.
func syncThoughtsRepo(opts syncOptions) error {
	// Stage all changes
	if err := runGitCommand(opts.RepoPath, "add", "-A"); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Check if there are changes to commit
	hasChanges, err := hasUncommittedChanges(opts.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}

	if hasChanges {
		// Determine commit message
		commitMessage, err := resolveCommitMessage(opts)
		if err != nil {
			return fmt.Errorf("failed to render commit message: %w", err)
		}

		if err := runGitCommand(opts.RepoPath, "commit", "-m", commitMessage); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
		opts.print(ui.Success("Thoughts committed"))
	} else {
		opts.print(ui.MutedBullet("No changes to commit"))
	}

	// Skip remote operations in local mode
	if opts.Mode == config.SyncModeLocal {
		warnUnpushedCommits(opts.RepoPath, "local mode", opts.getOutput())
		return nil
	}

	// Pull latest changes (after committing to avoid conflicts with staged changes)
	if err := pullWithRebase(opts.RepoPath, opts.getOutput()); err != nil {
		return err
	}

	// Push only in full mode
	if opts.Mode == config.SyncModeFull {
		if err := pushToRemote(opts.RepoPath, opts.getOutput()); err != nil {
			// Push errors are warnings, not fatal
			opts.print(ui.WarningF("%v", err))
		}
	} else {
		warnUnpushedCommits(opts.RepoPath, "pull mode", opts.getOutput())
	}

	return nil
}

// resolveCommitMessage determines the commit message based on options.
// Priority: explicit message > template rendering (hook or manual).
func resolveCommitMessage(opts syncOptions) (string, error) {
	// Explicit message always wins (unless from hook)
	if opts.ExplicitMessage != "" && !opts.FromHook {
		return opts.ExplicitMessage, nil
	}

	// Build template data
	data := config.CommitMessageData{
		Date:    time.Now(),
		Repo:    opts.ProjectName,
		Profile: opts.ProfileName,
		User:    opts.User,
	}

	// Choose template based on context
	var tmpl string
	if opts.FromHook {
		data.CommitMessage = opts.TriggerMessage
		tmpl = opts.Config.GetCommitMessageHook(opts.ProfileName)
	} else {
		tmpl = opts.Config.GetCommitMessage(opts.ProfileName)
	}

	return config.RenderCommitMessage(tmpl, data)
}

// warnUnpushedCommits prints a warning if there are unpushed commits.
func warnUnpushedCommits(repoPath, reason string, w io.Writer) {
	if count := getUnpushedCommitCount(repoPath); count > 0 {
		noun := "commits"
		if count == 1 {
			noun = "commit"
		}
		fmt.Fprintln(w, ui.WarningF("%d %s not pushed (%s)", count, noun, reason))
		fmt.Fprintf(w, "  Run %s or %s in %s to push\n",
			ui.Accent("thts sync --mode=full"),
			ui.Accent("git push"),
			ui.Accent(repoPath))
	}
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
func pullWithRebase(repoPath string, w io.Writer) error {
	// Check if remote exists first
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		// No remote configured
		return nil
	}

	// Print before git pull which may require auth (e.g., yubikey touch)
	fmt.Fprintln(w, ui.MutedBullet("Pulling from remote..."))

	cmd = exec.Command("git", "pull", "--rebase")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)

		// Check for merge conflict indicators
		if isConflictError(outputStr) {
			fmt.Fprintln(w)
			fmt.Fprintln(w, ui.Error("Sync conflict detected in thoughts repository."))
			fmt.Fprintln(w)
			fmt.Fprintln(w, "To resolve:")
			fmt.Fprintf(w, "  %s\n", ui.Accent(fmt.Sprintf("cd %s", repoPath)))
			fmt.Fprintf(w, "  %s        # See conflicting files\n", ui.Accent("git status"))
			fmt.Fprintln(w, "  # Fix conflicts manually")
			fmt.Fprintf(w, "  %s\n", ui.Accent("git rebase --continue"))
			fmt.Fprintf(w, "  %s          # Retry sync\n", ui.Accent("thts sync"))
			fmt.Fprintln(w)
			return fmt.Errorf("merge conflict - manual resolution required")
		}

		// Other pull errors are warnings
		fmt.Fprintln(w, ui.WarningF("Could not pull latest changes: %s", strings.TrimSpace(outputStr)))
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
func pushToRemote(repoPath string, w io.Writer) error {
	// Check if remote exists
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(w, ui.Muted("No remote configured for thoughts repository"))
		return nil
	}

	// Skip push if nothing to push
	if getUnpushedCommitCount(repoPath) == 0 {
		fmt.Fprintln(w, ui.MutedBullet("Nothing to push"))
		return nil
	}

	// Print before git push which may require auth (e.g., yubikey touch)
	fmt.Fprintln(w, ui.MutedBullet("Pushing to remote..."))

	cmd = exec.Command("git", "push")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not push to remote - you may need to push manually")
	}

	fmt.Fprintln(w, ui.Success("Pushed to remote"))
	return nil
}

// getUnpushedCommitCount returns the number of commits ahead of the upstream.
func getUnpushedCommitCount(repoPath string) int {
	cmd := exec.Command("git", "-C", repoPath, "rev-list", "--count", "@{u}..HEAD")
	output, err := cmd.Output()
	if err != nil {
		return 0 // no upstream or error, skip warning
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(output)))
	return count
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
