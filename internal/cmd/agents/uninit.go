package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/scottames/thts/internal/agents"
	fsutil "github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	uninitAgents string
	uninitForce  bool
	uninitDryRun bool
	uninitAll    bool
)

var uninitCmd = &cobra.Command{
	Use:   "uninit",
	Short: "Remove agent integration from this project",
	Long: `Remove thts agent integration files from agent directories.

This removes:
  - AGENTS.md and agent-specific instruction files
  - skills/, commands/, agents/ files installed by thts
  - settings files if created by thts
  - @include directives from instruction files
  - gitignore patterns added by thts

The agent directories themselves are preserved if they contain other files.

Agent selection:
  --agents claude,codex   Remove specific agents
  --all                   Remove all detected agent integrations`,
	RunE: runAgentsUninit,
}

func init() {
	uninitCmd.Flags().StringVarP(&uninitAgents, "agents", "a", "", "Comma-separated list of agents to remove")
	uninitCmd.Flags().BoolVarP(&uninitForce, "force", "f", false, "Skip confirmation prompt")
	uninitCmd.Flags().BoolVar(&uninitDryRun, "dry-run", false, "Show what would be removed without removing")
	uninitCmd.Flags().BoolVar(&uninitAll, "all", false, "Remove all detected agent integrations")
}

func runAgentsUninit(cmd *cobra.Command, args []string) error {
	fmt.Println(ui.Header("Remove Agent Integration"))
	fmt.Println()

	targetDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Resolve which agents to uninitialize
	agentTypes, err := resolveUninitAgents(targetDir)
	if err != nil {
		return err
	}
	if len(agentTypes) == 0 {
		fmt.Println(ui.Error("No agents to remove."))
		return nil
	}

	agents.SortAgentTypes(agentTypes)
	fmt.Printf("%s Removing: %s\n", ui.Info(""), strings.Join(agents.AgentTypesToStrings(agentTypes), ", "))
	fmt.Println()

	// Collect removal plans for all agents
	var allPlans []*removalPlan
	for _, agentType := range agentTypes {
		plan, err := buildRemovalPlan(targetDir, agentType)
		if err != nil {
			fmt.Println(ui.WarningF("Could not analyze %s: %v", agentType, err))
			continue
		}
		if plan != nil {
			allPlans = append(allPlans, plan)
		}
	}

	if len(allPlans) == 0 {
		fmt.Println(ui.Info("No thts installations detected."))
		return nil
	}

	// Show removal plans
	for _, plan := range allPlans {
		printRemovalPlan(plan)
	}

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

	// Perform removal for each agent
	for _, plan := range allPlans {
		if err := performRemoval(plan); err != nil {
			fmt.Println(ui.ErrorF("Error removing %s: %v", plan.agentType, err))
		}
	}

	fmt.Println()
	fmt.Println(ui.Success("Successfully removed thts integration."))

	return nil
}

// resolveUninitAgents determines which agents to remove.
func resolveUninitAgents(projectDir string) ([]agents.AgentType, error) {
	// Check --agents flag
	if uninitAgents != "" {
		return agents.ParseAgentTypes(uninitAgents)
	}

	// Check --all flag or detect existing agents
	if uninitAll {
		detected := agents.DetectExistingAgents(projectDir)
		if len(detected) == 0 {
			return nil, nil
		}
		return detected, nil
	}

	// Detect agents with thts manifests
	var found []agents.AgentType
	for _, agentType := range agents.AllAgentTypes() {
		cfg := agents.GetConfig(agentType)
		agentDir := filepath.Join(projectDir, cfg.RootDir)
		manifestPath := filepath.Join(agentDir, ManifestFile)
		if fsutil.Exists(manifestPath) {
			found = append(found, agentType)
		}
	}

	if len(found) == 0 {
		// Try detection as fallback
		detected := agents.DetectExistingAgents(projectDir)
		for _, agentType := range detected {
			cfg := agents.GetConfig(agentType)
			agentDir := filepath.Join(projectDir, cfg.RootDir)
			// Check if any thts files exist
			if hasThtsFiles(agentDir) {
				found = append(found, agentType)
			}
		}
	}

	return found, nil
}

// hasThtsFiles checks if an agent directory contains thts-related files.
func hasThtsFiles(agentDir string) bool {
	knownFiles := []string{
		"AGENTS.md",
		"thts-instructions.md",
		ManifestFile,
	}
	for _, f := range knownFiles {
		if fsutil.Exists(filepath.Join(agentDir, f)) {
			return true
		}
	}
	return false
}

// removalPlan holds information about what to remove for an agent.
type removalPlan struct {
	agentType     agents.AgentType
	agentDir      string
	projectDir    string
	manifest      *Manifest
	filesToRemove []string
	modifications ManifestModifications
}

