package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/scottames/thts/internal/agents"
	agentscmd "github.com/scottames/thts/internal/cmd/agents"
	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	uninitForce bool
	uninitAll   bool
)

var uninitCmd = &cobra.Command{
	Use:   "uninit",
	Short: "Remove thoughts setup from current repository",
	Long: `Remove the thoughts directory and configuration from the current repository.

This removes:
  - The thoughts/ directory (symlinks only)
  - Any agent integrations (if present)

Your actual thoughts content remains safe in the central thoughts repository.

By default, this only removes local setup in the current worktree/checkout.
Use --all to also remove the shared repository mapping from thts state.

To remove only agent integrations without removing thoughts/, use:
  thts uninit agents`,
	RunE: runUninit,
}

func init() {
	uninitCmd.Flags().BoolVarP(&uninitForce, "force", "f", false, "Skip confirmation prompt")
	uninitCmd.Flags().BoolVar(&uninitAll, "all", false, "Also remove shared repository mapping from thts state")

	// Register agents subcommand
	uninitCmd.AddCommand(agentscmd.UninitCmd)

	rootCmd.AddCommand(uninitCmd)
}

func runUninit(cmd *cobra.Command, args []string) error {
	// Get current repo path
	currentRepo, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	thoughtsDir := filepath.Join(currentRepo, "thoughts")
	hasLocalThoughts := fs.Exists(thoughtsDir)

	// Load config and state
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(ui.Error("Thoughts configuration not found."))
		return nil
	}

	state := config.LoadStateOrDefault()

	repoIdentity, _ := git.GetRepoIdentityAt(currentRepo)
	mappingKey, mapping := state.ResolveRepoMapping(currentRepo, repoIdentity)
	mappedName := ""
	profileName := ""

	if mapping != nil {
		mappedName = mapping.GetRepoName()
		profileName = mapping.Profile
	}

	if !hasLocalThoughts && !uninitAll {
		fmt.Println(ui.Error("Thoughts not initialized for this repository."))
		if mappedName != "" {
			fmt.Printf("Run %s to remove shared mapping too.\n", ui.Accent("thts uninit --all"))
		}
		return nil
	}

	if mappedName == "" && uninitAll {
		fmt.Println(ui.Error("No shared repository mapping found in thts state."))
		return nil
	}

	// Detect existing agent integrations
	detectedAgents := agents.DetectExistingAgents(currentRepo)
	hasAgentIntegrations := len(detectedAgents) > 0

	// Confirm unless --force
	if !uninitForce {
		fmt.Println(ui.Info("Removing thoughts setup from current repository..."))
		fmt.Println()
		if hasLocalThoughts {
			fmt.Printf("This will remove: %s\n", ui.Accent(thoughtsDir))
		}
		if uninitAll && mappedName != "" {
			fmt.Printf("This will also remove repo mapping: %s\n", ui.Accent(mappedName))
		}
		if hasAgentIntegrations {
			fmt.Printf("This will also remove agent integrations: %s\n",
				ui.Accent(strings.Join(agents.AgentTypesToStrings(detectedAgents), ", ")))
		}
		fmt.Println()

		description := "Your actual thoughts content will remain safe in the central repository."
		if !uninitAll {
			description += " Shared repository mapping will be kept."
		}
		if hasAgentIntegrations {
			description += " Agent integration files will also be removed."
		}

		var confirm bool
		err := huh.NewConfirm().
			Title("Are you sure you want to continue?").
			Description(description).
			Affirmative("Yes, remove").
			Negative("Cancel").
			Value(&confirm).
			Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Println(ui.Info("Removing thoughts setup from current repository..."))

	if hasLocalThoughts {
		// Handle searchable directory if it exists
		searchableDir := filepath.Join(thoughtsDir, "searchable")
		if fs.Exists(searchableDir) {
			fmt.Println(ui.Muted("Removing searchable directory..."))
			if err := fs.RemoveAll(searchableDir); err != nil {
				fmt.Println(ui.WarningF("Could not remove searchable directory: %v", err))
			}
		}

		// Remove the entire thoughts directory
		fmt.Println(ui.Muted("Removing thoughts directory (symlinks only)..."))
		if err := fs.RemoveAll(thoughtsDir); err != nil {
			fmt.Println(ui.ErrorF("Could not remove thoughts directory: %v", err))
			fmt.Printf("You may need to manually remove: %s\n", thoughtsDir)
			return nil
		}
	}

	// Remove from state only when explicitly requested.
	if uninitAll && mappedName != "" {
		fmt.Println(ui.Muted("Removing repository mapping from thoughts state..."))
		delete(state.RepoMappings, mappingKey)
		if err := config.SaveState(state); err != nil {
			fmt.Println(ui.WarningF("Could not update state: %v", err))
		}
	}

	// Also remove agent integrations if present (leave no trace)
	if hasAgentIntegrations {
		fmt.Printf("%s Removing agent integrations (%s)...\n",
			ui.Muted(""),
			strings.Join(agents.AgentTypesToStrings(detectedAgents), ", "))
		if err := agentscmd.Uninit(currentRepo, true, nil); err != nil {
			fmt.Println(ui.WarningF("Could not remove agent integrations: %v", err))
		} else {
			fmt.Println(ui.Success("Removed agent integrations"))
		}
	}

	fmt.Println()
	if uninitAll {
		fmt.Println(ui.Success("Thoughts mapping removed from repository"))
	} else {
		fmt.Println(ui.Success("Local thoughts setup removed from repository"))
		fmt.Printf("Run %s to remove shared mapping as well.\n", ui.Accent("thts uninit --all"))
	}

	// Provide info about what was preserved
	if mappedName != "" {
		fmt.Println()
		fmt.Println(ui.Muted("Note: Your thoughts content remains safe in:"))

		// Look up the profile that was used
		var profile *config.ProfileConfig
		var displayProfileName string

		if profileName != "" && cfg.Profiles != nil {
			if p, exists := cfg.Profiles[profileName]; exists {
				profile = p
				displayProfileName = profileName
			}
		}
		if profile == nil {
			// Fall back to default profile
			profile, displayProfileName = cfg.GetDefaultProfile()
		}

		if profile != nil {
			fmt.Printf("  %s\n", ui.Muted(fmt.Sprintf("%s/%s/%s", profile.ThoughtsRepo, profile.ReposDir, mappedName)))
			fmt.Printf("  %s\n", ui.Muted(fmt.Sprintf("(profile: %s)", displayProfileName)))
		}

		if uninitAll {
			fmt.Println(ui.Muted("Local setup removed and shared mapping detached."))
		} else {
			fmt.Println(ui.Muted("Only local setup was removed; shared mapping is still active."))
		}
	}

	return nil
}
