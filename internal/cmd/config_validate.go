package cmd

import (
	"fmt"
	"os"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  `Checks the configuration file for errors and warnings.`,
	RunE:  runConfigValidate,
}

func init() {
	configCmd.AddCommand(configValidateCmd)
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	fmt.Println("Validating configuration...")
	fmt.Println()

	var errors, warnings int

	// Check 1: Config file exists
	configPath := config.ThtsConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("  %s Config file not found\n", ui.Error(""))
		fmt.Printf("    Run %s to create one.\n", ui.Accent("thts setup"))
		fmt.Println()
		fmt.Println(ui.Error("Configuration invalid."))
		return exitWithCode(1)
	}
	fmt.Printf("  %s Config file found\n", ui.Success(""))

	// Check 2: Config loads (valid YAML)
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			fmt.Printf("  %s Config file not found\n", ui.Error(""))
		} else {
			fmt.Printf("  %s Invalid YAML: %v\n", ui.Error(""), err)
		}
		fmt.Println()
		fmt.Println(ui.Error("Configuration invalid."))
		return exitWithCode(1)
	}
	fmt.Printf("  %s YAML syntax valid\n", ui.Success(""))

	// Check 3: User is set
	if cfg.User == "" {
		fmt.Printf("  %s User not configured\n", ui.Error(""))
		errors++
	} else {
		fmt.Printf("  %s User configured: %s\n", ui.Success(""), cfg.User)
	}

	// Check 4: At least one profile exists
	if len(cfg.Profiles) == 0 {
		fmt.Printf("  %s No profiles configured\n", ui.Error(""))
		errors++
	} else {
		fmt.Printf("  %s %d profile(s) configured\n", ui.Success(""), len(cfg.Profiles))
	}

	// Check 5: Default profile is set
	defaultProfile, defaultName := cfg.GetDefaultProfile()
	if defaultProfile == nil {
		fmt.Printf("  %s No default profile set\n", ui.Error(""))
		errors++
	} else {
		fmt.Printf("  %s Default profile set: %s\n", ui.Success(""), defaultName)
	}

	// Check 6: Each profile has thoughtsRepo
	missingThoughtsRepo := []string{}
	for name, profile := range cfg.Profiles {
		if profile.ThoughtsRepo == "" {
			missingThoughtsRepo = append(missingThoughtsRepo, name)
		}
	}
	if len(missingThoughtsRepo) > 0 {
		for _, name := range missingThoughtsRepo {
			fmt.Printf("  %s Profile %s missing thoughtsRepo\n", ui.Error(""), ui.Accent(name))
			errors++
		}
	} else if len(cfg.Profiles) > 0 {
		fmt.Printf("  %s All profiles have thoughtsRepo\n", ui.Success(""))
	}

	// Check 7: ThoughtsRepo paths exist (warning only)
	for name, profile := range cfg.Profiles {
		if profile.ThoughtsRepo != "" {
			expandedPath := config.ExpandPath(profile.ThoughtsRepo)
			if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
				fmt.Printf("  %s ThoughtsRepo not found: %s (profile: %s)\n", ui.Warning(""), profile.ThoughtsRepo, name)
				warnings++
			}
		}
	}

	// Check 8: RepoMappings reference valid profiles (warning only)
	// Note: RepoMappings are now in state file, not config
	state := config.LoadStateOrDefault()
	invalidMappings := []string{}
	for repoPath, mapping := range state.RepoMappings {
		if mapping != nil && mapping.Profile != "" {
			if !cfg.ValidateProfile(mapping.Profile) {
				invalidMappings = append(invalidMappings, fmt.Sprintf("%s → %s", repoPath, mapping.Profile))
			}
		}
	}
	if len(invalidMappings) > 0 {
		for _, m := range invalidMappings {
			fmt.Printf("  %s Repo mapping references unknown profile: %s\n", ui.Warning(""), m)
			warnings++
		}
	} else if len(state.RepoMappings) > 0 {
		fmt.Printf("  %s All repo mappings reference valid profiles\n", ui.Success(""))
	}

	// Check 9: ComponentMode values are valid
	validModes := map[config.ComponentMode]bool{
		config.ComponentModeGlobal:   true,
		config.ComponentModeLocal:    true,
		config.ComponentModeDisabled: true,
		"":                           true, // empty is valid (defaults to local)
	}
	if cfg.Gitignore != "" && !validModes[cfg.Gitignore] {
		fmt.Printf("  %s Invalid gitignore mode: %s\n", ui.Warning(""), cfg.Gitignore)
		warnings++
	}
	if cfg.Agents != nil {
		if cfg.Agents.Skills != "" && !validModes[cfg.Agents.Skills] {
			fmt.Printf("  %s Invalid agents.skills mode: %s\n", ui.Warning(""), cfg.Agents.Skills)
			warnings++
		}
		if cfg.Agents.Commands != "" && !validModes[cfg.Agents.Commands] {
			fmt.Printf("  %s Invalid agents.commands mode: %s\n", ui.Warning(""), cfg.Agents.Commands)
			warnings++
		}
		if cfg.Agents.Agents != "" && !validModes[cfg.Agents.Agents] {
			fmt.Printf("  %s Invalid agents.agents mode: %s\n", ui.Warning(""), cfg.Agents.Agents)
			warnings++
		}
	}

	// Summary
	fmt.Println()
	if errors > 0 {
		fmt.Printf("%s Fix %d error(s) above and run again.\n", ui.Error("Configuration invalid."), errors)
		return exitWithCode(1)
	}
	if warnings > 0 {
		fmt.Printf("%s with %d warning(s).\n", ui.Success("Configuration valid"), warnings)
	} else {
		fmt.Println(ui.Success("Configuration valid."))
	}

	return nil
}

// exitWithCode returns an error that signals the desired exit code.
// This is a simple approach - cobra will exit with code 1 on any error.
func exitWithCode(code int) error {
	if code == 0 {
		return nil
	}
	// Return a silent error - we've already printed the message
	return silentError{}
}

type silentError struct{}

func (silentError) Error() string { return "" }
