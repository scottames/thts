package cmd

import (
	"fmt"

	thtsfiles "github.com/scottames/thts"
	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/thts"
	"github.com/spf13/cobra"
)

var agentInstructionsCmd = &cobra.Command{
	Use:   "agent-instructions",
	Short: "Output thts instructions to stdout",
	Long: `Outputs the templated thts instructions for agent integration.

This is primarily used by hooks and plugins to inject instructions dynamically,
without requiring a per-project instructions file.`,
	RunE:   runAgentInstructions,
	Hidden: true, // Internal use by hooks/plugins
}

func init() {
	rootCmd.AddCommand(agentInstructionsCmd)
}

func runAgentInstructions(_ *cobra.Command, _ []string) error {
	cfg := config.LoadOrDefault()
	data := thts.BuildInstructionsData(cfg)
	content, err := thtsfiles.GetInstructions(data)
	if err != nil {
		return fmt.Errorf("failed to generate instructions: %w", err)
	}
	fmt.Print(content)
	return nil
}
