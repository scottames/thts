package claude

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	thtsfiles "github.com/scottames/thts"
	fsutil "github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	initForce        bool
	initInteractive  bool
	initWithSettings bool
)

// ModelType represents the available Claude models.
type ModelType string

const (
	ModelHaiku  ModelType = "haiku"
	ModelSonnet ModelType = "sonnet"
	ModelOpus   ModelType = "opus"
)

var validModels = []ModelType{ModelHaiku, ModelSonnet, ModelOpus}

func isValidModel(m string) bool {
	for _, vm := range validModels {
		if string(vm) == m {
			return true
		}
	}
	return false
}

// IntegrationLevel represents how thoughts/ integration is activated.
type IntegrationLevel string

const (
	// IntegrationAlwaysOn adds @include to project CLAUDE.md (checked into git).
	IntegrationAlwaysOn IntegrationLevel = "always-on"
	// IntegrationLocalOnly creates .claude/CLAUDE.local.md (gitignored).
	IntegrationLocalOnly IntegrationLevel = "local-only"
	// IntegrationOnDemand only installs commands/skills for manual invocation.
	IntegrationOnDemand IntegrationLevel = "on-demand"
)

// ManifestFile is the name of the manifest file that tracks init operations.
const ManifestFile = ".thts-manifest.json"

// Manifest tracks files created by claude init for clean uninit.
type Manifest struct {
	Version          int                   `json:"version"`
	CreatedAt        string                `json:"createdAt"`
	IntegrationLevel IntegrationLevel      `json:"integrationLevel"`
	Files            []string              `json:"files"`
	SettingsCreated  bool                  `json:"settingsCreated,omitempty"`
	Modifications    ManifestModifications `json:"modifications,omitempty"`
}

// ManifestModifications tracks changes to existing files.
type ManifestModifications struct {
	ClaudeMD  *ClaudeMDModification  `json:"claudeMD,omitempty"`
	Gitignore *GitignoreModification `json:"gitignore,omitempty"`
}

// ClaudeMDModification tracks changes made to CLAUDE.md.
type ClaudeMDModification struct {
	Path    string `json:"path"`
	Action  string `json:"action"` // "appended" or "created"
	Pattern string `json:"pattern"`
}

// GitignoreModification tracks patterns added to .gitignore.
type GitignoreModification struct {
	Patterns []string `json:"patterns"`
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Claude Code configuration for this project",
	Long: `Initialize Claude Code configuration by copying thts integration files
to the project's .claude/ directory.

This enables thoughts/ integration with Claude Code including:
  - /thts-integrate skill (activate integration for current task)
  - /thts-handoff command (create session handoff documents)
  - /thts-resume command (resume from handoff documents)
  - Specialized agents (thoughts-locator, thoughts-analyzer)

Integration levels:
  - Always-on (CLAUDE.md): Adds @include to project CLAUDE.md
  - Always-on (local): Creates .claude/CLAUDE.local.md (gitignored)
  - On-demand: Just installs skill/commands for manual invocation`,
	RunE: runClaudeInit,
}

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing files")
	initCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", false, "Interactively select which files to copy")
	initCmd.Flags().BoolVar(&initWithSettings, "with-settings", false, "Also create settings.json with model/thinking configuration")
}

