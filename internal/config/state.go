package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// State represents machine-specific state that should not be synced.
// This is separate from Config which contains portable preferences.
type State struct {
	RepoMappings map[string]*RepoMapping `yaml:"repoMappings,omitempty"`
}

// ErrStateNotFound is returned when no state file exists.
var ErrStateNotFound = errors.New("state not found")

// StatePath returns the path to the thts state file.
// Uses XDG_STATE_HOME with fallback to ~/.local/state/thts/state.yaml.
func StatePath() string {
	return filepath.Join(XDGStateHome(), "thts", "state.yaml")
}

// LoadState loads the thts state from the state file.
// If the state file doesn't exist, falls back to HumanLayer's config for compatibility.
func LoadState() (*State, error) {
	path := StatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Try HumanLayer fallback for compatibility
			if hlState := LoadStateFromHumanLayer(); hlState != nil {
				return hlState, nil
			}
			return nil, ErrStateNotFound
		}
		return nil, err
	}

	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	// Initialize map if nil
	if state.RepoMappings == nil {
		state.RepoMappings = make(map[string]*RepoMapping)
	}

	return &state, nil
}

// LoadStateOrDefault loads the state or returns an empty state if not found.
func LoadStateOrDefault() *State {
	state, err := LoadState()
	if err != nil {
		return &State{
			RepoMappings: make(map[string]*RepoMapping),
		}
	}
	return state
}

// SaveState saves the state to the state file.
func SaveState(state *State) error {
	path := StatePath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

// ResolveProfileForRepo resolves the profile configuration for a given repository path.
// It requires both the state (for repo mappings) and config (for profile definitions).
func (s *State) ResolveProfileForRepo(cfg *Config, repoPath string) *ResolvedProfile {
	mapping := s.RepoMappings[repoPath]

	// Get default profile for fallback
	defaultProf, defaultName := cfg.GetDefaultProfile()

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
	if mapping.Profile != "" && cfg.Profiles != nil {
		if profile, exists := cfg.Profiles[mapping.Profile]; exists {
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

// CountReposUsingProfile counts how many repositories are using a given profile.
func (s *State) CountReposUsingProfile(profileName string) int {
	count := 0
	for _, mapping := range s.RepoMappings {
		if mapping != nil && mapping.Profile == profileName {
			count++
		}
	}
	return count
}

// CountReposUsingProfileWithImplicit counts repos using a profile,
// distinguishing explicit assignments from implicit (via default) usage.
func (s *State) CountReposUsingProfileWithImplicit(cfg *Config, profileName string) ProfileUsageCounts {
	counts := ProfileUsageCounts{}
	_, defaultName := cfg.GetDefaultProfile()
	isDefault := profileName == defaultName

	for _, mapping := range s.RepoMappings {
		if mapping == nil {
			continue
		}
		if mapping.Profile == profileName {
			counts.Explicit++
		} else if mapping.Profile == "" && isDefault {
			counts.Implicit++
		}
	}
	counts.Total = counts.Explicit + counts.Implicit
	return counts
}

// GetReposUsingProfile returns paths of repositories using a given profile.
func (s *State) GetReposUsingProfile(profileName string) []string {
	var repos []string
	for repoPath, mapping := range s.RepoMappings {
		if mapping != nil && mapping.Profile == profileName {
			repos = append(repos, repoPath)
		}
	}
	return repos
}
