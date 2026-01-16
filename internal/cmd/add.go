package cmd

import (
	"fmt"
	"io"
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

	// Title
	Title string // --title/-t: title for the thought

	// Content input (mutually exclusive)
	Content   string // positional arg: inline content
	FromFile  string // --from: read content from file
	FromStdin bool   // --stdin: read content from stdin

	// Editor control
	NoEdit bool // --no-edit: skip opening editor

	// Output control
	Quiet bool // --quiet/-q: only output file path to stdout
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
	Use:   "add [content]",
	Short: "Add a new thought to the thoughts directory",
	Long: `Add a new thought file to the thoughts directory.

The thought will be created in the specified category (or default category)
with a date-prefixed filename based on the provided title.

Target Resolution (in order):
  1. --repo <path>    Use that repo's thoughts directory
  2. --profile <name> Use that profile's global thoughts directory
  3. Current git repo Use current repo's thoughts directory (if initialized)
  4. Default profile  Use default profile's global directory

Content Sources (mutually exclusive):
  [content]          Positional argument
  --from <file>      Read content from file
  --stdin            Read content from stdin (auto-detected when piped)

By default, opens the editor after creating the file. Use --no-edit to skip.
Piped input is automatically detected - no need to specify --stdin explicitly.

Examples:
  thts add -t "API design decisions"
  thts add -t "New feature" --in plans
  thts add -t "Sprint 12 work" --in plans/active
  thts add -t "Team gotcha" --in notes --shared
  thts add -t "Work note" --profile work --in notes
  echo "Quick note" | thts add -t "piped note" --in notes
  thts add -t "imported plan" --from draft.md --in plans
  thts add -t "quick todo" "TODO: investigate" --in notes
  thts add -t "placeholder" --no-edit --in notes
  filepath=$(thts add -t "note" -q) && echo "Created: $filepath"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().StringP("title", "t", "", "title for the thought")
	addCmd.Flags().String("in", "", "category/sub-category path (e.g., plans, plans/active)")
	addCmd.Flags().Bool("shared", false, "write to shared/ directory")
	addCmd.Flags().Bool("personal", false, "write to {user}/ directory")
	addCmd.Flags().String("repo", "", "target a specific repo's thoughts directory")
	addCmd.Flags().String("profile", "", "target a specific profile's thoughts repo")

	// Content input flags (mutually exclusive)
	addCmd.Flags().String("from", "", "read content from file")
	addCmd.Flags().Bool("stdin", false, "read content from stdin")

	// Editor control
	addCmd.Flags().Bool("no-edit", false, "skip opening editor")

	// Output control
	addCmd.Flags().BoolP("quiet", "q", false, "only output file path (for scripting)")

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

	// Handle missing title: prompt if TTY, error otherwise
	if opts.Title == "" {
		title, err := ui.PromptForInput("Enter a title:", "my-thought")
		if err != nil {
			if err == ui.ErrNotTerminal {
				return fmt.Errorf("title is required")
			}
			if err == ui.ErrEmptyInput {
				return fmt.Errorf("title cannot be empty")
			}
			return err
		}
		opts.Title = title
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
	filename, ok := generateFilename(opts.Title)
	if !ok {
		return fmt.Errorf("title %q does not produce a valid filename (must contain alphanumeric characters)", opts.Title)
	}

	// Ensure target directory exists
	dirsCreated, err := ensureTargetDir(target.FullPath)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Resolve content and editor behavior
	templateName := cfg.GetTemplate(target.Category, target.SubCategory)
	content, shouldOpenEditor, err := resolveContent(opts, target.ThoughtsDir, templateName)
	if err != nil {
		return err
	}

	// Warn if a configured template wasn't found (only when using template)
	if opts.Content == "" && opts.FromFile == "" && !opts.FromStdin {
		_, templateFound := getTemplateContent(target.ThoughtsDir, templateName)
		if !templateFound && templateName != "default.md" {
			// Warnings go to stderr regardless of quiet mode
			fmt.Fprintln(os.Stderr, ui.WarningF("Template %q not found, using default", templateName))
		}
	}

	// Create the file
	filePath := filepath.Join(target.FullPath, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Report what was created
	if opts.Quiet {
		// In quiet mode, only output the file path to stdout
		fmt.Println(filePath)
	} else {
		printAddResult(filePath, target, dirsCreated, templateName)
	}

	// Open editor if needed
	if !shouldOpenEditor {
		return nil
	}

	editor := resolveEditor(cfg)
	if editor == "" {
		if !opts.Quiet {
			fmt.Println()
			fmt.Println(ui.Info("No editor configured - file created but not opened"))
			fmt.Println(ui.Muted("Set $EDITOR or $VISUAL, or add 'editor' to your config"))
		}
		return nil
	}

	return openEditor(editor, filePath)
}

// parseAddOptions extracts AddOptions from cobra command and args.
func parseAddOptions(cmd *cobra.Command, args []string) (*AddOptions, error) {
	opts := &AddOptions{}

	// Content from positional arg
	if len(args) > 0 {
		opts.Content = args[0]
	}

	// Flags
	var err error
	opts.Title, err = cmd.Flags().GetString("title")
	if err != nil {
		return nil, err
	}

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

	// Content input flags
	opts.FromFile, err = cmd.Flags().GetString("from")
	if err != nil {
		return nil, err
	}

	opts.FromStdin, err = cmd.Flags().GetBool("stdin")
	if err != nil {
		return nil, err
	}

	// Editor control
	opts.NoEdit, err = cmd.Flags().GetBool("no-edit")
	if err != nil {
		return nil, err
	}

	// Output control
	opts.Quiet, err = cmd.Flags().GetBool("quiet")
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

	// Content sources are mutually exclusive
	contentSources := 0
	if opts.Content != "" {
		contentSources++
	}
	if opts.FromFile != "" {
		contentSources++
	}
	if opts.FromStdin {
		contentSources++
	}
	if contentSources > 1 {
		return fmt.Errorf("content argument, --from, and --stdin are mutually exclusive")
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
// Returns the filename and a boolean indicating if the slug was valid.
func generateFilename(title string) (string, bool) {
	date := time.Now().Format("2006-01-02")
	slug := slugify(title)

	if slug == "" {
		return "", false
	}

	return fmt.Sprintf("%s-%s.md", date, slug), true
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
// Returns the content and whether the configured template was found.
func getTemplateContent(thoughtsDir, templateName string) (string, bool) {
	// Try to read from .templates/ in the thoughts dir
	templatePath := filepath.Join(thoughtsDir, ".templates", templateName)
	content, err := os.ReadFile(templatePath)
	if err == nil {
		return string(content), true
	}

	// For global dirs, try parent's .templates/ (thoughts repo root)
	// This handles the case where globalDir is a subdirectory
	parentTemplates := filepath.Join(filepath.Dir(thoughtsDir), ".templates", templateName)
	content, err = os.ReadFile(parentTemplates)
	if err == nil {
		return string(content), true
	}

	// Return a minimal default template
	defaultContent := `---
date: ` + time.Now().Format("2006-01-02") + `
tags: []
---

#

`
	return defaultContent, false
}

// resolveContent determines the content for the new thought file.
// Returns (content, shouldOpenEditor, error).
func resolveContent(opts *AddOptions, thoughtsDir, templateName string) (string, bool, error) {
	switch {
	case opts.Content != "":
		return opts.Content, false, nil
	case opts.FromFile != "":
		content, err := os.ReadFile(opts.FromFile)
		if err != nil {
			if os.IsNotExist(err) {
				return "", false, fmt.Errorf("file not found: %s", opts.FromFile)
			}
			return "", false, fmt.Errorf("cannot read file %s: %w", opts.FromFile, err)
		}
		return string(content), false, nil
	case opts.FromStdin || !ui.IsTerminal():
		// Explicit --stdin flag OR auto-detect piped input
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", false, fmt.Errorf("cannot read from stdin: %w", err)
		}
		return string(content), false, nil
	default:
		// Use template, open editor (unless --no-edit)
		content, _ := getTemplateContent(thoughtsDir, templateName)
		return content, !opts.NoEdit, nil
	}
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