func runClaudeInit(cmd *cobra.Command, args []string) error {
	fmt.Println(ui.Header("Initialize Claude Code Configuration"))
	fmt.Println()

	// Check for interactive terminal if interactive mode requested
	if initInteractive && !isTerminal() {
		fmt.Println(ui.Error("--interactive requires a terminal."))
		return nil
	}

	targetDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	claudeTargetDir := filepath.Join(targetDir, ".claude")

	// Step 1: Select integration level
	integrationLevel, err := selectIntegrationLevel()
	if err != nil {
		return err
	}

	// Step 2: Determine what to copy (in interactive mode)
	selectedCategories, err := selectCategories()
	if err != nil {
		return err
	}
	if len(selectedCategories) == 0 {
		fmt.Println("No items selected.")
		return nil
	}

	// Create .claude directory
	if err := os.MkdirAll(claudeTargetDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	// Initialize manifest to track what we create
	manifest := &Manifest{
		IntegrationLevel: integrationLevel,
		Files:            []string{},
	}

	var filesCopied, filesSkipped int
	filesToCopyByCategory := make(map[string][]string)

	// Interactive file selection for each category
	if initInteractive {
		for _, category := range []string{"commands", "agents", "skills"} {
			if !contains(selectedCategories, category) {
				continue
			}
			embedFS, dir := getEmbedFSForCategory(category)
			if embedFS == nil {
				continue
			}
			files, err := listEmbeddedFiles(embedFS, dir)
			if err == nil && len(files) > 0 {
				selected, err := selectFiles(category, files)
				if err != nil {
					return err
				}
				filesToCopyByCategory[category] = selected
				if len(selected) == 0 {
					filesSkipped += len(files)
				}
			}
		}
	}

	// Always copy instructions file first
	if err := copyInstructionsFile(claudeTargetDir); err != nil {
		fmt.Println(ui.WarningF("Could not copy instructions: %v", err))
	} else {
		filesCopied++
		manifest.Files = append(manifest.Files, "thts-instructions.md")
		fmt.Println(ui.Success("Copied thts-instructions.md"))
	}

	// Copy selected categories
	for _, category := range selectedCategories {
		embedFS, dir := getEmbedFSForCategory(category)
		if embedFS == nil {
			continue
		}
		copied, skipped, copiedFiles, err := copyEmbeddedCategory(
			embedFS, dir,
			filepath.Join(claudeTargetDir, category),
			filesToCopyByCategory[category],
		)
		if err != nil {
			fmt.Println(ui.WarningF("Could not copy %s: %v", category, err))
			continue
		}
		filesCopied += copied
		filesSkipped += skipped
		// Track copied files in manifest with category prefix
		for _, f := range copiedFiles {
			manifest.Files = append(manifest.Files, filepath.Join(category, f))
		}
		if copied > 0 {
			fmt.Println(ui.SuccessF("Copied %d %s file(s)", copied, category))
		}
	}

	// Handle integration level setup
	claudeMDMod, gitignorePatterns, err := setupIntegrationLevel(targetDir, claudeTargetDir, integrationLevel)
	if err != nil {
		fmt.Println(ui.WarningF("Could not setup integration: %v", err))
	} else {
		if claudeMDMod != nil {
			manifest.Modifications.ClaudeMD = claudeMDMod
		}
		if len(gitignorePatterns) > 0 {
			manifest.Modifications.Gitignore = &GitignoreModification{Patterns: gitignorePatterns}
		}
	}

	// Handle settings if --with-settings flag is set
	if initWithSettings {
		alwaysThinking, maxThinkingTokens, model, err := configureSettings()
		if err != nil {
			return err
		}
		if err := writeSettings(claudeTargetDir, alwaysThinking, maxThinkingTokens, model); err != nil {
			fmt.Println(ui.WarningF("Could not write settings: %v", err))
		} else {
			filesCopied++
			manifest.SettingsCreated = true
			manifest.Files = append(manifest.Files, "settings.json")
			fmt.Println(ui.SuccessF("Created settings.json (model: %s, alwaysThinking: %v, maxTokens: %d)",
				model, alwaysThinking, maxThinkingTokens))
		}

		// Update .gitignore for settings.local.json
		added, err := fsutil.AddToGitignore(targetDir, ".claude/settings.local.json", "project")
		if err != nil {
			fmt.Println(ui.WarningF("Could not update .gitignore: %v", err))
		} else if added {
			// Track gitignore pattern
			if manifest.Modifications.Gitignore == nil {
				manifest.Modifications.Gitignore = &GitignoreModification{Patterns: []string{}}
			}
			manifest.Modifications.Gitignore.Patterns = append(
				manifest.Modifications.Gitignore.Patterns, ".claude/settings.local.json")
			fmt.Println(ui.Info("Updated .gitignore to exclude settings.local.json"))
		}
	}

	// Write manifest to track what we created
	if err := writeManifest(claudeTargetDir, manifest); err != nil {
		fmt.Println(ui.WarningF("Could not write manifest: %v", err))
	}

	// Summary
	fmt.Println()
	fmt.Println(ui.SuccessF("Successfully copied %d file(s) to %s", filesCopied, claudeTargetDir))
	if filesSkipped > 0 {
		fmt.Println(ui.Muted(fmt.Sprintf("   Skipped %d file(s)", filesSkipped)))
	}
	fmt.Println(ui.Muted("   You can now use /thts-integrate, /thts-handoff, /thts-resume in Claude Code."))

	return nil
}

// selectIntegrationLevel prompts user to select how thoughts/ integration is activated.
func selectIntegrationLevel() (IntegrationLevel, error) {
	if !initInteractive {
		// Default to always-on in non-interactive mode
		return IntegrationAlwaysOn, nil
	}

	var level string
	err := huh.NewSelect[string]().
		Title("How would you like to integrate thoughts/ with Claude?").
		Options(
			huh.NewOption("Always-on (add to CLAUDE.md) - Adds @include to project CLAUDE.md", string(IntegrationAlwaysOn)),
			huh.NewOption("Always-on (local only) - Creates .claude/CLAUDE.local.md (gitignored)", string(IntegrationLocalOnly)),
			huh.NewOption("On-demand only - Just installs skill and commands", string(IntegrationOnDemand)),
		).
		Value(&level).
		Run()
	if err != nil {
		return "", err
	}
	return IntegrationLevel(level), nil
}

// selectCategories prompts user to select categories to copy.
func selectCategories() ([]string, error) {
	// Default: copy all
	if !initInteractive {
		return []string{"commands", "agents", "skills"}, nil
	}

	// Interactive: let user choose
	var selected []string
	err := huh.NewMultiSelect[string]().
		Title("What would you like to copy?").
		Options(
			huh.NewOption("Skills (/thts-integrate for on-demand activation)", "skills").Selected(true),
			huh.NewOption("Commands (/thts-handoff, /thts-resume for session continuity)", "commands").Selected(true),
			huh.NewOption("Agents (thoughts-locator, thoughts-analyzer)", "agents").Selected(true),
		).
		Value(&selected).
		Run()
	if err != nil {
		return nil, err
	}
	return selected, nil
}

// getEmbedFSForCategory returns the embedded FS and directory name for a category.
func getEmbedFSForCategory(category string) (fs.FS, string) {
	switch category {
	case "commands":
		return thtsfiles.Commands, "commands"
	case "agents":
		return thtsfiles.Agents, "agents"
	case "skills":
		return thtsfiles.Skills, "skills"
	default:
		return nil, ""
	}
}

// copyInstructionsFile copies the thts-instructions.md file to .claude/.
func copyInstructionsFile(claudeDir string) error {
	content, err := fs.ReadFile(thtsfiles.Instructions, "instructions/thts-instructions.md")
	if err != nil {
		return fmt.Errorf("failed to read instructions: %w", err)
	}
	targetPath := filepath.Join(claudeDir, "thts-instructions.md")
	return os.WriteFile(targetPath, content, 0644)
}

// setupIntegrationLevel configures the integration based on the selected level.
// Returns the ClaudeMD modification info (if any) and any gitignore patterns added.
func setupIntegrationLevel(projectDir, claudeDir string, level IntegrationLevel) (*ClaudeMDModification, []string, error) {
	var gitignorePatterns []string

	switch level {
	case IntegrationAlwaysOn:
		// Find git root and append to CLAUDE.md there
		gitRoot, err := git.GetRepoTopLevelAt(projectDir)
		if err != nil {
			// Fall back to project directory if not in a git repo
			gitRoot = projectDir
		}
		mod, err := appendToClaudeMD(gitRoot)
		return mod, nil, err

	case IntegrationLocalOnly:
		// Create .claude/CLAUDE.local.md and add to .gitignore
		if err := createLocalClaudeMD(claudeDir); err != nil {
			return nil, nil, err
		}
		// Add to .gitignore
		added, err := fsutil.AddToGitignore(projectDir, ".claude/CLAUDE.local.md", "project")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update .gitignore: %w", err)
		}
		if added {
			gitignorePatterns = append(gitignorePatterns, ".claude/CLAUDE.local.md")
			fmt.Println(ui.Info("Updated .gitignore to exclude CLAUDE.local.md"))
		}
		fmt.Println(ui.Success("Created .claude/CLAUDE.local.md with @include"))
		return nil, gitignorePatterns, nil

	case IntegrationOnDemand:
		// Nothing to do - files are already copied
		fmt.Println(ui.Info("On-demand mode: use /thts-integrate to activate"))
		return nil, nil, nil
	}
	return nil, nil, nil
}

