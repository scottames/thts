package profile

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/scottames/tpd/internal/config"
	"github.com/spf13/cobra"
)

var deleteForce bool

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Long: `Delete a profile from the configuration.

This removes the profile configuration but does NOT delete the thoughts
repository files. Use --force to skip confirmation and delete even if
repositories are using this profile.`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation and delete even if in use")
}

func runDelete(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			fmt.Println(styleError.Render("Error: Thoughts not configured."))
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.ValidateProfile(profileName) {
		fmt.Println(styleError.Render(fmt.Sprintf("Error: Profile \"%s\" not found.", profileName)))
		return nil
	}

	// Check if any repositories are using this profile
	usingRepos := cfg.GetReposUsingProfile(profileName)

	if len(usingRepos) > 0 && !deleteForce {
		fmt.Println(styleError.Render(fmt.Sprintf("Error: Profile \"%s\" is in use by %d repository(ies):", profileName, len(usingRepos))))
		fmt.Println()
		for _, repo := range usingRepos {
			displayPath := config.ContractPath(repo)
			fmt.Printf("  - %s\n", styleMuted.Render(displayPath))
		}
		fmt.Println()
		fmt.Println(styleWarning.Render("Options:"))
		fmt.Printf("  1. Run %s in each repository\n", styleCyan.Render("tpd uninit"))
		fmt.Printf("  2. Use %s to delete anyway (repos will fall back to default config)\n", styleCyan.Render("--force"))
		return nil
	}

	// Confirm deletion
	if !deleteForce {
		fmt.Println()
		fmt.Println(styleWarning.Render(fmt.Sprintf("You are about to delete profile: %s", styleCyan.Render(profileName))))
		fmt.Println(styleMuted.Render("This will remove the profile configuration."))
		fmt.Println(styleMuted.Render("The thoughts repository files will NOT be deleted."))
		fmt.Println()

		var confirm bool
		err := huh.NewConfirm().
			Title("Are you sure?").
			Value(&confirm).
			Run()
		if err != nil {
			return err
		}

		if !confirm {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	// Delete profile
	cfg.DeleteProfile(profileName)

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Println(styleSuccess.Render(fmt.Sprintf("✅ Profile \"%s\" deleted", profileName)))

	if len(usingRepos) > 0 {
		fmt.Println()
		fmt.Println(styleWarning.Render("⚠️  Warning: Repositories using this profile will fall back to default config"))
	}

	return nil
}
