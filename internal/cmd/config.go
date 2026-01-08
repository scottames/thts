package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/scottames/tpd/internal/config"
	"github.com/spf13/cobra"
)

var (
	configEdit bool
	configJSON bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or edit tpd configuration",
	Long: `View or edit the tpd configuration.

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
			fmt.Println(styleWarning.Render("No configuration found."))
			fmt.Printf("Run %s to create one.\n", styleCyan.Render("tpd setup"))
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
	configPath := config.TPDConfigPath()

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println(styleWarning.Render("No configuration file found."))
		fmt.Printf("Run %s to create one first.\n", styleCyan.Render("tpd setup"))
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
	fmt.Println(styleInfo.Render("=== tpd Configuration ==="))
	fmt.Println()

	fmt.Printf("  %s %s\n", styleMuted.Render("Thoughts repo:"), styleCyan.Render(cfg.ThoughtsRepo))
	fmt.Printf("  %s       %s\n", styleMuted.Render("Repos dir:"), cfg.ReposDir)
	fmt.Printf("  %s      %s\n", styleMuted.Render("Global dir:"), cfg.GlobalDir)
	fmt.Printf("  %s           %s\n", styleMuted.Render("User:"), styleSuccess.Render(cfg.User))

	// Optional settings
	if cfg.GitIgnore != "" {
		fmt.Printf("  %s     %s\n", styleMuted.Render("Git ignore:"), string(cfg.GitIgnore))
	}
	fmt.Printf("  %s %v\n", styleMuted.Render("Auto-sync in worktrees:"), cfg.AutoSyncInWorktrees)

	// Profiles
	if len(cfg.Profiles) > 0 {
		fmt.Println()
		fmt.Println(styleInfo.Render("Profiles:"))
		for name, profile := range cfg.Profiles {
			fmt.Printf("  %s:\n", styleCyan.Render(name))
			fmt.Printf("    Thoughts repo: %s\n", profile.ThoughtsRepo)
			fmt.Printf("    Repos dir: %s\n", profile.ReposDir)
			fmt.Printf("    Global dir: %s\n", profile.GlobalDir)
		}
	}

	// Repo mappings
	if len(cfg.RepoMappings) > 0 {
		fmt.Println()
		fmt.Println(styleInfo.Render("Repository Mappings:"))
		for repoPath, mapping := range cfg.RepoMappings {
			displayPath := config.ContractPath(repoPath)
			if mapping.Profile != "" {
				fmt.Printf("  %s → %s %s\n",
					styleMuted.Render(displayPath),
					mapping.Repo,
					styleMuted.Render(fmt.Sprintf("(profile: %s)", mapping.Profile)))
			} else {
				fmt.Printf("  %s → %s\n", styleMuted.Render(displayPath), mapping.Repo)
			}
		}
	}

	fmt.Println()
	fmt.Printf("Config file: %s\n", styleMuted.Render(config.TPDConfigPath()))

	return nil
}
