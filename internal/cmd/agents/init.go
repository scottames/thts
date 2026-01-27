package agents

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
	"github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
	fsutil "github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	initAgents       string
	initForce        bool
	initInteractive  bool
	initWithSettings bool
	initGlobal       string
	initRefresh      bool
	initDryRun       bool
)

// ModelType represents the available Claude models.
type ModelType string

const (
	ModelHaiku  ModelType = "haiku"
	ModelSonnet ModelType = "sonnet"
	ModelOpus   ModelType = "opus"
)

// IntegrationLevel represents how thoughts/ integration is activated.
type IntegrationLevel string

const (
	IntegrationHook               IntegrationLevel = "hook"                 // NEW DEFAULT: Hook-based integration
	IntegrationAgentsContent      IntegrationLevel = "agents-content"       // Marker injection into AGENTS.md
	IntegrationAgentsContentLocal IntegrationLevel = "agents-content-local" // Local file, gitignored
	IntegrationOnDemand           IntegrationLevel = "on-demand"            // Just skills/commands

	// Deprecated aliases for backwards compatibility
	IntegrationAlwaysOn  IntegrationLevel = "always-on"  // Alias for agents-content
	IntegrationLocalOnly IntegrationLevel = "local-only" // Alias for agents-content-local
)

// normalizeIntegrationLevel converts legacy level names to current names.
// Used when reading manifests from older thts versions.
func normalizeIntegrationLevel(level IntegrationLevel) IntegrationLevel {
	switch level {
	case IntegrationAlwaysOn:
		return IntegrationAgentsContent
	case IntegrationLocalOnly:
		return IntegrationAgentsContentLocal
	default:
		return level
	}
}

// ManifestFile is the name of the manifest file that tracks init operations.
const ManifestFile = "thts-manifest.json"

// Marker constants for thts integration blocks.
const (
	// ThtsMarkerStart marks the beginning of thts-managed content.
	ThtsMarkerStart = "<!-- thts-start -->"
	// ThtsMarkerEnd marks the end of thts-managed content.
	ThtsMarkerEnd = "<!-- thts-end -->"
	// ThtsInstructionsFile is the filename for thts instructions.
	ThtsInstructionsFile = "thts-instructions.md"
)

// Manifest tracks files created by agents init for clean uninit.
type Manifest struct {
	Version          int                   `json:"version"`
	CreatedAt        string                `json:"createdAt"`
	Agent            string                `json:"agent"`
	IntegrationLevel IntegrationLevel      `json:"integrationLevel"`
	Files            []string              `json:"files"`
	SettingsCreated  bool                  `json:"settingsCreated,omitempty"`
	Modifications    ManifestModifications `json:"modifications,omitempty"`
}

// ManifestModifications tracks changes to existing files.
type ManifestModifications struct {
	InstructionsMD *InstructionsMDModification `json:"instructionsMD,omitempty"`
	Gitignore      *GitignoreModification      `json:"gitignore,omitempty"`
	Hooks          *HooksModification          `json:"hooks,omitempty"`
}

// InstructionsMDModification tracks changes made to instruction files.
type InstructionsMDModification struct {
	Path            string `json:"path"`
	Action          string `json:"action"`          // "appended", "created"
	IntegrationType string `json:"integrationType"` // "marker" or "config"
	// MarkerBased is true when HTML comment markers were used.
	MarkerBased bool `json:"markerBased,omitempty"`
	// ConfigKey is the config key modified (for config-based integration).
	ConfigKey string `json:"configKey,omitempty"`
	// Pattern is kept for backward compatibility with old manifests.
	Pattern string `json:"pattern,omitempty"`
}

// GitignoreModification tracks patterns added to .gitignore.
type GitignoreModification struct {
	Patterns []string `json:"patterns"`
}

// HooksModification tracks hooks added to settings files.
type HooksModification struct {
	SettingsFile string   `json:"settingsFile"`
	HookCommands []string `json:"hookCommands"`
}

// installationPlan holds information about what would be installed for an agent.
type installationPlan struct {
	agentType        agents.AgentType
	agentDir         string // Local agent dir (e.g., .claude/)
	globalDir        string // Global agent dir (e.g., ~/.claude/)
	projectDir       string
	integrationLevel IntegrationLevel

	// Local files to create (relative paths)
	skillFiles   []string
	commandFiles []string
	agentFiles   []string
	hookFiles    []string
	pluginFiles  []string

	// Global files (relative paths, when component is global)
	globalSkillFiles   []string
	globalCommandFiles []string
	globalAgentFiles   []string

	// Modifications
	settingsFile          string // If --with-settings
	settingsLocalFile     string // For hook integration
	instructionsFile      string // For marker integration
	gitignorePatterns     []string
	hooksSettingsModified bool // Whether hooks config will be added

	localFileCount  int
	globalFileCount int
}

// InitCmd is the command for initializing agent integration.
// It is exported so it can be registered as a subcommand of `thts init`.
var InitCmd = &cobra.Command{
	Use:   "agents",
	Short: "Initialize agent integration for this project",
	Long: `Initialize agent integration by copying thts integration files
to the project's agent-specific directories.

This enables thoughts/ integration with supported agent tools including:
  - /thts-integrate skill (activate integration for current task)
  - /thts-handoff command (create session handoff documents)
  - /thts-resume command (resume from handoff documents)
  - Specialized agents (thoughts-locator, thoughts-analyzer)

Note: Codex uses "prompts" instead of "commands" and they're global-only.

Agent selection priority:
  1. --agents flag if provided
  2. Profile's defaultAgents if configured
  3. Existing agent directories detected in project
  4. Interactive prompt (or error in non-interactive mode)

Integration levels:
  - Hook-based (default): Loads on keyword detection, keeps CLAUDE.md clean
  - Always-on (AGENTS.md): Adds @include to project's AGENTS.md/CLAUDE.md
  - Always-on (local): Creates local instructions file (gitignored)
  - On-demand: Just installs skill/commands for manual invocation

Use --dry-run to preview what files would be created without actually creating them.

Usage: thts init agents [flags]`,
	RunE: runAgentsInit,
}

func init() {
	InitCmd.Flags().StringVarP(&initAgents, "agents", "a", "", "Comma-separated list of agents (claude,codex,opencode)")
	InitCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing files")
	InitCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", false, "Interactively select options")
	InitCmd.Flags().BoolVar(&initWithSettings, "with-settings", false, "Also create settings files")
	InitCmd.Flags().StringVar(&initGlobal, "global", "", "Install components globally (all, or: skills,commands,agents)")
	// NoOptDefVal allows --global without value to trigger interactive mode
	InitCmd.Flags().Lookup("global").NoOptDefVal = "interactive"
	InitCmd.Flags().BoolVar(&initRefresh, "refresh", false, "Update agent files with current config (skip prompt)")
	InitCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Show what would be installed without installing")

	_ = InitCmd.RegisterFlagCompletionFunc("agents", completeAgentTypes)
}

// completeAgentTypes provides shell completion for agent types.
func completeAgentTypes(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return agents.AgentTypesToStrings(agents.AllAgentTypes()), cobra.ShellCompDirectiveNoFileComp
}

// RunInit runs the agents init command programmatically.
// This is exported so that `thts init` can call it after the interactive prompt.
func RunInit(cmd *cobra.Command, args []string) error {
	return runAgentsInit(cmd, args)
}

func runAgentsInit(cmd *cobra.Command, args []string) error {
	// Check if --global flag was provided
	if initGlobal != "" {
		return runGlobalInit(cmd, args)
	}

	fmt.Println(ui.Header("Initialize Agent Integration"))
	fmt.Println()

	if initInteractive && !isTerminal() {
		fmt.Println(ui.Error("--interactive requires a terminal."))
		return nil
	}

	targetDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check for existing agent initializations (via manifests)
	existingAgents := detectExistingAgentManifests(targetDir)

	// Handle --refresh flag or prompt for action if agents exist
	if len(existingAgents) > 0 && !initForce {
		if initRefresh {
			return refreshAgentSetup(targetDir, existingAgents)
		}

		fmt.Println(ui.WarningF("Agent integration already initialized: %s",
			strings.Join(agents.AgentTypesToStrings(existingAgents), ", ")))

		var action string
		err := huh.NewSelect[string]().
			Title("What would you like to do?").
			Options(
				huh.NewOption("Refresh files with current config", "refresh"),
				huh.NewOption("Reinitialize (may prompt for options)", "reinit"),
				huh.NewOption("Cancel", "cancel"),
			).
			Value(&action).
			Run()
		if err != nil {
			return err
		}
		switch action {
		case "refresh":
			return refreshAgentSetup(targetDir, existingAgents)
		case "cancel":
			fmt.Println("Setup cancelled.")
			return nil
		}
		// "reinit" falls through to normal init
	}

	// Resolve which agents to initialize
	agentTypes, err := resolveAgentSelection(targetDir)
	if err != nil {
		return err
	}
	if len(agentTypes) == 0 {
		fmt.Println(ui.Error("No agents selected."))
		return nil
	}

	agents.SortAgentTypes(agentTypes)
	fmt.Printf("%s Initializing: %s\n", ui.Info(""), strings.Join(agents.AgentTypesToStrings(agentTypes), ", "))
	fmt.Println()

	// Select integration level
	integrationLevel, err := selectIntegrationLevel()
	if err != nil {
		return err
	}

	// Build installation plans for all agents
	var allPlans []*installationPlan
	for _, agentType := range agentTypes {
		plan, err := buildInstallationPlan(targetDir, agentType, integrationLevel)
		if err != nil {
			fmt.Println(ui.WarningF("Could not plan %s: %v", agentType, err))
			continue
		}
		allPlans = append(allPlans, plan)
	}

	// Print installation plans
	for _, plan := range allPlans {
		printInstallationPlan(plan)
	}

	// Dry-run exit point
	if initDryRun {
		fmt.Println(ui.Info("Dry run complete. No files were created."))
		return nil
	}

	// Initialize each agent
	for _, agentType := range agentTypes {
		if err := initAgent(targetDir, agentType, integrationLevel); err != nil {
			fmt.Println(ui.ErrorF("Failed to initialize %s: %v", agentType, err))
			continue
		}
	}

	// Add gitignore patterns for all initialized agents
	if err := updateGitignoreForAgents(targetDir, agentTypes); err != nil {
		fmt.Println(ui.WarningF("Could not update .gitignore: %v", err))
	}

	fmt.Println()
	fmt.Println(ui.Success("Agent initialization complete."))

	return nil
}

