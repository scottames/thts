package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/thts"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	initProfile     string
	initName        string
	initForce       bool
	initInteractive bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize thoughts in the current repository",
	Long: `Initialize thoughts tracking for the current git repository.

This command creates a thoughts/ directory with symlinks to your central
thoughts repository, enabling synchronized note-taking across projects.

The project name is determined by (in order):
  1. --name flag (explicit override)
  2. Git remote URL (extracted repo name)
  3. Interactive selection (--interactive or fallback)`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().
		StringVarP(&initProfile, "profile", "p", "", "Profile to use for this repository")
	initCmd.Flags().
		StringVarP(&initName, "name", "n", "", "Project name (overrides auto-detection)")
	initCmd.Flags().
		BoolVarP(&initForce, "force", "f", false, "Force re-initialization even if thoughts exists")
	initCmd.Flags().
		BoolVarP(&initInteractive, "interactive", "i", false, "Force interactive project name selection")

	_ = initCmd.RegisterFlagCompletionFunc("profile", CompleteProfiles)

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if we're in a git repository
	if !git.IsInGitRepo() {
		return fmt.Errorf("not in a git repository")
	}

	// Get current repo path
	currentRepo, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if config exists
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(ui.Error("thts not configured."))
		fmt.Printf(
			"Run %s first to set up your thoughts repository.\n",
			ui.Accent("thts setup"),
		)
		return nil
	}

	// Validate profile if specified
	if initProfile != "" {
		if !cfg.ValidateProfile(initProfile) {
			fmt.Println(ui.ErrorF("Profile %q does not exist.", initProfile))
			fmt.Println()
			fmt.Println(ui.Muted("Available profiles:"))
			if len(cfg.Profiles) > 0 {
				for name := range cfg.Profiles {
					fmt.Printf("  - %s\n", name)
				}
			} else {
				fmt.Println(ui.Muted("  (none)"))
			}
			fmt.Println()
			fmt.Printf(
				"Create a profile first: %s\n",
				ui.Accent(fmt.Sprintf("thts profile create %s", initProfile)),
			)
			return nil
		}
	}

	// Resolve profile config
	profileConfig := resolveInitProfile(cfg, currentRepo)
	if profileConfig == nil {
		if len(cfg.Profiles) == 0 {
			fmt.Println(ui.Error("No profiles configured."))
			fmt.Printf(
				"Run %s first to create a profile.\n",
				ui.Accent("thts setup"),
			)
		} else {
			fmt.Println(ui.Error("No default profile set."))
			fmt.Printf(
				"Run %s to set a default profile.\n",
				ui.Accent("thts profile set-default <name>"),
			)
		}
		return nil
	}

	// Check existing setup
	thoughtsDir := filepath.Join(currentRepo, "thoughts")
	if fs.Exists(thoughtsDir) && !initForce {
		// Check if it's a valid setup
		if isValidThoughtsSetup(thoughtsDir, cfg.User) {
			fmt.Println(ui.Warning("Thoughts directory already configured for this repository."))
			var reconfigure bool
			err := huh.NewConfirm().
				Title("Do you want to reconfigure?").
				Value(&reconfigure).
				Run()
			if err != nil {
				return err
			}
			if !reconfigure {
				fmt.Println("Setup cancelled.")
				return nil
			}
		} else {
			fmt.Println(ui.Warning("Thoughts directory exists but setup is incomplete."))
			var fix bool
			err := huh.NewConfirm().
				Title("Do you want to fix the setup?").
				Affirmative("Yes").
				Negative("No").
				Value(&fix).
				Run()
			if err != nil {
				return err
			}
			if !fix {
				fmt.Println("Setup cancelled.")
				return nil
			}
		}
	}

	// Determine project name
	projectName, err := determineProjectName(profileConfig, currentRepo)
	if err != nil {
		return err
	}

	if projectName == "" {
		fmt.Println(ui.Error("Could not determine project name."))
		return nil
	}

	fmt.Println()
	fmt.Printf("Setting up thoughts for: %s\n", ui.Accent(currentRepo))
	fmt.Printf("Project name: %s\n", ui.Accent(projectName))
	fmt.Println()

	// Create directory structure in central thoughts repo
	if err := createThoughtsStructure(profileConfig, projectName, cfg.User); err != nil {
		return fmt.Errorf("failed to create thoughts structure: %w", err)
	}

	// Remove existing thoughts directory if it exists
	if fs.Exists(thoughtsDir) {
		if err := fs.RemoveAll(thoughtsDir); err != nil {
			return fmt.Errorf("failed to remove existing thoughts directory: %w", err)
		}
	}

	// Create thoughts directory with symlinks
	if err := createThoughtsSymlinks(thoughtsDir, profileConfig, projectName, cfg.User); err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	// Generate CLAUDE.md for Claude Code integration
	created, err := thts.WriteClaudeMD(thoughtsDir, projectName, cfg.User)
	if err != nil {
		fmt.Println(ui.WarningF("Could not create CLAUDE.md: %v", err))
	} else if created {
		fmt.Println(ui.Success("Created thoughts/CLAUDE.md"))
	}

	// Add to gitignore
	gitignoreMode := cfg.GetGitignoreMode()
	added, err := fs.AddToGitignore(currentRepo, "thoughts/", gitignoreMode)
	if err != nil {
		fmt.Println(ui.WarningF("Could not add to gitignore: %v", err))
	} else if added {
		fmt.Println(ui.SuccessF("Added 'thoughts/' to %s", gitignoreLocationName(gitignoreMode)))
	}

	// Update config with repo mapping
	if cfg.RepoMappings == nil {
		cfg.RepoMappings = make(map[string]*config.RepoMapping)
	}

	mapping := &config.RepoMapping{
		Repo:    projectName,
		Profile: profileConfig.ProfileName,
	}
	cfg.RepoMappings[currentRepo] = mapping

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Install git hooks
	hookOpts := git.HookOptions{
		AutoSyncInWorktrees: cfg.AutoSyncInWorktrees,
	}
	hookResult, err := git.SetupHooks(currentRepo, hookOpts)
	if err != nil {
		fmt.Println(ui.WarningF("Could not install git hooks: %v", err))
	} else if len(hookResult.Updated) > 0 {
		fmt.Println(ui.SuccessF("Installed git hooks: %s", strings.Join(hookResult.Updated, ", ")))
	}

	// Print success summary
	fmt.Println()
	fmt.Println(ui.Success("Thoughts setup complete!"))
	fmt.Println()
	fmt.Println(ui.Header("Summary"))
	fmt.Println()
	fmt.Println("Repository structure created:")
	fmt.Printf("  %s/\n", ui.Accent(config.ContractPath(currentRepo)))
	fmt.Printf("    └── thoughts/\n")
	fmt.Printf(
		"         ├── %s/     %s\n",
		cfg.User,
		ui.Muted(
			fmt.Sprintf(
				"→ %s/%s/%s/%s/",
				config.ContractPath(profileConfig.ThoughtsRepo),
				profileConfig.ReposDir,
				projectName,
				cfg.User,
			),
		),
	)
	fmt.Printf(
		"         ├── shared/      %s\n",
		ui.Muted(
			fmt.Sprintf(
				"→ %s/%s/%s/shared/",
				config.ContractPath(profileConfig.ThoughtsRepo),
				profileConfig.ReposDir,
				projectName,
			),
		),
	)
	fmt.Printf(
		"         ├── global/      %s\n",
		ui.Muted(
			fmt.Sprintf(
				"→ %s/%s/",
				config.ContractPath(profileConfig.ThoughtsRepo),
				profileConfig.GlobalDir,
			),
		),
	)
	fmt.Printf("         └── CLAUDE.md    %s\n", ui.Muted("(Claude Code documentation)"))
	fmt.Println()
	fmt.Println("Protection enabled:")
	fmt.Println(ui.Success("Pre-commit hook: Prevents committing thoughts/"))
	fmt.Println(ui.Success("Post-commit hook: Auto-syncs thoughts after commits"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Run %s to sync and create the searchable index\n", ui.Accent("thts sync"))
	fmt.Printf("  2. Create markdown files in %s for your notes\n", ui.Accent(fmt.Sprintf("thoughts/%s/", cfg.User)))
	fmt.Println()

	return nil
}

// resolveInitProfile resolves the profile configuration for init.
func resolveInitProfile(cfg *config.Config, repoPath string) *config.ResolvedProfile {
	// If profile flag is set, use that profile
	if initProfile != "" && cfg.Profiles != nil {
		if profile, exists := cfg.Profiles[initProfile]; exists {
			return &config.ResolvedProfile{
				ThoughtsRepo: profile.ThoughtsRepo,
				ReposDir:     profile.ReposDir,
				GlobalDir:    profile.GlobalDir,
				ProfileName:  initProfile,
			}
		}
	}

	// Use default profile
	defaultProfile, defaultName := cfg.GetDefaultProfile()
	if defaultProfile == nil {
		return nil
	}

	return &config.ResolvedProfile{
		ThoughtsRepo: defaultProfile.ThoughtsRepo,
		ReposDir:     defaultProfile.ReposDir,
		GlobalDir:    defaultProfile.GlobalDir,
		ProfileName:  defaultName,
	}
}

// determineProjectName determines the project name for this repository.
func determineProjectName(profile *config.ResolvedProfile, repoPath string) (string, error) {
	// 1. Check --name flag
	if initName != "" {
		return git.SanitizeRepoName(initName), nil
	}

	// 2. Try git remote
	if !initInteractive {
		remoteURL, err := git.GetRemoteURL()
		if err == nil && remoteURL != "" {
			repoName := git.GetRepoNameFromRemote(remoteURL)
			if repoName != "" {
				return repoName, nil
			}
		}
	}

	// 3. Interactive selection
	return interactiveProjectName(profile, repoPath)
}

// interactiveProjectName prompts for project name selection.
func interactiveProjectName(profile *config.ResolvedProfile, repoPath string) (string, error) {
	expandedRepo := config.ExpandPath(profile.ThoughtsRepo)
	reposDir := filepath.Join(expandedRepo, profile.ReposDir)

	// Get existing repo directories
	var existingRepos []string
	entries, err := os.ReadDir(reposDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() && !startsWithDot(entry.Name()) {
				existingRepos = append(existingRepos, entry.Name())
			}
		}
	}

	// Default name from directory
	defaultName := filepath.Base(repoPath)

	// Build options
	var options []huh.Option[string]

	// Add "Create new" option with default name
	options = append(
		options,
		huh.NewOption(fmt.Sprintf("Create new: %s", defaultName), "new:"+defaultName),
	)

	// Add existing repos
	for _, repo := range existingRepos {
		options = append(
			options,
			huh.NewOption(fmt.Sprintf("Use existing: %s", repo), "existing:"+repo),
		)
	}

	// Add "Custom name" option
	options = append(options, huh.NewOption("Enter custom name...", "custom"))

	fmt.Println(ui.Header("Project Name Selection"))
	fmt.Println()
	fmt.Printf("Setting up thoughts for: %s\n", ui.Accent(repoPath))
	fmt.Printf("Thoughts will be stored in: %s/%s/\n",
		ui.Accent(config.ContractPath(expandedRepo)),
		ui.Accent(profile.ReposDir))
	fmt.Println()

	var selection string
	err = huh.NewSelect[string]().
		Title("Select or create a project directory:").
		Options(options...).
		Value(&selection).
		Run()
	if err != nil {
		return "", err
	}

	// Handle selection
	switch {
	case selection == "custom":
		var customName string
		err = huh.NewInput().
			Title("Enter project name:").
			Placeholder(defaultName).
			Value(&customName).
			Run()
		if err != nil {
			return "", err
		}
		if customName == "" {
			customName = defaultName
		}
		return git.SanitizeRepoName(customName), nil

	case len(selection) > 4 && selection[:4] == "new:":
		return git.SanitizeRepoName(selection[4:]), nil

	case len(selection) > 9 && selection[:9] == "existing:":
		return selection[9:], nil

	default:
		return git.SanitizeRepoName(defaultName), nil
	}
}

