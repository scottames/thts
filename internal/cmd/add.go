package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/fs"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/ui"
	"github.com/spf13/cobra"
)

// AddOptions holds all options for the add command.
// Designed for extensibility - future input modes can add fields here.
type AddOptions struct {
	// Target selection (mutually exclusive)
	RepoPath    string // --repo: target a specific repo's thoughts
	ProfileName string // --profile: target a profile's global dir

	// Location within target
	Category    string // --in: category/sub-category path
	ForceShared bool   // --shared: override scope to shared/
	ForceUser   bool   // --personal: override scope to {user}/

	// Content
	Title string // positional arg: title for the thought

	// Future extensibility:
	// Content     string // --content: inline content (future)
	// FromFile    string // --from: read content from file (future)
	// FromStdin   bool   // --stdin: read content from stdin (future)
	// NoEdit      bool   // --no-edit: skip opening editor (future)
}

// AddTarget represents the resolved target location for a new thought.
type AddTarget struct {
	ThoughtsDir string // Base thoughts directory (e.g., ~/project/thoughts or ~/thoughts/global)
	ScopePath   string // "shared" or username
	Category    string // Category name (e.g., "plans")
	SubCategory string // Sub-category name (e.g., "active"), empty if none
	FullPath    string // Complete directory path where file will be created
	IsGlobal    bool   // True if targeting global dir (not repo-specific)
}

// AddResult holds the result of an add operation.
type AddResult struct {
	FilePath       string   // Full path to created file
	DirsCreated    []string // Directories that were created
	TemplateUsed   string   // Template filename that was used
	OpenedInEditor bool     // Whether editor was launched
}

