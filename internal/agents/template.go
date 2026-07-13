package agents

// EmbedTemplateData holds all data needed to render embedded templates
// for a specific agent type.
type EmbedTemplateData struct {
	// AgentDir is the agent's root directory (e.g., ".claude", ".codex")
	AgentDir string

	// AgentName is the human-readable name for the agent (e.g., "Claude")
	// Empty string for agents that don't use agent-specific naming
	AgentName string

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

	// IncludeAgentMode indicates whether to include agent mode in frontmatter
	// True for OpenCode, false for others
	IncludeAgentMode bool

	// AgentModel is the model to use for agents (e.g., "haiku" for Claude)
	AgentModel string

	// IncludeClaudePlanDirective controls whether Claude-specific plan directive
	// content is rendered into session-start hooks.
	IncludeClaudePlanDirective bool
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
		data.HasTaskList = true
		data.TaskTracking = "Use TodoWrite"
		data.HasSpawnTasks = true
		data.IncludeToolsMetadata = true
		data.AgentModel = "haiku"
		data.IncludeClaudePlanDirective = true

	case AgentCodex:
		data.AgentName = ""
		data.HasTaskList = true
		data.TaskTracking = "Use your task tracking"
		data.HasSpawnTasks = true
		data.IncludeToolsMetadata = false
		data.AgentModel = ""
		data.IncludeClaudePlanDirective = false

	case AgentOpenCode:
		data.AgentName = ""
		data.HasTaskList = true
		data.TaskTracking = "Use your task tracking"
		data.HasSpawnTasks = true
		data.IncludeToolsMetadata = false
		data.IncludeAgentMode = true
		data.AgentModel = ""
		data.IncludeClaudePlanDirective = false

	case AgentGemini:
		data.AgentName = ""
		data.HasTaskList = false
		data.TaskTracking = ""
		data.HasSpawnTasks = false
		data.IncludeToolsMetadata = false
		data.AgentModel = ""
		data.IncludeClaudePlanDirective = false
	}

	return data
}
