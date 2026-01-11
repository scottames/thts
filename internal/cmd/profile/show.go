package profile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/ui"
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
			fmt.Println(ui.Error("Thoughts not configured."))
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.ValidateProfile(profileName) {
		fmt.Println(ui.ErrorF("Profile %q not found.", profileName))
		fmt.Println()
		fmt.Println(ui.Muted("Available profiles:"))
		if len(cfg.Profiles) > 0 {
			for name := range cfg.Profiles {
				fmt.Printf("  - %s\n", ui.Muted(name))
			}
		} else {
			fmt.Println(ui.Muted("  (none)"))
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

	fmt.Println(ui.Header(fmt.Sprintf("Profile: %s", profileName)))
	fmt.Println()

	fmt.Println(ui.SubHeader("Configuration"))
	tbl := ui.NewTable("Setting", "Value")
	tbl.Row("Thoughts repository", profile.ThoughtsRepo)
	tbl.Row("Repos directory", profile.ReposDir)
	tbl.Row("Global directory", profile.GlobalDir)
	if len(profile.DefaultAgents) > 0 {
		tbl.Row("Default agents", strings.Join(profile.DefaultAgents, ", "))
	} else {
		tbl.Row("Default agents", "(none)")
	}
	fmt.Println(tbl)
	fmt.Println()

	// Count repositories using this profile
	repoCount := cfg.CountReposUsingProfile(profileName)

	fmt.Println(ui.SubHeader("Usage"))
	fmt.Printf("  Repositories using this profile: %s\n", ui.Accent(fmt.Sprintf("%d", repoCount)))

	return nil
}
