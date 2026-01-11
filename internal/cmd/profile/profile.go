package profile

import (
	"github.com/spf13/cobra"
)

// ProfileCmd represents the profile parent command.
var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage thts profiles",
	Long: `Manage thts profiles for different thoughts repositories.

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
