package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	fsutil "github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	uninitForce  bool
	uninitDryRun bool
)

var uninitCmd = &cobra.Command{
	Use:   "uninit",
	Short: "Remove Claude Code thts integration from this project",
	Long: `Remove all thts integration files from .claude/ directory.

This removes:
  - thts-instructions.md
  - skills/, commands/, agents/ files installed by thts
  - settings.json if created by thts
  - @include directive from CLAUDE.md
  - gitignore patterns added by thts

The .claude/ directory itself is preserved if it contains other files.`,
	RunE: runClaudeUninit,
}

func init() {
	uninitCmd.Flags().BoolVarP(&uninitForce, "force", "f", false, "Skip confirmation prompt")
	uninitCmd.Flags().BoolVar(&uninitDryRun, "dry-run", false, "Show what would be removed without removing")
}

func runClaudeUninit(cmd *cobra.Command, args []string) error {
	fmt.Println(ui.Header("Remove Claude Code thts Integration"))
	fmt.Println()

	targetDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	claudeDir := filepath.Join(targetDir, ".claude")

	// Check if .claude exists
	if !fsutil.Exists(claudeDir) {
		fmt.Println(ui.Error("No .claude/ directory found."))
		return nil
	}

	// Try to load manifest (preferred path)
	manifest, err := loadManifest(claudeDir)
	if err != nil {
		// Manifest doesn't exist or is corrupted - fall back to detection
		manifest = detectInstallation(claudeDir, targetDir)
		if manifest == nil {
			fmt.Println(ui.Error("No thts installation detected."))
			return nil
		}
		fmt.Println(ui.Warning("No manifest found, using detection."))
		fmt.Println()
	}

	// Show what will be removed
	printRemovalPlan(manifest, claudeDir)

	if uninitDryRun {
		fmt.Println()
		fmt.Println(ui.Info("Dry run complete. No files were removed."))
		return nil
	}

	// Confirm unless --force
	if !uninitForce && !confirmRemoval() {
		fmt.Println("Cancelled.")
		return nil
	}

	fmt.Println()

	// Perform removal
	return performRemoval(manifest, claudeDir, targetDir)
}

