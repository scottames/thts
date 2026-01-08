package profile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scottames/tpd/internal/config"
	"github.com/spf13/cobra"
)

var showJSON bool

var showCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show profile details",
	Long:  `Show detailed information about a specific profile.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	showCmd.Flags().BoolVar(&showJSON, "json", false, "Output as JSON")
}

func runShow(cmd *cobra.Command, args []string) error {
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
		fmt.Println()
		fmt.Println(styleMuted.Render("Available profiles:"))
		if len(cfg.Profiles) > 0 {
			for name := range cfg.Profiles {
				fmt.Printf("  - %s\n", styleMuted.Render(name))
			}
		} else {
			fmt.Println(styleMuted.Render("  (none)"))
		}
		return nil
	}

	profile := cfg.Profiles[profileName]

	if showJSON {
		data, err := json.MarshalIndent(profile, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal profile: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println(styleInfo.Render(fmt.Sprintf("Profile: %s", profileName)))
	fmt.Println(styleMuted.Render(strings.Repeat("=", 50)))
	fmt.Println()
	fmt.Println(styleWarning.Render("Configuration:"))
	fmt.Printf("  Thoughts repository: %s\n", styleCyan.Render(profile.ThoughtsRepo))
	fmt.Printf("  Repos directory: %s\n", styleCyan.Render(profile.ReposDir))
	fmt.Printf("  Global directory: %s\n", styleCyan.Render(profile.GlobalDir))
	fmt.Println()

	// Count repositories using this profile
	repoCount := cfg.CountReposUsingProfile(profileName)

	fmt.Println(styleWarning.Render("Usage:"))
	fmt.Printf("  Repositories using this profile: %s\n", styleCyan.Render(fmt.Sprintf("%d", repoCount)))

	return nil
}
