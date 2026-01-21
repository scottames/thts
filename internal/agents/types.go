// Package agents provides multi-agent tool support for thts.
// Currently supports Claude Code, OpenAI Codex CLI, OpenCode, and Google Gemini CLI.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AgentType represents a supported AI coding agent.
type AgentType string

const (
	AgentClaude   AgentType = "claude"
	AgentCodex    AgentType = "codex"
	AgentOpenCode AgentType = "opencode"
	AgentGemini   AgentType = "gemini"
)

// AllAgentTypes returns all supported agent types in canonical order.
func AllAgentTypes() []AgentType {
	return []AgentType{AgentClaude, AgentCodex, AgentOpenCode, AgentGemini}
}

// AgentTypeLabels provides human-readable labels for agent types.
var AgentTypeLabels = map[AgentType]string{
	AgentClaude:   "Claude Code",
	AgentCodex:    "OpenAI Codex CLI",
	AgentOpenCode: "OpenCode",
	AgentGemini:   "Google Gemini CLI",
}

// AgentConfig describes the configuration and conventions for a specific agent.
type AgentConfig struct {
	Type AgentType

	// RootDir is the agent's config directory (e.g., ".claude", ".codex", ".opencode").
	RootDir string

	// InstructionsFile is the thts instructions file name copied to agent directory.
	// This is "thts-instructions.md" for all agents.
	InstructionsFile string

	// IntegrationType specifies how thts instructions are integrated.
	// "marker" = append with HTML comment markers (Claude, Codex)
	// "config" = add to config file's instructions array (OpenCode)
	IntegrationType string

	// InstructionTargetFile is the file to modify for marker-based integration.
	// For Claude: "CLAUDE.md", for Codex: "AGENTS.md", empty for config-based.
	InstructionTargetFile string

	// SkillsDir is the directory name for skills (e.g., "skills", "skill").
	SkillsDir string

	// SkillNeedsDir indicates if skills require a subdirectory with SKILL.md.
	// Codex and OpenCode use skills/skill-name/SKILL.md format.
	SkillNeedsDir bool

	// AgentsDir is the directory name for agents.
	AgentsDir string

	// SupportsCommands indicates if this agent supports commands/prompts.
	SupportsCommands bool

	// CommandsDir is the directory name for commands/prompts (e.g., "commands", "prompts", "command").
	CommandsDir string

	// CommandsGlobalOnly indicates commands can only be installed globally (e.g., Codex prompts).
	CommandsGlobalOnly bool

	// GlobalUsesXDG indicates global config uses ~/.config/<name>/ instead of ~/.<name>/.
	GlobalUsesXDG bool

	// SettingsFile is the settings file name for this agent.
	SettingsFile string

	// SettingsFormat is the format of the settings file ("json", "toml").
	SettingsFormat string

	// CommandsFormat specifies the format of command files ("md" or "toml").
	// Defaults to "md" (markdown) if empty.
	CommandsFormat string

	// SettingsContextKey is the JSON key used in settings to point to the context file.
	// If non-empty, thts will ensure this key is set to InstructionTargetFile.
	// Example: Gemini uses "contextFileName": "AGENTS.md"
	SettingsContextKey string

	// SupportsHooks indicates if this agent supports hook-based integration.
	// Claude, Gemini, and OpenCode support hooks. Codex does not.
	SupportsHooks bool

	// HooksDir is the directory name for hooks (e.g., "hooks" for Claude/Gemini).
	// Empty for agents using plugins (OpenCode) or no hook support (Codex).
	HooksDir string

	// PluginsDir is the directory name for plugins (e.g., "plugins" for OpenCode).
	// Empty for agents using hooks or no plugin support.
	PluginsDir string
}

