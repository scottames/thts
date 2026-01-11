package claude

import (
	"github.com/spf13/cobra"
)

// ClaudeCmd represents the claude parent command.
var ClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Claude Code integration commands",
	Long: `Manage Claude Code integration for thts.

These commands help configure and manage Claude Code features including
commands, agents, and settings for your project.`,
}

func init() {
	ClaudeCmd.AddCommand(initCmd)
	ClaudeCmd.AddCommand(uninitCmd)
}