// resolveAgentSelection determines which agents to initialize.
// Priority: --agents flag > profile defaultAgents > detected agents > interactive
func resolveAgentSelection(projectDir string) ([]agents.AgentType, error) {
	// 1. Check --agents flag
	if initAgents != "" {
		return agents.ParseAgentTypes(initAgents)
	}

	// 2. Check profile's defaultAgents
	cfg, err := config.Load()
	if err == nil {
		profile, _ := cfg.GetDefaultProfile()
		if profile != nil && len(profile.DefaultAgents) > 0 {
			agentTypes, err := agents.StringsToAgentTypes(profile.DefaultAgents)
			if err == nil && len(agentTypes) > 0 {
				fmt.Println(ui.InfoF("Using profile default agents: %s",
					strings.Join(profile.DefaultAgents, ", ")))
				return agentTypes, nil
			}
		}
	}

	// 3. Detect existing agent directories
	detected := agents.DetectExistingAgents(projectDir)
	if len(detected) > 0 {
		fmt.Println(ui.InfoF("Detected existing agents: %s",
			strings.Join(agents.AgentTypesToStrings(detected), ", ")))
		return detected, nil
	}

	// 4. Interactive prompt or error
	if initInteractive {
		return promptAgentSelection()
	}

	return nil, fmt.Errorf("no agents specified. Use --agents flag, configure defaultAgents in profile, or use --interactive")
}

// promptAgentSelection shows an interactive multi-select for agent types.
func promptAgentSelection() ([]agents.AgentType, error) {
	var selected []string

	var options []huh.Option[string]
	for _, at := range agents.AllAgentTypes() {
		label := fmt.Sprintf("%s (%s)", at, agents.AgentTypeLabels[at])
		options = append(options, huh.NewOption(label, string(at)))
	}

	err := huh.NewMultiSelect[string]().
		Title("Which agents would you like to initialize?").
		Options(options...).
		Value(&selected).
		Run()
	if err != nil {
		return nil, err
	}

	return agents.StringsToAgentTypes(selected)
}

// selectIntegrationLevel prompts user to select how thoughts/ integration is activated.
func selectIntegrationLevel() (IntegrationLevel, error) {
	if !initInteractive {
		return IntegrationHook, nil // New default
	}

	var level string
	err := huh.NewSelect[string]().
		Title("How would you like to integrate thoughts/?").
		Options(
			huh.NewOption("Hook-based (recommended) - Loads on keyword detection, keeps CLAUDE.md clean", string(IntegrationHook)),
			huh.NewOption("Always-on (AGENTS.md) - Adds @include to instructions", string(IntegrationAgentsContent)),
			huh.NewOption("Always-on (local only) - Creates local instructions file (gitignored)", string(IntegrationAgentsContentLocal)),
			huh.NewOption("On-demand only - Just installs skill and commands", string(IntegrationOnDemand)),
		).
		Value(&level).
		Run()
	if err != nil {
		return "", err
	}
	return IntegrationLevel(level), nil
}

// buildInstallationPlan computes what would be installed for an agent without writing files.
func buildInstallationPlan(projectDir string, agentType agents.AgentType, level IntegrationLevel) (*installationPlan, error) {
	agentConfig := agents.GetConfig(agentType)
	if agentConfig == nil {
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}

	agentDir := filepath.Join(projectDir, agentConfig.RootDir)
	globalDir := config.GlobalAgentDir(string(agentType))
	cfg := config.LoadOrDefault()

	plan := &installationPlan{
		agentType:        agentType,
		agentDir:         agentDir,
		globalDir:        globalDir,
		projectDir:       projectDir,
		integrationLevel: level,
	}

	// Plan skills (check component mode)
	skillsMode := cfg.GetAgentComponentMode("skills")
	for _, skillName := range thtsfiles.GetAvailableSkills() {
		var relPath string
		if agentConfig.SkillNeedsDir {
			relPath = filepath.Join(agentConfig.SkillsDir, skillName, "SKILL.md")
		} else {
			relPath = filepath.Join(agentConfig.SkillsDir, skillName+".md")
		}
		switch skillsMode {
		case config.ComponentModeGlobal:
			plan.globalSkillFiles = append(plan.globalSkillFiles, relPath)
		case config.ComponentModeLocal:
			plan.skillFiles = append(plan.skillFiles, relPath)
		}
	}

	// Plan commands/prompts (check component mode)
	if agentConfig.SupportsCommands && !agentConfig.CommandsGlobalOnly {
		commandsMode := cfg.GetAgentComponentMode("commands")
		ext := ".md"
		if agentConfig.CommandsFormat == "toml" {
			ext = ".toml"
		}
		for _, cmdName := range thtsfiles.GetAvailableCommands() {
			relPath := filepath.Join(agentConfig.CommandsDir, cmdName+ext)
			switch commandsMode {
			case config.ComponentModeGlobal:
				plan.globalCommandFiles = append(plan.globalCommandFiles, relPath)
			case config.ComponentModeLocal:
				plan.commandFiles = append(plan.commandFiles, relPath)
			}
		}
	}

	// Plan agents (check component mode)
	if agentConfig.AgentsDir != "" {
		agentsMode := cfg.GetAgentComponentMode("agents")
		for _, agentName := range thtsfiles.GetAvailableAgents() {
			relPath := filepath.Join(agentConfig.AgentsDir, agentName+".md")
			switch agentsMode {
			case config.ComponentModeGlobal:
				plan.globalAgentFiles = append(plan.globalAgentFiles, relPath)
			case config.ComponentModeLocal:
				plan.agentFiles = append(plan.agentFiles, relPath)
			}
		}
	}

	// Plan hooks and settings based on integration level
	level = normalizeIntegrationLevel(level)
	if level == IntegrationHook && agentConfig.SupportsHooks {
		// Hook-based integration
		if agentConfig.HooksDir != "" {
			for _, hookName := range thtsfiles.GetAvailableHooks() {
				relPath := filepath.Join(agentConfig.HooksDir, hookName+".sh")
				plan.hookFiles = append(plan.hookFiles, relPath)
			}
			plan.settingsLocalFile = "settings.local.json"
			plan.hooksSettingsModified = true
			plan.gitignorePatterns = append(plan.gitignorePatterns, filepath.Join(agentConfig.RootDir, "settings.local.json"))
		}
		if agentConfig.PluginsDir != "" {
			relPath := filepath.Join(agentConfig.PluginsDir, "thts-integration.ts")
			plan.pluginFiles = append(plan.pluginFiles, relPath)
		}
	} else if level == IntegrationAgentsContent {
		plan.instructionsFile = agentConfig.InstructionTargetFile
	} else if level == IntegrationAgentsContentLocal {
		if agentConfig.Type == agents.AgentClaude {
			plan.instructionsFile = "CLAUDE.local.md"
		} else {
			plan.instructionsFile = "AGENTS.local.md"
		}
		plan.gitignorePatterns = append(plan.gitignorePatterns, filepath.Join(agentConfig.RootDir, plan.instructionsFile))
	}

	// Plan settings file if --with-settings
	if initWithSettings {
		plan.settingsFile = agentConfig.SettingsFile
	}

	// Calculate file counts
	plan.localFileCount = len(plan.skillFiles) + len(plan.commandFiles) + len(plan.agentFiles) +
		len(plan.hookFiles) + len(plan.pluginFiles)
	if plan.settingsFile != "" {
		plan.localFileCount++
	}
	plan.globalFileCount = len(plan.globalSkillFiles) + len(plan.globalCommandFiles) + len(plan.globalAgentFiles)

	return plan, nil
}

// printInstallationPlan displays what would be installed for an agent using a tree structure.
func printInstallationPlan(plan *installationPlan) {
	agentConfig := agents.GetConfig(plan.agentType)
	localDir := agentConfig.RootDir
	globalDir := config.ContractPath(plan.globalDir)

	fmt.Printf("%s (%s)\n", ui.SubHeader(agents.AgentTypeLabels[plan.agentType]), localDir)

	// Build list of sections to determine which is last (for tree rendering)
	type section struct {
		name    string
		global  bool
		files   []string
		isLast  bool
		hasData bool
	}

	cmdLabel := capitalize(agents.CommandsDirLabel(plan.agentType))
	sections := []section{
		{name: "Skills", global: len(plan.globalSkillFiles) > 0, files: plan.skillFiles, hasData: len(plan.skillFiles) > 0 || len(plan.globalSkillFiles) > 0},
		{name: cmdLabel, global: len(plan.globalCommandFiles) > 0, files: plan.commandFiles, hasData: len(plan.commandFiles) > 0 || len(plan.globalCommandFiles) > 0},
		{name: "Agents", global: len(plan.globalAgentFiles) > 0, files: plan.agentFiles, hasData: len(plan.agentFiles) > 0 || len(plan.globalAgentFiles) > 0},
		{name: "Hooks", files: plan.hookFiles, hasData: len(plan.hookFiles) > 0},
		{name: "Plugins", files: plan.pluginFiles, hasData: len(plan.pluginFiles) > 0},
	}

	// Add settings section if applicable
	if plan.settingsFile != "" {
		sections = append(sections, section{name: "Settings", files: []string{plan.settingsFile}, hasData: true})
	}

	// Add modifications section
	hasModifications := plan.hooksSettingsModified || plan.instructionsFile != "" || len(plan.gitignorePatterns) > 0
	if hasModifications {
		sections = append(sections, section{name: "Modifications", hasData: true})
	}

	// Mark the last section with data
	for i := len(sections) - 1; i >= 0; i-- {
		if sections[i].hasData {
			sections[i].isLast = true
			break
		}
	}

	// Print each section
	for _, sec := range sections {
		if !sec.hasData {
			continue
		}

		branch := "├─"
		childPrefix := "│  "
		if sec.isLast {
			branch = "└─"
			childPrefix = "   "
		}

		// Handle special sections
		if sec.name == "Modifications" {
			fmt.Printf("%s Modifications\n", branch)
			printModificationsTree(plan, childPrefix)
			continue
		}

		// Handle global vs local files
		if sec.global {
			var globalFiles []string
			switch sec.name {
			case "Skills":
				globalFiles = plan.globalSkillFiles
			case cmdLabel:
				globalFiles = plan.globalCommandFiles
			case "Agents":
				globalFiles = plan.globalAgentFiles
			}
			fmt.Printf("%s %s %s\n", branch, sec.name, ui.Muted(fmt.Sprintf("(global: %s)", globalDir)))
			printFilesTree(globalFiles, childPrefix)
		} else if len(sec.files) > 0 {
			fmt.Printf("%s %s %s\n", branch, sec.name, ui.Muted("(local)"))
			printFilesTree(sec.files, childPrefix)
		}
	}

	// Print summary
	var parts []string
	if plan.localFileCount > 0 {
		parts = append(parts, fmt.Sprintf("%d local", plan.localFileCount))
	}
	if plan.globalFileCount > 0 {
		parts = append(parts, fmt.Sprintf("%d global", plan.globalFileCount))
	}
	if len(parts) == 0 {
		parts = append(parts, "0 files")
	}
	fmt.Printf("Summary: %s\n\n", strings.Join(parts, ", "))
}