// AgentConfigs contains the configuration for each supported agent.
var AgentConfigs = map[AgentType]*AgentConfig{
	AgentClaude: {
		Type:                  AgentClaude,
		RootDir:               ".claude",
		InstructionsFile:      "", // Uses hooks for dynamic injection
		IntegrationType:       "marker",
		InstructionTargetFile: "CLAUDE.md",
		SkillsDir:             "skills",
		SkillNeedsDir:         false,
		AgentsDir:             "agents",
		SupportsCommands:      true,
		CommandsDir:           "commands",
		CommandsGlobalOnly:    false,
		GlobalUsesXDG:         false,
		SettingsFile:          "settings.json",
		SettingsFormat:        "json",
		SupportsHooks:         true,
		HooksDir:              "hooks",
		PluginsDir:            "",
	},
	AgentCodex: {
		Type:                  AgentCodex,
		RootDir:               ".codex",
		InstructionsFile:      "",
		IntegrationType:       "marker",
		InstructionTargetFile: "AGENTS.md",
		SkillsDir:             "skills",
		SkillNeedsDir:         true,
		AgentsDir:             "agents",
		SupportsCommands:      true,
		CommandsDir:           "prompts",
		CommandsGlobalOnly:    true, // Codex prompts are global-only per docs
		GlobalUsesXDG:         false,
		SettingsFile:          "config.toml",
		SettingsFormat:        "toml",
		SupportsHooks:         false, // Codex does not support hooks
		HooksDir:              "",
		PluginsDir:            "",
	},
	AgentOpenCode: {
		Type:                  AgentOpenCode,
		RootDir:               ".opencode",
		InstructionsFile:      "", // Uses plugin for dynamic injection
		IntegrationType:       "marker",
		InstructionTargetFile: "AGENTS.md",
		SkillsDir:             "skill",
		SkillNeedsDir:         true,
		AgentsDir:             "agent",
		SupportsCommands:      true,
		CommandsDir:           "command",
		CommandsGlobalOnly:    false,
		GlobalUsesXDG:         true, // OpenCode uses ~/.config/opencode/ for global
		SettingsFile:          "opencode.json",
		SettingsFormat:        "json",
		SupportsHooks:         true,
		HooksDir:              "",
		PluginsDir:            "plugins",
	},
	AgentGemini: {
		Type:                  AgentGemini,
		RootDir:               ".gemini",
		InstructionsFile:      "",
		IntegrationType:       "marker",
		InstructionTargetFile: "AGENTS.md",
		SkillsDir:             "skills",
		SkillNeedsDir:         true,
		AgentsDir:             "", // Gemini doesn't support agents
		SupportsCommands:      true,
		CommandsDir:           "commands",
		CommandsGlobalOnly:    false,
		GlobalUsesXDG:         false, // Uses ~/.gemini/
		SettingsFile:          "settings.json",
		SettingsFormat:        "json",
		CommandsFormat:        "toml", // Gemini uses TOML for commands
		SettingsContextKey:    "contextFileName",
		SupportsHooks:         true,
		HooksDir:              "hooks",
		PluginsDir:            "",
	},
}

// GetConfig returns the configuration for an agent type.
func GetConfig(agentType AgentType) *AgentConfig {
	return AgentConfigs[agentType]
}

// CommandsDirLabel returns the user-facing label for commands directory.
// Codex uses "prompts", others use "commands".
func CommandsDirLabel(agentType AgentType) string {
	config := GetConfig(agentType)
	if config != nil && config.CommandsDir == "prompts" {
		return "prompts"
	}
	return "commands"
}

// ParseAgentType parses a string into an AgentType.
func ParseAgentType(s string) (AgentType, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))
	switch normalized {
	case "claude":
		return AgentClaude, nil
	case "codex":
		return AgentCodex, nil
	case "opencode":
		return AgentOpenCode, nil
	case "gemini":
		return AgentGemini, nil
	default:
		return "", fmt.Errorf("unknown agent type: %q (valid: claude, codex, opencode, gemini)", s)
	}
}

// ParseAgentTypes parses a comma-separated list of agent types.
func ParseAgentTypes(s string) ([]AgentType, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	parts := strings.Split(s, ",")
	seen := make(map[AgentType]bool)
	var agents []AgentType

	for _, part := range parts {
		agentType, err := ParseAgentType(part)
		if err != nil {
			return nil, err
		}
		if !seen[agentType] {
			seen[agentType] = true
			agents = append(agents, agentType)
		}
	}

	return agents, nil
}

// DetectExistingAgents looks for existing agent directories in a project.
func DetectExistingAgents(projectDir string) []AgentType {
	var found []AgentType

	for _, agentType := range AllAgentTypes() {
		config := GetConfig(agentType)
		agentDir := filepath.Join(projectDir, config.RootDir)
		if info, err := os.Stat(agentDir); err == nil && info.IsDir() {
			found = append(found, agentType)
		}
	}

	return found
}

// AgentTypesToStrings converts a slice of AgentTypes to strings.
func AgentTypesToStrings(agents []AgentType) []string {
	result := make([]string, len(agents))
	for i, a := range agents {
		result[i] = string(a)
	}
	return result
}

// StringsToAgentTypes converts a slice of strings to AgentTypes.
func StringsToAgentTypes(strings []string) ([]AgentType, error) {
	result := make([]AgentType, len(strings))
	for i, s := range strings {
		agentType, err := ParseAgentType(s)
		if err != nil {
			return nil, err
		}
		result[i] = agentType
	}
	return result, nil
}

// SortAgentTypes sorts agent types in canonical order (derived from AllAgentTypes).
func SortAgentTypes(agents []AgentType) {
	// Build order map from AllAgentTypes slice position
	order := make(map[AgentType]int)
	for i, at := range AllAgentTypes() {
		order[at] = i
	}
	sort.Slice(agents, func(i, j int) bool {
		return order[agents[i]] < order[agents[j]]
	})
}

// ValidateAgentTypes checks if all provided agent types are valid.
func ValidateAgentTypes(agents []AgentType) error {
	for _, a := range agents {
		if GetConfig(a) == nil {
			return fmt.Errorf("unknown agent type: %q", a)
		}
	}
	return nil
}
