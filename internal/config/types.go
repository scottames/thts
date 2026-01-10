package config

// RepoMapping represents a repository mapping that can be either a simple string
// (repo name only) or a full object with profile information.
type RepoMapping struct {
	Repo    string `yaml:"repo"`
	Profile string `yaml:"profile,omitempty"`
}

// ProfileConfig represents a named profile with its own thoughts repository.
type ProfileConfig struct {
	ThoughtsRepo string `yaml:"thoughtsRepo"`
	ReposDir     string `yaml:"reposDir"`
	GlobalDir    string `yaml:"globalDir"`
	Default      bool   `yaml:"default,omitempty"`
}

// GitIgnoreMode specifies where to add the thoughts/ ignore rule.
type GitIgnoreMode string

const (
	GitIgnoreProject  GitIgnoreMode = "project"  // Add to project's .gitignore
	GitIgnoreLocal    GitIgnoreMode = "local"    // Add to .git/info/exclude
	GitIgnoreGlobal   GitIgnoreMode = "global"   // Add to ~/.config/git/ignore
	GitIgnoreDisabled GitIgnoreMode = "disabled" // Don't add anywhere
)

// Config represents the tpd configuration.
type Config struct {
	User                string                    `yaml:"user"`
	AutoSyncInWorktrees bool                      `yaml:"autoSyncInWorktrees,omitempty"`
	GitIgnore           GitIgnoreMode             `yaml:"gitIgnore,omitempty"`
	RepoMappings        map[string]*RepoMapping   `yaml:"repoMappings,omitempty"`
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
func Defaults() *Config {
	return &Config{
		AutoSyncInWorktrees: true,
		GitIgnore:           GitIgnoreProject,
		RepoMappings:        make(map[string]*RepoMapping),
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

// GetDefaultProfile returns the default profile and its name.
// If no profile is marked as default, returns the first profile found and warns.
// Returns nil, "" if no profiles exist.
func (c *Config) GetDefaultProfile() (*ProfileConfig, string) {
	if len(c.Profiles) == 0 {
		return nil, ""
	}

	// Look for explicitly marked default
	for name, profile := range c.Profiles {
		if profile.Default {
			return profile, name
		}
	}

	// No explicit default - use first profile (map iteration order is random,
	// but this is a fallback for misconfigured state)
	for name, profile := range c.Profiles {
		return profile, name
	}

	return nil, ""
}

// ResolveProfileForRepo resolves the profile configuration for a given repository path.
func (c *Config) ResolveProfileForRepo(repoPath string) *ResolvedProfile {
	mapping := c.RepoMappings[repoPath]

	// Get default profile for fallback
	defaultProf, defaultName := c.GetDefaultProfile()

	// Build default resolved profile
	var defaultResolved *ResolvedProfile
	if defaultProf != nil {
		defaultResolved = &ResolvedProfile{
			ThoughtsRepo: defaultProf.ThoughtsRepo,
			ReposDir:     defaultProf.ReposDir,
			GlobalDir:    defaultProf.GlobalDir,
			ProfileName:  defaultName,
		}
	}

	if mapping == nil {
		return defaultResolved
	}

	// If profile specified, look it up
	if mapping.Profile != "" && c.Profiles != nil {
		if profile, exists := c.Profiles[mapping.Profile]; exists {
			return &ResolvedProfile{
				ThoughtsRepo: profile.ThoughtsRepo,
				ReposDir:     profile.ReposDir,
				GlobalDir:    profile.GlobalDir,
				ProfileName:  mapping.Profile,
			}
		}
	}

	return defaultResolved
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

// CountReposUsingProfile counts how many repositories are using a given profile.
func (c *Config) CountReposUsingProfile(profileName string) int {
	count := 0
	for _, mapping := range c.RepoMappings {
		if mapping != nil && mapping.Profile == profileName {
			count++
		}
	}
	return count
}

// GetReposUsingProfile returns paths of repositories using a given profile.
func (c *Config) GetReposUsingProfile(profileName string) []string {
	var repos []string
	for repoPath, mapping := range c.RepoMappings {
		if mapping != nil && mapping.Profile == profileName {
			repos = append(repos, repoPath)
		}
	}
	return repos
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
