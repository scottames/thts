package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestShouldPromptForAgentInit(t *testing.T) {
	t.Run("defaults to prompting in interactive mode", func(t *testing.T) {
		cmd := &cobra.Command{Use: "init"}
		cmd.Flags().Bool("no-agents", false, "")

		if !shouldPromptForAgentInit(cmd, true) {
			t.Fatal("expected prompt when --no-agents is not set")
		}
	})

	t.Run("skips prompting when --no-agents is set", func(t *testing.T) {
		cmd := &cobra.Command{Use: "init"}
		cmd.Flags().Bool("no-agents", false, "")
		if err := cmd.Flags().Set("no-agents", "true"); err != nil {
			t.Fatalf("failed to set no-agents flag: %v", err)
		}

		if shouldPromptForAgentInit(cmd, true) {
			t.Fatal("expected prompt to be skipped when --no-agents is set")
		}
	})

	t.Run("skips prompting in non-interactive mode", func(t *testing.T) {
		cmd := &cobra.Command{Use: "init"}
		cmd.Flags().Bool("no-agents", false, "")

		if shouldPromptForAgentInit(cmd, false) {
			t.Fatal("expected prompt to be skipped when stdin is not interactive")
		}
	})
}