// printFilesTree prints files with tree characters.
func printFilesTree(files []string, prefix string) {
	for i, f := range files {
		branch := "├─"
		if i == len(files)-1 {
			branch = "└─"
		}
		fmt.Printf("%s%s %s\n", prefix, branch, f)
	}
}

// printModificationsTree prints the modifications section with tree characters.
func printModificationsTree(plan *installationPlan, prefix string) {
	var mods []string
	if plan.hooksSettingsModified {
		mods = append(mods, fmt.Sprintf("%s (hooks config)", plan.settingsLocalFile))
	}
	if plan.instructionsFile != "" {
		if plan.integrationLevel == IntegrationAgentsContent {
			mods = append(mods, fmt.Sprintf("%s (append thts block)", plan.instructionsFile))
		} else {
			mods = append(mods, plan.instructionsFile)
		}
	}
	for _, p := range plan.gitignorePatterns {
		mods = append(mods, fmt.Sprintf(".gitignore += %s", p))
	}

	for i, m := range mods {
		branch := "├─"
		if i == len(mods)-1 {
			branch = "└─"
		}
		fmt.Printf("%s%s %s\n", prefix, branch, m)
	}
}

// initAgent initializes a single agent type.
func initAgent(projectDir string, agentType agents.AgentType, level IntegrationLevel) error {
	agentConfig := agents.GetConfig(agentType)
	if agentConfig == nil {
		return fmt.Errorf("unknown agent type: %s", agentType)
	}

	agentDir := filepath.Join(projectDir, agentConfig.RootDir)
	fmt.Println(ui.SubHeader(fmt.Sprintf("Initializing %s:", agents.AgentTypeLabels[agentType])))

	// Create agent directory
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", agentConfig.RootDir, err)
	}

	// Load config to check component modes
	cfg := config.LoadOrDefault()

	manifest := &Manifest{
		Agent:            string(agentType),
		IntegrationLevel: level,
		Files:            []string{},
	}

	var filesCopied int

	// Copy skills (check component mode)
	skillsMode := cfg.GetAgentComponentMode("skills")
	switch skillsMode {
	case config.ComponentModeGlobal:
		fmt.Println(ui.InfoF("  Skills: using global installation"))
	case config.ComponentModeDisabled:
		// Skip silently
	default:
		skillsCopied, skillFiles, err := copySkills(agentDir, agentType, agentConfig)
		if err != nil {
			fmt.Println(ui.WarningF("  Could not copy skills: %v", err))
		} else if skillsCopied > 0 {
			filesCopied += skillsCopied
			for _, f := range skillFiles {
				manifest.Files = append(manifest.Files, filepath.Join(agentConfig.SkillsDir, f))
			}
			fmt.Println(ui.SuccessF("  Copied %d skill(s)", skillsCopied))
		}
	}

	// Copy commands/prompts (check component mode)
	if agentConfig.SupportsCommands {
		cmdLabel := agents.CommandsDirLabel(agentType)
		commandsMode := cfg.GetAgentComponentMode("commands")
		switch commandsMode {
		case config.ComponentModeGlobal:
			fmt.Println(ui.InfoF("  %s: using global installation", capitalize(cmdLabel)))
		case config.ComponentModeDisabled:
			// Skip silently
		default:
			// Warn if Codex prompts are being installed to project (they're global-only)
			if agentConfig.CommandsGlobalOnly {
				fmt.Println(ui.WarningF("  %s %s are global-only. Use --global to install to %s",
					string(agentType), cmdLabel, config.GlobalAgentDir(string(agentType))+"/"+agentConfig.CommandsDir+"/"))
			} else {
				cmdsCopied, cmdFiles, err := copyCommands(agentDir, agentType)
				if err != nil {
					fmt.Println(ui.WarningF("  Could not copy %s: %v", cmdLabel, err))
				} else if cmdsCopied > 0 {
					filesCopied += cmdsCopied
					for _, f := range cmdFiles {
						manifest.Files = append(manifest.Files, filepath.Join(agentConfig.CommandsDir, f))
					}
					fmt.Println(ui.SuccessF("  Copied %d %s", cmdsCopied, cmdLabel))
				}
			}
		}
	}

	// Copy agents (check component mode)
	agentsMode := cfg.GetAgentComponentMode("agents")
	switch agentsMode {
	case config.ComponentModeGlobal:
		fmt.Println(ui.InfoF("  Agents: using global installation"))
	case config.ComponentModeDisabled:
		// Skip silently
	default:
		// Check if agent supports agents feature
		if agentConfig.AgentsDir == "" {
			fmt.Println(ui.InfoF("  Agents: not supported by %s", agents.AgentTypeLabels[agentType]))
		} else {
			agentsCopied, agentFiles, err := copyAgents(agentDir, agentType, agentConfig)
			if err != nil {
				fmt.Println(ui.WarningF("  Could not copy agents: %v", err))
			} else if agentsCopied > 0 {
				filesCopied += agentsCopied
				for _, f := range agentFiles {
					manifest.Files = append(manifest.Files, filepath.Join(agentConfig.AgentsDir, f))
				}
				fmt.Println(ui.SuccessF("  Copied %d agent(s)", agentsCopied))
			}
		}
	}

	// Setup integration level
	// Normalize level for legacy manifests
	level = normalizeIntegrationLevel(level)

	if level == IntegrationHook {
		// Hook-based integration
		if err := setupHookIntegration(projectDir, agentDir, agentType, agentConfig, manifest); err != nil {
			fmt.Println(ui.WarningF("  Could not setup hook integration: %v", err))
		}
	} else {
		// Traditional integration (markers or config)
		instMod, gitignorePatterns, err := setupIntegrationLevel(projectDir, agentDir, agentConfig, level)
		if err != nil {
			fmt.Println(ui.WarningF("  Could not setup integration: %v", err))
		} else {
			if instMod != nil {
				manifest.Modifications.InstructionsMD = instMod
			}
			if len(gitignorePatterns) > 0 {
				manifest.Modifications.Gitignore = &GitignoreModification{Patterns: gitignorePatterns}
			}
		}
	}

	// Handle settings if --with-settings flag is set
	if initWithSettings {
		if err := writeAgentSettings(agentDir, agentType); err != nil {
			fmt.Println(ui.WarningF("  Could not write settings: %v", err))
		} else {
			manifest.SettingsCreated = true
			manifest.Files = append(manifest.Files, agents.GetConfig(agentType).SettingsFile)
			fmt.Println(ui.SuccessF("  Created %s", agents.GetConfig(agentType).SettingsFile))
		}
	}

	// Write manifest
	if err := writeManifest(agentDir, manifest); err != nil {
		fmt.Println(ui.WarningF("  Could not write manifest: %v", err))
	}

	fmt.Println(ui.SuccessF("  Initialized with %d file(s)", filesCopied))
	return nil
}

// readThtsInstructions returns the rendered thts-instructions.md content with current config.
func readThtsInstructions() ([]byte, error) {
	cfg := config.LoadOrDefault()
	data := buildInstructionsData(cfg)
	content, err := thtsfiles.GetInstructions(data)
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

// adjustHeaderLevels increments all markdown headers by the given offset.
// Skips content inside fenced code blocks to avoid modifying markdown examples.
func adjustHeaderLevels(content string, offset int) string {
	if offset <= 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	prefix := strings.Repeat("#", offset)
	inCodeBlock := false

	for i, line := range lines {
		// Track fenced code blocks (``` or ~~~)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Only adjust headers outside code blocks
		if !inCodeBlock && strings.HasPrefix(line, "#") {
			lines[i] = prefix + line
		}
	}

	return strings.Join(lines, "\n")
}

// symlinkTargetType indicates what a symlink points to.
type symlinkTargetType int

const (
	symlinkNone            symlinkTargetType = iota // Not a symlink
	symlinkToLocalAgentsMD                          // Symlink to AGENTS.md in same directory
	symlinkToElsewhere                              // Symlink to something else
)

// checkClaudeMDSymlink checks if CLAUDE.md is a symlink and what it points to.
func checkClaudeMDSymlink(gitRoot string) symlinkTargetType {
	claudePath := filepath.Join(gitRoot, "CLAUDE.md")

	// Check if file exists and is a symlink
	info, err := os.Lstat(claudePath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return symlinkNone
	}

	// Resolve the symlink target
	target, err := filepath.EvalSymlinks(claudePath)
	if err != nil {
		return symlinkToElsewhere // Can't resolve, treat as elsewhere
	}

	// Check if target is AGENTS.md in the same directory
	agentsPath := filepath.Join(gitRoot, "AGENTS.md")
	absAgents, err := filepath.Abs(agentsPath)
	if err != nil {
		return symlinkToElsewhere
	}

	if target == absAgents {
		return symlinkToLocalAgentsMD
	}

	return symlinkToElsewhere
}

// copySkills copies skill files for an agent type using templates.
func copySkills(agentDir string, agentType agents.AgentType, cfg *agents.AgentConfig) (int, []string, error) {
	skillsDir := filepath.Join(agentDir, cfg.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return 0, nil, err
	}

	var copied int
	var copiedPaths []string

	for _, skillName := range thtsfiles.GetAvailableSkills() {
		content, err := thtsfiles.RenderSkill(agentType, skillName)
		if err != nil {
			return copied, copiedPaths, fmt.Errorf("failed to render skill %s: %w", skillName, err)
		}

		var targetPath string
		var relPath string
		if cfg.SkillNeedsDir {
			// Codex/OpenCode/Gemini: skills/skill-name/SKILL.md
			skillDir := filepath.Join(skillsDir, skillName)
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				return copied, copiedPaths, err
			}
			targetPath = filepath.Join(skillDir, "SKILL.md")
			relPath = filepath.Join(skillName, "SKILL.md")
		} else {
			// Claude: skills/skill-name.md (flat)
			targetPath = filepath.Join(skillsDir, skillName+".md")
			relPath = skillName + ".md"
		}

		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return copied, copiedPaths, err
		}
		copied++
		copiedPaths = append(copiedPaths, relPath)
	}

	return copied, copiedPaths, nil
}

