package profile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scottames/tpd/internal/config"
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
			fmt.Println(styleError.Render("Error: Thoughts not configured."))
			fmt.Printf("Run %s first to set up the base configuration.\n", styleCyan.Render("tpd setup"))
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

	fmt.Println(styleInfo.Render("Thoughts Profiles"))
	fmt.Println(styleMuted.Render(strings.Repeat("=", 50)))
	fmt.Println()

	// Show profiles
	if len(cfg.Profiles) == 0 {
		fmt.Println(styleMuted.Render("No profiles configured."))
		fmt.Println()
		fmt.Printf("Run %s to create your first profile.\n", styleCyan.Render("tpd setup"))
	} else {
		fmt.Println(styleWarning.Render(fmt.Sprintf("Profiles (%d):", len(cfg.Profiles))))
		fmt.Println()

		for name, profile := range cfg.Profiles {
			// Mark default profile with *
			nameDisplay := name
			if profile.Default {
				nameDisplay = name + " *"
			}
			fmt.Printf("  %s:\n", styleCyan.Render(nameDisplay))
			fmt.Printf("    Thoughts repository: %s\n", profile.ThoughtsRepo)
			fmt.Printf("    Repos directory: %s\n", profile.ReposDir)
			fmt.Printf("    Global directory: %s\n", profile.GlobalDir)
			fmt.Println()
		}

		fmt.Println(styleMuted.Render("* = default profile"))
	}

	return nil
}
