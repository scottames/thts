package profile

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
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
)

// ProfileCmd represents the profile parent command.
var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage tpd profiles",
	Long: `Manage tpd profiles for different thoughts repositories.

Profiles allow you to have separate thoughts repositories for different contexts
(e.g., work vs personal projects).`,
}

func init() {
	ProfileCmd.AddCommand(listCmd)
	ProfileCmd.AddCommand(createCmd)
	ProfileCmd.AddCommand(showCmd)
	ProfileCmd.AddCommand(deleteCmd)
	ProfileCmd.AddCommand(setDefaultCmd)
}
