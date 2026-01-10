package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/scottames/tpd/internal/cmd/claude"
	"github.com/scottames/tpd/internal/config"
	"github.com/scottames/tpd/internal/fs"
	"github.com/spf13/cobra"
)

var uninitForce bool

var uninitCmd = &cobra.Command{
	Use:   "uninit",
	Short: "Remove thoughts setup from current repository",
	Long: `Remove the thoughts directory and configuration from the current repository.

This only removes the local symlinks and configuration. Your actual thoughts
content remains safe in the central thoughts repository.`,
	RunE: runUninit,
}

func init() {
	uninitCmd.Flags().BoolVarP(&uninitForce, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(uninitCmd)
}

func runUninit(cmd *cobra.Command, args []string) error {
	// Get current repo path
	currentRepo, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	thoughtsDir := filepath.Join(currentRepo, "thoughts")

	// Check if thoughts directory exists
	if !fs.Exists(thoughtsDir) {
		fmt.Println(styleError.Render("Error: Thoughts not initialized for this repository."))
		return nil
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(styleError.Render("Error: Thoughts configuration not found."))
		return nil
	}

	// Get current repo mapping
	mapping := cfg.RepoMappings[currentRepo]
	mappedName := ""
	profileName := ""

	if mapping != nil {
		mappedName = mapping.GetRepoName()
		profileName = mapping.Profile
	}

	if mappedName == "" && !uninitForce {
		fmt.Println(styleError.Render("Error: This repository is not in the thoughts configuration."))
		fmt.Printf("Use %s to remove the thoughts directory anyway.\n", styleCyan.Render("--force"))
		return nil
	}

	// Confirm unless --force
	if !uninitForce {
		fmt.Println(styleInfo.Render("Removing thoughts setup from current repository..."))
		fmt.Println()
		fmt.Printf("This will remove: %s\n", styleCyan.Render(thoughtsDir))
		fmt.Println()

		var confirm bool
		err := huh.NewConfirm().
			Title("Are you sure you want to remove thoughts from this repository?").
			Description("Your actual thoughts content will remain safe in the central repository.").
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

	fmt.Println(styleInfo.Render("Removing thoughts setup from current repository..."))

	// Handle searchable directory if it exists
	searchableDir := filepath.Join(thoughtsDir, "searchable")
	if fs.Exists(searchableDir) {
		fmt.Println(styleMuted.Render("Removing searchable directory..."))
		if err := fs.RemoveAll(searchableDir); err != nil {
			fmt.Printf("%s Could not remove searchable directory: %v\n", styleWarning.Render("Warning:"), err)
		}
	}

	// Remove the entire thoughts directory
	fmt.Println(styleMuted.Render("Removing thoughts directory (symlinks only)..."))
	if err := fs.RemoveAll(thoughtsDir); err != nil {
		fmt.Printf("%s Could not remove thoughts directory: %v\n", styleError.Render("Error:"), err)
		fmt.Printf("You may need to manually remove: %s\n", thoughtsDir)
		return nil
	}

	// Remove from config if mapped
	if mappedName != "" {
		fmt.Println(styleMuted.Render("Removing repository from thoughts configuration..."))
		delete(cfg.RepoMappings, currentRepo)
		if err := config.Save(cfg); err != nil {
			fmt.Printf("%s Could not update configuration: %v\n", styleWarning.Render("Warning:"), err)
		}
	}

	// Also remove Claude integration if present (leave no trace)
	fmt.Println(styleMuted.Render("Checking for Claude integration..."))
	if err := claude.Uninit(currentRepo, true); err != nil {
		fmt.Printf("%s Could not remove Claude integration: %v\n", styleWarning.Render("Warning:"), err)
	}

	fmt.Println()
	fmt.Println(styleSuccess.Render("Thoughts removed from repository"))

	// Provide info about what was preserved
	if mappedName != "" {
		fmt.Println()
		fmt.Println(styleMuted.Render("Note: Your thoughts content remains safe in:"))

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
			fmt.Printf("  %s\n", styleMuted.Render(fmt.Sprintf("%s/%s/%s", profile.ThoughtsRepo, profile.ReposDir, mappedName)))
			fmt.Printf("  %s\n", styleMuted.Render(fmt.Sprintf("(profile: %s)", displayProfileName)))
		}

		fmt.Println(styleMuted.Render("Only the local symlinks and configuration were removed."))
	}

	return nil
}
