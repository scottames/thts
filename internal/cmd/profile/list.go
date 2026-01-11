package profile

import (
	"encoding/json"
	"fmt"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	Long:  `List all configured profiles and the default configuration.`,
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			fmt.Println(ui.Error("Thoughts not configured."))
			fmt.Printf("Run %s first to set up the base configuration.\n", ui.Accent("thts setup"))
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	if listJSON {
		profiles := cfg.Profiles
		if profiles == nil {
			profiles = make(map[string]*config.ProfileConfig)
		}
		data, err := json.MarshalIndent(profiles, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal profiles: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println(ui.Header("Thoughts Profiles"))
	fmt.Println()

	// Show profiles
	if len(cfg.Profiles) == 0 {
		fmt.Println(ui.Muted("No profiles configured."))
		fmt.Println()
		fmt.Printf("Run %s to create your first profile.\n", ui.Accent("thts setup"))
	} else {
		fmt.Println(ui.SubHeader(fmt.Sprintf("Profiles (%d)", len(cfg.Profiles))))
		fmt.Println()

		for name, profile := range cfg.Profiles {
			// Mark default profile with *
			nameDisplay := name
			if profile.Default {
				nameDisplay = name + " *"
			}
			fmt.Printf("  %s:\n", ui.Accent(nameDisplay))
			fmt.Printf("    Thoughts repository: %s\n", profile.ThoughtsRepo)
			fmt.Printf("    Repos directory: %s\n", profile.ReposDir)
			fmt.Printf("    Global directory: %s\n", profile.GlobalDir)
			fmt.Println()
		}

		fmt.Println(ui.Muted("* = default profile"))
	}

	return nil
}
