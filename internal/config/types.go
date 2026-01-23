package config

// Scope specifies whether content goes to shared/ or {user}/ directories.
type Scope string

const (
	ScopeUser   Scope = "user"   // Write to {user}/ directory (default)
	ScopeShared Scope = "shared" // Write to shared/ directory
)

// CategoryScope specifies which scope(s) a category is typically used in.
type CategoryScope string

const (
	CategoryScopeShared CategoryScope = "shared" // Category used in shared/ only
	CategoryScopeUser   CategoryScope = "user"   // Category used in {user}/ only
	CategoryScopeBoth   CategoryScope = "both"   // Category used in both scopes
)

// SubCategory represents a sub-category within a category.
type SubCategory struct {
	Description string        `yaml:"description"`
	Template    string        `yaml:"template,omitempty"`
	Trigger     string        `yaml:"trigger,omitempty"`
	Scope       CategoryScope `yaml:"scope,omitempty"` // Overrides parent category scope
}

// Category represents a top-level category for organizing thoughts.
type Category struct {
	Description   string                  `yaml:"description"`
	Template      string                  `yaml:"template,omitempty"`
	Trigger       string                  `yaml:"trigger,omitempty"`
	Scope         CategoryScope           `yaml:"scope,omitempty"` // shared, user, or both
	SubCategories map[string]*SubCategory `yaml:"subCategories,omitempty"`
}

// RepoMapping represents a repository mapping that can be either a simple string
// (repo name only) or a full object with profile information.
type RepoMapping struct {
	Repo    string `yaml:"repo"`
	Profile string `yaml:"profile,omitempty"`
}

// ProfileConfig represents a named profile with its own thoughts repository.
type ProfileConfig struct {
	ThoughtsRepo  string               `yaml:"thoughtsRepo"`
	ReposDir      string               `yaml:"reposDir"`
	GlobalDir     string               `yaml:"globalDir"`
	Default       bool                 `yaml:"default,omitempty"`
	DefaultAgents []string             `yaml:"defaultAgents,omitempty"`
	Categories    map[string]*Category `yaml:"categories,omitempty"` // Overrides global categories when set
	Sync          *SyncConfig          `yaml:"sync,omitempty"`       // Overrides global sync settings when set
}

// ComponentMode specifies where a component is managed.
type ComponentMode string

const (
	ComponentModeGlobal   ComponentMode = "global"   // Managed globally (e.g., ~/.claude/, ~/.config/git/ignore)
	ComponentModeLocal    ComponentMode = "local"    // Managed per-project (default)
	ComponentModeDisabled ComponentMode = "disabled" // Not managed by thts, user handles
)

// AgentsConfig holds configuration for agent component management.
type AgentsConfig struct {
	Skills   ComponentMode `yaml:"skills,omitempty"`
	Commands ComponentMode `yaml:"commands,omitempty"`
	Agents   ComponentMode `yaml:"agents,omitempty"`
}

// SyncMode specifies the sync behavior for remote operations.
type SyncMode string

const (
	SyncModeFull  SyncMode = "full"  // Pull and push (default)
	SyncModePull  SyncMode = "pull"  // Pull only, no push
	SyncModeLocal SyncMode = "local" // No remote operations
)

// SyncConfig holds configuration for sync behavior.
type SyncConfig struct {
	Mode              SyncMode `yaml:"mode,omitempty"`
	CommitMessage     string   `yaml:"commitMessage,omitempty"`
	CommitMessageHook string   `yaml:"commitMessageHook,omitempty"`
}

// HooksConfig holds configuration for hook-based integration.
type HooksConfig struct {
	// Keywords that trigger full instruction injection.
	// Default: research, plan, decision, thoughts, handoff, notes, save,
	// document, capture, findings, learnings, gotchas, ADR, architecture,
	// resume, wrap up, end session
	Keywords []string `yaml:"keywords,omitempty"`
}

// Config represents the thts configuration.
// Note: RepoMappings have been moved to State (machine-specific state file).
type Config struct {
	User                string                    `yaml:"user"`
	Editor              string                    `yaml:"editor,omitempty"`
	AutoSyncInWorktrees bool                      `yaml:"autoSyncInWorktrees,omitempty"`
	Gitignore           ComponentMode             `yaml:"gitignore,omitempty"`
	DefaultScope        Scope                     `yaml:"defaultScope,omitempty"`    // "user" or "shared" - where thts add writes by default
	DefaultTemplate     string                    `yaml:"defaultTemplate,omitempty"` // Falls back to built-in default.md
	Sync                *SyncConfig               `yaml:"sync,omitempty"`
	Agents              *AgentsConfig             `yaml:"agents,omitempty"`
	Hooks               *HooksConfig              `yaml:"hooks,omitempty"`
	Categories          map[string]*Category      `yaml:"categories,omitempty"` // Global categories
	Profiles            map[string]*ProfileConfig `yaml:"profiles"`
}

