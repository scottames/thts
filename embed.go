// Package thtsfiles provides embedded agent integration files for thts.
// This package exists at the repo root to enable go:embed access to
// instructions/, skills/, commands/, and agents/ directories.
//
// The file structure supports multiple agent tools (Claude, Codex, OpenCode, Gemini):
//
//	instructions/thts-instructions.md - Shared thts instructions for all agents
//	skills/{agent}/*.md               - Agent-specific skills (flat for Claude)
//	skills/{agent}/*/SKILL.md         - Agent-specific skills (subdirs for Codex/OpenCode)
//	commands/{agent}/*.md             - Agent commands (prompts for Codex, global-only)
//	agents/{agent}/*.md               - Agent definitions per tool
package thtsfiles

import "embed"

// Instructions contains the shared thts-instructions.md file.
//
//go:embed instructions/thts-instructions.md
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
//
//go:embed commands/claude/*.md
var ClaudeCommands embed.FS

// CodexCommands contains embedded prompt markdown files for Codex CLI.
// Codex calls these "prompts" and they're global-only.
//
//go:embed commands/codex/*.md
var CodexCommands embed.FS

// OpenCodeCommands contains embedded command markdown files for OpenCode.
//
//go:embed commands/opencode/*.md
var OpenCodeCommands embed.FS

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

// GeminiSkills contains embedded skill files for Gemini CLI.
// Gemini uses subdirectories: skills/gemini/skill-name/SKILL.md
//
//go:embed skills/gemini/*/SKILL.md
var GeminiSkills embed.FS

// GeminiCommands contains embedded command TOML files for Gemini CLI.
// Gemini uses TOML format for commands, not markdown.
//
//go:embed commands/gemini/*.toml
var GeminiCommands embed.FS

// Templates contains embedded template files for thoughts/ documents.
// These are copied to thoughts/.templates/ during init.
//
//go:embed templates/*.md
var Templates embed.FS

// Settings contains embedded default settings files for agents.
// Files are named by agent type: codex.toml, opencode.json, etc.
// Claude settings are built dynamically and not embedded.
//
//go:embed settings/*
var Settings embed.FS

// GetDefaultSettings returns the default settings content for an agent.
// Returns empty string if no default settings exist (e.g., Claude builds dynamically).
func GetDefaultSettings(filename string) string {
	content, err := Settings.ReadFile("settings/" + filename)
	if err != nil {
		return ""
	}
	return string(content)
}
