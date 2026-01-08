package cmd

import (
	"os"

	"github.com/scottames/tpd/internal/cmd/profile"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "tpd",
	Short: "Thoughts, plans, and dreams - manage developer thoughts across repositories",
	Long: `tpd (thoughts, plans, and dreams) is a CLI tool for managing developer
thoughts and notes across multiple repositories.

It synchronizes thoughts to a central git repository and provides easy access
to personal and shared notes within any project.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = Version
	rootCmd.AddCommand(profile.ProfileCmd)
}
