package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	configEdit bool
	configJSON bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or edit thts configuration",
	Long: `View or edit the thts configuration.

By default, displays the current configuration in a human-readable format.

Use --edit to open the config file in your default editor ($EDITOR).
Use --json to output the configuration as JSON.`,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().BoolVarP(&configEdit, "edit", "e", false, "Open config in editor")
	configCmd.Flags().BoolVar(&configJSON, "json", false, "Output as JSON")
}

func runConfig(cmd *cobra.Command, args []string) error {
	if configEdit {
		return editConfig()
	}

	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			fmt.Println(ui.Warning("No configuration found."))
			fmt.Printf("Run %s to create one.\n", ui.Accent("thts setup"))
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	if configJSON {
		return outputJSON(cfg)
	}

	return displayConfig(cfg)
}

func editConfig() error {
	configPath := config.ThtsConfigPath()

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println(ui.Warning("No configuration file found."))
		fmt.Printf("Run %s to create one first.\n", ui.Accent("thts setup"))
		return nil
	}

	// Get editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	// Open editor
	editorCmd := exec.Command(editor, configPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	return nil
}

func outputJSON(cfg *config.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func displayConfig(cfg *config.Config) error {
	fmt.Println(ui.Header("thts Configuration"))
	fmt.Println()

	// General settings table
	generalRows := [][]string{
		{"User", cfg.User},
	}
	if cfg.Gitignore != "" {
		generalRows = append(generalRows, []string{"Gitignore", string(cfg.Gitignore)})
	}
	generalRows = append(generalRows, []string{"Auto-sync in worktrees", fmt.Sprintf("%v", cfg.AutoSyncInWorktrees)})
	// Show agents config if set
	if cfg.Agents != nil {
		if cfg.Agents.Skills != "" {
			generalRows = append(generalRows, []string{"Agents: skills", string(cfg.Agents.Skills)})
		}
		if cfg.Agents.Commands != "" {
			generalRows = append(generalRows, []string{"Agents: commands", string(cfg.Agents.Commands)})
		}
		if cfg.Agents.Agents != "" {
			generalRows = append(generalRows, []string{"Agents: agents", string(cfg.Agents.Agents)})
		}
	}
	fmt.Println(ui.KeyValueTable(generalRows))

	// Profiles
	fmt.Println()
	fmt.Println(ui.SubHeader("Profiles"))
	if len(cfg.Profiles) == 0 {
		fmt.Println(ui.Muted("  No profiles configured."))
		fmt.Printf("  Run %s to create one.\n", ui.Accent("thts setup"))
	} else {
		for name, profile := range cfg.Profiles {
			nameDisplay := name
			if profile.Default {
				nameDisplay = name + " *"
			}
			fmt.Printf("  %s:\n", ui.Accent(nameDisplay))
			fmt.Printf("    Thoughts repo: %s\n", profile.ThoughtsRepo)
			fmt.Printf("    Repos dir: %s\n", profile.ReposDir)
			fmt.Printf("    Global dir: %s\n", profile.GlobalDir)
		}
		fmt.Println()
		fmt.Println(ui.Muted("  * = default profile"))
	}

	// Repo mappings
	if len(cfg.RepoMappings) > 0 {
		fmt.Println()
		fmt.Println(ui.SubHeader("Repository Mappings"))
		for repoPath, mapping := range cfg.RepoMappings {
			displayPath := config.ContractPath(repoPath)
			if mapping.Profile != "" {
				fmt.Printf("  %s → %s %s\n",
					ui.Muted(displayPath),
					mapping.Repo,
					ui.Muted(fmt.Sprintf("(profile: %s)", mapping.Profile)))
			} else {
				fmt.Printf("  %s → %s\n", ui.Muted(displayPath), mapping.Repo)
			}
		}
	}

	fmt.Println()
	fmt.Printf("Config file: %s\n", ui.Muted(config.ThtsConfigPath()))

	return nil
}