// buildRemovalPlan analyzes what needs to be removed for an agent.
func buildRemovalPlan(projectDir string, agentType agents.AgentType) (*removalPlan, error) {
	cfg := agents.GetConfig(agentType)
	agentDir := filepath.Join(projectDir, cfg.RootDir)

	if !fsutil.Exists(agentDir) {
		return nil, nil
	}

	plan := &removalPlan{
		agentType:  agentType,
		agentDir:   agentDir,
		projectDir: projectDir,
	}

	// Try to load manifest
	manifest, err := loadManifest(agentDir)
	if err != nil {
		// Fall back to detection
		manifest = detectInstallation(agentDir, projectDir, agentType)
	}

	if manifest == nil {
		return nil, nil
	}

	plan.manifest = manifest
	plan.filesToRemove = manifest.Files
	plan.modifications = manifest.Modifications

	return plan, nil
}

// loadManifest reads and parses the manifest file.
func loadManifest(agentDir string) (*Manifest, error) {
	manifestPath := filepath.Join(agentDir, ManifestFile)
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

// detectInstallation detects thts installation without a manifest.
func detectInstallation(agentDir, projectDir string, agentType agents.AgentType) *Manifest {
	cfg := agents.GetConfig(agentType)
	manifest := &Manifest{
		Agent: string(agentType),
		Files: []string{},
	}

	// Check for known thts files
	knownFiles := []string{
		"AGENTS.md",
		"thts-instructions.md",
	}

	// Add agent-specific instruction file
	if cfg.InstructionsFile != "AGENTS.md" {
		knownFiles = append(knownFiles, cfg.InstructionsFile)
	}

	// Add skill/command/agent files
	skillPaths := []string{
		filepath.Join(cfg.SkillsDir, "thts-integrate.md"),
		filepath.Join(cfg.SkillsDir, "thts-integrate", "SKILL.md"),
	}
	for _, sp := range skillPaths {
		if fsutil.Exists(filepath.Join(agentDir, sp)) {
			knownFiles = append(knownFiles, sp)
			break
		}
	}

	if cfg.SupportsCommands {
		knownFiles = append(knownFiles, "commands/thts-handoff.md")
		knownFiles = append(knownFiles, "commands/thts-resume.md")
	}

	agentFiles := []string{
		filepath.Join(cfg.AgentsDir, "thoughts-locator.md"),
		filepath.Join(cfg.AgentsDir, "thoughts-analyzer.md"),
	}
	knownFiles = append(knownFiles, agentFiles...)

	for _, f := range knownFiles {
		if fsutil.Exists(filepath.Join(agentDir, f)) {
			manifest.Files = append(manifest.Files, f)
		}
	}

	// Detect instruction file modification
	gitRoot, err := git.GetRepoTopLevelAt(projectDir)
	if err != nil {
		gitRoot = projectDir
	}

	// Check for @include in CLAUDE.md or AGENTS.md
	for _, instFile := range []string{"CLAUDE.md", "AGENTS.md"} {
		instPath := filepath.Join(gitRoot, instFile)
		if fsutil.Exists(instPath) {
			content, err := os.ReadFile(instPath)
			if err == nil {
				pattern := fmt.Sprintf("@%s/AGENTS.md", cfg.RootDir)
				if strings.Contains(string(content), pattern) {
					manifest.Modifications.InstructionsMD = &InstructionsMDModification{
						Path:    instPath,
						Action:  "appended",
						Pattern: pattern,
					}
					break
				}
			}
		}
	}

	// Detect gitignore patterns
	gitignorePath := filepath.Join(projectDir, ".gitignore")
	if fsutil.Exists(gitignorePath) {
		content, _ := os.ReadFile(gitignorePath)
		var patterns []string
		localPatterns := []string{
			filepath.Join(cfg.RootDir, "CLAUDE.local.md"),
			filepath.Join(cfg.RootDir, "AGENTS.local.md"),
			filepath.Join(cfg.RootDir, "settings.local.json"),
		}
		for _, p := range localPatterns {
			if strings.Contains(string(content), p) {
				patterns = append(patterns, p)
			}
		}
		if len(patterns) > 0 {
			manifest.Modifications.Gitignore = &GitignoreModification{Patterns: patterns}
		}
	}

	// Infer integration level
	if fsutil.Exists(filepath.Join(agentDir, "CLAUDE.local.md")) ||
		fsutil.Exists(filepath.Join(agentDir, "AGENTS.local.md")) {
		manifest.IntegrationLevel = IntegrationLocalOnly
	} else if manifest.Modifications.InstructionsMD != nil {
		manifest.IntegrationLevel = IntegrationAlwaysOn
	} else {
		manifest.IntegrationLevel = IntegrationOnDemand
	}

	if len(manifest.Files) == 0 && manifest.Modifications.InstructionsMD == nil {
		return nil
	}

	return manifest
}

// printRemovalPlan shows what will be removed.
func printRemovalPlan(plan *removalPlan) {
	fmt.Println(ui.SubHeader(fmt.Sprintf("%s:", agents.AgentTypeLabels[plan.agentType])))

	if len(plan.filesToRemove) > 0 {
		fmt.Println("  Files to remove:")
		for _, f := range plan.filesToRemove {
			fmt.Printf("    %s\n", filepath.Join(plan.agentDir, f))
		}
	}

	if plan.manifest != nil && plan.manifest.SettingsCreated {
		cfg := agents.GetConfig(plan.agentType)
		fmt.Printf("    %s\n", filepath.Join(plan.agentDir, cfg.SettingsFile))
	}

	if plan.modifications.InstructionsMD != nil {
		fmt.Println("  Modifications to revert:")
		fmt.Printf("    Remove @include from: %s\n", plan.modifications.InstructionsMD.Path)
	}

	if plan.modifications.Gitignore != nil && len(plan.modifications.Gitignore.Patterns) > 0 {
		fmt.Println("  Gitignore patterns to remove:")
		for _, p := range plan.modifications.Gitignore.Patterns {
			fmt.Printf("    %s\n", p)
		}
	}

	fmt.Println()
}

// performRemoval removes all thts integration files and reverts modifications.
func performRemoval(plan *removalPlan) error {
	cfg := agents.GetConfig(plan.agentType)
	var warnings []string

	// 1. Remove files
	for _, f := range plan.filesToRemove {
		path := filepath.Join(plan.agentDir, f)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("failed to remove %s: %v", f, err))
		} else if err == nil {
			fmt.Println(ui.SuccessF("Removed %s", f))
		}
	}

	// 2. Remove settings if created by thts
	if plan.manifest != nil && plan.manifest.SettingsCreated {
		settingsPath := filepath.Join(plan.agentDir, cfg.SettingsFile)
		if err := os.Remove(settingsPath); err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("failed to remove %s: %v", cfg.SettingsFile, err))
		} else if err == nil {
			fmt.Println(ui.SuccessF("Removed %s", cfg.SettingsFile))
		}
	}

	// 3. Clean up empty subdirectories
	subdirs := []string{cfg.SkillsDir, cfg.AgentsDir}
	if cfg.SupportsCommands {
		subdirs = append(subdirs, "commands")
	}
	for _, subdir := range subdirs {
		dir := filepath.Join(plan.agentDir, subdir)
		cleanEmptyDirs(dir)
	}

	// 4. Revert instruction file modification
	if plan.modifications.InstructionsMD != nil {
		if err := removeFromInstructionsMD(plan.modifications.InstructionsMD); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to clean instruction file: %v", err))
		} else {
			fmt.Println(ui.Success("Removed @include from instruction file"))
		}
	}

	// 5. Remove gitignore patterns
	if plan.modifications.Gitignore != nil {
		for _, pattern := range plan.modifications.Gitignore.Patterns {
			removed, err := fsutil.RemoveFromGitignore(plan.projectDir, pattern, "project")
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to remove gitignore pattern %s: %v", pattern, err))
			} else if removed {
				fmt.Println(ui.SuccessF("Removed %s from .gitignore", pattern))
			}
		}
	}

	// 6. Remove manifest itself
	manifestPath := filepath.Join(plan.agentDir, ManifestFile)
	_ = os.Remove(manifestPath)

	if len(warnings) > 0 {
		fmt.Println(ui.Warning("Completed with warnings:"))
		for _, w := range warnings {
			fmt.Printf("  %s\n", w)
		}
	}

	return nil
}

