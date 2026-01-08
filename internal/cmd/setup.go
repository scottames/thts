package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/scottames/tpd/internal/config"
	"github.com/spf13/cobra"
)

var (
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue
	styleWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	styleMuted   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // gray
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "First-time setup for tpd",
	Long: `Configure tpd for first-time use.

This command will prompt you for:
- The location of your thoughts repository
- Your username (used for personal notes directories)

The thoughts repository will be initialized as a git repo if it doesn't exist.`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	// Check if config already exists
	if config.Exists() {
		cfg, _ := config.Load()
		fmt.Println(styleWarning.Render("Configuration already exists:"))
		fmt.Printf("  Thoughts repo: %s\n", styleCyan.Render(cfg.ThoughtsRepo))
		fmt.Printf("  User: %s\n", styleCyan.Render(cfg.User))
		fmt.Println()

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
	}

	fmt.Println(styleInfo.Render("=== tpd Setup ==="))
	fmt.Println()
	fmt.Println("Let's configure your thoughts system.")
	fmt.Println()

	// Get thoughts repository location
	defaultRepo := config.DefaultThoughtsRepo()
	var thoughtsRepo string

	err := huh.NewInput().
		Title("Thoughts repository location").
		Description("This is where all your thoughts across all projects will be stored.").
		Placeholder(defaultRepo).
		Value(&thoughtsRepo).
		Run()
	if err != nil {
		return err
	}
	if thoughtsRepo == "" {
		thoughtsRepo = defaultRepo
	}

	// Get username
	defaultUser := config.DefaultUser()
	var user string

	for {
		err = huh.NewInput().
			Title("Your username").
			Description("Used for your personal notes directories.").
			Placeholder(defaultUser).
			Value(&user).
			Run()
		if err != nil {
			return err
		}
		if user == "" {
			user = defaultUser
		}

		// Validate username
		if strings.ToLower(user) == "global" {
			fmt.Println(styleError.Render("Username cannot be \"global\" - it's reserved for cross-project thoughts."))
			user = ""
			continue
		}
		if strings.ToLower(user) == "shared" {
			fmt.Println(styleError.Render("Username cannot be \"shared\" - it's reserved for team-shared notes."))
			user = ""
			continue
		}
		break
	}

	// Create config
	cfg := config.Defaults()
	cfg.ThoughtsRepo = thoughtsRepo
	cfg.User = user

	// Show what will be created
	fmt.Println()
	fmt.Println(styleWarning.Render("Creating thoughts structure:"))
	displayRepo := config.ContractPath(config.ExpandPath(thoughtsRepo))
	fmt.Printf("  %s/\n", styleCyan.Render(displayRepo))
	fmt.Printf("    ├── %s/     %s\n", styleCyan.Render(cfg.ReposDir), styleMuted.Render("(project-specific thoughts)"))
	fmt.Printf("    └── %s/    %s\n", styleCyan.Render(cfg.GlobalDir), styleMuted.Render("(cross-project thoughts)"))
	fmt.Println()

	// Ensure thoughts repo exists and is a git repo
	if err := ensureThoughtsRepoExists(cfg); err != nil {
		return fmt.Errorf("failed to create thoughts repository: %w", err)
	}

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(styleSuccess.Render("✓ Configuration saved to ") + styleMuted.Render(config.TPDConfigPath()))
	fmt.Println()
	fmt.Println(styleSuccess.Render("✅ Setup complete!"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Navigate to a git repository\n")
	fmt.Printf("  2. Run %s to initialize thoughts for that project\n", styleCyan.Render("tpd init"))
	fmt.Println()

	return nil
}

// ensureThoughtsRepoExists creates the thoughts repo structure and initializes git if needed.
func ensureThoughtsRepoExists(cfg *config.Config) error {
	expandedRepo := config.ExpandPath(cfg.ThoughtsRepo)

	// Create thoughts repo if it doesn't exist
	if err := os.MkdirAll(expandedRepo, 0755); err != nil {
		return fmt.Errorf("failed to create thoughts repo: %w", err)
	}

	// Create subdirectories
	reposDir := filepath.Join(expandedRepo, cfg.ReposDir)
	globalDir := filepath.Join(expandedRepo, cfg.GlobalDir)

	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return fmt.Errorf("failed to create repos dir: %w", err)
	}

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return fmt.Errorf("failed to create global dir: %w", err)
	}

	// Check if it's already a git repo
	gitPath := filepath.Join(expandedRepo, ".git")
	info, err := os.Stat(gitPath)
	if err == nil && (info.IsDir() || info.Mode().IsRegular()) {
		// Already a git repo (either .git dir or .git file for worktree)
		fmt.Println(styleSuccess.Render("✓ Thoughts repository exists"))
		return nil
	}

	// Initialize as git repo
	fmt.Println(styleInfo.Render("Initializing thoughts repository as git repo..."))

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
	gitAdd := exec.Command("git", "add", ".gitignore")
	gitAdd.Dir = expandedRepo
	if err := gitAdd.Run(); err != nil {
		return fmt.Errorf("failed to add .gitignore: %w", err)
	}

	gitCommit := exec.Command("git", "commit", "-m", "Initial thoughts repository setup")
	gitCommit.Dir = expandedRepo
	if err := gitCommit.Run(); err != nil {
		return fmt.Errorf("failed to create initial commit: %w", err)
	}

	fmt.Println(styleSuccess.Render("✓ Thoughts repository initialized"))
	return nil
}