// appendToClaudeMD appends the @include directive to CLAUDE.md.
// Returns modification info if changes were made, or nil if already present.
func appendToClaudeMD(gitRoot string) (*ClaudeMDModification, error) {
	claudeMDPath := filepath.Join(gitRoot, "CLAUDE.md")
	includeDirective := "\n@.claude/thts-instructions.md\n"
	pattern := "@.claude/thts-instructions.md"

	// Check if file exists and already has the include
	if fsutil.Exists(claudeMDPath) {
		content, err := os.ReadFile(claudeMDPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CLAUDE.md: %w", err)
		}
		if strings.Contains(string(content), pattern) {
			fmt.Println(ui.Info("CLAUDE.md already includes thts-instructions.md"))
			return nil, nil
		}
		// Append to existing file
		f, err := os.OpenFile(claudeMDPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open CLAUDE.md: %w", err)
		}
		if _, err := f.WriteString(includeDirective); err != nil {
			_ = f.Close() // Ignore close error, returning write error
			return nil, fmt.Errorf("failed to append to CLAUDE.md: %w", err)
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close CLAUDE.md: %w", err)
		}
		fmt.Println(ui.Success("Appended @include to existing CLAUDE.md"))
		return &ClaudeMDModification{
			Path:    claudeMDPath,
			Action:  "appended",
			Pattern: pattern,
		}, nil
	}
	// Create new file
	if err := os.WriteFile(claudeMDPath, []byte("# Claude Code Instructions\n"+includeDirective), 0644); err != nil {
		return nil, fmt.Errorf("failed to create CLAUDE.md: %w", err)
	}
	fmt.Println(ui.Success("Created CLAUDE.md with @include"))
	return &ClaudeMDModification{
		Path:    claudeMDPath,
		Action:  "created",
		Pattern: pattern,
	}, nil
}