// isValidThoughtsSetup checks if the thoughts directory has valid symlinks.
func isValidThoughtsSetup(thoughtsDir, user string) bool {
	userPath := filepath.Join(thoughtsDir, user)
	sharedPath := filepath.Join(thoughtsDir, "shared")
	globalPath := filepath.Join(thoughtsDir, "global")

	return fs.IsSymlink(userPath) && fs.IsSymlink(sharedPath) && fs.IsSymlink(globalPath)
}

// createThoughtsStructure creates the directory structure in the central thoughts repo.
func createThoughtsStructure(profile *config.ResolvedProfile, projectName, user string) error {
	expandedRepo := config.ExpandPath(profile.ThoughtsRepo)

	// Create repo-specific directories
	repoThoughtsPath := filepath.Join(expandedRepo, profile.ReposDir, projectName)
	repoUserPath := filepath.Join(repoThoughtsPath, user)
	repoSharedPath := filepath.Join(repoThoughtsPath, "shared")

	// Create global directories
	globalPath := filepath.Join(expandedRepo, profile.GlobalDir)
	globalUserPath := filepath.Join(globalPath, user)
	globalSharedPath := filepath.Join(globalPath, "shared")

	// Create all directories
	for _, dir := range []string{repoUserPath, repoSharedPath, globalUserPath, globalSharedPath} {
		if err := fs.EnsureDir(dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create README files if they don't exist
	repoReadme := filepath.Join(repoThoughtsPath, "README.md")
	if !fs.Exists(repoReadme) {
		content := fmt.Sprintf(`# %s Thoughts

This directory contains thoughts and notes specific to the %s repository.

- `+"`%s/`"+` - Your personal notes for this repository
- `+"`shared/`"+` - Team-shared notes for this repository
`, projectName, projectName, user)
		if err := os.WriteFile(repoReadme, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create README: %w", err)
		}
	}

	globalReadme := filepath.Join(globalPath, "README.md")
	if !fs.Exists(globalReadme) {
		content := fmt.Sprintf(`# Global Thoughts

This directory contains thoughts and notes that apply across all repositories.

- `+"`%s/`"+` - Your personal cross-repository notes
- `+"`shared/`"+` - Team-shared cross-repository notes
`, user)
		if err := os.WriteFile(globalReadme, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create global README: %w", err)
		}
	}

	fmt.Println(ui.Success("Thoughts structure created in central repository"))
	return nil
}

// createThoughtsSymlinks creates the thoughts directory with symlinks.
func createThoughtsSymlinks(
	thoughtsDir string,
	profile *config.ResolvedProfile,
	projectName, user string,
) error {
	// Create thoughts directory
	if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
		return fmt.Errorf("failed to create thoughts directory: %w", err)
	}

	expandedRepo := config.ExpandPath(profile.ThoughtsRepo)
	repoThoughtsPath := filepath.Join(expandedRepo, profile.ReposDir, projectName)
	globalPath := filepath.Join(expandedRepo, profile.GlobalDir)

	// Create symlinks
	symlinks := map[string]string{
		filepath.Join(thoughtsDir, user):     filepath.Join(repoThoughtsPath, user),
		filepath.Join(thoughtsDir, "shared"): filepath.Join(repoThoughtsPath, "shared"),
		filepath.Join(thoughtsDir, "global"): globalPath,
	}

	for linkPath, target := range symlinks {
		if err := fs.CreateSymlink(target, linkPath); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", linkPath, target, err)
		}
	}

	fmt.Println(ui.Success("Symlinks created"))
	return nil
}

// gitignoreLocationName returns a human-readable name for the gitignore location.
func gitignoreLocationName(mode config.ComponentMode) string {
	switch mode {
	case config.ComponentModeLocal:
		return ".gitignore"
	case config.ComponentModeGlobal:
		return "global gitignore (~/.config/git/ignore)"
	default:
		return ".gitignore"
	}
}

// startsWithDot returns true if the string starts with a dot.
func startsWithDot(s string) bool {
	return len(s) > 0 && s[0] == '.'
}
