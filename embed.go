// Package thtsfiles provides embedded agent integration files for thts.
// This package exists at the repo root to enable go:embed access to
// instructions/, skills/, commands/, and agents/ directories.
//
// The file structure supports multiple agent tools (Claude, Codex, OpenCode):
//
//	instructions/AGENTS.md          - Shared instructions for all agents
//	skills/{agent}/*.md             - Agent-specific skills (flat for Claude)
//	skills/{agent}/*/SKILL.md       - Agent-specific skills (subdirs for Codex/OpenCode)
//	commands/claude/*.md            - Claude-only commands
//	agents/{agent}/*.md             - Agent definitions per tool
package thtsfiles

import "embed"

// Instructions contains the shared AGENTS.md instruction file.
//
//go:embed instructions/*.md
var Instructions embed.FS

// ClaudeSkills contains embedded skill markdown files for Claude Code.
// Claude uses flat files: skills/claude/skill-name.md
//
//go:embed skills/claude/*.md
var ClaudeSkills embed.FS

// CodexSkills contains embedded skill files for Codex CLI.
// Codex uses subdirectories: skills/codex/skill-name/SKILL.md
//
//go:embed skills/codex/*/SKILL.md
var CodexSkills embed.FS

// OpenCodeSkills contains embedded skill files for OpenCode.
// OpenCode uses subdirectories: skills/opencode/skill-name/SKILL.md
//
//go:embed skills/opencode/*/SKILL.md
var OpenCodeSkills embed.FS

// ClaudeCommands contains embedded command markdown files for Claude Code.
// Only Claude supports commands.
//
//go:embed commands/claude/*.md
var ClaudeCommands embed.FS

// ClaudeAgents contains embedded agent files for Claude Code.
//
//go:embed agents/claude/*.md
var ClaudeAgents embed.FS

// CodexAgents contains embedded agent files for Codex CLI.
//
//go:embed agents/codex/*.md
var CodexAgents embed.FS

// OpenCodeAgents contains embedded agent files for OpenCode.
//
//go:embed agents/opencode/*.md
var OpenCodeAgents embed.FS

// DefaultSettingsJSON provides a default settings.json template for Claude.
var DefaultSettingsJSON = `{
  "permissions": {
    "allow": []
  },
  "enableAllProjectMcpServers": false
}
`

// DefaultCodexConfigTOML provides a default config.toml template for Codex.
var DefaultCodexConfigTOML = `# Codex CLI configuration
# See: https://github.com/openai/codex

[model]
name = "gpt-4o"

[sandbox]
enabled = true

[approval]
# auto-edit | suggest | full-auto
policy = "suggest"
`

// DefaultOpenCodeJSON provides a default opencode.json template for OpenCode.
var DefaultOpenCodeJSON = `{
  "model": "anthropic/claude-sonnet-4-20250514",
  "permissions": {
    "allow": []
  }
}
`
