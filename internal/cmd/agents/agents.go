// Package agents provides the "thts agents" commands for managing
// agent tool integrations (Claude, Codex, OpenCode).
package agents

import "github.com/spf13/cobra"

// AgentsCmd is the parent command for agent-related operations.
var AgentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agent tool integrations",
	Long: `Manage agent tool integrations for Claude Code, OpenAI Codex CLI, and OpenCode.

Available subcommands:
  init     Initialize agent integration for this project
  uninit   Remove agent integration from this project

Supported agents: claude, codex, opencode`,
}

func init() {
	AgentsCmd.AddCommand(initCmd)
	AgentsCmd.AddCommand(uninitCmd)
}
