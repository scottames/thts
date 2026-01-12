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
	IntegrationAlwaysOn  IntegrationLevel = "always-on"
	IntegrationLocalOnly IntegrationLevel = "local-only"
	IntegrationOnDemand  IntegrationLevel = "on-demand"
)

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

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize agent integration for this project",
	Long: `Initialize agent integration by copying thts integration files
to the project's agent-specific directories.

This enables thoughts/ integration with supported agent tools including:
  - /thts-integrate skill (activate integration for current task)
  - /thts-handoff command (create session handoff documents) [Claude only]
  - /thts-resume command (resume from handoff documents) [Claude only]
  - Specialized agents (thoughts-locator, thoughts-analyzer)

Agent selection priority:
  1. --agents flag if provided
  2. Profile's defaultAgents if configured
  3. Existing agent directories detected in project
  4. Interactive prompt (or error in non-interactive mode)

Integration levels:
  - Always-on (AGENTS.md): Adds @include to project's AGENTS.md/CLAUDE.md
  - Always-on (local): Creates local instructions file (gitignored)
  - On-demand: Just installs skill/commands for manual invocation`,
	RunE: runAgentsInit,
}

func init() {
	initCmd.Flags().StringVarP(&initAgents, "agents", "a", "", "Comma-separated list of agents (claude,codex,opencode)")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing files")
	initCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", false, "Interactively select options")
	initCmd.Flags().BoolVar(&initWithSettings, "with-settings", false, "Also create settings files")
	initCmd.Flags().StringVar(&initGlobal, "global", "", "Install components globally (all, or: skills,commands,agents)")
	// NoOptDefVal allows --global without value to trigger interactive mode
	initCmd.Flags().Lookup("global").NoOptDefVal = "interactive"
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
		return IntegrationAlwaysOn, nil
	}

	var level string
	err := huh.NewSelect[string]().
		Title("How would you like to integrate thoughts/?").
		Options(
			huh.NewOption("Always-on (add to AGENTS.md/CLAUDE.md) - Adds @include to instructions", string(IntegrationAlwaysOn)),
			huh.NewOption("Always-on (local only) - Creates local instructions file (gitignored)", string(IntegrationLocalOnly)),
			huh.NewOption("On-demand only - Just installs skill and commands", string(IntegrationOnDemand)),
		).
		Value(&level).
		Run()
	if err != nil {
		return "", err
	}
	return IntegrationLevel(level), nil
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

	// Copy thts-instructions.md (shared instructions)
	// Skip if: agent uses inline AGENTS.md, OR Claude with CLAUDE.md symlinked to AGENTS.md
	copyInstructions := agentConfig.InstructionsFile != ""
	if copyInstructions && agentConfig.Type == agents.AgentClaude {
		if checkClaudeMDSymlink(projectDir) == symlinkToLocalAgentsMD {
			copyInstructions = false // Will use inline mode instead
		}
	}
	if copyInstructions {
		if err := copyInstructionsFile(agentDir, agentConfig); err != nil {
			fmt.Println(ui.WarningF("  Could not copy instructions: %v", err))
		} else {
			filesCopied++
			manifest.Files = append(manifest.Files, ThtsInstructionsFile)
			fmt.Println(ui.SuccessF("  Copied %s", ThtsInstructionsFile))
		}
	}

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

	// Copy commands (Claude only, check component mode)
	if agentConfig.SupportsCommands {
		commandsMode := cfg.GetAgentComponentMode("commands")
		switch commandsMode {
		case config.ComponentModeGlobal:
			fmt.Println(ui.InfoF("  Commands: using global installation"))
		case config.ComponentModeDisabled:
			// Skip silently
		default:
			cmdsCopied, cmdFiles, err := copyCommands(agentDir, agentType)
			if err != nil {
				fmt.Println(ui.WarningF("  Could not copy commands: %v", err))
			} else if cmdsCopied > 0 {
				filesCopied += cmdsCopied
				for _, f := range cmdFiles {
					manifest.Files = append(manifest.Files, filepath.Join("commands", f))
				}
				fmt.Println(ui.SuccessF("  Copied %d command(s)", cmdsCopied))
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

	// Setup integration level
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

// copyInstructionsFile copies thts-instructions.md to the agent directory.
func copyInstructionsFile(agentDir string, cfg *agents.AgentConfig) error {
	content, err := fs.ReadFile(thtsfiles.Instructions, "instructions/thts-instructions.md")
	if err != nil {
		return fmt.Errorf("failed to read thts-instructions.md: %w", err)
	}
	targetPath := filepath.Join(agentDir, ThtsInstructionsFile)
	return os.WriteFile(targetPath, content, 0644)
}

// readThtsInstructions reads the embedded thts-instructions.md content.
func readThtsInstructions() ([]byte, error) {
	return fs.ReadFile(thtsfiles.Instructions, "instructions/thts-instructions.md")
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

// copySkills copies skill files for an agent type.
func copySkills(agentDir string, agentType agents.AgentType, cfg *agents.AgentConfig) (int, []string, error) {
	skillsDir := filepath.Join(agentDir, cfg.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return 0, nil, err
	}

	embedFS := getSkillsFS(agentType)
	if embedFS == nil {
		return 0, nil, nil
	}

	srcDir := fmt.Sprintf("skills/%s", agentType)

	if cfg.SkillNeedsDir {
		// Codex/OpenCode: skills/agent/skill-name/SKILL.md
		return copySkillsWithSubdirs(embedFS, srcDir, skillsDir)
	}

	// Claude: skills/agent/skill-name.md (flat)
	return copyFlatFiles(embedFS, srcDir, skillsDir)
}

// copySkillsWithSubdirs copies skills that require subdirectories (Codex/OpenCode format).
func copySkillsWithSubdirs(embedFS fs.FS, srcDir, targetDir string) (int, []string, error) {
	entries, err := fs.ReadDir(embedFS, srcDir)
	if err != nil {
		return 0, nil, err
	}

	var copied int
	var copiedPaths []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		skillFile := filepath.Join(srcDir, skillName, "SKILL.md")
		content, err := fs.ReadFile(embedFS, skillFile)
		if err != nil {
			continue
		}

		targetSkillDir := filepath.Join(targetDir, skillName)
		if err := os.MkdirAll(targetSkillDir, 0755); err != nil {
			return copied, copiedPaths, err
		}

		targetPath := filepath.Join(targetSkillDir, "SKILL.md")
		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return copied, copiedPaths, err
		}
		copied++
		copiedPaths = append(copiedPaths, filepath.Join(skillName, "SKILL.md"))
	}

	return copied, copiedPaths, nil
}

// copyFlatFiles copies markdown files from an embedded FS to a target directory.
func copyFlatFiles(embedFS fs.FS, srcDir, targetDir string) (int, []string, error) {
	entries, err := fs.ReadDir(embedFS, srcDir)
	if err != nil {
		return 0, nil, err
	}

	var copied int
	var copiedFiles []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
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

// copyCommands copies command files (Claude only).
func copyCommands(agentDir string, agentType agents.AgentType) (int, []string, error) {
	if agentType != agents.AgentClaude {
		return 0, nil, nil
	}

	commandsDir := filepath.Join(agentDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return 0, nil, err
	}

	return copyFlatFiles(thtsfiles.ClaudeCommands, "commands/claude", commandsDir)
}

// copyAgents copies agent files for an agent type.
func copyAgents(agentDir string, agentType agents.AgentType, cfg *agents.AgentConfig) (int, []string, error) {
	agentsTargetDir := filepath.Join(agentDir, cfg.AgentsDir)
	if err := os.MkdirAll(agentsTargetDir, 0755); err != nil {
		return 0, nil, err
	}

	embedFS := getAgentsFS(agentType)
	if embedFS == nil {
		return 0, nil, nil
	}

	srcDir := fmt.Sprintf("agents/%s", agentType)
	return copyFlatFiles(embedFS, srcDir, agentsTargetDir)
}

// getSkillsFS returns the embedded FS for skills for an agent type.
func getSkillsFS(agentType agents.AgentType) fs.FS {
	switch agentType {
	case agents.AgentClaude:
		return thtsfiles.ClaudeSkills
	case agents.AgentCodex:
		return thtsfiles.CodexSkills
	case agents.AgentOpenCode:
		return thtsfiles.OpenCodeSkills
	default:
		return nil
	}
}

// getAgentsFS returns the embedded FS for agents for an agent type.
func getAgentsFS(agentType agents.AgentType) fs.FS {
	switch agentType {
	case agents.AgentClaude:
		return thtsfiles.ClaudeAgents
	case agents.AgentCodex:
		return thtsfiles.CodexAgents
	case agents.AgentOpenCode:
		return thtsfiles.OpenCodeAgents
	default:
		return nil
	}
}

// setupIntegrationLevel configures the integration based on the selected level.
func setupIntegrationLevel(projectDir, agentDir string, cfg *agents.AgentConfig, level IntegrationLevel) (*InstructionsMDModification, []string, error) {
	var gitignorePatterns []string

	switch level {
	case IntegrationAlwaysOn:
		gitRoot, err := git.GetRepoTopLevelAt(projectDir)
		if err != nil {
			gitRoot = projectDir
		}

		// Choose integration strategy based on agent type
		switch cfg.IntegrationType {
		case "marker":
			mod, err := appendWithMarkers(gitRoot, agentDir, cfg)
			return mod, nil, err
		case "config":
			mod, err := updateOpenCodeConfig(projectDir, agentDir, cfg)
			return mod, nil, err
		default:
			return nil, nil, fmt.Errorf("unknown integration type: %s", cfg.IntegrationType)
		}

	case IntegrationLocalOnly:
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
// For Claude: adds @include directive wrapped in markers (unless CLAUDE.md is symlinked).
// For Codex/OpenCode: adds full content inline wrapped in markers.
func appendWithMarkers(gitRoot, agentDir string, cfg *agents.AgentConfig) (*InstructionsMDModification, error) {
	if cfg.InstructionTargetFile == "" {
		return nil, nil // No target file
	}

	// For Claude, check if CLAUDE.md is a symlink
	useInlineMode := cfg.Type != agents.AgentClaude
	targetFile := cfg.InstructionTargetFile

	if cfg.Type == agents.AgentClaude {
		switch checkClaudeMDSymlink(gitRoot) {
		case symlinkToLocalAgentsMD:
			// CLAUDE.md -> AGENTS.md: use inline mode with AGENTS.md as target
			fmt.Println(ui.InfoF("  CLAUDE.md is symlinked to AGENTS.md, using inline mode"))
			useInlineMode = true
			targetFile = "AGENTS.md"
		case symlinkToElsewhere:
			// CLAUDE.md -> elsewhere: warn and skip
			fmt.Println(ui.WarningF("  CLAUDE.md is symlinked elsewhere, skipping instruction integration"))
			fmt.Println(ui.Warning("  To integrate, either remove the symlink or manually add thts instructions"))
			return nil, nil
		}
	}

	filePath := filepath.Join(gitRoot, targetFile)

	// Build the content to insert based on mode
	var insertContent string
	if useInlineMode {
		// Inline the full content (Codex/OpenCode, or Claude with symlinked CLAUDE.md)
		thtsContent, err := readThtsInstructions()
		if err != nil {
			return nil, fmt.Errorf("failed to read instructions: %w", err)
		}
		insertContent = fmt.Sprintf("\n%s\n%s\n%s\n",
			ThtsMarkerStart,
			string(thtsContent),
			ThtsMarkerEnd)
	} else {
		// Claude supports @include
		insertContent = fmt.Sprintf("\n%s\n@%s/%s\n%s\n",
			ThtsMarkerStart,
			cfg.RootDir,
			ThtsInstructionsFile,
			ThtsMarkerEnd)
	}

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
		// For inline content, adjust header levels to fit under existing structure
		appendContent := insertContent
		if useInlineMode {
			appendContent = adjustHeaderLevels(insertContent, 1)
		}
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
	var header string
	createContent := insertContent
	if useInlineMode {
		header = "# Agent Instructions\n"
		// For inline content, adjust header levels to fit under top-level header
		createContent = adjustHeaderLevels(insertContent, 1)
	} else {
		header = "# Claude Code Instructions\n"
	}
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
		content = buildClaudeSettings()
	case agents.AgentCodex:
		content = thtsfiles.DefaultCodexConfigTOML
	case agents.AgentOpenCode:
		content = thtsfiles.DefaultOpenCodeJSON
	default:
		return fmt.Errorf("no default settings for agent: %s", agentType)
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
	// - */thts-* matches skills/thts-integrate*, commands/thts-*
	// - */thoughts-* matches agents/thoughts-*
	return []string{
		cfg.RootDir + "/thts-*",
		cfg.RootDir + "/*/thts-*",
		cfg.RootDir + "/*/thoughts-*",
	}
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
		return promptGlobalComponentSelection()
	}

	if value == "all" {
		return []string{"skills", "commands", "agents"}, nil
	}

	// Parse comma-separated list
	var components []string
	validComponents := map[string]bool{
		"skills":   true,
		"commands": true,
		"agents":   true,
	}

	for _, c := range strings.Split(value, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if !validComponents[c] {
			return nil, fmt.Errorf("invalid component: %s (valid: skills, commands, agents)", c)
		}
		components = append(components, c)
	}

	return components, nil
}

// promptGlobalComponentSelection shows an interactive multi-select for components.
func promptGlobalComponentSelection() ([]string, error) {
	if !isTerminal() {
		return nil, fmt.Errorf("interactive mode requires a terminal. Specify components: --global all or --global skills,commands,agents")
	}

	var selected []string
	options := []huh.Option[string]{
		huh.NewOption("Skills (thts-integrate)", "skills"),
		huh.NewOption("Commands (thts-handoff, thts-resume) [Claude only]", "commands"),
		huh.NewOption("Agents (thoughts-locator, thoughts-analyzer)", "agents"),
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
			if agentType == agents.AgentClaude {
				_, files, err = copyCommands(globalDir, agentType)
			} else {
				continue // commands only for Claude
			}
		case "agents":
			_, files, err = copyAgents(globalDir, agentType, agentCfg)
		default:
			return fmt.Errorf("unknown component: %s", component)
		}

		if err != nil {
			return fmt.Errorf("failed to copy %s for %s: %w", component, agentType, err)
		}

		// Convert relative file names to absolute paths
		for _, f := range files {
			var fullPath string
			switch component {
			case "skills":
				fullPath = filepath.Join(globalDir, agentCfg.SkillsDir, f)
			case "commands":
				fullPath = filepath.Join(globalDir, "commands", f)
			case "agents":
				fullPath = filepath.Join(globalDir, agentCfg.AgentsDir, f)
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