// ResolvedProfile represents a resolved profile configuration for a repository.
type ResolvedProfile struct {
	ThoughtsRepo string
	ReposDir     string
	GlobalDir    string
	ProfileName  string // empty for default config
}

// Defaults returns a Config with default values set.
//
// NOTE: When adding fields to Config, also update FullDefaults() and
// config_template.yaml, then run tests to verify completeness.
func Defaults() *Config {
	return &Config{
		AutoSyncInWorktrees: true,
		Gitignore:           ComponentModeLocal,
		Profiles: map[string]*ProfileConfig{
			"default": {
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				Default:      true,
			},
		},
	}
}

// FullDefaults returns a Config with ALL default values explicitly set.
// This is useful for showing users what options exist and their defaults.
//
// NOTE: When adding fields to Config, also update this function and
// config_template.yaml, then run tests to verify completeness.
func FullDefaults() *Config {
	// Start with base defaults to avoid duplication
	cfg := Defaults()

	// Expand implicit defaults (values from getter methods)
	cfg.DefaultScope = DefaultScopeValue
	cfg.DefaultTemplate = "default.md"
	cfg.Sync = &SyncConfig{
		Mode:              SyncModeFull,
		CommitMessage:     DefaultCommitMessage(),
		CommitMessageHook: DefaultCommitMessageHook(),
	}
	cfg.Agents = &AgentsConfig{
		Skills:   ComponentModeLocal,
		Commands: ComponentModeLocal,
		Agents:   ComponentModeLocal,
	}
	cfg.Hooks = &HooksConfig{Keywords: DefaultHookKeywords()}
	cfg.Categories = DefaultCategories()

	return cfg
}

// GetDefaultProfile returns the default profile and its name.
// Returns nil, "" if no profiles exist or no profile is marked as default.
func (c *Config) GetDefaultProfile() (*ProfileConfig, string) {
	if len(c.Profiles) == 0 {
		return nil, ""
	}

	for name, profile := range c.Profiles {
		if profile.Default {
			return profile, name
		}
	}

	// No profile marked as default - return nil to surface misconfiguration
	return nil, ""
}

// GetDefaultProfileResolved returns the default profile as a ResolvedProfile.
// This is useful when syncing from a non-initialized directory.
func (c *Config) GetDefaultProfileResolved() *ResolvedProfile {
	prof, name := c.GetDefaultProfile()
	if prof == nil {
		return nil
	}
	return &ResolvedProfile{
		ThoughtsRepo: prof.ThoughtsRepo,
		ReposDir:     prof.ReposDir,
		GlobalDir:    prof.GlobalDir,
		ProfileName:  name,
	}
}

// ValidateProfile checks if a profile exists in the configuration.
func (c *Config) ValidateProfile(profileName string) bool {
	if c.Profiles == nil {
		return false
	}
	_, exists := c.Profiles[profileName]
	return exists
}

// GetRepoName returns the repo name from a mapping.
func (m *RepoMapping) GetRepoName() string {
	if m == nil {
		return ""
	}
	return m.Repo
}

// SanitizeProfileName sanitizes a profile name for use as a directory name.
func SanitizeProfileName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}

// ProfileUsageCounts holds counts of repos using a profile.
type ProfileUsageCounts struct {
	Explicit int // mapping.Profile == profileName
	Implicit int // mapping.Profile == "" AND this profile is default
	Total    int
}

// DeleteProfile removes a profile from the configuration.
func (c *Config) DeleteProfile(profileName string) {
	if c.Profiles != nil {
		delete(c.Profiles, profileName)
	}
}

// SetDefaultProfile sets the specified profile as the default.
// It clears the default flag from all other profiles.
func (c *Config) SetDefaultProfile(profileName string) bool {
	if c.Profiles == nil {
		return false
	}
	profile, exists := c.Profiles[profileName]
	if !exists {
		return false
	}

	// Clear default from all profiles
	for _, p := range c.Profiles {
		p.Default = false
	}

	// Set new default
	profile.Default = true
	return true
}

// GetGitignoreMode returns the gitignore mode, defaulting to local.
func (c *Config) GetGitignoreMode() ComponentMode {
	if c.Gitignore == "" {
		return ComponentModeLocal
	}
	return c.Gitignore
}

// GetSyncMode returns the sync mode, defaulting to full.
func (c *Config) GetSyncMode() SyncMode {
	if c.Sync != nil && c.Sync.Mode != "" {
		return c.Sync.Mode
	}
	return SyncModeFull
}

// GetCommitMessage returns the commit message template for a profile.
// Resolution order: profile sync.commitMessage > global sync.commitMessage > default.
func (c *Config) GetCommitMessage(profileName string) string {
	// Check profile-level override first
	if c.Profiles != nil {
		if profile, exists := c.Profiles[profileName]; exists {
			if profile.Sync != nil && profile.Sync.CommitMessage != "" {
				return profile.Sync.CommitMessage
			}
		}
	}

	// Check global sync config
	if c.Sync != nil && c.Sync.CommitMessage != "" {
		return c.Sync.CommitMessage
	}

	// Return default
	return DefaultCommitMessage()
}

