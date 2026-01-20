package agents

// EmbedTemplateData holds all data needed to render embedded templates
// for a specific agent type.
type EmbedTemplateData struct {
	// AgentDir is the agent's root directory (e.g., ".claude", ".codex")
	AgentDir string

	// AgentName is the human-readable name for the agent (e.g., "Claude")
	// Empty string for agents that don't use agent-specific naming
	AgentName string

	// InstructionRef is the full text for referencing instructions
	// Examples:
	//   "@.claude/thts-instructions.md" (Claude)
	//   "from thts-instructions.md in .codex/" (Codex/OpenCode)
	//   "from AGENTS.md in the project root" (Gemini)
	InstructionRef string

	// UseAtInclude indicates whether the agent uses @ syntax for includes
	// True for Claude, false for others
	UseAtInclude bool

	// HasTaskList indicates whether to include the TaskList step in commands
	// True for Claude/Codex/OpenCode, false for Gemini
	HasTaskList bool

	// TaskTracking is the text to use for task tracking
	// "Use TodoWrite" for Claude, "Use your task tracking" for others
	TaskTracking string

	// HasSpawnTasks indicates whether to include "Spawn research tasks" text
	// True for Claude/Codex/OpenCode, false for Gemini
	HasSpawnTasks bool

	// HasAgentsFeature indicates whether the agent supports the agents feature
	// True for Claude/Codex/OpenCode, false for Gemini
	HasAgentsFeature bool

	// IncludeToolsMetadata indicates whether to include tools/model in agent frontmatter
	// True for Claude, false for others
	IncludeToolsMetadata bool

	// AgentModel is the model to use for agents (e.g., "haiku" for Claude)
	AgentModel string
}

// GetEmbedTemplateData returns the template data for a specific agent type.
func GetEmbedTemplateData(agentType AgentType) EmbedTemplateData {
	cfg := GetConfig(agentType)
	if cfg == nil {
		return EmbedTemplateData{}
	}

	data := EmbedTemplateData{
		AgentDir:         cfg.RootDir,
		HasAgentsFeature: cfg.AgentsDir != "",
	}

	switch agentType {
	case AgentClaude:
		data.AgentName = "Claude"
		data.InstructionRef = "@.claude/thts-instructions.md"
		data.UseAtInclude = true
		data.HasTaskList = true
		data.TaskTracking = "Use TodoWrite"
		data.HasSpawnTasks = true
		data.IncludeToolsMetadata = true
		data.AgentModel = "haiku"

	case AgentCodex:
		data.AgentName = ""
		data.InstructionRef = "from thts-instructions.md in .codex/"
		data.UseAtInclude = false
		data.HasTaskList = true
		data.TaskTracking = "Use your task tracking"
		data.HasSpawnTasks = true
		data.IncludeToolsMetadata = false
		data.AgentModel = ""

	case AgentOpenCode:
		data.AgentName = ""
		data.InstructionRef = "from thts-instructions.md in .opencode/"
		data.UseAtInclude = false
		data.HasTaskList = true
		data.TaskTracking = "Use your task tracking"
		data.HasSpawnTasks = true
		data.IncludeToolsMetadata = false
		data.AgentModel = ""

	case AgentGemini:
		data.AgentName = ""
		data.InstructionRef = "from AGENTS.md in the project root"
		data.UseAtInclude = false
		data.HasTaskList = false
		data.TaskTracking = ""
		data.HasSpawnTasks = false
		data.IncludeToolsMetadata = false
		data.AgentModel = ""
	}

	return data
}
