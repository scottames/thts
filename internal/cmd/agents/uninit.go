package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
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
	uninitCmd.Flags().BoolVar(&uninitGlobal, "global", false, "Remove globally installed components")
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

	// Update gitignore - remove marker block or rebuild with remaining agents
	if err := updateGitignoreAfterUninit(targetDir, agentTypes); err != nil {
		fmt.Println(ui.WarningF("Could not update .gitignore: %v", err))
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
	settingsLocalPath := filepath.Join(agentDir, "settings.local.json")
	if fsutil.Exists(settingsLocalPath) {
		data, err := os.ReadFile(settingsLocalPath)
		if err == nil {
			var settings map[string]any
			if json.Unmarshal(data, &settings) == nil {
				if hooks, ok := settings["hooks"].([]any); ok {
					thtsHooks := getThtsHookNames(agentType, false)
					thtsSet := make(map[string]bool)
					for _, h := range thtsHooks {
						thtsSet[h] = true
					}
					var foundHooks []string
					for _, hook := range hooks {
						if hookMap, ok := hook.(map[string]any); ok {
							if cmd, ok := hookMap["command"].(string); ok && thtsSet[cmd] {
								foundHooks = append(foundHooks, cmd)
							}
						}
					}
					if len(foundHooks) > 0 {
						manifest.Modifications.Hooks = &HooksModification{
							SettingsFile: "settings.local.json",
							HookCommands: foundHooks,
						}
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
	} else if manifest.Modifications.InstructionsMD != nil {
		manifest.IntegrationLevel = IntegrationAgentsContent
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
	if plan.modifications.Hooks != nil {
		if err := removeHooksFromSettings(plan.agentDir, plan.modifications.Hooks); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to remove hooks from settings: %v", err))
		} else {
			fmt.Println(ui.Success("Removed thts hooks from settings"))
		}
	}

	// 6. Revert instruction file modification
	if plan.modifications.InstructionsMD != nil {
		if err := removeThtsIntegration(plan.modifications.InstructionsMD, plan.agentType); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to clean instruction file: %v", err))
		} else {
			fmt.Println(ui.Success("Removed thts integration from instruction file"))
		}
	}

	// 7. Remove gitignore patterns
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

	// 8. Remove manifest itself
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

// removeThtsIntegration removes thts integration based on the integration type.
func removeThtsIntegration(mod *InstructionsMDModification, agentType agents.AgentType) error {
	// Dispatch based on integration type
	switch mod.IntegrationType {
	case "marker":
		return removeMarkerBlock(mod.Path)
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

	return os.WriteFile(filePath, []byte(newContent), 0644)
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

	return os.WriteFile(configPath, append(newData, '\n'), 0644)
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

	return os.WriteFile(settingsPath, append(newData, '\n'), 0644)
}

// removeHooksFromSettings removes thts hooks from settings.local.json.
func removeHooksFromSettings(agentDir string, mod *HooksModification) error {
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

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	// Get existing hooks
	hooks, ok := settings["hooks"].([]any)
	if !ok {
		return nil // No hooks to remove
	}

	// Build set of commands to remove
	removeSet := make(map[string]bool)
	for _, cmd := range mod.HookCommands {
		removeSet[cmd] = true
	}

	// Filter out thts hooks
	var newHooks []any
	for _, hook := range hooks {
		hookMap, ok := hook.(map[string]any)
		if !ok {
			newHooks = append(newHooks, hook)
			continue
		}
		cmd, _ := hookMap["command"].(string)
		if !removeSet[cmd] {
			newHooks = append(newHooks, hook)
		}
	}

	// Update or remove hooks key
	if len(newHooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = newHooks
	}

	// Write back (or delete if empty)
	if len(settings) == 0 {
		return os.Remove(settingsPath)
	}

	newData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, append(newData, '\n'), 0644)
}

// removeGlobalHooksFromSettings removes thts hooks from a global settings.local.json file.
// Takes the full path to the settings file and the list of agent names that had hooks installed.
func removeGlobalHooksFromSettings(settingsPath string, agentNames []string) error {
	if !fsutil.Exists(settingsPath) {
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	hooks, ok := settings["hooks"].([]any)
	if !ok {
		return nil // No hooks to remove
	}

	// Build set of thts hook commands to remove (global paths)
	removeSet := make(map[string]bool)
	for _, agentName := range agentNames {
		agentType := agents.AgentType(agentName)
		thtsHooks := getThtsHookNames(agentType, true) // true = global paths
		for _, cmd := range thtsHooks {
			removeSet[cmd] = true
		}
	}

	// Filter out thts hooks
	var newHooks []any
	for _, hook := range hooks {
		hookMap, ok := hook.(map[string]any)
		if !ok {
			newHooks = append(newHooks, hook)
			continue
		}
		cmd, _ := hookMap["command"].(string)
		if !removeSet[cmd] {
			newHooks = append(newHooks, hook)
		}
	}

	// Update or remove hooks key
	if len(newHooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = newHooks
	}

	// Write back (or delete if empty)
	if len(settings) == 0 {
		return os.Remove(settingsPath)
	}

	newData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, append(newData, '\n'), 0644)
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
		removed := false
		for _, ra := range removedAgents {
			if ra == agentType {
				removed = true
				break
			}
		}
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

	// Load global manifest
	manifest, err := LoadGlobalManifest()
	if err != nil {
		return fmt.Errorf("failed to load global manifest: %w", err)
	}
	if manifest == nil || manifest.IsEmpty() {
		fmt.Println(ui.Info("No global installation found."))
		return nil
	}

	// Show what will be removed
	fmt.Println(ui.SubHeader("Files to remove:"))
	for component, info := range manifest.Components {
		fmt.Printf("  %s (%d files for %s):\n", ui.Accent(component), len(info.Files), strings.Join(info.Agents, ", "))
		for _, f := range info.Files {
			fmt.Printf("    %s\n", ui.Muted(config.ContractPath(f)))
		}
	}
	fmt.Println()

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

	// Remove files, with special handling for hooks
	var removed int
	hooksInfo := manifest.Components["hooks"]

	for _, f := range manifest.GetAllFiles() {
		// Special handling for settings.local.json - remove hooks from it rather than deleting
		if hooksInfo != nil && strings.HasSuffix(f, "settings.local.json") {
			if err := removeGlobalHooksFromSettings(f, hooksInfo.Agents); err != nil {
				fmt.Println(ui.WarningF("  Could not remove hooks from %s: %v", config.ContractPath(f), err))
			} else {
				fmt.Printf("  %s Removed hooks from %s\n", ui.Success(""), config.ContractPath(f))
			}
			continue
		}

		if err := os.Remove(f); err != nil {
			if !os.IsNotExist(err) {
				fmt.Println(ui.WarningF("  Could not remove %s: %v", config.ContractPath(f), err))
			}
		} else {
			removed++
		}
	}
	fmt.Printf("%s Removed %d file(s)\n", ui.Success(""), removed)

	// Clean up empty directories
	for _, agentType := range agents.AllAgentTypes() {
		globalDir := config.GlobalAgentDir(string(agentType))
		if globalDir == "" {
			continue
		}
		cleanEmptyDirs(globalDir)
	}

	// Reset config to local
	cfg := config.LoadOrDefault()
	for component := range manifest.Components {
		cfg.SetAgentComponentMode(component, config.ComponentModeLocal)
	}
	if err := config.Save(cfg); err != nil {
		fmt.Println(ui.WarningF("Could not update config: %v", err))
	} else {
		fmt.Println(ui.Success("Reset config to local mode"))
	}

	// Remove manifest
	if err := DeleteGlobalManifest(); err != nil {
		fmt.Println(ui.WarningF("Could not remove manifest: %v", err))
	} else {
		fmt.Println(ui.SuccessF("Removed manifest: %s", config.ContractPath(config.GlobalManifestPath())))
	}

	fmt.Println()
	fmt.Println(ui.Success("Global uninstallation complete."))

	return nil
}
