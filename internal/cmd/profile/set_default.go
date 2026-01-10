package profile

import (
	"fmt"

	"github.com/scottames/tpd/internal/config"
	"github.com/spf13/cobra"
)

var setDefaultCmd = &cobra.Command{
	Use:   "set-default <name>",
	Short: "Set the default profile",
	Long: `Set a profile as the default.

The default profile is used when initializing repositories without the --profile flag.
Only one profile can be the default at a time.`,
	Args: cobra.ExactArgs(1),
	RunE: runSetDefault,
}

func runSetDefault(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			fmt.Println(styleError.Render("Error: Thoughts not configured."))
			fmt.Printf("Run %s first to set up.\n", styleCyan.Render("tpd setup"))
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.ValidateProfile(profileName) {
		fmt.Println(styleError.Render(fmt.Sprintf("Error: Profile \"%s\" not found.", profileName)))
		fmt.Println()
		fmt.Println(styleMuted.Render("Available profiles:"))
		for name := range cfg.Profiles {
			fmt.Printf("  - %s\n", name)
		}
		return nil
	}

	// Check if it's already the default
	profile := cfg.Profiles[profileName]
	if profile.Default {
		fmt.Println(styleMuted.Render(fmt.Sprintf("Profile \"%s\" is already the default.", profileName)))
		return nil
	}

	// Set as default
	if !cfg.SetDefaultProfile(profileName) {
		return fmt.Errorf("failed to set default profile")
	}

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(styleSuccess.Render(fmt.Sprintf("Profile \"%s\" is now the default.", profileName)))
	return nil
}
