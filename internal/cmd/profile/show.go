package profile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	showJSON bool
	showPath bool
)

var showCmd = &cobra.Command{
	Use:               "show [name]",
	Short:             "Show profile details",
	Long:              `Show detailed information about a profile. If no name is provided, shows the default profile.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProfileNames,
	RunE:              runShow,
}

// completeProfileNames provides shell completion for profile names.
func completeProfileNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	showCmd.Flags().BoolVar(&showJSON, "json", false, "Output as JSON")
	showCmd.Flags().BoolVar(&showPath, "path", false, "Output only the thoughts repo path")
}

func runShow(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			return fmt.Errorf("thoughts not configured, run 'thts setup' first")
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	var profile *config.ProfileConfig
	var profileName string
	var isDefault bool

	if len(args) == 0 {
		// Use default profile
		profile, profileName = cfg.GetDefaultProfile()
		if profile == nil {
			return fmt.Errorf("no default profile configured")
		}
		isDefault = true
	} else {
		// Use named profile
		profileName = args[0]
		if !cfg.ValidateProfile(profileName) {
			return fmt.Errorf("profile %q not found, run 'thts profile list' to see available profiles", profileName)
		}
		profile = cfg.Profiles[profileName]
		isDefault = profile.Default
	}

	// Handle --path flag: output only the expanded path
	if showPath {
		if profile.ThoughtsRepo == "" {
			return fmt.Errorf("profile %q has no thoughts repo configured", profileName)
		}
		fmt.Println(config.ExpandPath(profile.ThoughtsRepo))
		return nil
	}

	if showJSON {
		data, err := json.MarshalIndent(profile, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal profile: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	headerText := fmt.Sprintf("Profile: %s", profileName)
	if isDefault {
		headerText += " (default)"
	}
	fmt.Println(ui.Header(headerText))
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
	counts := cfg.CountReposUsingProfileWithImplicit(profileName)

	fmt.Println(ui.SubHeader("Usage"))
	if counts.Implicit > 0 {
		fmt.Printf("  Repositories using this profile: %s (%s explicit, %s via default)\n",
			ui.Accent(fmt.Sprintf("%d", counts.Total)),
			ui.Accent(fmt.Sprintf("%d", counts.Explicit)),
			ui.Accent(fmt.Sprintf("%d", counts.Implicit)),
		)
	} else {
		fmt.Printf("  Repositories using this profile: %s\n",
			ui.Accent(fmt.Sprintf("%d", counts.Total)))
	}

	return nil
}
