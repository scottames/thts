package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	thtsfiles "github.com/scottames/thts"
	"github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

var (
	createRepo          string
	createReposDir      string
	createGlobalDir     string
	createDefaultAgents string
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new profile",
	Long: `Create a new profile for a separate thoughts repository.

Profiles allow you to have different thoughts repositories for different contexts
(e.g., work vs personal projects).

You can specify all options via flags for non-interactive usage, or run
interactively to be prompted for each option.`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVar(&createRepo, "repo", "", "Thoughts repository path for this profile")
	createCmd.Flags().StringVar(&createReposDir, "repos-dir", "", "Repository-specific thoughts directory name (default: repos)")
	createCmd.Flags().StringVar(&createGlobalDir, "global-dir", "", "Global thoughts directory name (default: global)")
	createCmd.Flags().StringVar(&createDefaultAgents, "default-agents", "", "Comma-separated list of default agents (claude,codex,opencode)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	// Load existing config
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			fmt.Println(ui.Error("Thoughts not configured."))
			fmt.Printf("Run %s first to set up the base configuration.\n", ui.Accent("thts setup"))
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Sanitize profile name
	sanitizedName := config.SanitizeProfileName(profileName)
	if sanitizedName != profileName {
		fmt.Println(ui.WarningF("Profile name sanitized: %q → %q", profileName, sanitizedName))
	}

	// Check if profile already exists
	if cfg.ValidateProfile(sanitizedName) {
		fmt.Println(ui.ErrorF("Profile %q already exists.", sanitizedName))
		fmt.Println("Use a different name or delete the existing profile first.")
		return nil
	}

	// Get profile configuration
	var thoughtsRepo, reposDir, globalDir string
	var defaultAgentsList []string

	// Parse default agents flag
	if createDefaultAgents != "" {
		agentTypes, err := agents.ParseAgentTypes(createDefaultAgents)
		if err != nil {
			return fmt.Errorf("invalid default agents: %w", err)
		}
		defaultAgentsList = agents.AgentTypesToStrings(agentTypes)
	}

	if createRepo != "" {
		// Non-interactive mode (at least repo is specified)
		thoughtsRepo = createRepo
		reposDir = createReposDir
		if reposDir == "" {
			reposDir = "repos"
		}
		globalDir = createGlobalDir
		if globalDir == "" {
			globalDir = "global"
		}
	} else {
		// Interactive mode
		fmt.Println()
		fmt.Println(ui.Header(fmt.Sprintf("Creating Profile: %s", sanitizedName)))
		fmt.Println()

		defaultRepo := config.DefaultThoughtsRepo() + "-" + sanitizedName

		fmt.Println(ui.Muted("Specify the thoughts repository location for this profile."))

		err = huh.NewInput().
			Title("Thoughts repository").
			Placeholder(defaultRepo).
			Value(&thoughtsRepo).
			Run()
		if err != nil {
			return err
		}
		if thoughtsRepo == "" {
			thoughtsRepo = defaultRepo
		}

		fmt.Println()

		err = huh.NewInput().
			Title("Repository-specific thoughts directory").
			Placeholder("repos").
			Value(&reposDir).
			Run()
		if err != nil {
			return err
		}
		if reposDir == "" {
			reposDir = "repos"
		}

		err = huh.NewInput().
			Title("Global thoughts directory").
			Placeholder("global").
			Value(&globalDir).
			Run()
		if err != nil {
			return err
		}
		if globalDir == "" {
			globalDir = "global"
		}

		// Ask about default agents in interactive mode
		if len(defaultAgentsList) == 0 {
			var selectedAgents []string
			var options []huh.Option[string]
			for _, at := range agents.AllAgentTypes() {
				label := fmt.Sprintf("%s (%s)", at, agents.AgentTypeLabels[at])
				options = append(options, huh.NewOption(label, string(at)))
			}

			err = huh.NewMultiSelect[string]().
				Title("Default agents for this profile (optional)").
				Description("These will be used by 'thts agents init' when no --agents flag is provided").
				Options(options...).
				Value(&selectedAgents).
				Run()
			if err != nil {
				return err
			}
			defaultAgentsList = selectedAgents
		}
	}

	// Create profile config
	profileConfig := &config.ProfileConfig{
		ThoughtsRepo:  thoughtsRepo,
		ReposDir:      reposDir,
		GlobalDir:     globalDir,
		DefaultAgents: defaultAgentsList,
	}

	// Initialize profiles map if nil
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]*config.ProfileConfig)
	}

	// Add profile
	cfg.Profiles[sanitizedName] = profileConfig

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create the profile's thoughts repository structure
	fmt.Println()
	fmt.Println(ui.Muted("Initializing profile thoughts repository..."))
	if err := ensureProfileRepoExists(sanitizedName, profileConfig); err != nil {
		return fmt.Errorf("failed to create profile repository: %w", err)
	}

	fmt.Println()
	fmt.Println(ui.SuccessF("Profile %q created successfully!", sanitizedName))
	fmt.Println()
	fmt.Println(ui.Header("Profile Configuration"))
	fmt.Printf("  Name: %s\n", ui.Accent(sanitizedName))
	fmt.Printf("  Thoughts repository: %s\n", ui.Accent(thoughtsRepo))
	fmt.Printf("  Repos directory: %s\n", ui.Accent(reposDir))
	fmt.Printf("  Global directory: %s\n", ui.Accent(globalDir))
	if len(defaultAgentsList) > 0 {
		fmt.Printf("  Default agents: %s\n", ui.Accent(strings.Join(defaultAgentsList, ", ")))
	}
	fmt.Println()
	fmt.Println(ui.Muted("Next steps:"))
	fmt.Printf("  1. Run %s in a repository\n", ui.Accent(fmt.Sprintf("thts init --profile %s", sanitizedName)))
	fmt.Println("  2. Your thoughts will sync to the profile's repository")

	return nil
}