// removeFromInstructionsMD removes the @include directive from the instruction file.
func removeFromInstructionsMD(mod *InstructionsMDModification) error {
	content, err := os.ReadFile(mod.Path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		if strings.TrimSpace(line) != mod.Pattern {
			newLines = append(newLines, line)
		}
	}

	newContent := strings.Join(newLines, "\n")
	return os.WriteFile(mod.Path, []byte(newContent), 0644)
}

// cleanEmptyDirs removes empty directories recursively.
func cleanEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			cleanEmptyDirs(filepath.Join(dir, entry.Name()))
		}
	}

	// Re-check if directory is now empty
	entries, _ = os.ReadDir(dir)
	if len(entries) == 0 {
		_ = os.Remove(dir)
	}
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

// Uninit removes thts integration from the given directory for specified agents.
// This is exported so that `thts uninit` can call it to clean up agent files.
func Uninit(targetDir string, force bool, agentTypesToRemove []agents.AgentType) error {
	if len(agentTypesToRemove) == 0 {
		// Remove all detected agents
		agentTypesToRemove = agents.DetectExistingAgents(targetDir)
	}

	for _, agentType := range agentTypesToRemove {
		plan, err := buildRemovalPlan(targetDir, agentType)
		if err != nil {
			continue
		}
		if plan != nil {
			if err := performRemoval(plan); err != nil {
				return err
			}
		}
	}

	return nil
}