// createLocalClaudeMD creates .claude/CLAUDE.local.md with the @include directive.
func createLocalClaudeMD(claudeDir string) error {
	localPath := filepath.Join(claudeDir, "CLAUDE.local.md")
	content := "# Local Claude Code Instructions\n\n@thts-instructions.md\n"
	return os.WriteFile(localPath, []byte(content), 0644)
}

// listEmbeddedFiles returns the list of files in an embedded FS directory.
func listEmbeddedFiles(embedFS fs.FS, dir string) ([]string, error) {
	var files []string
	entries, err := fs.ReadDir(embedFS, dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// selectFiles prompts user to select specific files from a category.
func selectFiles(category string, files []string) ([]string, error) {
	// Default: copy all files
	if !initInteractive {
		return files, nil
	}

	var options []huh.Option[string]
	for _, f := range files {
		options = append(options, huh.NewOption(f, f).Selected(true))
	}

	var selected []string
	err := huh.NewMultiSelect[string]().
		Title(fmt.Sprintf("Select %s files to copy:", category)).
		Options(options...).
		Value(&selected).
		Run()
	if err != nil {
		return nil, err
	}
	return selected, nil
}

// copyEmbeddedCategory copies files from embedded FS to target directory.
// Returns count of copied/skipped files and the list of files actually copied.
func copyEmbeddedCategory(embedFS fs.FS, srcDir, targetDir string, selectedFiles []string) (copied, skipped int, copiedFiles []string, err error) {
	allFiles, err := listEmbeddedFiles(embedFS, srcDir)
	if err != nil {
		return 0, 0, nil, err
	}

	// If selectedFiles is nil (--all mode), copy all files
	if selectedFiles == nil {
		selectedFiles = allFiles
	}

	if len(selectedFiles) == 0 {
		return 0, len(allFiles), nil, nil
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, 0, nil, err
	}

	selectedSet := make(map[string]bool)
	for _, f := range selectedFiles {
		selectedSet[f] = true
	}

	for _, filename := range allFiles {
		if !selectedSet[filename] {
			skipped++
			continue
		}

		content, err := fs.ReadFile(embedFS, filepath.Join(srcDir, filename))
		if err != nil {
			return copied, skipped, copiedFiles, fmt.Errorf("failed to read %s: %w", filename, err)
		}

		targetPath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return copied, skipped, copiedFiles, fmt.Errorf("failed to write %s: %w", targetPath, err)
		}
		copied++
		copiedFiles = append(copiedFiles, filename)
	}

	return copied, skipped, copiedFiles, nil
}

// configureSettings prompts for settings values or uses defaults.
func configureSettings() (alwaysThinking bool, maxThinkingTokens int, model ModelType, err error) {
	// Default values
	alwaysThinking = true
	maxThinkingTokens = 32000
	model = ModelOpus

	// Use defaults in non-interactive mode
	if !initInteractive {
		return alwaysThinking, maxThinkingTokens, model, nil
	}

	// Interactive prompts
	err = huh.NewConfirm().
		Title("Enable always-on thinking mode for Claude Code?").
		Affirmative("Yes").
		Negative("No").
		Value(&alwaysThinking).
		Run()
	if err != nil {
		return
	}

	var tokensStr string
	err = huh.NewInput().
		Title("Maximum thinking tokens:").
		Value(&tokensStr).
		Placeholder("32000").
		Validate(func(s string) error {
			if s == "" {
				return nil // Use default
			}
			n, err := strconv.Atoi(s)
			if err != nil || n < 1000 {
				return fmt.Errorf("please enter a valid number (minimum 1000)")
			}
			return nil
		}).
		Run()
	if err != nil {
		return
	}
	if tokensStr != "" {
		maxThinkingTokens, _ = strconv.Atoi(tokensStr)
	}

	var modelStr string
	err = huh.NewSelect[string]().
		Title("Select default model:").
		Options(
			huh.NewOption("Opus (most capable)", string(ModelOpus)),
			huh.NewOption("Sonnet (balanced)", string(ModelSonnet)),
			huh.NewOption("Haiku (fastest)", string(ModelHaiku)),
		).
		Value(&modelStr).
		Run()
	if err != nil {
		return
	}
	model = ModelType(modelStr)

	return alwaysThinking, maxThinkingTokens, model, nil
}

// writeSettings creates the settings.json file.
func writeSettings(claudeDir string, alwaysThinking bool, maxThinkingTokens int, model ModelType) error {
	settings := map[string]any{
		"permissions": map[string]any{
			"allow": []string{},
		},
		"enableAllProjectMcpServers": false,
		"env": map[string]string{
			"MAX_THINKING_TOKENS":              strconv.Itoa(maxThinkingTokens),
			"CLAUDE_BASH_MAINTAIN_WORKING_DIR": "1",
		},
	}

	if alwaysThinking {
		settings["alwaysThinkingEnabled"] = true
	}
	if model != "" {
		settings["model"] = string(model)
	}

	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	return os.WriteFile(settingsPath, append(content, '\n'), 0644)
}

// isTerminal checks if stdin is a terminal.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// writeManifest writes the manifest file tracking init operations.
func writeManifest(claudeDir string, manifest *Manifest) error {
	manifest.Version = 1
	manifest.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(claudeDir, ManifestFile)
	return os.WriteFile(manifestPath, append(data, '\n'), 0644)
}