// ensureProfileRepoExists creates the profile's thoughts repo structure and initializes git if needed.
func ensureProfileRepoExists(profileName string, profile *config.ProfileConfig) error {
	expandedRepo := config.ExpandPath(profile.ThoughtsRepo)

	// Create thoughts repo if it doesn't exist
	if err := os.MkdirAll(expandedRepo, 0755); err != nil {
		return fmt.Errorf("failed to create thoughts repo: %w", err)
	}

	// Create subdirectories
	reposDir := filepath.Join(expandedRepo, profile.ReposDir)
	globalDir := filepath.Join(expandedRepo, profile.GlobalDir)

	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return fmt.Errorf("failed to create repos dir: %w", err)
	}

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return fmt.Errorf("failed to create global dir: %w", err)
	}

	// Create README if it doesn't exist
	readmePath := filepath.Join(expandedRepo, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		readmeContent, err := thtsfiles.GetDefaultReadme(thtsfiles.ReadmeData{
			Profile:   profileName,
			ReposDir:  profile.ReposDir,
			GlobalDir: profile.GlobalDir,
		})
		if err != nil {
			return fmt.Errorf("failed to generate README: %w", err)
		}
		if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
			return fmt.Errorf("failed to create README: %w", err)
		}
	}

	// Check if it's already a git repo
	gitPath := filepath.Join(expandedRepo, ".git")
	info, err := os.Stat(gitPath)
	if err == nil && (info.IsDir() || info.Mode().IsRegular()) {
		// Already a git repo
		fmt.Println(ui.Success("Profile thoughts repository exists"))
		return nil
	}

	// Initialize as git repo
	fmt.Println(ui.Info("Initializing profile thoughts repository as git repo..."))

	gitInit := exec.Command("git", "init")
	gitInit.Dir = expandedRepo
	if err := gitInit.Run(); err != nil {
		return fmt.Errorf("failed to init git repo: %w", err)
	}

	// Create initial .gitignore
	gitignoreContent := `# OS files
.DS_Store
Thumbs.db

# Editor files
.vscode/
.idea/
*.swp
*.swo
*~

# Temporary files
*.tmp
*.bak
`
	gitignorePath := filepath.Join(expandedRepo, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	// Initial commit
	gitAdd := exec.Command("git", "add", ".gitignore", "README.md")
	gitAdd.Dir = expandedRepo
	if err := gitAdd.Run(); err != nil {
		return fmt.Errorf("failed to add files: %w", err)
	}

	gitCommit := exec.Command("git", "commit", "-m", "Initial thoughts repository setup")
	gitCommit.Dir = expandedRepo
	if err := gitCommit.Run(); err != nil {
		return fmt.Errorf("failed to create initial commit: %w", err)
	}

	fmt.Println(ui.Success("Profile thoughts repository initialized"))
	return nil
}