// copyFlatFilesWithExt copies files with a specific extension from an embedded FS to a target directory.
// This is used for plugins which are not templated.
func copyFlatFilesWithExt(embedFS fs.FS, srcDir, targetDir, ext string) (int, []string, error) {
	entries, err := fs.ReadDir(embedFS, srcDir)
	if err != nil {
		return 0, nil, err
	}

	var copied int
	var copiedFiles []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ext) {
			continue
		}
		content, err := fs.ReadFile(embedFS, filepath.Join(srcDir, entry.Name()))
		if err != nil {
			return copied, copiedFiles, err
		}
		targetPath := filepath.Join(targetDir, entry.Name())
		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return copied, copiedFiles, err
		}
		copied++
		copiedFiles = append(copiedFiles, entry.Name())
	}

	return copied, copiedFiles, nil
}

// copyCommands copies command/prompt files for an agent type using templates.
func copyCommands(agentDir string, agentType agents.AgentType) (int, []string, error) {
	agentConfig := agents.GetConfig(agentType)
	if agentConfig == nil || !agentConfig.SupportsCommands {
		return 0, nil, nil
	}

	commandsDir := filepath.Join(agentDir, agentConfig.CommandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return 0, nil, err
	}

	// Determine file extension based on agent's command format
	ext := ".md"
	if agentConfig.CommandsFormat == "toml" {
		ext = ".toml"
	}

	var copied int
	var copiedPaths []string

	for _, cmdName := range thtsfiles.GetAvailableCommands() {
		content, err := thtsfiles.RenderCommand(agentType, cmdName)
		if err != nil {
			return copied, copiedPaths, fmt.Errorf("failed to render command %s: %w", cmdName, err)
		}

		filename := cmdName + ext
		targetPath := filepath.Join(commandsDir, filename)
		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return copied, copiedPaths, err
		}
		copied++
		copiedPaths = append(copiedPaths, filename)
	}

	return copied, copiedPaths, nil
}

// copyAgents copies agent files for an agent type using templates.
// Returns 0, nil, nil if the agent doesn't support the agents feature (AgentsDir is empty).
func copyAgents(agentDir string, agentType agents.AgentType, cfg *agents.AgentConfig) (int, []string, error) {
	// Skip if agent doesn't support agents (e.g., Gemini)
	if cfg.AgentsDir == "" {
		return 0, nil, nil
	}

	agentsTargetDir := filepath.Join(agentDir, cfg.AgentsDir)
	if err := os.MkdirAll(agentsTargetDir, 0755); err != nil {
		return 0, nil, err
	}

	var copied int
	var copiedPaths []string

	for _, agentName := range thtsfiles.GetAvailableAgents() {
		content, err := thtsfiles.RenderAgent(agentType, agentName)
		if err != nil {
			return copied, copiedPaths, fmt.Errorf("failed to render agent %s: %w", agentName, err)
		}

		filename := agentName + ".md"
		targetPath := filepath.Join(agentsTargetDir, filename)
		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return copied, copiedPaths, err
		}
		copied++
		copiedPaths = append(copiedPaths, filename)
	}

	return copied, copiedPaths, nil
}

// getPluginsFS returns the embedded FS for plugins for an agent type.
// Returns nil for agents that don't support plugins.
func getPluginsFS(agentType agents.AgentType) fs.FS {
	switch agentType {
	case agents.AgentOpenCode:
		return thtsfiles.OpenCodePlugins
	default:
		return nil
	}
}

// copyHooks copies hook scripts for an agent type using templates.
// Returns 0, nil, nil if the agent doesn't support hooks.
func copyHooks(agentDir string, agentType agents.AgentType, cfg *agents.AgentConfig) (int, []string, error) {
	if cfg.HooksDir == "" {
		return 0, nil, nil
	}

	hooksDir := filepath.Join(agentDir, cfg.HooksDir)
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return 0, nil, err
	}

	var copied int
	var copiedPaths []string

	for _, hookName := range thtsfiles.GetAvailableHooks() {
		content, err := thtsfiles.RenderHook(agentType, hookName)
		if err != nil {
			return copied, copiedPaths, fmt.Errorf("failed to render hook %s: %w", hookName, err)
		}

		filename := hookName + ".sh"
		targetPath := filepath.Join(hooksDir, filename)
		if err := os.WriteFile(targetPath, []byte(content), 0755); err != nil {
			return copied, copiedPaths, err
		}
		copied++
		copiedPaths = append(copiedPaths, filename)
	}

	return copied, copiedPaths, nil
}

// copyPlugins copies plugin files for an agent type.
// Returns 0, nil, nil if the agent doesn't support plugins.
func copyPlugins(agentDir string, agentType agents.AgentType, cfg *agents.AgentConfig) (int, []string, error) {
	if cfg.PluginsDir == "" {
		return 0, nil, nil
	}

	pluginsDir := filepath.Join(agentDir, cfg.PluginsDir)
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return 0, nil, err
	}

	embedFS := getPluginsFS(agentType)
	if embedFS == nil {
		return 0, nil, nil
	}

	srcDir := fmt.Sprintf("embedded/plugins/%s", agentType)
	return copyFlatFilesWithExt(embedFS, srcDir, pluginsDir, ".ts")
}

// makeHooksExecutable sets executable permission on hook scripts.
func makeHooksExecutable(agentDir string, cfg *agents.AgentConfig, files []string) error {
	if cfg.HooksDir == "" {
		return nil
	}
	for _, f := range files {
		path := filepath.Join(agentDir, cfg.HooksDir, f)
		if err := os.Chmod(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

// ensureSettingsContextKey ensures the agent's settings file has the context key configured.
// For agents like Gemini, this sets "contextFileName": "AGENTS.md" in settings.json.
// This function is non-destructive: it preserves any existing configuration.
func ensureSettingsContextKey(agentDir string, cfg *agents.AgentConfig) error {
	if cfg.SettingsContextKey == "" {
		return nil // Agent doesn't need this
	}

	settingsPath := filepath.Join(agentDir, cfg.SettingsFile)
	contextValue := cfg.InstructionTargetFile

	var settings map[string]any

	if fsutil.Exists(settingsPath) {
		data, err := os.ReadFile(settingsPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", cfg.SettingsFile, err)
		}
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("failed to parse %s: %w", cfg.SettingsFile, err)
		}
	} else {
		settings = make(map[string]any)
	}

	// Check if already set correctly
	if existing, ok := settings[cfg.SettingsContextKey].(string); ok && existing == contextValue {
		return nil // Already configured
	}

	// Set the context key
	settings[cfg.SettingsContextKey] = contextValue

	// Write back with proper formatting
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", cfg.SettingsFile, err)
	}

	fmt.Println(ui.SuccessF("  Set %s in %s", cfg.SettingsContextKey, cfg.SettingsFile))
	return nil
}

// setupIntegrationLevel configures the integration based on the selected level.
func setupIntegrationLevel(projectDir, agentDir string, cfg *agents.AgentConfig, level IntegrationLevel) (*InstructionsMDModification, []string, error) {
	var gitignorePatterns []string

	// Normalize legacy level names
	level = normalizeIntegrationLevel(level)

	switch level {
	case IntegrationHook:
		// Hook mode is handled separately in initAgent via setupHookIntegration
		return nil, nil, fmt.Errorf("hook integration should be set up via setupHookIntegration")

	case IntegrationAgentsContent:
		gitRoot, err := git.GetRepoTopLevelAt(projectDir)
		if err != nil {
			gitRoot = projectDir
		}

		// Choose integration strategy based on agent type
		switch cfg.IntegrationType {
		case "marker":
			mod, err := appendWithMarkers(gitRoot, agentDir, cfg)
			if err != nil {
				return mod, nil, err
			}
			// Ensure settings context key is set (for agents like Gemini)
			if cfg.SettingsContextKey != "" {
				if err := ensureSettingsContextKey(agentDir, cfg); err != nil {
					fmt.Println(ui.WarningF("  Could not update settings: %v", err))
				}
			}
			return mod, nil, nil
		case "config":
			mod, err := updateOpenCodeConfig(projectDir, agentDir, cfg)
			return mod, nil, err
		default:
			return nil, nil, fmt.Errorf("unknown integration type: %s", cfg.IntegrationType)
		}

	case IntegrationAgentsContentLocal:
		var localFile string
		if cfg.Type == agents.AgentClaude {
			// For Claude, use CLAUDE.local.md
			localFile = "CLAUDE.local.md"
		} else {
			localFile = "AGENTS.local.md"
		}
		if err := createLocalInstructionsMD(agentDir, localFile, cfg); err != nil {
			return nil, nil, err
		}
		pattern := filepath.Join(cfg.RootDir, localFile)
		added, err := fsutil.AddToGitignore(projectDir, pattern, "project")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update .gitignore: %w", err)
		}
		if added {
			gitignorePatterns = append(gitignorePatterns, pattern)
			fmt.Println(ui.InfoF("  Updated .gitignore to exclude %s", localFile))
		}
		fmt.Println(ui.SuccessF("  Created %s with @include", localFile))
		return nil, gitignorePatterns, nil

	case IntegrationOnDemand:
		fmt.Println(ui.Info("  On-demand mode: use /thts-integrate to activate"))
		return nil, nil, nil
	}
	return nil, nil, nil
}

