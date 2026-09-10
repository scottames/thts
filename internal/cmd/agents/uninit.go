package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
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
	uninitGlobal bool
)

var (
	saveGlobalManifestForUninit = SaveGlobalManifest
	saveConfigForGlobalUninit   = config.Save
)

// UninitCmd is the command for removing agent integration.
// It is exported so it can be registered as a subcommand of `thts uninit`.
var UninitCmd = &cobra.Command{
	Use:   "agents",
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
  --all                   Remove all detected agent integrations

Usage: thts uninit agents [flags]`,
	RunE: runAgentsUninit,
}

func init() {
	UninitCmd.Flags().StringVarP(&uninitAgents, "agents", "a", "", "Comma-separated list of agents to remove (claude,codex,opencode,gemini,pi,droid)")
	UninitCmd.Flags().BoolVarP(&uninitForce, "force", "f", false, "Skip confirmation prompt")
	UninitCmd.Flags().BoolVar(&uninitDryRun, "dry-run", false, "Show what would be removed without removing")
	UninitCmd.Flags().BoolVar(&uninitAll, "all", false, "Remove all detected agent integrations")
	UninitCmd.Flags().BoolVar(&uninitGlobal, "global", false, "Remove globally installed components")
	_ = UninitCmd.RegisterFlagCompletionFunc("agents", completeAgentTypes)
}

func runAgentsUninit(cmd *cobra.Command, args []string) error {
	// Check if --global flag was provided
	if uninitGlobal {
		return runGlobalUninit(cmd, args)
	}

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
	var analysisErrors []error
	for _, agentType := range agentTypes {
		plan, err := buildRemovalPlan(targetDir, agentType)
		if err != nil {
			fmt.Println(ui.WarningF("Could not analyze %s: %v", agentType, err))
			analysisErrors = append(analysisErrors, fmt.Errorf("analyze %s: %w", agentType, err))
			continue
		}
		if plan != nil {
			allPlans = append(allPlans, plan)
		}
	}

	if len(allPlans) == 0 {
		if len(analysisErrors) > 0 {
			return errors.Join(analysisErrors...)
		}
		fmt.Println(ui.Info("No thts installations detected."))
		return nil
	}

	// Show removal plans
	for _, plan := range allPlans {
		printRemovalPlan(plan)
	}
	if len(analysisErrors) > 0 {
		return errors.Join(analysisErrors...)
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
	var removedAgents []agents.AgentType
	var removalErrors []error
	for _, plan := range allPlans {
		if err := performRemoval(plan); err != nil {
			fmt.Println(ui.ErrorF("Error removing %s: %v", plan.agentType, err))
			removalErrors = append(removalErrors, fmt.Errorf("remove %s: %w", plan.agentType, err))
			continue
		}
		removedAgents = append(removedAgents, plan.agentType)
	}

	// Update gitignore - remove marker block or rebuild with remaining agents
	if err := updateGitignoreAfterUninit(targetDir, removedAgents); err != nil {
		fmt.Println(ui.WarningF("Could not update .gitignore: %v", err))
	}

	fmt.Println()
	if len(removalErrors) > 0 {
		return errors.Join(removalErrors...)
	}
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
		// Droid support has no legacy manifestless installations. Inferring
		// ownership from native Factory filenames could delete user resources.
		if agentType == agents.AgentDroid {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		// Fall back to detection for agents with legacy installations.
		manifest = detectInstallation(agentDir, projectDir, agentType)
	}

	if manifest == nil {
		return nil, nil
	}

	plan.manifest = manifest
	plan.filesToRemove = slices.Clone(manifest.Files)
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
	knownFiles := []string{"thts-instructions.md"}
	if agentType != agents.AgentDroid {
		knownFiles = append(knownFiles, "AGENTS.md")
	}

	// Add agent-specific instruction file
	if cfg.InstructionsFile != "" && cfg.InstructionsFile != "AGENTS.md" {
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

	if cfg.SupportsCommands && cfg.CommandsDir != "" {
		// Determine command file extension based on agent's command format
		ext := ".md"
		if cfg.CommandsFormat == "toml" {
			ext = ".toml"
		}
		knownFiles = append(knownFiles, filepath.Join(cfg.CommandsDir, "thts-handoff"+ext))
		knownFiles = append(knownFiles, filepath.Join(cfg.CommandsDir, "thts-resume"+ext))
	}

	// Only add agent files if agent supports agents feature
	if cfg.AgentsDir != "" {
		agentFiles := []string{
			filepath.Join(cfg.AgentsDir, "thoughts-locator.md"),
			filepath.Join(cfg.AgentsDir, "thoughts-analyzer.md"),
		}
		knownFiles = append(knownFiles, agentFiles...)
	}

	// Add hook files if agent supports hooks
	if cfg.HooksDir != "" {
		hookFiles := []string{
			filepath.Join(cfg.HooksDir, "thts-session-start.sh"),
			filepath.Join(cfg.HooksDir, "thts-prompt-check.sh"),
		}
		knownFiles = append(knownFiles, hookFiles...)
	}

	// Add plugin files if agent supports plugins
	if cfg.PluginsDir != "" {
		pluginFiles := []string{
			filepath.Join(cfg.PluginsDir, "thts-integration.ts"),
		}
		knownFiles = append(knownFiles, pluginFiles...)
	}

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

	// Check for marker-based integration in CLAUDE.md or AGENTS.md
	for _, instFile := range []string{"CLAUDE.md", "AGENTS.md"} {
		instPath := filepath.Join(gitRoot, instFile)
		if fsutil.Exists(instPath) {
			content, err := os.ReadFile(instPath)
			if err == nil {
				contentStr := string(content)
				// Check for new marker-based integration
				if strings.Contains(contentStr, ThtsMarkerStart) {
					manifest.Modifications.InstructionsMD = &InstructionsMDModification{
						Path:            instPath,
						Action:          "appended",
						IntegrationType: "marker",
						MarkerBased:     true,
					}
					break
				}
				// Check for legacy @include pattern
				pattern := fmt.Sprintf("@%s/AGENTS.md", cfg.RootDir)
				if strings.Contains(contentStr, pattern) {
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
	if agentType == agents.AgentDroid {
		localPath := filepath.Join(agentDir, "AGENTS.md")
		if content, err := os.ReadFile(localPath); err == nil && strings.Contains(string(content), ThtsMarkerStart) {
			manifest.Modifications.InstructionsMD = &InstructionsMDModification{
				Path:            localPath,
				Action:          "appended",
				IntegrationType: "marker",
				MarkerBased:     true,
			}
		}
	}

	// Check for config-based integration (OpenCode)
	if cfg.IntegrationType == "config" && manifest.Modifications.InstructionsMD == nil {
		configPath := filepath.Join(projectDir, cfg.SettingsFile)
		if fsutil.Exists(configPath) {
			data, err := os.ReadFile(configPath)
			if err == nil {
				var config map[string]any
				if json.Unmarshal(data, &config) == nil {
					if instructions, ok := config["instructions"].([]any); ok {
						instructionPath := fmt.Sprintf("%s/%s", cfg.RootDir, ThtsInstructionsFile)
						for _, inst := range instructions {
							if instStr, ok := inst.(string); ok && instStr == instructionPath {
								manifest.Modifications.InstructionsMD = &InstructionsMDModification{
									Path:            configPath,
									Action:          "appended",
									IntegrationType: "config",
									ConfigKey:       "instructions",
								}
								break
							}
						}
					}
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
		if agentType == agents.AgentDroid {
			localPatterns = append(localPatterns, filepath.Join(cfg.RootDir, "AGENTS.md"))
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

	// Check for hook-based integration
	hooksFile := hookConfigFile(cfg, false)
	hooksPath := filepath.Join(agentDir, hooksFile)
	if fsutil.Exists(hooksPath) {
		data, err := os.ReadFile(hooksPath)
		if err == nil {
			var document map[string]any
			if json.Unmarshal(data, &document) == nil {
				hooks := hookEventsFromDocument(document, cfg.HookConfigFile != "")
				foundHooks := findThtsHookCommands(hooks, getThtsHookEventNames(agentType), getThtsHookNames(agentType, false))
				if len(foundHooks) > 0 {
					manifest.Modifications.Hooks = &HooksModification{
						SettingsFile: hooksFile,
						HookCommands: foundHooks,
					}
				}
			}
		}
	}

	// Infer integration level
	if manifest.Modifications.Hooks != nil {
		manifest.IntegrationLevel = IntegrationHook
	} else if fsutil.Exists(filepath.Join(agentDir, "CLAUDE.local.md")) ||
		fsutil.Exists(filepath.Join(agentDir, "AGENTS.local.md")) {
		manifest.IntegrationLevel = IntegrationAgentsContentLocal
	} else if agentType == agents.AgentDroid && manifest.Modifications.InstructionsMD != nil &&
		filepath.Clean(manifest.Modifications.InstructionsMD.Path) == filepath.Join(agentDir, "AGENTS.md") {
		manifest.IntegrationLevel = IntegrationAgentsContentLocal
	} else if manifest.Modifications.InstructionsMD != nil {
		manifest.IntegrationLevel = IntegrationAgentsContent
	} else {
		manifest.IntegrationLevel = IntegrationOnDemand
	}

	if len(manifest.Files) == 0 && manifest.Modifications.InstructionsMD == nil && manifest.Modifications.Hooks == nil {
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
		mod := plan.modifications.InstructionsMD
		switch mod.IntegrationType {
		case "marker":
			fmt.Printf("    Remove thts marker block from: %s\n", mod.Path)
		case "config":
			fmt.Printf("    Remove thts from instructions array in: %s\n", mod.Path)
		default:
			fmt.Printf("    Remove @include from: %s\n", mod.Path)
		}
	}

	if plan.modifications.Gitignore != nil && len(plan.modifications.Gitignore.Patterns) > 0 {
		fmt.Println("  Gitignore patterns to remove:")
		for _, p := range plan.modifications.Gitignore.Patterns {
			fmt.Printf("    %s\n", p)
		}
	}

	if plan.modifications.Hooks != nil && len(plan.modifications.Hooks.HookCommands) > 0 {
		fmt.Println("  Hooks to remove from settings:")
		for _, cmd := range plan.modifications.Hooks.HookCommands {
			fmt.Printf("    %s\n", cmd)
		}
	}

	fmt.Println()
}

// isPathSafeForRemoval checks if a relative file path is safe to remove.
// Returns false for empty strings, absolute paths, paths that escape the
// directory via "..", or paths that would resolve to dangerous locations.
func isPathSafeForRemoval(relativePath, baseDir string) bool {
	// Skip empty strings or current directory
	if relativePath == "" || relativePath == "." {
		return false
	}

	// Skip absolute paths - all files should be relative to agent dir
	if filepath.IsAbs(relativePath) {
		return false
	}

	// Skip paths starting with ".." (directory escape)
	if strings.HasPrefix(relativePath, "..") {
		return false
	}

	// Resolve the full path and ensure it's within the base directory
	fullPath := filepath.Join(baseDir, relativePath)
	cleanPath := filepath.Clean(fullPath)
	cleanBase := filepath.Clean(baseDir)

	// The resolved path must be within the base directory (not equal to it, not outside it)
	if cleanPath == cleanBase {
		return false
	}
	if !strings.HasPrefix(cleanPath, cleanBase+string(filepath.Separator)) {
		return false
	}

	// Extra safety: never allow removal of root paths
	if cleanPath == "/" || cleanPath == filepath.VolumeName(cleanPath)+string(filepath.Separator) {
		return false
	}

	return true
}

// performRemoval removes all thts integration files and reverts modifications.
func performRemoval(plan *removalPlan) error {
	cfg := agents.GetConfig(plan.agentType)
	var warnings []string
	droidHooksRemoved := false
	if plan.agentType == agents.AgentDroid && plan.modifications.Hooks != nil {
		if err := removeHooksFromSettings(plan.agentDir, cfg, plan.modifications.Hooks); err != nil {
			return fmt.Errorf("failed to remove hooks from settings: %w", err)
		}
		droidHooksRemoved = true
		plan.manifest.Modifications.Hooks = nil
		fmt.Println(ui.Success("Removed thts hooks from settings"))
	}

	// 1. Remove files
	for _, f := range plan.filesToRemove {
		if plan.modifications.Hooks != nil && filepath.Clean(f) == filepath.Clean(plan.modifications.Hooks.SettingsFile) {
			continue
		}
		// Validate path is safe to remove
		if !isPathSafeForRemoval(f, plan.agentDir) {
			if plan.agentType == agents.AgentDroid {
				warnings = append(warnings, fmt.Sprintf("refused to remove unsafe manifest path %s", f))
			}
			continue
		}
		path := filepath.Join(plan.agentDir, f)
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("failed to remove %s: %v", f, err))
		} else if err == nil {
			fmt.Println(ui.SuccessF("Removed %s", f))
		}
		if plan.agentType == agents.AgentDroid && (err == nil || os.IsNotExist(err)) {
			plan.manifest.Files = removeStringValue(plan.manifest.Files, f)
		}
	}

	// 2. Remove settings if created by thts
	if plan.manifest != nil && plan.manifest.SettingsCreated {
		settingsPath := filepath.Join(plan.agentDir, cfg.SettingsFile)
		err := os.Remove(settingsPath)
		if err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("failed to remove %s: %v", cfg.SettingsFile, err))
		} else if err == nil {
			fmt.Println(ui.SuccessF("Removed %s", cfg.SettingsFile))
		}
		if plan.agentType == agents.AgentDroid && (err == nil || os.IsNotExist(err)) {
			plan.manifest.SettingsCreated = false
			plan.manifest.Files = removeStringValue(plan.manifest.Files, cfg.SettingsFile)
		}
	}

	// 3. Clean up empty subdirectories
	subdirs := []string{cfg.SkillsDir}
	if cfg.AgentsDir != "" {
		subdirs = append(subdirs, cfg.AgentsDir)
	}
	if cfg.SupportsCommands && cfg.CommandsDir != "" {
		subdirs = append(subdirs, cfg.CommandsDir)
	}
	if cfg.HooksDir != "" {
		subdirs = append(subdirs, cfg.HooksDir)
	}
	if cfg.PluginsDir != "" {
		subdirs = append(subdirs, cfg.PluginsDir)
	}
	for _, subdir := range subdirs {
		dir := filepath.Join(plan.agentDir, subdir)
		cleanEmptyDirs(dir)
	}

	// 4. Remove settings context key if applicable (non-destructive)
	if cfg.SettingsContextKey != "" {
		if err := removeSettingsContextKey(plan.agentDir, cfg); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to clean settings context key: %v", err))
		} else {
			fmt.Println(ui.SuccessF("Removed %s from %s", cfg.SettingsContextKey, cfg.SettingsFile))
		}
	}

	// 5. Remove hooks from settings
	if plan.modifications.Hooks != nil && !droidHooksRemoved {
		if err := removeHooksFromSettings(plan.agentDir, cfg, plan.modifications.Hooks); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to remove hooks from settings: %v", err))
		} else {
			fmt.Println(ui.Success("Removed thts hooks from settings"))
		}
	}

	// 6. Revert instruction file modification
	if plan.modifications.InstructionsMD != nil {
		if err := removeThtsIntegration(plan.modifications.InstructionsMD, plan.agentType, plan.projectDir); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to clean instruction file: %v", err))
		} else {
			if plan.agentType == agents.AgentDroid {
				plan.manifest.Modifications.InstructionsMD = nil
			}
			fmt.Println(ui.Success("Removed thts integration from instruction file"))
		}
	}

	// 7. Remove gitignore patterns
	if plan.modifications.Gitignore != nil {
		for _, pattern := range slices.Clone(plan.modifications.Gitignore.Patterns) {
			removed, err := fsutil.RemoveFromGitignore(plan.projectDir, pattern, "project")
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to remove gitignore pattern %s: %v", pattern, err))
			} else if removed {
				fmt.Println(ui.SuccessF("Removed %s from .gitignore", pattern))
			}
			if plan.agentType == agents.AgentDroid && err == nil {
				plan.manifest.Modifications.Gitignore.Patterns = removeStringValue(plan.manifest.Modifications.Gitignore.Patterns, pattern)
			}
		}
		if plan.agentType == agents.AgentDroid && len(plan.manifest.Modifications.Gitignore.Patterns) == 0 {
			plan.manifest.Modifications.Gitignore = nil
		}
	}

	if len(warnings) > 0 {
		fmt.Println(ui.Warning("Completed with warnings:"))
		for _, w := range warnings {
			fmt.Printf("  %s\n", w)
		}
		if plan.agentType == agents.AgentDroid {
			if err := writeManifest(plan.agentDir, plan.manifest); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to retain manifest: %v", err))
			}
			return errors.New(strings.Join(warnings, "; "))
		}
	}

	// 8. Remove manifest itself
	manifestPath := filepath.Join(plan.agentDir, ManifestFile)
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove manifest: %w", err)
	}

	return nil
}

// removeThtsIntegration removes thts integration based on the integration type.
func removeThtsIntegration(mod *InstructionsMDModification, agentType agents.AgentType, projectDir string) error {
	// Dispatch based on integration type
	switch mod.IntegrationType {
	case "marker":
		transferred, err := transferSharedMarkerOwnership(mod, agentType, projectDir)
		if err != nil {
			return err
		}
		if transferred {
			return nil
		}
		if err := removeMarkerBlock(mod.Path); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if mod.Action == "created" {
			content, err := os.ReadFile(mod.Path)
			if err == nil && strings.TrimSpace(string(content)) == "# Agent Instructions" {
				return os.Remove(mod.Path)
			}
		}
		return nil
	case "config":
		cfg := agents.GetConfig(agentType)
		return removeFromOpenCodeConfig(mod.Path, cfg)
	default:
		// Legacy: try marker removal first, fall back to pattern removal
		if err := removeMarkerBlock(mod.Path); err == nil {
			return nil
		}
		return removeFromInstructionsMDLegacy(mod)
	}
}

func transferSharedMarkerOwnership(mod *InstructionsMDModification, agentType agents.AgentType, projectDir string) (bool, error) {
	// Instructions are written at the Git root even when initialization starts
	// from a nested directory, where the per-agent manifests remain.
	gitRoot, err := git.GetRepoTopLevelAt(projectDir)
	if err != nil {
		gitRoot = projectDir
	}
	if filepath.Clean(mod.Path) != filepath.Join(filepath.Clean(gitRoot), "AGENTS.md") {
		return false, nil
	}
	for _, candidate := range agents.AllAgentTypes() {
		if candidate == agentType {
			continue
		}
		cfg := agents.GetConfig(candidate)
		manifestPath := filepath.Join(projectDir, cfg.RootDir)
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, err
		}
		if normalizeIntegrationLevel(manifest.IntegrationLevel) != IntegrationAgentsContent {
			continue
		}
		if manifest.Modifications.InstructionsMD != nil {
			if filepath.Clean(manifest.Modifications.InstructionsMD.Path) == filepath.Clean(mod.Path) {
				return true, nil
			}
			continue
		}
		if cfg.InstructionTargetFile != "AGENTS.md" || cfg.IntegrationType != "marker" {
			continue
		}
		transferred := *mod
		manifest.Modifications.InstructionsMD = &transferred
		if err := writeManifest(manifestPath, manifest); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// removeMarkerBlock removes content between thts markers from a file.
// Returns nil if markers not found (nothing to remove).
// Returns error if markers are corrupted (only one found).
func removeMarkerBlock(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	text := string(content)
	startIdx := strings.Index(text, ThtsMarkerStart)
	endIdx := strings.Index(text, ThtsMarkerEnd)

	// No markers found - nothing to remove
	if startIdx == -1 && endIdx == -1 {
		return nil
	}

	// Corrupted: only one marker
	if startIdx == -1 || endIdx == -1 {
		return fmt.Errorf("corrupted markers in %s: found only %s",
			filePath,
			map[bool]string{true: "start", false: "end"}[startIdx != -1])
	}

	// Check for multiple pairs (warn but continue)
	secondStart := strings.Index(text[startIdx+len(ThtsMarkerStart):], ThtsMarkerStart)
	if secondStart != -1 {
		fmt.Println(ui.Warning("  Multiple marker pairs detected, removing first only"))
	}

	// Calculate removal range
	endIdx += len(ThtsMarkerEnd)

	// Trim trailing newline if present
	if endIdx < len(text) && text[endIdx] == '\n' {
		endIdx++
	}
	// Trim leading newline if present
	if startIdx > 0 && text[startIdx-1] == '\n' {
		startIdx--
	}

	newContent := text[:startIdx] + text[endIdx:]

	// Clean up multiple blank lines
	for strings.Contains(newContent, "\n\n\n") {
		newContent = strings.ReplaceAll(newContent, "\n\n\n", "\n\n")
	}

	return os.WriteFile(filePath, []byte(newContent), 0o644)
}

// removeFromOpenCodeConfig removes thts-instructions.md from the instructions array.
func removeFromOpenCodeConfig(configPath string, cfg *agents.AgentConfig) error {
	if !fsutil.Exists(configPath) {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	instructions, ok := config["instructions"].([]any)
	if !ok {
		return nil // No instructions array
	}

	instructionPath := fmt.Sprintf("%s/%s", cfg.RootDir, ThtsInstructionsFile)
	var newInstructions []any
	removed := false

	for _, inst := range instructions {
		if instStr, ok := inst.(string); ok && instStr == instructionPath {
			removed = true
			continue
		}
		newInstructions = append(newInstructions, inst)
	}

	if !removed {
		return nil
	}

	if len(newInstructions) == 0 {
		delete(config, "instructions")
	} else {
		config["instructions"] = newInstructions
	}

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, append(newData, '\n'), 0o644)
}

// removeFromInstructionsMDLegacy removes the @include directive using legacy pattern matching.
// This is for backward compatibility with old manifests.
func removeFromInstructionsMDLegacy(mod *InstructionsMDModification) error {
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
	return os.WriteFile(mod.Path, []byte(newContent), 0o644)
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

// removeSettingsContextKey removes the context key from settings if present.
// This function is non-destructive: it only removes the specific key added by thts.
func removeSettingsContextKey(agentDir string, cfg *agents.AgentConfig) error {
	if cfg.SettingsContextKey == "" {
		return nil // Agent doesn't use this feature
	}

	settingsPath := filepath.Join(agentDir, cfg.SettingsFile)
	if !fsutil.Exists(settingsPath) {
		return nil // No settings file to clean
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	// Check if key exists
	if _, ok := settings[cfg.SettingsContextKey]; !ok {
		return nil // Nothing to remove
	}

	delete(settings, cfg.SettingsContextKey)

	// Write back the modified settings (even if empty, preserve user's file)
	newData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, append(newData, '\n'), 0o644)
}

// removeHooksFromSettings removes thts hooks from a project hook configuration.
func removeHooksFromSettings(agentDir string, cfg *agents.AgentConfig, mod *HooksModification) error {
	if mod == nil || mod.SettingsFile == "" {
		return nil
	}

	settingsPath := filepath.Join(agentDir, mod.SettingsFile)
	if !fsutil.Exists(settingsPath) {
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	standalone := cfg.HookConfigFile != ""
	return removeHooksFromDocument(settingsPath, document, standalone, getThtsHookEventNames(cfg.Type), mod.HookCommands, !standalone || mod.ConfigCreated)
}

// removeGlobalHooksFromSettings removes thts hooks from a settings file (settings.json or settings.local.json).
// Takes the full path to the settings file and the list of agent names that had hooks installed.
// Handles the new hooks format (map with event names as keys).
func removeGlobalHooksFromSettings(settingsPath string, agentNames []string, removeEmpty bool) error {
	if !fsutil.Exists(settingsPath) {
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}

	// Build set of thts hook commands to remove (global paths)
	var commands []string
	var events []string
	standalone := false
	for _, agentName := range agentNames {
		agentType := agents.AgentType(agentName)
		cfg := agents.GetConfig(agentType)
		if cfg == nil || filepath.Clean(settingsPath) != filepath.Join(config.GlobalAgentDir(agentName), hookConfigFile(cfg, true)) {
			continue
		}
		standalone = cfg.HookConfigFile != ""
		commands = append(commands, getThtsHookNames(agentType, true)...)
		events = append(events, getThtsHookEventNames(agentType)...)
	}
	if len(commands) == 0 {
		return nil
	}
	return removeHooksFromDocument(settingsPath, document, standalone, events, commands, removeEmpty)
}

func hookEventsFromDocument(document map[string]any, standalone bool) map[string]any {
	if standalone {
		return document
	}
	hooks, _ := document["hooks"].(map[string]any)
	return hooks
}

func removeHooksFromDocument(path string, document map[string]any, standalone bool, events, commands []string, removeEmpty bool) error {
	hooks := hookEventsFromDocument(document, standalone)
	if hooks == nil {
		return nil
	}
	removeSet := make(map[string]bool, len(commands))
	for _, command := range commands {
		removeSet[command] = true
	}
	newHooks := filterOutThtsHooksFromMapForRemoval(hooks, events, removeSet)
	if standalone {
		document = newHooks
	} else if len(newHooks) == 0 {
		delete(document, "hooks")
	} else {
		document["hooks"] = newHooks
	}
	if len(document) == 0 && removeEmpty {
		return os.Remove(path)
	}
	newData, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(newData, '\n'), 0o644)
}

// filterOutThtsHooksFromMapForRemoval removes thts hooks from a hooks map.
// Takes a set of command paths to remove.
func filterOutThtsHooksFromMapForRemoval(hooks map[string]any, events []string, removeSet map[string]bool) map[string]any {
	result := make(map[string]any)
	eventSet := make(map[string]bool, len(events))
	for _, event := range events {
		eventSet[event] = true
	}

	for event, eventHooks := range hooks {
		if !eventSet[event] {
			result[event] = eventHooks
			continue
		}
		hookList, ok := eventHooks.([]any)
		if !ok {
			result[event] = eventHooks
			continue
		}

		var filteredList []any
		for _, hookEntry := range hookList {
			entryMap, ok := hookEntry.(map[string]any)
			if !ok {
				filteredList = append(filteredList, hookEntry)
				continue
			}

			innerHooks, ok := entryMap["hooks"].([]any)
			if !ok {
				filteredList = append(filteredList, hookEntry)
				continue
			}

			var filteredInner []any
			for _, inner := range innerHooks {
				innerMap, ok := inner.(map[string]any)
				if !ok {
					filteredInner = append(filteredInner, inner)
					continue
				}
				cmd, _ := innerMap["command"].(string)
				if !removeSet[cmd] {
					filteredInner = append(filteredInner, inner)
				}
			}

			if len(filteredInner) > 0 {
				newEntry := make(map[string]any)
				maps.Copy(newEntry, entryMap)
				newEntry["hooks"] = filteredInner
				filteredList = append(filteredList, newEntry)
			}
		}

		if len(filteredList) > 0 {
			result[event] = filteredList
		}
	}

	return result
}

func findThtsHookCommands(hooks map[string]any, events, commands []string) []string {
	commandSet := make(map[string]bool, len(commands))
	for _, command := range commands {
		commandSet[command] = true
	}
	eventSet := make(map[string]bool, len(events))
	for _, event := range events {
		eventSet[event] = true
	}
	var found []string
	for event, value := range hooks {
		if !eventSet[event] {
			continue
		}
		entries, _ := value.([]any)
		for _, entry := range entries {
			entryMap, _ := entry.(map[string]any)
			innerHooks, _ := entryMap["hooks"].([]any)
			for _, inner := range innerHooks {
				innerMap, _ := inner.(map[string]any)
				command, _ := innerMap["command"].(string)
				if commandSet[command] && !slicesContains(found, command) {
					found = append(found, command)
				}
			}
		}
	}
	return found
}

func slicesContains(values []string, target string) bool {
	return slices.Contains(values, target)
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
			return err
		}
		if plan != nil {
			if err := performRemoval(plan); err != nil {
				return err
			}
		}
	}

	// Update gitignore after removal
	if err := updateGitignoreAfterUninit(targetDir, agentTypesToRemove); err != nil {
		return err
	}

	return nil
}

// updateGitignoreAfterUninit updates the gitignore marker block after uninitializing agents.
// If no agents remain, the marker block is removed entirely.
// Otherwise, the block is rebuilt with only the remaining agents' patterns.
func updateGitignoreAfterUninit(projectDir string, removedAgents []agents.AgentType) error {
	// Check if marker block exists
	if !fsutil.HasGitignoreMarkerBlock(projectDir) {
		return nil
	}

	// Find which agents still have thts installed
	var remainingAgents []agents.AgentType
	for _, agentType := range agents.AllAgentTypes() {
		// Skip agents we just removed
		removed := slices.Contains(removedAgents, agentType)
		if removed {
			continue
		}

		// Check if this agent still has thts files
		cfg := agents.GetConfig(agentType)
		manifestPath := filepath.Join(projectDir, cfg.RootDir, ManifestFile)
		if fsutil.Exists(manifestPath) {
			remainingAgents = append(remainingAgents, agentType)
		}
	}

	if len(remainingAgents) == 0 {
		// No agents remain - remove the entire marker block
		removed, err := fsutil.RemoveGitignoreMarkerBlock(projectDir)
		if err != nil {
			return err
		}
		if len(removed) > 0 {
			fmt.Println(ui.Success("Removed thts patterns from .gitignore"))
		}
		return nil
	}

	// Rebuild the block with remaining agents' patterns
	var patterns []string
	for _, agentType := range remainingAgents {
		patterns = append(patterns, getGitignorePatterns(agentType)...)
	}

	_, err := fsutil.AddGitignoreMarkerBlock(projectDir, patterns)
	if err != nil {
		return err
	}
	fmt.Println(ui.Info("Updated .gitignore patterns for remaining agents"))
	return nil
}

// runGlobalUninit removes globally installed agent components.
func runGlobalUninit(_ *cobra.Command, _ []string) error {
	fmt.Println(ui.Header("Remove Global Agent Components"))
	fmt.Println()

	// Parse requested agents from -a flag
	var requestedAgents []agents.AgentType
	var requestedAgentStrings []string
	if uninitAgents != "" {
		var err error
		requestedAgents, err = agents.ParseAgentTypes(uninitAgents)
		if err != nil {
			return err
		}
		requestedAgentStrings = agents.AgentTypesToStrings(requestedAgents)
		fmt.Printf("%s Filtering to agents: %s\n", ui.Info(""), strings.Join(requestedAgentStrings, ", "))
		fmt.Println()
	}

	// Load global manifest
	manifest, err := LoadGlobalManifest()
	if err != nil {
		return fmt.Errorf("failed to load global manifest: %w", err)
	}
	if manifest == nil || manifest.IsEmpty() {
		fmt.Println(ui.Info("No global installation found."))
		return nil
	}
	originalManifest, err := cloneGlobalManifest(manifest)
	if err != nil {
		return fmt.Errorf("copy global manifest for rollback: %w", err)
	}

	// Filter manifest by requested agents
	filteredComponents := manifest.FilterByAgents(requestedAgentStrings)
	if len(filteredComponents) == 0 {
		if len(requestedAgents) > 0 {
			fmt.Println(ui.Info("No matching global installation found for specified agents."))
		} else {
			fmt.Println(ui.Info("No global installation found."))
		}
		return nil
	}

	// Show what will be removed
	fmt.Println(ui.SubHeader("Files to remove:"))
	for component, info := range filteredComponents {
		fmt.Printf("  %s (%d files for %s):\n", ui.Accent(component), len(info.Files), strings.Join(info.Agents, ", "))
		for _, f := range info.Files {
			fmt.Printf("    %s\n", ui.Muted(config.ContractPath(f)))
		}
	}
	fmt.Println()

	// Handle dry-run
	if uninitDryRun {
		fmt.Println(ui.Info("Dry run complete. No files were removed."))
		return nil
	}

	// Confirm removal
	if !uninitForce {
		var confirmed bool
		err := huh.NewConfirm().
			Title("Remove these global files?").
			Affirmative("Yes, remove").
			Negative("No, cancel").
			Value(&confirmed).
			Run()
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.Info("Cancelled."))
			return nil
		}
	}

	selectedComponentAgents := make(map[string][]string, len(filteredComponents))
	for component, info := range filteredComponents {
		selectedComponentAgents[component] = append([]string(nil), info.Agents...)
	}

	// Remove files from filtered components, with special handling for hooks.
	// Only successfully cleaned paths are removed from the manifest below.
	var removed int
	removedFiles := make(map[string]bool)
	var cleanupErrors []error
	hooksInfo := filteredComponents["hooks"]

	for component, info := range filteredComponents {
		for _, f := range info.Files {
			if component == "hooks" && hooksInfo != nil && isGlobalHookConfigPath(f, info.Agents) {
				// Only remove hooks for the agents we're uninstalling
				agentsToRemove := hooksInfo.Agents
				if len(requestedAgentStrings) > 0 {
					agentsToRemove = intersectStrings(hooksInfo.Agents, requestedAgentStrings)
				}
				removeEmpty := !slices.Contains(info.PreexistingFiles, f)
				if err := removeGlobalHooksFromSettings(f, agentsToRemove, removeEmpty); err != nil {
					fmt.Println(ui.WarningF("  Could not remove hooks from %s: %v", config.ContractPath(f), err))
					cleanupErrors = append(cleanupErrors, fmt.Errorf("remove hooks from %s: %w", f, err))
				} else {
					removedFiles[f] = true
					fmt.Printf("  %s Removed hooks from %s\n", ui.Success(""), config.ContractPath(f))
				}
				continue
			}

			if err := os.Remove(f); err != nil {
				if !os.IsNotExist(err) {
					fmt.Println(ui.WarningF("  Could not remove %s: %v", config.ContractPath(f), err))
					cleanupErrors = append(cleanupErrors, fmt.Errorf("remove %s: %w", f, err))
					continue
				}
				removedFiles[f] = true
			} else {
				removedFiles[f] = true
				removed++
			}
		}
	}
	fmt.Printf("%s Removed %d file(s)\n", ui.Success(""), removed)

	// Clean up empty directories for affected agents
	agentsToClean := requestedAgents
	if len(agentsToClean) == 0 {
		agentsToClean = agents.AllAgentTypes()
	}
	for _, agentType := range agentsToClean {
		globalDir := config.GlobalAgentDir(string(agentType))
		if globalDir == "" {
			continue
		}
		cleanEmptyDirs(globalDir)
	}

	selectedAgents := requestedAgentStrings
	if len(selectedAgents) == 0 {
		for _, agentNames := range selectedComponentAgents {
			selectedAgents = append(selectedAgents, agentNames...)
		}
	}
	removeCleanedGlobalManifestFiles(manifest, selectedAgents, removedFiles)
	if manifest.IsEmpty() {
		if err := DeleteGlobalManifest(); err != nil {
			return fmt.Errorf("remove global manifest: %w", err)
		} else {
			fmt.Println(ui.SuccessF("Removed manifest: %s", config.ContractPath(config.GlobalManifestPath())))
		}
	} else {
		if err := saveGlobalManifestForUninit(manifest); err != nil {
			return fmt.Errorf("save global manifest: %w", err)
		} else {
			fmt.Println(ui.Success("Updated manifest with remaining agents"))
		}
	}

	// Reset only the removed agents' component modes. Other manifest owners
	// remain global and must retain their explicit overrides.
	cfg := config.LoadOrDefault()
	for component, agentNames := range selectedComponentAgents {
		for _, agent := range agentNames {
			if !manifest.HasAgentComponent(agent, component) {
				cfg.SetAgentComponentOverride(agent, component, config.ComponentModeLocal)
			}
		}
		if cfg.GetAgentComponentMode(component) == config.ComponentModeGlobal && !manifest.HasComponent(component) {
			cfg.SetAgentComponentMode(component, config.ComponentModeLocal)
		}
	}
	if err := saveConfigForGlobalUninit(cfg); err != nil {
		configErr := fmt.Errorf("save config: %w", err)
		if restoreErr := SaveGlobalManifest(originalManifest); restoreErr != nil {
			return errors.Join(configErr, fmt.Errorf("restore global manifest ownership: %w", restoreErr))
		}
		return configErr
	} else {
		fmt.Println(ui.Success("Reset config to local mode"))
	}

	if len(cleanupErrors) > 0 {
		return errors.Join(cleanupErrors...)
	}

	fmt.Println()
	fmt.Println(ui.Success("Global uninstallation complete."))

	return nil
}

func cloneGlobalManifest(manifest *GlobalManifest) (*GlobalManifest, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var clone GlobalManifest
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

func isGlobalHookConfigPath(path string, agentNames []string) bool {
	for _, agentName := range agentNames {
		cfg := agents.GetConfig(agents.AgentType(agentName))
		if cfg != nil && filepath.Clean(path) == filepath.Join(config.GlobalAgentDir(agentName), hookConfigFile(cfg, true)) {
			return true
		}
	}
	return false
}

func removeCleanedGlobalManifestFiles(manifest *GlobalManifest, agentsToRemove []string, removedFiles map[string]bool) {
	removeSet := make(map[string]bool, len(agentsToRemove))
	for _, agent := range agentsToRemove {
		removeSet[agent] = true
	}

	for component, info := range manifest.Components {
		var remainingFiles []string
		for _, file := range info.Files {
			if removeSet[getAgentFromPath(file)] && removedFiles[file] {
				continue
			}
			remainingFiles = append(remainingFiles, file)
		}
		info.Files = remainingFiles
		info.PreexistingFiles = slices.DeleteFunc(info.PreexistingFiles, func(path string) bool {
			return !slices.Contains(remainingFiles, path)
		})

		var remainingAgents []string
		for _, agent := range info.Agents {
			if !removeSet[agent] || manifestComponentHasAgentPath(info, agent) {
				remainingAgents = append(remainingAgents, agent)
			}
		}
		info.Agents = remainingAgents
		if len(info.Agents) == 0 {
			delete(manifest.Components, component)
		}
	}
}

func manifestComponentHasAgentPath(info *GlobalComponentInfo, agent string) bool {
	for _, file := range info.Files {
		if getAgentFromPath(file) == agent {
			return true
		}
	}
	return false
}

// intersectStrings returns the intersection of two string slices.
func intersectStrings(a, b []string) []string {
	bSet := make(map[string]bool)
	for _, s := range b {
		bSet[s] = true
	}
	var result []string
	for _, s := range a {
		if bSet[s] {
			result = append(result, s)
		}
	}
	return result
}