var addCmd = &cobra.Command{
	Use:   "add [title]",
	Short: "Add a new thought to the thoughts directory",
	Long: `Add a new thought file to the thoughts directory.

The thought will be created in the specified category (or default category)
with a date-prefixed filename based on the provided title.

Target Resolution (in order):
  1. --repo <path>    Use that repo's thoughts directory
  2. --profile <name> Use that profile's global thoughts directory
  3. Current git repo Use current repo's thoughts directory (if initialized)
  4. Default profile  Use default profile's global directory

Examples:
  thts add "API design decisions"
  thts add --in plans "New feature implementation"
  thts add --in plans/active "Sprint 12 work"
  thts add --in notes --shared "Team gotcha"
  thts add --profile work --in notes "Work note"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().String("in", "", "category/sub-category path (e.g., plans, plans/active)")
	addCmd.Flags().Bool("shared", false, "write to shared/ directory")
	addCmd.Flags().Bool("personal", false, "write to {user}/ directory")
	addCmd.Flags().String("repo", "", "target a specific repo's thoughts directory")
	addCmd.Flags().String("profile", "", "target a specific profile's thoughts repo")

	// Wire up tab completion for --in flag
	_ = addCmd.RegisterFlagCompletionFunc("in", CompleteCategories)
	_ = addCmd.RegisterFlagCompletionFunc("profile", CompleteProfiles)

	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Parse options from flags
	opts, err := parseAddOptions(cmd, args)
	if err != nil {
		return err
	}

	// Validate mutually exclusive flags
	if err := validateAddOptions(opts); err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrConfigNotFound {
			return fmt.Errorf("thts not configured, run 'thts setup' first")
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve target location
	target, err := resolveAddTarget(cfg, opts)
	if err != nil {
		return err
	}

	// Generate filename
	filename := generateFilename(opts.Title)

	// Ensure target directory exists
	dirsCreated, err := ensureTargetDir(target.FullPath)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Get template content
	templateName := cfg.GetTemplate(target.Category, target.SubCategory)
	templateContent := getTemplateContent(target.ThoughtsDir, templateName)

	// Create the file
	filePath := filepath.Join(target.FullPath, filename)
	if err := os.WriteFile(filePath, []byte(templateContent), 0644); err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Report what was created
	printAddResult(filePath, target, dirsCreated, templateName)

	// Open in editor
	editor := resolveEditor(cfg)
	if editor == "" {
		fmt.Println()
		fmt.Println(ui.Info("No editor configured - file created but not opened"))
		fmt.Println(ui.Muted("Set $EDITOR or $VISUAL, or add 'editor' to your config"))
		return nil
	}

	return openEditor(editor, filePath)
}

// parseAddOptions extracts AddOptions from cobra command and args.
func parseAddOptions(cmd *cobra.Command, args []string) (*AddOptions, error) {
	opts := &AddOptions{}

	// Title from positional arg
	if len(args) > 0 {
		opts.Title = args[0]
	}

	// Flags
	var err error
	opts.Category, err = cmd.Flags().GetString("in")
	if err != nil {
		return nil, err
	}

	opts.ForceShared, err = cmd.Flags().GetBool("shared")
	if err != nil {
		return nil, err
	}

	opts.ForceUser, err = cmd.Flags().GetBool("personal")
	if err != nil {
		return nil, err
	}

	opts.RepoPath, err = cmd.Flags().GetString("repo")
	if err != nil {
		return nil, err
	}

	opts.ProfileName, err = cmd.Flags().GetString("profile")
	if err != nil {
		return nil, err
	}

	return opts, nil
}

// validateAddOptions checks for invalid flag combinations.
func validateAddOptions(opts *AddOptions) error {
	// --repo and --profile are mutually exclusive
	if opts.RepoPath != "" && opts.ProfileName != "" {
		return fmt.Errorf("--repo and --profile are mutually exclusive")
	}

	// --shared and --personal are mutually exclusive
	if opts.ForceShared && opts.ForceUser {
		return fmt.Errorf("--shared and --personal are mutually exclusive")
	}

	return nil
}

// resolveAddTarget determines where the thought file should be created.
func resolveAddTarget(cfg *config.Config, opts *AddOptions) (*AddTarget, error) {
	target := &AddTarget{}

	// Parse category/sub-category from --in flag
	target.Category, target.SubCategory = parseCategoryPath(opts.Category)

	// Default category if not specified
	if target.Category == "" {
		target.Category = "notes"
	}

	// Resolve scope (shared vs user)
	target.ScopePath = resolveScopePath(cfg, opts)

	// Resolve thoughts directory based on target selection
	thoughtsDir, isGlobal, err := resolveThoughtsDir(cfg, opts)
	if err != nil {
		return nil, err
	}
	target.ThoughtsDir = thoughtsDir
	target.IsGlobal = isGlobal

	// Build full path
	target.FullPath = buildTargetPath(target)

	return target, nil
}

// parseCategoryPath splits "plans/active" into ("plans", "active").
func parseCategoryPath(categoryPath string) (category, subCategory string) {
	if categoryPath == "" {
		return "", ""
	}

	parts := strings.SplitN(categoryPath, "/", 2)
	category = parts[0]
	if len(parts) > 1 {
		subCategory = parts[1]
	}
	return category, subCategory
}

// resolveScopePath determines whether to use shared/ or {user}/ directory.
func resolveScopePath(cfg *config.Config, opts *AddOptions) string {
	// Explicit flags take precedence
	if opts.ForceShared {
		return "shared"
	}
	if opts.ForceUser {
		return cfg.User
	}

	// Check category's default scope
	if opts.Category != "" {
		category, _ := parseCategoryPath(opts.Category)
		if cat := cfg.GetCategory(category); cat != nil {
			switch cat.GetScope() {
			case config.CategoryScopeShared:
				return "shared"
			case config.CategoryScopeUser:
				return cfg.User
				// CategoryScopeBoth falls through to config default
			}
		}
	}

	// Use config's default scope
	if cfg.GetDefaultScope() == config.ScopeShared {
		return "shared"
	}
	return cfg.User
}

// resolveThoughtsDir determines the base thoughts directory.
// Returns (thoughtsDir, isGlobal, error).
func resolveThoughtsDir(cfg *config.Config, opts *AddOptions) (string, bool, error) {
	// Case 1: --repo flag specified
	if opts.RepoPath != "" {
		return resolveRepoThoughtsDir(cfg, opts.RepoPath)
	}

	// Case 2: --profile flag specified (targets global dir)
	if opts.ProfileName != "" {
		return resolveProfileGlobalDir(cfg, opts.ProfileName)
	}

	// Case 3: In a git repo with thts initialized
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("failed to get current directory: %w", err)
	}

	if git.IsInGitRepoAt(cwd) {
		thoughtsDir := filepath.Join(cwd, "thoughts")
		if isValidThoughtsSetup(thoughtsDir, cfg.User) {
			return thoughtsDir, false, nil
		}
	}

	// Case 4: Fall back to default profile's global dir
	return resolveDefaultGlobalDir(cfg)
}

// resolveRepoThoughtsDir resolves the thoughts directory for a specific repo.
func resolveRepoThoughtsDir(cfg *config.Config, repoPath string) (string, bool, error) {
	// Expand and clean the path
	expandedPath := config.ExpandPath(repoPath)
	if !filepath.IsAbs(expandedPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false, fmt.Errorf("failed to get current directory: %w", err)
		}
		expandedPath = filepath.Join(cwd, expandedPath)
	}

	thoughtsDir := filepath.Join(expandedPath, "thoughts")

	// Verify thts is initialized in that repo
	if !fs.Exists(thoughtsDir) {
		return "", false, fmt.Errorf("thoughts not initialized in %s (run 'thts init' there first)", repoPath)
	}

	// We need to check if it's a valid setup - need to know the user
	// For --repo, we'll just check if shared exists as a basic validation
	if !fs.IsSymlink(filepath.Join(thoughtsDir, "shared")) {
		return "", false, fmt.Errorf("thoughts not properly initialized in %s", repoPath)
	}

	return thoughtsDir, false, nil
}

// resolveProfileGlobalDir resolves the global directory for a specific profile.
func resolveProfileGlobalDir(cfg *config.Config, profileName string) (string, bool, error) {
	if !cfg.ValidateProfile(profileName) {
		return "", false, fmt.Errorf("profile %q not found", profileName)
	}

	profile := cfg.Profiles[profileName]
	expandedRepo := config.ExpandPath(profile.ThoughtsRepo)
	globalDir := filepath.Join(expandedRepo, profile.GlobalDir)

	if !fs.Exists(globalDir) {
		return "", false, fmt.Errorf("global directory for profile %q does not exist: %s", profileName, globalDir)
	}

	return globalDir, true, nil
}

// resolveDefaultGlobalDir resolves the global directory for the default profile.
func resolveDefaultGlobalDir(cfg *config.Config) (string, bool, error) {
	defaultProfile, defaultName := cfg.GetDefaultProfile()
	if defaultProfile == nil {
		return "", false, fmt.Errorf("no default profile configured")
	}

	expandedRepo := config.ExpandPath(defaultProfile.ThoughtsRepo)
	globalDir := filepath.Join(expandedRepo, defaultProfile.GlobalDir)

	if !fs.Exists(globalDir) {
		return "", false, fmt.Errorf("global directory for default profile %q does not exist: %s", defaultName, globalDir)
	}

	return globalDir, true, nil
}

// buildTargetPath constructs the full directory path for the thought file.
func buildTargetPath(target *AddTarget) string {
	// Both global and repo dirs have the same structure: {base}/{scope}/{category}/{subcategory}
	parts := []string{target.ThoughtsDir, target.ScopePath, target.Category}

	if target.SubCategory != "" {
		parts = append(parts, target.SubCategory)
	}

	return filepath.Join(parts...)
}

// generateFilename creates a date-prefixed slug filename.
func generateFilename(title string) string {
	date := time.Now().Format("2006-01-02")

	if title == "" {
		return date + "-untitled.md"
	}

	slug := slugify(title)
	return fmt.Sprintf("%s-%s.md", date, slug)
}

// slugify converts a title to a URL/filename-safe slug.
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove non-alphanumeric characters (except hyphens)
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	s = reg.ReplaceAllString(s, "")

	// Collapse multiple hyphens
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	// Trim hyphens from ends
	s = strings.Trim(s, "-")

	// Limit length
	if len(s) > 50 {
		s = s[:50]
		// Don't end with a hyphen
		s = strings.TrimRight(s, "-")
	}

	if s == "" {
		return "untitled"
	}

	return s
}

// ensureTargetDir creates the target directory if it doesn't exist.
// Returns list of directories that were created.
func ensureTargetDir(targetPath string) ([]string, error) {
	var created []string

	// Walk up to find what exists
	pathsToCreate := []string{}
	checkPath := targetPath

	for !fs.Exists(checkPath) && checkPath != "/" && checkPath != "." {
		pathsToCreate = append([]string{checkPath}, pathsToCreate...)
		checkPath = filepath.Dir(checkPath)
	}

	// Create directories
	for _, p := range pathsToCreate {
		if err := os.MkdirAll(p, 0755); err != nil {
			return created, err
		}
		created = append(created, p)
	}

	return created, nil
}

// getTemplateContent reads template content from .templates/ or returns default.
func getTemplateContent(thoughtsDir, templateName string) string {
	// Try to read from .templates/ in the thoughts dir
	templatePath := filepath.Join(thoughtsDir, ".templates", templateName)
	content, err := os.ReadFile(templatePath)
	if err == nil {
		return string(content)
	}

	// For global dirs, try parent's .templates/ (thoughts repo root)
	// This handles the case where globalDir is a subdirectory
	parentTemplates := filepath.Join(filepath.Dir(thoughtsDir), ".templates", templateName)
	content, err = os.ReadFile(parentTemplates)
	if err == nil {
		return string(content)
	}

	// Return a minimal default template
	return `---
date: ` + time.Now().Format("2006-01-02") + `
tags: []
---

#

`
}

// printAddResult outputs information about the created file.
func printAddResult(filePath string, target *AddTarget, dirsCreated []string, templateUsed string) {
	// Show created directories
	for _, dir := range dirsCreated {
		fmt.Println(ui.InfoF("Created directory: %s", config.ContractPath(dir)))
	}

	// Build location description
	locationDesc := target.ScopePath + "/" + target.Category
	if target.SubCategory != "" {
		locationDesc += "/" + target.SubCategory
	}

	fmt.Println(ui.SuccessF("Created %s", config.ContractPath(filePath)))
	fmt.Printf("  Location: %s\n", ui.Accent(locationDesc))
	if templateUsed != "" && templateUsed != "default.md" {
		fmt.Printf("  Template: %s\n", ui.Muted(templateUsed))
	}
}
