package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show thoughts status",
	Long: `Display the status of your thoughts configuration and repository.

Shows information about:
- Your thoughts configuration
- Current repository mapping
- Thoughts repository git status
- Uncommitted changes`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(ui.Error("Thoughts not configured."))
		fmt.Printf("Run %s first to set up.\n", ui.Accent("thts setup"))
		return nil
	}

	fmt.Println(ui.Header("Thoughts Repository Status"))
	fmt.Println()

	// Get default profile for config display
	defaultProfile, defaultProfileName := cfg.GetDefaultProfile()

	// Show configuration
	fmt.Println(ui.SubHeader("Configuration"))
	if defaultProfile != nil {
		tbl := ui.NewTable("Setting", "Value")
		tbl.Row("Default profile", defaultProfileName)
		tbl.Row("Repository", defaultProfile.ThoughtsRepo)
		tbl.Row("Repos directory", defaultProfile.ReposDir)
		tbl.Row("Global directory", defaultProfile.GlobalDir)
		tbl.Row("User", cfg.User)
		tbl.Row("Profiles", fmt.Sprintf("%d", len(cfg.Profiles)))
		tbl.Row("Mapped repos", fmt.Sprintf("%d", len(cfg.RepoMappings)))
		fmt.Println(tbl)
	} else {
		fmt.Println(ui.Error("No profiles configured"))
		fmt.Printf("  Run %s to create a profile\n", ui.Accent("thts setup"))
	}
	fmt.Println()

	// Get current repo path
	currentRepo, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check current repo mapping
	mapping := cfg.RepoMappings[currentRepo]
	profileConfig := cfg.ResolveProfileForRepo(currentRepo)

	if mapping != nil && profileConfig != nil {
		fmt.Println(ui.SubHeader("Current Repository"))
		tbl := ui.NewTable("Setting", "Value")
		tbl.Row("Path", currentRepo)
		tbl.Row("Thoughts directory", fmt.Sprintf("%s/%s", profileConfig.ReposDir, mapping.GetRepoName()))
		tbl.Row("Profile", profileConfig.ProfileName)

		// Check if thoughts directory exists
		thoughtsDir := filepath.Join(currentRepo, "thoughts")
		if fs.Exists(thoughtsDir) {
			tbl.Row("Status", ui.StyleSuccess.Render(ui.SymbolSuccess+" Initialized"))
		} else {
			tbl.Row("Status", ui.StyleError.Render(ui.SymbolError+" Not initialized"))
		}
		fmt.Println(tbl)
	} else {
		fmt.Println(ui.Warning("Current repository not mapped to thoughts"))
	}
	fmt.Println()

	// Show thoughts repository git status (use default profile if available)
	if profileConfig == nil {
		fmt.Println(ui.Muted("No profile available for git status"))
		return nil
	}

	expandedRepo := config.ExpandPath(profileConfig.ThoughtsRepo)

	fmt.Println(ui.SubHeader("Thoughts Repository Git Status"))
	fmt.Printf("  %s\n", ui.Muted(fmt.Sprintf("(profile: %s)", profileConfig.ProfileName)))

	// Git branch status
	branchStatus := getGitBranchStatus(expandedRepo)
	fmt.Printf("  %s\n", branchStatus)

	// Remote status
	remoteStatus := getGitRemoteStatus(expandedRepo)
	fmt.Printf("  Remote: %s\n", remoteStatus)

	// Last commit
	lastCommit := getLastCommit(expandedRepo)
	fmt.Printf("  Last commit: %s\n", lastCommit)
	fmt.Println()

	// Show uncommitted changes
	changes := getUncommittedChanges(expandedRepo)
	if len(changes) > 0 {
		fmt.Println(ui.SubHeader("Uncommitted changes"))
		for _, change := range changes {
			fmt.Println(change)
		}
		fmt.Println()
		fmt.Printf("%s Run %s to commit these changes\n", ui.Muted("Tip:"), ui.Accent("thts sync"))
	} else {
		fmt.Println(ui.Success("No uncommitted changes"))
	}

	return nil
}

// getGitBranchStatus returns the git branch status line.
func getGitBranchStatus(repoPath string) string {
	cmd := exec.Command("git", "status", "-sb")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return ui.Muted("Not a git repository")
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 {
		return lines[0]
	}
	return ui.Muted("Unknown status")
}

// getGitRemoteStatus returns the remote sync status.
func getGitRemoteStatus(repoPath string) string {
	// Check if remote exists
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return ui.Muted("No remote configured")
	}

	// Try to fetch (silently)
	fetchCmd := exec.Command("git", "fetch")
	fetchCmd.Dir = repoPath
	_ = fetchCmd.Run() // Ignore errors

	// Check ahead/behind status
	statusCmd := exec.Command("git", "status", "-sb")
	statusCmd.Dir = repoPath
	output, err := statusCmd.Output()
	if err != nil {
		return ui.Muted("Unknown")
	}

	status := string(output)

	// Parse ahead/behind
	aheadRe := regexp.MustCompile(`ahead (\d+)`)
	behindRe := regexp.MustCompile(`behind (\d+)`)

	aheadMatch := aheadRe.FindStringSubmatch(status)
	behindMatch := behindRe.FindStringSubmatch(status)

	if aheadMatch != nil && behindMatch != nil {
		return ui.StyleWarning.Render(fmt.Sprintf("%s ahead, %s behind remote", aheadMatch[1], behindMatch[1]))
	} else if aheadMatch != nil {
		return ui.StyleWarning.Render(fmt.Sprintf("%s commits ahead of remote", aheadMatch[1]))
	} else if behindMatch != nil {
		return ui.StyleWarning.Render(fmt.Sprintf("%s commits behind remote", behindMatch[1]))
	}

	return ui.StyleSuccess.Render("Up to date with remote")
}

// getLastCommit returns the last commit info.
func getLastCommit(repoPath string) string {
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%h %s (%cr)")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return ui.Muted("No commits yet")
	}
	return strings.TrimSpace(string(output))
}

// getUncommittedChanges returns a list of uncommitted changes.
func getUncommittedChanges(repoPath string) []string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var changes []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if len(line) < 3 {
			continue
		}

		status := line[0:2]
		file := line[3:]

		var statusText string
		switch {
		case status[0] == 'M' || status[1] == 'M':
			statusText = "modified"
		case status[0] == 'A':
			statusText = "added"
		case status[0] == 'D':
			statusText = "deleted"
		case status[0] == '?':
			statusText = "untracked"
		case status[0] == 'R':
			statusText = "renamed"
		default:
			statusText = "changed"
		}

		changes = append(changes, fmt.Sprintf("  %s %s", ui.StyleWarning.Render(padRight(statusText, 10)), file))
	}

	return changes
}

// padRight pads a string to the right with spaces.
func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}