// appendWithMarkers appends thts integration with HTML comment markers.
// Always uses inline mode - full content is embedded directly in the target file.
func appendWithMarkers(gitRoot, agentDir string, cfg *agents.AgentConfig) (*InstructionsMDModification, error) {
	if cfg.InstructionTargetFile == "" {
		return nil, nil // No target file
	}

	targetFile := cfg.InstructionTargetFile

	// For Claude, check if CLAUDE.md is a symlink
	if cfg.Type == agents.AgentClaude {
		switch checkClaudeMDSymlink(gitRoot) {
		case symlinkToLocalAgentsMD:
			// CLAUDE.md -> AGENTS.md: target AGENTS.md instead
			fmt.Println(ui.InfoF("  CLAUDE.md is symlinked to AGENTS.md, targeting AGENTS.md"))
			targetFile = "AGENTS.md"
		case symlinkToElsewhere:
			// CLAUDE.md -> elsewhere: warn and skip
			fmt.Println(ui.WarningF("  CLAUDE.md is symlinked elsewhere, skipping instruction integration"))
			fmt.Println(ui.Warning("  To integrate, either remove the symlink or manually add thts instructions"))
			return nil, nil
		}
	}

	filePath := filepath.Join(gitRoot, targetFile)

	// Build inline content - always embed full instructions
	thtsContent, err := readThtsInstructions()
	if err != nil {
		return nil, fmt.Errorf("failed to read instructions: %w", err)
	}
	insertContent := fmt.Sprintf("\n%s\n%s\n%s\n",
		ThtsMarkerStart,
		string(thtsContent),
		ThtsMarkerEnd)

	// Check if already integrated
	if fsutil.Exists(filePath) {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", targetFile, err)
		}
		if strings.Contains(string(content), ThtsMarkerStart) {
			if !initForce {
				fmt.Println(ui.InfoF("  %s already includes thts integration", targetFile))
				return nil, nil
			}
			// Force mode: remove existing marker block and re-add
			fmt.Println(ui.InfoF("  Replacing existing thts integration in %s", targetFile))
			if err := removeMarkerBlock(filePath); err != nil {
				return nil, fmt.Errorf("failed to remove existing markers: %w", err)
			}
		}
		// Append to existing file
		// Adjust header levels to fit under existing structure
		appendContent := adjustHeaderLevels(insertContent, 1)
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", targetFile, err)
		}
		if _, err := f.WriteString(appendContent); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("failed to append to %s: %w", targetFile, err)
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close %s: %w", targetFile, err)
		}
		fmt.Println(ui.SuccessF("  Appended thts integration to %s", targetFile))
		return &InstructionsMDModification{
			Path:            filePath,
			Action:          "appended",
			IntegrationType: "marker",
			MarkerBased:     true,
		}, nil
	}

	// Create new file
	header := "# Agent Instructions\n"
	// Adjust header levels to fit under top-level header
	createContent := adjustHeaderLevels(insertContent, 1)
	if err := os.WriteFile(filePath, []byte(header+createContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", targetFile, err)
	}
	fmt.Println(ui.SuccessF("  Created %s with thts integration", targetFile))
	return &InstructionsMDModification{
		Path:            filePath,
		Action:          "created",
		IntegrationType: "marker",
		MarkerBased:     true,
	}, nil
}

// updateOpenCodeConfig adds thts-instructions.md to the instructions array in opencode.json.
func updateOpenCodeConfig(projectDir, agentDir string, cfg *agents.AgentConfig) (*InstructionsMDModification, error) {
	configPath := filepath.Join(projectDir, cfg.SettingsFile)
	instructionPath := fmt.Sprintf("%s/%s", cfg.RootDir, ThtsInstructionsFile)

	var config map[string]any

	if fsutil.Exists(configPath) {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", cfg.SettingsFile, err)
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", cfg.SettingsFile, err)
		}
	} else {
		// Create minimal config
		config = make(map[string]any)
	}

	// Get or create instructions array
	var instructions []any
	if existing, ok := config["instructions"].([]any); ok {
		instructions = existing
	}

	// Check if already present
	for _, inst := range instructions {
		if instStr, ok := inst.(string); ok && instStr == instructionPath {
			fmt.Println(ui.InfoF("  %s already in instructions array", instructionPath))
			return nil, nil
		}
	}

	// Append to instructions
	instructions = append(instructions, instructionPath)
	config["instructions"] = instructions

	// Write back
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0644); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", cfg.SettingsFile, err)
	}

	action := "appended"
	if len(instructions) == 1 {
		action = "created"
	}
	fmt.Println(ui.SuccessF("  Added thts instructions to %s", cfg.SettingsFile))
	return &InstructionsMDModification{
		Path:            configPath,
		Action:          action,
		IntegrationType: "config",
		ConfigKey:       "instructions",
	}, nil
}

// createLocalInstructionsMD creates a local instructions file with the @include directive.
func createLocalInstructionsMD(agentDir, localFile string, cfg *agents.AgentConfig) error {
	localPath := filepath.Join(agentDir, localFile)
	content := fmt.Sprintf("# Local Agent Instructions\n\n@%s\n", ThtsInstructionsFile)
	return os.WriteFile(localPath, []byte(content), 0644)
}

// writeAgentSettings writes the settings file for an agent.
func writeAgentSettings(agentDir string, agentType agents.AgentType) error {
	cfg := agents.GetConfig(agentType)
	if cfg == nil {
		return fmt.Errorf("unknown agent type: %s", agentType)
	}

	var content string
	switch agentType {
	case agents.AgentClaude:
		// Claude settings are built dynamically with user input
		content = buildClaudeSettings()
	default:
		// Other agents use embedded settings files
		content = thtsfiles.GetDefaultSettings(cfg.SettingsFile)
		if content == "" {
			return fmt.Errorf("no default settings for agent: %s", agentType)
		}
	}

	settingsPath := filepath.Join(agentDir, cfg.SettingsFile)
	return os.WriteFile(settingsPath, []byte(content), 0644)
}

// buildClaudeSettings builds the settings.json for Claude with interactive prompts if needed.
func buildClaudeSettings() string {
	alwaysThinking := true
	maxThinkingTokens := 32000
	model := ModelOpus

	if initInteractive {
		// Interactive prompts
		_ = huh.NewConfirm().
			Title("Enable always-on thinking mode for Claude Code?").
			Affirmative("Yes").
			Negative("No").
			Value(&alwaysThinking).
			Run()

		var tokensStr string
		_ = huh.NewInput().
			Title("Maximum thinking tokens:").
			Value(&tokensStr).
			Placeholder("32000").
			Validate(func(s string) error {
				if s == "" {
					return nil
				}
				n, err := strconv.Atoi(s)
				if err != nil || n < 1000 {
					return fmt.Errorf("please enter a valid number (minimum 1000)")
				}
				return nil
			}).
			Run()
		if tokensStr != "" {
			maxThinkingTokens, _ = strconv.Atoi(tokensStr)
		}

		var modelStr string
		_ = huh.NewSelect[string]().
			Title("Select default model:").
			Options(
				huh.NewOption("Opus (most capable)", string(ModelOpus)),
				huh.NewOption("Sonnet (balanced)", string(ModelSonnet)),
				huh.NewOption("Haiku (fastest)", string(ModelHaiku)),
			).
			Value(&modelStr).
			Run()
		model = ModelType(modelStr)
	}

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

	content, _ := json.MarshalIndent(settings, "", "  ")
	return string(content) + "\n"
}

// mergeHooksIntoSettings adds hook configuration to the appropriate settings file without clobbering existing config.
// For project-level: uses settings.local.json (personal, gitignored)
// For global-level: uses the main settings file (e.g., settings.json) since there's no .local variant globally
// Returns the path to the settings file and whether it was created/modified.
func mergeHooksIntoSettings(agentDir string, agentType agents.AgentType, cfg *agents.AgentConfig, isGlobal bool) (string, bool, error) {
	if cfg.SettingsFormat == "toml" {
		return "", false, fmt.Errorf("hook integration not supported for TOML settings (agent: %s)", agentType)
	}

	// Determine settings file based on scope
	// - Global: use main settings file (no .local variant exists at global level)
	// - Project: use settings.local.json (personal config, gitignored)
	var settingsFile string
	if isGlobal {
		settingsFile = cfg.SettingsFile
	} else {
		settingsFile = "settings.local.json"
	}

	settingsPath := filepath.Join(agentDir, settingsFile)

	// Load existing settings or create new
	var settings map[string]any
	if fsutil.Exists(settingsPath) {
		data, err := os.ReadFile(settingsPath)
		if err != nil {
			return "", false, fmt.Errorf("failed to read %s: %w", settingsFile, err)
		}
		if err := json.Unmarshal(data, &settings); err != nil {
			return "", false, fmt.Errorf("failed to parse %s: %w", settingsFile, err)
		}
	} else {
		settings = make(map[string]any)
	}

	// Build hooks configuration based on agent type
	hooksConfig := buildHooksConfig(agentType, cfg, isGlobal)
	if hooksConfig == nil {
		return "", false, nil // Agent doesn't support hooks
	}

	// Merge hooks into settings (hooks is a map with event names as keys)
	existingHooks, hasHooks := settings["hooks"].(map[string]any)
	if !hasHooks {
		existingHooks = make(map[string]any)
	}

	// Get the event names that thts uses
	thtsEventNames := getThtsHookEventNames(agentType)

	// Remove existing thts hooks, then merge in new ones
	mergedHooks := filterOutThtsHooksFromMap(existingHooks, thtsEventNames, getThtsHookNames(agentType, isGlobal))
	for event, newHooks := range hooksConfig {
		newHooksList, ok := newHooks.([]any)
		if !ok {
			mergedHooks[event] = newHooks
			continue
		}
		// Append thts hooks to existing hooks for this event (preserves user's custom hooks)
		if existingEventHooks, exists := mergedHooks[event].([]any); exists {
			mergedHooks[event] = append(existingEventHooks, newHooksList...)
		} else {
			mergedHooks[event] = newHooks
		}
	}
	settings["hooks"] = mergedHooks

	// Gemini requires explicit hook enabling at both tools and hooks level
	// See: https://geminicli.com/docs/hooks/
	if agentType == agents.AgentGemini {
		// Enable hooks at the tools level
		tools, _ := settings["tools"].(map[string]any)
		if tools == nil {
			tools = make(map[string]any)
		}
		tools["enableHooks"] = true
		settings["tools"] = tools

		// Enable hooks at the hooks level (merge with existing hooks map)
		if hooksMap, ok := settings["hooks"].(map[string]any); ok {
			hooksMap["enabled"] = true
		}
	}

	// Write back with proper formatting
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", false, fmt.Errorf("failed to marshal settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, append(data, '\n'), 0644); err != nil {
		return "", false, fmt.Errorf("failed to write %s: %w", settingsFile, err)
	}

	return settingsPath, true, nil
}

