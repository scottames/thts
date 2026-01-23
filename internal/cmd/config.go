package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/muesli/termenv"
	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	configEdit bool
	configJSON bool

	// dump-default flags
	dumpDefaultJSON bool
	dumpDefaultYAML bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or edit thts configuration",
	Long: `View or edit the thts configuration.

By default, displays the current configuration in a human-readable format.

Use --edit to open the config file in your default editor ($EDITOR).
Use --json to output the configuration as JSON.`,
	Args: cobra.NoArgs,
	RunE: runConfig,
}

var dumpDefaultCmd = &cobra.Command{
	Use:   "dump-default",
	Short: "Output the default configuration",
	Long: `Output the default thts configuration with all available options.

This helps users see what settings are available and their default values.
Use --yaml to output in YAML format with comments (suitable for config file).
Use --json to output in JSON format.`,
	RunE: runDumpDefault,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().BoolVarP(&configEdit, "edit", "e", false, "Open config in editor")
	configCmd.Flags().BoolVar(&configJSON, "json", false, "Output as JSON")

	// dump-default subcommand
	configCmd.AddCommand(dumpDefaultCmd)
	dumpDefaultCmd.Flags().BoolVar(&dumpDefaultJSON, "json", false, "Output as JSON")
	dumpDefaultCmd.Flags().BoolVar(&dumpDefaultYAML, "yaml", false, "Output as YAML with comments")
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
	// Show sync config if set
	if cfg.Sync != nil && cfg.Sync.Mode != "" {
		generalRows = append(generalRows, []string{"Sync: mode", string(cfg.Sync.Mode)})
	}
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

	// Repo mappings from state file
	state := config.LoadStateOrDefault()
	if len(state.RepoMappings) > 0 {
		fmt.Println()
		fmt.Println(ui.SubHeader("Repository Mappings"))
		fmt.Printf("  %s\n", ui.Muted(fmt.Sprintf("(from %s)", config.ContractPath(config.StatePath()))))
		for repoPath, mapping := range state.RepoMappings {
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
	fmt.Printf("State file:  %s\n", ui.Muted(config.StatePath()))

	return nil
}

func runDumpDefault(_ *cobra.Command, _ []string) error {
	if dumpDefaultYAML {
		return outputYAML(config.ConfigTemplate)
	}

	cfg := config.FullDefaults()

	if dumpDefaultJSON {
		return outputJSON(cfg)
	}

	return displayDefaultConfig(cfg)
}

// outputYAML outputs YAML content with syntax highlighting when stdout is a TTY.
// Falls back to plain text when redirected to a file or pipe.
func outputYAML(content string) error {
	output := termenv.NewOutput(os.Stdout)
	profile := output.ColorProfile()
	if profile == termenv.Ascii {
		// No color support - output plain text
		fmt.Print(content)
		return nil
	}

	// Pick formatter based on terminal color depth
	formatter := "terminal256"
	if profile == termenv.TrueColor {
		formatter = "terminal16m"
	}

	// Use a style that works well in both light and dark terminals
	return quick.Highlight(os.Stdout, content, "yaml", formatter, "catppuccin-mocha")
}

func displayDefaultConfig(cfg *config.Config) error {
	fmt.Println(ui.Header("thts Default Configuration"))
	fmt.Println()
	fmt.Println("These are the default values used when options are not set in your config.")
	fmt.Println()

	// General settings
	fmt.Println(ui.SubHeader("General"))
	generalRows := [][]string{
		{"Auto-sync in worktrees", fmt.Sprintf("%v", cfg.AutoSyncInWorktrees)},
		{"Gitignore", string(cfg.Gitignore)},
		{"Default scope", string(cfg.DefaultScope)},
		{"Default template", cfg.DefaultTemplate},
	}
	fmt.Println(ui.KeyValueTable(generalRows))

	// Sync settings
	fmt.Println()
	fmt.Println(ui.SubHeader("Sync"))
	syncRows := [][]string{
		{"Mode", string(cfg.Sync.Mode)},
	}
	fmt.Println(ui.KeyValueTable(syncRows))

	// Agents settings
	fmt.Println()
	fmt.Println(ui.SubHeader("Agents"))
	agentsRows := [][]string{
		{"Skills", string(cfg.Agents.Skills)},
		{"Commands", string(cfg.Agents.Commands)},
		{"Agents", string(cfg.Agents.Agents)},
	}
	fmt.Println(ui.KeyValueTable(agentsRows))

	// Hook keywords
	fmt.Println()
	fmt.Println(ui.SubHeader("Hook Keywords"))
	fmt.Println()
	fmt.Printf("  %s\n", ui.Muted(strings.Join(cfg.Hooks.Keywords, ", ")))

	// Categories
	fmt.Println()
	fmt.Println(ui.SubHeader("Categories"))
	for name, cat := range cfg.Categories {
		fmt.Println()
		fmt.Printf("  %s:\n", ui.Accent(name))
		fmt.Printf("    Description: %s\n", cat.Description)
		if cat.Trigger != "" {
			fmt.Printf("    Trigger:     %s\n", cat.Trigger)
		}
		if cat.Template != "" {
			fmt.Printf("    Template:    %s\n", cat.Template)
		}
		fmt.Printf("    Scope:       %s\n", cat.Scope)
	}

	// Default profile
	fmt.Println()
	fmt.Println(ui.SubHeader("Default Profile"))
	if profile, exists := cfg.Profiles["default"]; exists {
		profileRows := [][]string{
			{"Thoughts repo", profile.ThoughtsRepo},
			{"Repos dir", profile.ReposDir},
			{"Global dir", profile.GlobalDir},
		}
		fmt.Println(ui.KeyValueTable(profileRows))
	}

	fmt.Println()
	fmt.Printf("Use %s to get a config file template.\n", ui.Accent("--yaml"))

	return nil
}
