package claude

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue
	styleWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	styleMuted   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // gray
)

// ClaudeCmd represents the claude parent command.
var ClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Claude Code integration commands",
	Long: `Manage Claude Code integration for tpd.

These commands help configure and manage Claude Code features including
commands, agents, and settings for your project.`,
}

func init() {
	ClaudeCmd.AddCommand(initCmd)
	ClaudeCmd.AddCommand(uninitCmd)
}