// GetCommitMessageHook returns the commit message template for hook auto-sync.
// Resolution order: profile sync.commitMessageHook > global sync.commitMessageHook > default.
func (c *Config) GetCommitMessageHook(profileName string) string {
	// Check profile-level override first
	if c.Profiles != nil {
		if profile, exists := c.Profiles[profileName]; exists {
			if profile.Sync != nil && profile.Sync.CommitMessageHook != "" {
				return profile.Sync.CommitMessageHook
			}
		}
	}

	// Check global sync config
	if c.Sync != nil && c.Sync.CommitMessageHook != "" {
		return c.Sync.CommitMessageHook
	}

	// Return default
	return DefaultCommitMessageHook()
}

// GetAgentComponentMode returns the mode for an agent component, defaulting to local.
func (c *Config) GetAgentComponentMode(component string) ComponentMode {
	if c.Agents == nil {
		return ComponentModeLocal
	}
	switch component {
	case "skills":
		if c.Agents.Skills != "" {
			return c.Agents.Skills
		}
	case "commands":
		if c.Agents.Commands != "" {
			return c.Agents.Commands
		}
	case "agents":
		if c.Agents.Agents != "" {
			return c.Agents.Agents
		}
	}
	return ComponentModeLocal
}

// SetAgentComponentMode sets the mode for an agent component.
func (c *Config) SetAgentComponentMode(component string, mode ComponentMode) {
	if c.Agents == nil {
		c.Agents = &AgentsConfig{}
	}
	switch component {
	case "skills":
		c.Agents.Skills = mode
	case "commands":
		c.Agents.Commands = mode
	case "agents":
		c.Agents.Agents = mode
	}
}

// GetDefaultScope returns the default scope, defaulting to "user".
func (c *Config) GetDefaultScope() Scope {
	if c.DefaultScope != "" {
		return c.DefaultScope
	}
	return DefaultScopeValue
}

// GetCategories returns the categories for the config, falling back to defaults.
func (c *Config) GetCategories() map[string]*Category {
	if len(c.Categories) > 0 {
		return c.Categories
	}
	return DefaultCategories()
}

// GetCategoriesForProfile returns the categories for a profile.
// If the profile has categories defined, those are returned (complete override).
// Otherwise, returns global categories (or defaults if none set).
func (c *Config) GetCategoriesForProfile(profileName string) map[string]*Category {
	if c.Profiles != nil {
		if profile, exists := c.Profiles[profileName]; exists {
			if len(profile.Categories) > 0 {
				return profile.Categories
			}
		}
	}
	return c.GetCategories()
}

// GetCategory returns a category by name, or nil if not found.
func (c *Config) GetCategory(name string) *Category {
	categories := c.GetCategories()
	return categories[name]
}

// GetTemplate returns the template for a category/sub-category path.
// Resolution order: sub-category template > category template > defaultTemplate > "default.md".
func (c *Config) GetTemplate(categoryName, subCategoryName string) string {
	categories := c.GetCategories()
	cat, exists := categories[categoryName]
	if !exists {
		return c.getDefaultTemplate()
	}

	// Check sub-category template first
	if subCategoryName != "" && cat.SubCategories != nil {
		if subCat, subExists := cat.SubCategories[subCategoryName]; subExists {
			if subCat.Template != "" {
				return subCat.Template
			}
		}
	}

	// Fall back to category template
	if cat.Template != "" {
		return cat.Template
	}

	return c.getDefaultTemplate()
}

// getDefaultTemplate returns the default template name.
func (c *Config) getDefaultTemplate() string {
	if c.DefaultTemplate != "" {
		return c.DefaultTemplate
	}
	return "default.md"
}

// GetScope returns the category's scope, defaulting to "shared" if not set.
func (cat *Category) GetScope() CategoryScope {
	if cat.Scope != "" {
		return cat.Scope
	}
	return CategoryScopeShared
}

// GetScope returns the sub-category's scope, inheriting from parent if not set.
func (sub *SubCategory) GetScope(parentScope CategoryScope) CategoryScope {
	if sub.Scope != "" {
		return sub.Scope
	}
	return parentScope
}

// DefaultHookKeywords returns the default keywords that trigger instruction injection.
func DefaultHookKeywords() []string {
	return []string{
		"research", "plan", "decision", "thoughts", "handoff", "notes", "save",
		"document", "capture", "findings", "learnings", "gotchas", "ADR",
		"architecture", "resume", "wrap up", "end session",
	}
}

// GetHookKeywords returns the configured hook keywords or defaults.
func (c *Config) GetHookKeywords() []string {
	if c.Hooks != nil && len(c.Hooks.Keywords) > 0 {
		return c.Hooks.Keywords
	}
	return DefaultHookKeywords()
}