// loadManifest reads and parses the manifest file.
func loadManifest(claudeDir string) (*Manifest, error) {
	manifestPath := filepath.Join(claudeDir, ManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &manifest, nil
}

// detectInstallation detects thts installation without a manifest (backwards compat).
func detectInstallation(claudeDir, projectDir string) *Manifest {
	manifest := &Manifest{
		Files: []string{},
	}

	// Check for known thts files
	knownFiles := []string{
		"thts-instructions.md",
		"skills/thts-integrate.md",
		"commands/thts-handoff.md",
		"commands/thts-resume.md",
		"agents/thoughts-locator.md",
		"agents/thoughts-analyzer.md",
		"CLAUDE.local.md",
	}

	for _, f := range knownFiles {
		if fsutil.Exists(filepath.Join(claudeDir, f)) {
			manifest.Files = append(manifest.Files, f)
		}
	}

	// Detect CLAUDE.md modification
	gitRoot, err := git.GetRepoTopLevelAt(projectDir)
	if err != nil {
		gitRoot = projectDir
	}
	claudeMDPath := filepath.Join(gitRoot, "CLAUDE.md")
	if fsutil.Exists(claudeMDPath) {
		content, err := os.ReadFile(claudeMDPath)
		if err == nil && strings.Contains(string(content), "@.claude/thts-instructions.md") {
			manifest.Modifications.ClaudeMD = &ClaudeMDModification{
				Path:    claudeMDPath,
				Action:  "appended", // assume appended in detection
				Pattern: "@.claude/thts-instructions.md",
			}
		}
	}

	// Detect gitignore patterns
	gitignorePath := filepath.Join(projectDir, ".gitignore")
	if fsutil.Exists(gitignorePath) {
		content, _ := os.ReadFile(gitignorePath)
		var patterns []string
		for _, p := range []string{".claude/CLAUDE.local.md", ".claude/settings.local.json"} {
			if strings.Contains(string(content), p) {
				patterns = append(patterns, p)
			}
		}
		if len(patterns) > 0 {
			manifest.Modifications.Gitignore = &GitignoreModification{Patterns: patterns}
		}
	}

	// Return nil if nothing detected
	if len(manifest.Files) == 0 && manifest.Modifications.ClaudeMD == nil {
		return nil
	}

	// Attempt to infer integration level
	if fsutil.Exists(filepath.Join(claudeDir, "CLAUDE.local.md")) {
		manifest.IntegrationLevel = IntegrationLocalOnly
	} else if manifest.Modifications.ClaudeMD != nil {
		manifest.IntegrationLevel = IntegrationAlwaysOn
	} else {
		manifest.IntegrationLevel = IntegrationOnDemand
	}

	return manifest
}

// printRemovalPlan shows what will be removed.
func printRemovalPlan(manifest *Manifest, claudeDir string) {
	fmt.Println(ui.SubHeader("Files to remove:"))
	for _, f := range manifest.Files {
		fmt.Printf("  %s\n", filepath.Join(claudeDir, f))
	}

	if manifest.SettingsCreated {
		fmt.Printf("  %s\n", filepath.Join(claudeDir, "settings.json"))
	}

	if manifest.Modifications.ClaudeMD != nil {
		fmt.Println()
		fmt.Println(ui.SubHeader("Modifications to revert:"))
		fmt.Printf("  Remove @include from: %s\n", manifest.Modifications.ClaudeMD.Path)
	}

	if manifest.Modifications.Gitignore != nil && len(manifest.Modifications.Gitignore.Patterns) > 0 {
		fmt.Println()
		fmt.Println(ui.SubHeader("Gitignore patterns to remove:"))
		for _, p := range manifest.Modifications.Gitignore.Patterns {
			fmt.Printf("  %s\n", p)
		}
	}
}

// performRemoval removes all thts integration files and reverts modifications.
func performRemoval(manifest *Manifest, claudeDir, projectDir string) error {
	var warnings []string

	// 1. Remove files
	for _, f := range manifest.Files {
		path := filepath.Join(claudeDir, f)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("failed to remove %s: %v", f, err))
		} else if err == nil {
			fmt.Println(ui.SuccessF("Removed %s", f))
		}
	}

	// 2. Remove settings.json if created by thts
	if manifest.SettingsCreated {
		settingsPath := filepath.Join(claudeDir, "settings.json")
		if err := os.Remove(settingsPath); err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("failed to remove settings.json: %v", err))
		} else if err == nil {
			fmt.Println(ui.Success("Removed settings.json"))
		}
	}

	// 3. Clean up empty subdirectories
	for _, subdir := range []string{"skills", "commands", "agents"} {
		dir := filepath.Join(claudeDir, subdir)
		if isEmpty, _ := isDirEmpty(dir); isEmpty {
			_ = os.Remove(dir)
		}
	}

	// 4. Revert CLAUDE.md modification
	if manifest.Modifications.ClaudeMD != nil {
		if err := removeFromClaudeMD(manifest.Modifications.ClaudeMD); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to clean CLAUDE.md: %v", err))
		} else {
			fmt.Println(ui.Success("Removed @include from CLAUDE.md"))
		}
	}

	// 5. Remove gitignore patterns
	if manifest.Modifications.Gitignore != nil {
		for _, pattern := range manifest.Modifications.Gitignore.Patterns {
			removed, err := fsutil.RemoveFromGitignore(projectDir, pattern, "project")
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to remove gitignore pattern %s: %v", pattern, err))
			} else if removed {
				fmt.Println(ui.SuccessF("Removed %s from .gitignore", pattern))
			}
		}
	}

	// 6. Remove manifest itself
	manifestPath := filepath.Join(claudeDir, ManifestFile)
	_ = os.Remove(manifestPath)

	// Summary
	fmt.Println()
	if len(warnings) > 0 {
		fmt.Println(ui.Warning("Completed with warnings:"))
		for _, w := range warnings {
			fmt.Printf("  %s\n", w)
		}
	} else {
		fmt.Println(ui.Success("Successfully removed thts integration."))
	}

	return nil
}

// removeFromClaudeMD removes the @include directive from CLAUDE.md.
func removeFromClaudeMD(mod *ClaudeMDModification) error {
	content, err := os.ReadFile(mod.Path)
	if err != nil {
		return err
	}

	// Remove the include line while preserving surrounding structure
	// Split into lines to handle removal cleanly
	lines := strings.Split(string(content), "\n")
	var newLines []string
	pattern := "@.claude/thts-instructions.md"

	for _, line := range lines {
		if strings.TrimSpace(line) != pattern {
			newLines = append(newLines, line)
		}
	}

	newContent := strings.Join(newLines, "\n")

	// Don't delete CLAUDE.md even if empty (per requirements)
	// Just write back the cleaned content
	return os.WriteFile(mod.Path, []byte(newContent), 0644)
}

// isDirEmpty checks if a directory is empty.
func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// confirmRemoval prompts for confirmation.
func confirmRemoval() bool {
	var confirm bool
	err := huh.NewConfirm().
		Title("Remove thts integration from this project?").
		Description("Files listed above will be deleted.").
		Affirmative("Yes, remove").
		Negative("Cancel").
		Value(&confirm).
		Run()
	if err != nil {
		return false
	}
	return confirm
}

// Uninit removes thts integration from the given directory.
// This is exported so that `thts uninit` can call it to clean up claude files.
func Uninit(targetDir string, force bool) error {
	claudeDir := filepath.Join(targetDir, ".claude")

	// Check if .claude exists
	if !fsutil.Exists(claudeDir) {
		return nil // Nothing to do
	}

	// Try to load manifest (preferred path)
	manifest, err := loadManifest(claudeDir)
	if err != nil {
		// Manifest doesn't exist or is corrupted - fall back to detection
		manifest = detectInstallation(claudeDir, targetDir)
		if manifest == nil {
			return nil // Nothing to do
		}
	}

	// Perform removal (skip confirmation since called from thts uninit)
	return performRemoval(manifest, claudeDir, targetDir)
}
