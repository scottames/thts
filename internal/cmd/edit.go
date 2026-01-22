package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var editProfile string

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open thoughts directory in editor",
	Long:  `Opens the thoughts directory in your configured editor.`,
	RunE:  runEdit,
}

func init() {
	editCmd.Flags().StringVar(&editProfile, "profile", "", "Profile to edit (opens that profile's thoughts repo)")
	_ = editCmd.RegisterFlagCompletionFunc("profile", CompleteProfiles)
	rootCmd.AddCommand(editCmd)
}

func runEdit(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			return fmt.Errorf("thts not configured, run 'thts setup' first")
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve editor
	editor := resolveEditor(cfg)
	if editor == "" {
		fmt.Println(ui.Error("No editor configured"))
		fmt.Println("Set $EDITOR or $VISUAL, or add 'editor' to your config")
		return nil
	}

	// Resolve profile name: flag > env var
	profileName := editProfile
	if profileName == "" {
		profileName = os.Getenv("THTS_PROFILE")
	}

	// Resolve path to open
	path, err := resolveEditPath(cfg, profileName)
	if err != nil {
		return err
	}
	if path == "" {
		// Guidance message already printed
		return nil
	}

	// Open editor
	return openEditor(editor, path)
}

func resolveEditor(cfg *config.Config) string {
	if cfg.Editor != "" {
		return cfg.Editor
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	return os.Getenv("EDITOR")
}

func resolveEditPath(cfg *config.Config, profileName string) (string, error) {
	// Explicit --profile flag
	if profileName != "" {
		if !cfg.ValidateProfile(profileName) {
			return "", fmt.Errorf("profile %q not found", profileName)
		}
		profile := cfg.Profiles[profileName]
		return config.ExpandPath(profile.ThoughtsRepo), nil
	}

	// Check current directory context
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	thoughtsDir := filepath.Join(cwd, "thoughts")

	// Case 1: In thts-enabled repo
	if fs.IsSymlink(filepath.Join(thoughtsDir, "shared")) {
		return thoughtsDir, nil
	}

	// Case 2: In git repo without thts
	if git.IsInGitRepoAt(cwd) {
		_, defaultName := cfg.GetDefaultProfile()
		fmt.Println(ui.Error("thts not enabled in this repo"))
		fmt.Printf("Run %s to enable or %s to edit your default thoughts repo\n",
			ui.Accent("thts init"),
			ui.Accent(fmt.Sprintf("thts edit --profile %s", defaultName)))
		return "", nil
	}

	// Case 3: Not in git repo - use default profile
	defaultProfile, _ := cfg.GetDefaultProfile()
	if defaultProfile == nil {
		return "", fmt.Errorf("no default profile configured")
	}
	return config.ExpandPath(defaultProfile.ThoughtsRepo), nil
}

func openEditor(editor, path string) error {
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
