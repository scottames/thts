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
const ManifestFile = ".thts-manifest.json"

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

// InstructionsMDModification tracks changes made to AGENTS.md or CLAUDE.md.
type InstructionsMDModification struct {
	Path    string `json:"path"`
	Action  string `json:"action"` // "appended", "created", or "symlinked"
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
}

func runAgentsInit(cmd *cobra.Command, args []string) error {
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

	manifest := &Manifest{
		Agent:            string(agentType),
		IntegrationLevel: level,
		Files:            []string{},
	}

	var filesCopied int

	// Copy AGENTS.md (shared instructions)
	if err := copyInstructionsFile(agentDir, agentConfig); err != nil {
		fmt.Println(ui.WarningF("  Could not copy instructions: %v", err))
	} else {
		filesCopied++
		manifest.Files = append(manifest.Files, "AGENTS.md")
		fmt.Println(ui.Success("  Copied AGENTS.md"))
	}

	// For Claude, create CLAUDE.md symlink to AGENTS.md
	if agentConfig.SymlinkToAgents {
		if err := createInstructionsSymlink(agentDir, agentConfig); err != nil {
			fmt.Println(ui.WarningF("  Could not create symlink: %v", err))
		} else {
			manifest.Files = append(manifest.Files, agentConfig.InstructionsFile)
			fmt.Println(ui.SuccessF("  Created %s -> AGENTS.md", agentConfig.InstructionsFile))
		}
	}

	// Copy skills
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

	// Copy commands (Claude only)
	if agentConfig.SupportsCommands {
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

	// Copy agents
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

// copyInstructionsFile copies AGENTS.md to the agent directory.
func copyInstructionsFile(agentDir string, cfg *agents.AgentConfig) error {
	content, err := fs.ReadFile(thtsfiles.Instructions, "instructions/AGENTS.md")
	if err != nil {
		return fmt.Errorf("failed to read AGENTS.md: %w", err)
	}
	targetPath := filepath.Join(agentDir, "AGENTS.md")
	return os.WriteFile(targetPath, content, 0644)
}

// createInstructionsSymlink creates a symlink from the agent's instruction file to AGENTS.md.
// For Claude: CLAUDE.md -> AGENTS.md
func createInstructionsSymlink(agentDir string, cfg *agents.AgentConfig) error {
	if !cfg.SymlinkToAgents {
		return nil
	}
	linkPath := filepath.Join(agentDir, cfg.InstructionsFile)
	// Remove existing file if it exists
	_ = os.Remove(linkPath)
	return os.Symlink("AGENTS.md", linkPath)
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
		mod, err := appendToInstructionsMD(gitRoot, cfg)
		return mod, nil, err

	case IntegrationLocalOnly:
		var localFile string
		if cfg.SymlinkToAgents {
			// For Claude, use CLAUDE.local.md
			localFile = strings.Replace(cfg.InstructionsFile, ".md", ".local.md", 1)
		} else {
			localFile = "AGENTS.local.md"
		}
		if err := createLocalInstructionsMD(agentDir, localFile); err != nil {
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

// appendToInstructionsMD appends the @include directive to AGENTS.md (or CLAUDE.md for Claude).
func appendToInstructionsMD(gitRoot string, cfg *agents.AgentConfig) (*InstructionsMDModification, error) {
	// For Claude, use CLAUDE.md; for others, use AGENTS.md
	var fileName string
	if cfg.SymlinkToAgents {
		fileName = cfg.InstructionsFile // CLAUDE.md
	} else {
		fileName = "AGENTS.md"
	}

	filePath := filepath.Join(gitRoot, fileName)
	includeDirective := fmt.Sprintf("\n@%s/AGENTS.md\n", cfg.RootDir)
	pattern := fmt.Sprintf("@%s/AGENTS.md", cfg.RootDir)

	if fsutil.Exists(filePath) {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", fileName, err)
		}
		if strings.Contains(string(content), pattern) {
			fmt.Println(ui.InfoF("  %s already includes thts integration", fileName))
			return nil, nil
		}
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", fileName, err)
		}
		if _, err := f.WriteString(includeDirective); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("failed to append to %s: %w", fileName, err)
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close %s: %w", fileName, err)
		}
		fmt.Println(ui.SuccessF("  Appended @include to existing %s", fileName))
		return &InstructionsMDModification{
			Path:    filePath,
			Action:  "appended",
			Pattern: pattern,
		}, nil
	}

	// Create new file
	header := "# Agent Instructions\n"
	if cfg.SymlinkToAgents {
		header = "# Claude Code Instructions\n"
	}
	if err := os.WriteFile(filePath, []byte(header+includeDirective), 0644); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", fileName, err)
	}
	fmt.Println(ui.SuccessF("  Created %s with @include", fileName))
	return &InstructionsMDModification{
		Path:    filePath,
		Action:  "created",
		Pattern: pattern,
	}, nil
}

// createLocalInstructionsMD creates a local instructions file with the @include directive.
func createLocalInstructionsMD(agentDir, localFile string) error {
	localPath := filepath.Join(agentDir, localFile)
	content := "# Local Agent Instructions\n\n@AGENTS.md\n"
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