// buildHooksConfig returns the hooks configuration for an agent type.
// If isGlobal is true, uses absolute paths; otherwise uses relative paths.
// Returns a map with event names as keys (Claude Code's new hooks format).
func buildHooksConfig(agentType agents.AgentType, cfg *agents.AgentConfig, isGlobal bool) map[string]any {
	if !cfg.SupportsHooks || cfg.HooksDir == "" {
		return nil
	}

	// Build the path prefix based on global vs project scope
	var pathPrefix string
	if isGlobal {
		// Global: use absolute path to global agent directory
		pathPrefix = filepath.Join(config.GlobalAgentDir(string(agentType)), cfg.HooksDir)
	} else {
		// Project: use relative path from project root
		pathPrefix = fmt.Sprintf("./%s/%s", cfg.RootDir, cfg.HooksDir)
	}

	// Helper to create a hook entry in the new format
	makeHookEntry := func(command string) []any {
		return []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": command,
					},
				},
			},
		}
	}

	switch agentType {
	case agents.AgentClaude:
		return map[string]any{
			"SessionStart":     makeHookEntry(filepath.Join(pathPrefix, "thts-session-start.sh")),
			"UserPromptSubmit": makeHookEntry(filepath.Join(pathPrefix, "thts-prompt-check.sh")),
		}
	case agents.AgentGemini:
		return map[string]any{
			"SessionStart": makeHookEntry(filepath.Join(pathPrefix, "thts-session-start.sh")),
			"BeforeAgent":  makeHookEntry(filepath.Join(pathPrefix, "thts-prompt-check.sh")),
		}
	default:
		return nil
	}
}

// getThtsHookNames returns the command patterns for thts hooks.
// If isGlobal is true, returns absolute paths; otherwise returns relative paths.
func getThtsHookNames(agentType agents.AgentType, isGlobal bool) []string {
	cfg := agents.GetConfig(agentType)
	if cfg == nil || cfg.HooksDir == "" {
		return nil
	}

	var pathPrefix string
	if isGlobal {
		pathPrefix = filepath.Join(config.GlobalAgentDir(string(agentType)), cfg.HooksDir)
	} else {
		pathPrefix = fmt.Sprintf("./%s/%s", cfg.RootDir, cfg.HooksDir)
	}

	return []string{
		filepath.Join(pathPrefix, "thts-session-start.sh"),
		filepath.Join(pathPrefix, "thts-prompt-check.sh"),
	}
}

// getThtsHookEventNames returns the event names that thts hooks register for.
func getThtsHookEventNames(agentType agents.AgentType) []string {
	switch agentType {
	case agents.AgentClaude:
		return []string{"SessionStart", "UserPromptSubmit"}
	case agents.AgentGemini:
		return []string{"SessionStart", "BeforeAgent"}
	default:
		return nil
	}
}

// filterOutThtsHooksFromMap removes thts hooks from a hooks map (new format).
// It filters out thts commands from the specified events, removing events entirely if empty.
func filterOutThtsHooksFromMap(hooks map[string]any, thtsEvents, thtsCommands []string) map[string]any {
	result := make(map[string]any)
	thtsCommandSet := make(map[string]bool)
	for _, cmd := range thtsCommands {
		thtsCommandSet[cmd] = true
	}

	for event, eventHooks := range hooks {
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

			// Check if this entry has thts commands
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
				if !thtsCommandSet[cmd] {
					filteredInner = append(filteredInner, inner)
				}
			}

			if len(filteredInner) > 0 {
				newEntry := make(map[string]any)
				for k, v := range entryMap {
					newEntry[k] = v
				}
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

// setupHookIntegration sets up hook-based integration for an agent.
// Returns files created and any error.
func setupHookIntegration(projectDir, agentDir string, agentType agents.AgentType, cfg *agents.AgentConfig, manifest *Manifest) error {
	// Check if agent supports hooks
	if !cfg.SupportsHooks {
		// Fall back to agents-content with warning
		fmt.Println(ui.WarningF("  %s does not support hooks, falling back to agents-content mode", agents.AgentTypeLabels[agentType]))
		_, _, err := setupIntegrationLevel(projectDir, agentDir, cfg, IntegrationAgentsContent)
		if err != nil {
			return err
		}
		manifest.IntegrationLevel = IntegrationAgentsContent
		return nil
	}

	// Copy hook scripts or plugins
	if cfg.HooksDir != "" {
		copied, files, err := copyHooks(agentDir, agentType, cfg)
		if err != nil {
			return fmt.Errorf("failed to copy hooks: %w", err)
		}
		if copied > 0 {
			// Make hooks executable
			if err := makeHooksExecutable(agentDir, cfg, files); err != nil {
				return fmt.Errorf("failed to make hooks executable: %w", err)
			}
			for _, f := range files {
				manifest.Files = append(manifest.Files, filepath.Join(cfg.HooksDir, f))
			}
			fmt.Println(ui.SuccessF("  Copied %d hook script(s)", copied))
		}
	}

	if cfg.PluginsDir != "" {
		copied, files, err := copyPlugins(agentDir, agentType, cfg)
		if err != nil {
			return fmt.Errorf("failed to copy plugins: %w", err)
		}
		if copied > 0 {
			for _, f := range files {
				manifest.Files = append(manifest.Files, filepath.Join(cfg.PluginsDir, f))
			}
			fmt.Println(ui.SuccessF("  Copied %d plugin(s)", copied))
		}
	}

	// Merge hooks into settings.local.json (Claude/Gemini only)
	if cfg.HooksDir != "" {
		settingsPath, modified, err := mergeHooksIntoSettings(agentDir, agentType, cfg, false)
		if err != nil {
			return fmt.Errorf("failed to configure hooks in settings: %w", err)
		}
		if modified {
			manifest.Files = append(manifest.Files, filepath.Base(settingsPath))
			fmt.Println(ui.SuccessF("  Configured hooks in %s", filepath.Base(settingsPath)))
		}
	}

	// Add gitignore pattern for settings.local.json
	pattern := filepath.Join(cfg.RootDir, "settings.local.json")
	added, err := fsutil.AddToGitignore(projectDir, pattern, "project")
	if err != nil {
		fmt.Println(ui.WarningF("  Could not update .gitignore: %v", err))
	} else if added {
		if manifest.Modifications.Gitignore == nil {
			manifest.Modifications.Gitignore = &GitignoreModification{}
		}
		manifest.Modifications.Gitignore.Patterns = append(manifest.Modifications.Gitignore.Patterns, pattern)
		fmt.Println(ui.InfoF("  Updated .gitignore to exclude %s", filepath.Base(pattern)))
	}

	// Track hooks in manifest for uninit
	if cfg.HooksDir != "" {
		manifest.Modifications.Hooks = &HooksModification{
			SettingsFile: "settings.local.json",
			HookCommands: getThtsHookNames(agentType, false),
		}
	}

	fmt.Println(ui.Info("  Hook mode: instructions load on keyword detection"))
	return nil
}

// installGlobalHooks installs hook scripts to the global agent directory.
// Returns the list of installed files as full paths.
func installGlobalHooks(globalDir string, agentType agents.AgentType, cfg *agents.AgentConfig) ([]string, error) {
	var installedFiles []string

	// Copy hook scripts (Claude/Gemini)
	if cfg.HooksDir != "" {
		_, hookFiles, err := copyHooks(globalDir, agentType, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to copy hooks: %w", err)
		}
		if len(hookFiles) > 0 {
			// Make hooks executable
			if err := makeHooksExecutable(globalDir, cfg, hookFiles); err != nil {
				return nil, fmt.Errorf("failed to make hooks executable: %w", err)
			}
			// Convert to full paths
			for _, f := range hookFiles {
				installedFiles = append(installedFiles, filepath.Join(globalDir, cfg.HooksDir, f))
			}
		}
	}

	// Copy plugins (OpenCode)
	if cfg.PluginsDir != "" {
		_, pluginFiles, err := copyPlugins(globalDir, agentType, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to copy plugins: %w", err)
		}
		for _, f := range pluginFiles {
			installedFiles = append(installedFiles, filepath.Join(globalDir, cfg.PluginsDir, f))
		}
	}

	// Merge hooks into global settings.local.json
	if cfg.HooksDir != "" {
		settingsPath, modified, err := mergeHooksIntoSettings(globalDir, agentType, cfg, true)
		if err != nil {
			return nil, fmt.Errorf("failed to configure hooks in settings: %w", err)
		}
		if modified {
			installedFiles = append(installedFiles, settingsPath)
		}
	}

	return installedFiles, nil
}

// writeManifest writes the manifest file tracking init operations.
func writeManifest(agentDir string, manifest *Manifest) error {
	manifest.Version = 1
	manifest.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(agentDir, ManifestFile)
	return os.WriteFile(manifestPath, append(data, '\n'), 0644)
}

// isTerminal checks if stdin is a terminal.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// updateGitignoreForAgents adds gitignore patterns for thts-managed files.
// This collects patterns from ALL agents with manifests (not just the ones being initialized)
// to ensure the gitignore block is complete.
func updateGitignoreForAgents(projectDir string, _ []agents.AgentType) error {
	var allPatterns []string

	// Collect patterns from all agents that have manifests installed
	for _, agentType := range agents.AllAgentTypes() {
		cfg := agents.GetConfig(agentType)
		manifestPath := filepath.Join(projectDir, cfg.RootDir, ManifestFile)
		if fsutil.Exists(manifestPath) {
			patterns := getGitignorePatterns(agentType)
			allPatterns = append(allPatterns, patterns...)
		}
	}

	if len(allPatterns) == 0 {
		return nil
	}

	// Check if block already exists with same patterns
	existingPatterns := fsutil.GetGitignoreMarkerPatterns(projectDir)
	if patternsEqual(existingPatterns, allPatterns) {
		fmt.Println(ui.Info("Gitignore patterns already up to date"))
		return nil
	}

	added, err := fsutil.AddGitignoreMarkerBlock(projectDir, allPatterns)
	if err != nil {
		return err
	}

	if len(added) > 0 {
		fmt.Println()
		fmt.Println(ui.Success("Updated .gitignore with thts patterns:"))
		for _, p := range added {
			fmt.Printf("  %s\n", ui.Muted(p))
		}
	}

	return nil
}

// getGitignorePatterns returns the gitignore patterns for a specific agent type.
// Uses wildcard patterns for cleaner, more maintainable gitignore entries.
func getGitignorePatterns(agentType agents.AgentType) []string {
	cfg := agents.GetConfig(agentType)
	if cfg == nil {
		return nil
	}

	// Use wildcards for clean patterns:
	// - thts-* matches thts-manifest.json, thts-instructions.md in root
	// - */thts-* matches skills/thts-integrate*, commands/thts-*, hooks/thts-*, plugins/thts-*
	// - */thoughts-* matches agents/thoughts-*
	// - settings.local.json for hook configuration
	patterns := []string{
		cfg.RootDir + "/thts-*",
		cfg.RootDir + "/*/thts-*",
		cfg.RootDir + "/*/thoughts-*",
		cfg.RootDir + "/settings.local.json",
	}

	return patterns
}

// patternsEqual checks if two pattern slices are equal.
func patternsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, p := range a {
		aMap[p] = true
	}
	for _, p := range b {
		if !aMap[p] {
			return false
		}
	}
	return true
}

// detectExistingAgentManifests returns agent types that have manifests in the project.
func detectExistingAgentManifests(projectDir string) []agents.AgentType {
	var existing []agents.AgentType
	for _, agentType := range agents.AllAgentTypes() {
		cfg := agents.GetConfig(agentType)
		manifestPath := filepath.Join(projectDir, cfg.RootDir, ManifestFile)
		if fsutil.Exists(manifestPath) {
			existing = append(existing, agentType)
		}
	}
	return existing
}

// refreshAgentSetup updates agent files with current config for existing agents.
// This re-copies skills, commands, agents, and updates instructions without
// prompting for integration level (preserves existing level from manifest).
func refreshAgentSetup(projectDir string, agentTypes []agents.AgentType) error {
	fmt.Println(ui.Header("Refreshing Agent Integration"))
	fmt.Println()

	cfg := config.LoadOrDefault()

	for _, agentType := range agentTypes {
		agentConfig := agents.GetConfig(agentType)
		agentDir := filepath.Join(projectDir, agentConfig.RootDir)

		// Load existing manifest to get integration level
		manifest, err := loadManifest(agentDir)
		if err != nil {
			fmt.Println(ui.WarningF("Could not read manifest for %s: %v", agentType, err))
			continue
		}

		fmt.Println(ui.SubHeader(fmt.Sprintf("Refreshing %s:", agents.AgentTypeLabels[agentType])))

		var filesUpdated int

		// Re-copy skills (check component mode)
		skillsMode := cfg.GetAgentComponentMode("skills")
		if skillsMode == config.ComponentModeLocal {
			copied, _, err := copySkills(agentDir, agentType, agentConfig)
			if err != nil {
				fmt.Println(ui.WarningF("  Could not update skills: %v", err))
			} else if copied > 0 {
				filesUpdated += copied
				fmt.Println(ui.SuccessF("  Updated %d skill(s)", copied))
			}
		}

		// Re-copy commands (check component mode)
		if agentConfig.SupportsCommands && !agentConfig.CommandsGlobalOnly {
			commandsMode := cfg.GetAgentComponentMode("commands")
			if commandsMode == config.ComponentModeLocal {
				copied, _, err := copyCommands(agentDir, agentType)
				if err != nil {
					fmt.Println(ui.WarningF("  Could not update commands: %v", err))
				} else if copied > 0 {
					filesUpdated += copied
					fmt.Println(ui.SuccessF("  Updated %d command(s)", copied))
				}
			}
		}

		// Re-copy agents (check component mode)
		if agentConfig.AgentsDir != "" {
			agentsMode := cfg.GetAgentComponentMode("agents")
			if agentsMode == config.ComponentModeLocal {
				copied, _, err := copyAgents(agentDir, agentType, agentConfig)
				if err != nil {
					fmt.Println(ui.WarningF("  Could not update agents: %v", err))
				} else if copied > 0 {
					filesUpdated += copied
					fmt.Println(ui.SuccessF("  Updated %d agent(s)", copied))
				}
			}
		}

		// Update integration based on level
		level := normalizeIntegrationLevel(manifest.IntegrationLevel)
		switch level {
		case IntegrationHook:
			// Refresh hook scripts
			if agentConfig.HooksDir != "" {
				copied, files, err := copyHooks(agentDir, agentType, agentConfig)
				if err != nil {
					fmt.Println(ui.WarningF("  Could not update hooks: %v", err))
				} else if copied > 0 {
					if err := makeHooksExecutable(agentDir, agentConfig, files); err != nil {
						fmt.Println(ui.WarningF("  Could not make hooks executable: %v", err))
					}
					filesUpdated += copied
					fmt.Println(ui.SuccessF("  Updated %d hook script(s)", copied))
				}
			}
			// Refresh plugins
			if agentConfig.PluginsDir != "" {
				copied, _, err := copyPlugins(agentDir, agentType, agentConfig)
				if err != nil {
					fmt.Println(ui.WarningF("  Could not update plugins: %v", err))
				} else if copied > 0 {
					filesUpdated += copied
					fmt.Println(ui.SuccessF("  Updated %d plugin(s)", copied))
				}
			}
		case IntegrationAgentsContent:
			// Refresh marker block content
			if err := refreshIntegration(projectDir, agentDir, agentConfig, cfg); err != nil {
				fmt.Println(ui.WarningF("  Could not update integration: %v", err))
			}
		}

		// Update manifest timestamp
		manifest.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := writeManifest(agentDir, manifest); err != nil {
			fmt.Println(ui.WarningF("  Could not update manifest: %v", err))
		}

		fmt.Println(ui.SuccessF("  Refreshed %d file(s)", filesUpdated))
	}

	fmt.Println()
	fmt.Println(ui.Success("Refresh complete."))
	return nil
}

// buildInstructionsData creates InstructionsData from config.
// This is a simplified version that uses the thts package's full implementation.
func buildInstructionsData(cfg *config.Config) thtsfiles.InstructionsData {
	categories := cfg.GetCategories()
	var rows []thtsfiles.CategoryRow

	for name, cat := range categories {
		rows = append(rows, thtsfiles.CategoryRow{
			Name:        name,
			Description: cat.Description,
			Location:    buildLocationString(name, cat.GetScope()),
			Trigger:     cat.Trigger,
			Template:    cat.Template,
		})

		for subName, subCat := range cat.SubCategories {
			fullPath := fmt.Sprintf("%s/%s", name, subName)
			rows = append(rows, thtsfiles.CategoryRow{
				Name:        fullPath,
				Description: subCat.Description,
				Location:    buildLocationString(fullPath, subCat.GetScope(cat.GetScope())),
				Trigger:     subCat.Trigger,
				Template:    subCat.Template,
			})
		}
	}

	return thtsfiles.InstructionsData{
		User:       cfg.User,
		Categories: rows,
	}
}

// buildLocationString creates the location string based on scope.
func buildLocationString(path string, scope config.CategoryScope) string {
	switch scope {
	case config.CategoryScopeUser:
		return fmt.Sprintf("`thoughts/{user}/%s/`", path)
	case config.CategoryScopeBoth:
		return fmt.Sprintf("`thoughts/shared/%s/` or `thoughts/{user}/%s/`", path, path)
	default:
		return fmt.Sprintf("`thoughts/shared/%s/`", path)
	}
}

// refreshIntegration updates the integration for always-on mode.
// For marker-based integration, it removes and re-adds the marker block with current config.
// Always uses inline mode - full content is embedded directly.
func refreshIntegration(projectDir, agentDir string, agentCfg *agents.AgentConfig, cfg *config.Config) error {
	if agentCfg.IntegrationType != "marker" {
		// Config-based integration (OpenCode) doesn't need refresh for instructions
		return nil
	}

	gitRoot, err := git.GetRepoTopLevelAt(projectDir)
	if err != nil {
		gitRoot = projectDir
	}

	targetFile := agentCfg.InstructionTargetFile

	// For Claude, check if CLAUDE.md is a symlink
	if agentCfg.Type == agents.AgentClaude {
		switch checkClaudeMDSymlink(gitRoot) {
		case symlinkToLocalAgentsMD:
			targetFile = "AGENTS.md"
		case symlinkToElsewhere:
			return nil // Skip - symlink elsewhere
		}
	}

	filePath := filepath.Join(gitRoot, targetFile)
	if !fsutil.Exists(filePath) {
		return nil // File doesn't exist, nothing to refresh
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Check if markers exist
	if !strings.Contains(string(content), ThtsMarkerStart) {
		return nil // No markers, nothing to refresh
	}

	// Remove existing marker block
	if err := removeMarkerBlock(filePath); err != nil {
		return fmt.Errorf("failed to remove existing markers: %w", err)
	}

	// Build new inline content with current config
	data := buildInstructionsData(cfg)
	rendered, err := thtsfiles.GetInstructions(data)
	if err != nil {
		return fmt.Errorf("failed to render instructions: %w", err)
	}
	insertContent := fmt.Sprintf("\n%s\n%s\n%s\n",
		ThtsMarkerStart,
		rendered,
		ThtsMarkerEnd)
	insertContent = adjustHeaderLevels(insertContent, 1)

	// Re-read file after marker removal
	updatedContent, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Append new content
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	if _, err := f.WriteString(string(updatedContent) + insertContent); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	fmt.Println(ui.SuccessF("  Updated integration in %s", targetFile))
	return nil
}

// globalInstallationPlan holds information about what would be installed globally.
type globalInstallationPlan struct {
	component  string
	agentPlans map[agents.AgentType]*globalAgentPlan // agent -> plan
	totalFiles int
}

// globalAgentPlan holds file info for a single agent in global install.
type globalAgentPlan struct {
	globalDir string   // e.g., ~/.claude
	files     []string // relative paths within globalDir
}

// buildGlobalInstallationPlans computes what would be installed globally.
func buildGlobalInstallationPlans(components []string, agentTypes []agents.AgentType) []*globalInstallationPlan {
	var plans []*globalInstallationPlan

	for _, component := range components {
		plan := &globalInstallationPlan{
			component:  component,
			agentPlans: make(map[agents.AgentType]*globalAgentPlan),
		}

		for _, agentType := range agentTypes {
			globalDir := config.GlobalAgentDir(string(agentType))
			if globalDir == "" {
				continue
			}

			agentCfg := agents.GetConfig(agentType)
			if agentCfg == nil {
				continue
			}

			var files []string
			switch component {
			case "skills":
				for _, skillName := range thtsfiles.GetAvailableSkills() {
					var relPath string
					if agentCfg.SkillNeedsDir {
						relPath = filepath.Join(agentCfg.SkillsDir, skillName, "SKILL.md")
					} else {
						relPath = filepath.Join(agentCfg.SkillsDir, skillName+".md")
					}
					files = append(files, relPath)
				}
			case "commands":
				if !agentCfg.SupportsCommands {
					continue
				}
				ext := ".md"
				if agentCfg.CommandsFormat == "toml" {
					ext = ".toml"
				}
				for _, cmdName := range thtsfiles.GetAvailableCommands() {
					relPath := filepath.Join(agentCfg.CommandsDir, cmdName+ext)
					files = append(files, relPath)
				}
			case "agents":
				if agentCfg.AgentsDir == "" {
					continue
				}
				for _, agentName := range thtsfiles.GetAvailableAgents() {
					relPath := filepath.Join(agentCfg.AgentsDir, agentName+".md")
					files = append(files, relPath)
				}
			case "hooks":
				if !agentCfg.SupportsHooks {
					continue
				}
				if agentCfg.HooksDir != "" {
					for _, hookName := range thtsfiles.GetAvailableHooks() {
						relPath := filepath.Join(agentCfg.HooksDir, hookName+".sh")
						files = append(files, relPath)
					}
					// Settings file for hooks config
					files = append(files, agentCfg.SettingsFile)
				}
				if agentCfg.PluginsDir != "" {
					relPath := filepath.Join(agentCfg.PluginsDir, "thts-integration.ts")
					files = append(files, relPath)
				}
			}

			if len(files) > 0 {
				plan.agentPlans[agentType] = &globalAgentPlan{
					globalDir: globalDir,
					files:     files,
				}
				plan.totalFiles += len(files)
			}
		}

		if plan.totalFiles > 0 {
			plans = append(plans, plan)
		}
	}

	return plans
}

// printGlobalInstallationPlan displays what would be installed globally using tree structure.
func printGlobalInstallationPlan(plan *globalInstallationPlan) {
	fmt.Printf("%s\n", ui.SubHeader(capitalize(plan.component)))

	// Get sorted agent types for consistent output
	agentTypes := make([]agents.AgentType, 0, len(plan.agentPlans))
	for agentType := range plan.agentPlans {
		agentTypes = append(agentTypes, agentType)
	}
	agents.SortAgentTypes(agentTypes)

	for i, agentType := range agentTypes {
		agentPlan := plan.agentPlans[agentType]
		isLast := i == len(agentTypes)-1

		branch := "├─"
		childPrefix := "│  "
		if isLast {
			branch = "└─"
			childPrefix = "   "
		}

		globalDirDisplay := config.ContractPath(agentPlan.globalDir)
		fmt.Printf("%s %s (%s)\n", branch, agents.AgentTypeLabels[agentType], ui.Muted(globalDirDisplay))
		printFilesTree(agentPlan.files, childPrefix)
	}

	fmt.Printf("Total: %d file(s)\n\n", plan.totalFiles)
}

// runGlobalInit handles global installation of agent components.
func runGlobalInit(_ *cobra.Command, _ []string) error {
	fmt.Println(ui.Header("Initialize Global Agent Components"))
	fmt.Println()

	// Parse components from flag value
	components, err := parseGlobalComponents(initGlobal)
	if err != nil {
		return err
	}

	if len(components) == 0 {
		fmt.Println(ui.Error("No components selected."))
		return nil
	}

	fmt.Printf("%s Installing globally: %s\n", ui.Info(""), strings.Join(components, ", "))
	fmt.Println()

	// Resolve which agents to use (from profile's defaultAgents or all)
	agentTypes, err := resolveGlobalAgentSelection()
	if err != nil {
		return err
	}

	fmt.Printf("%s For agents: %s\n", ui.Info(""), strings.Join(agents.AgentTypesToStrings(agentTypes), ", "))
	fmt.Println()

	// Build and print installation plans
	plans := buildGlobalInstallationPlans(components, agentTypes)
	for _, plan := range plans {
		printGlobalInstallationPlan(plan)
	}

	// Dry-run exit point
	if initDryRun {
		fmt.Println(ui.Info("Dry run complete. No files were created."))
		return nil
	}

	// Load or create global manifest
	manifest, err := LoadGlobalManifest()
	if err != nil {
		return fmt.Errorf("failed to load global manifest: %w", err)
	}
	if manifest == nil {
		manifest = NewGlobalManifest()
	}

	// Install each component globally
	for _, component := range components {
		if err := installGlobalComponent(component, agentTypes, manifest); err != nil {
			fmt.Println(ui.ErrorF("Failed to install %s globally: %v", component, err))
			continue
		}
	}

	// Save manifest
	if err := SaveGlobalManifest(manifest); err != nil {
		return fmt.Errorf("failed to save global manifest: %w", err)
	}

	// Update config to mark components as global
	cfg := config.LoadOrDefault()

	for _, component := range components {
		cfg.SetAgentComponentMode(component, config.ComponentModeGlobal)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Println(ui.Success("Global installation complete."))
	fmt.Println()
	fmt.Println(ui.InfoF("Config updated: %s", config.ContractPath(config.ThtsConfigPath())))
	fmt.Println(ui.InfoF("Manifest saved: %s", config.ContractPath(config.GlobalManifestPath())))

	return nil
}

// parseGlobalComponents parses the --global flag value into component names.
func parseGlobalComponents(value string) ([]string, error) {
	if value == "interactive" {
		// In dry-run mode, default to all components instead of prompting
		if initDryRun {
			return []string{"skills", "commands", "agents", "hooks"}, nil
		}
		return promptGlobalComponentSelection()
	}

	if value == "all" {
		return []string{"skills", "commands", "agents", "hooks"}, nil
	}

	// Parse comma-separated list
	var components []string
	validComponents := map[string]bool{
		"skills":   true,
		"commands": true,
		"agents":   true,
		"hooks":    true,
	}

	for _, c := range strings.Split(value, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if !validComponents[c] {
			return nil, fmt.Errorf("invalid component: %s (valid: skills, commands, agents, hooks)", c)
		}
		components = append(components, c)
	}

	return components, nil
}

// promptGlobalComponentSelection shows an interactive multi-select for components.
func promptGlobalComponentSelection() ([]string, error) {
	if !isTerminal() {
		return nil, fmt.Errorf("interactive mode requires a terminal. Specify components: --global all or --global skills,commands,agents,hooks")
	}

	var selected []string
	options := []huh.Option[string]{
		huh.NewOption("Skills (thts-integrate)", "skills"),
		huh.NewOption("Commands/Prompts (thts-handoff, thts-resume)", "commands"),
		huh.NewOption("Agents (thoughts-locator, thoughts-analyzer)", "agents"),
		huh.NewOption("Hooks (keyword detection for thoughts/ loading)", "hooks"),
	}

	err := huh.NewMultiSelect[string]().
		Title("Which components should be installed globally?").
		Options(options...).
		Value(&selected).
		Run()
	if err != nil {
		return nil, err
	}

	return selected, nil
}

// resolveGlobalAgentSelection determines which agents to install globally.
// Uses profile's defaultAgents or falls back to all agents.
func resolveGlobalAgentSelection() ([]agents.AgentType, error) {
	// Check profile's defaultAgents
	cfg, err := config.Load()
	if err == nil {
		profile, _ := cfg.GetDefaultProfile()
		if profile != nil && len(profile.DefaultAgents) > 0 {
			agentTypes, err := agents.StringsToAgentTypes(profile.DefaultAgents)
			if err == nil && len(agentTypes) > 0 {
				return agentTypes, nil
			}
		}
	}

	// Fall back to all agents
	return agents.AllAgentTypes(), nil
}

// installGlobalComponent installs a component to global directories for all specified agents.
func installGlobalComponent(component string, agentTypes []agents.AgentType, manifest *GlobalManifest) error {
	var allFiles []string
	var agentNames []string

	for _, agentType := range agentTypes {
		globalDir := config.GlobalAgentDir(string(agentType))
		if globalDir == "" {
			continue
		}

		agentCfg := agents.GetConfig(agentType)
		if agentCfg == nil {
			continue
		}

		var files []string
		var err error

		switch component {
		case "skills":
			_, files, err = copySkills(globalDir, agentType, agentCfg)
		case "commands":
			if agentCfg.SupportsCommands {
				_, files, err = copyCommands(globalDir, agentType)
			} else {
				continue // agent doesn't support commands
			}
		case "agents":
			_, files, err = copyAgents(globalDir, agentType, agentCfg)
		case "hooks":
			if !agentCfg.SupportsHooks {
				fmt.Printf("  %s hooks: %s does not support hooks\n", ui.Warning(""), agents.AgentTypeLabels[agentType])
				continue
			}
			files, err = installGlobalHooks(globalDir, agentType, agentCfg)
		default:
			return fmt.Errorf("unknown component: %s", component)
		}

		if err != nil {
			return fmt.Errorf("failed to copy %s for %s: %w", component, agentType, err)
		}

		// Skip if no files were copied (e.g., Gemini doesn't support agents)
		if len(files) == 0 {
			continue
		}

		// Convert relative file names to absolute paths
		for _, f := range files {
			var fullPath string
			switch component {
			case "skills":
				fullPath = filepath.Join(globalDir, agentCfg.SkillsDir, f)
			case "commands":
				fullPath = filepath.Join(globalDir, agentCfg.CommandsDir, f)
			case "agents":
				fullPath = filepath.Join(globalDir, agentCfg.AgentsDir, f)
			case "hooks":
				// Hooks files are already full paths from installGlobalHooks
				fullPath = f
			}
			allFiles = append(allFiles, fullPath)
		}

		agentNames = append(agentNames, string(agentType))
		fmt.Printf("  %s %s: installed to %s\n", ui.Success(""), component, config.ContractPath(globalDir))
	}

	// Update manifest
	manifest.AddComponent(component, &GlobalComponentInfo{
		Agents: agentNames,
		Files:  allFiles,
	})

	return nil
}

// capitalize returns the string with the first letter capitalized.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
