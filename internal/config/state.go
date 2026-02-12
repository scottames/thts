package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"
)

// State represents machine-specific state that should not be synced.
// This is separate from Config which contains portable preferences.
type State struct {
	Meta         *StateMeta              `yaml:"meta,omitempty"`
	RepoMappings map[string]*RepoMapping `yaml:"repoMappings,omitempty"`
}

// StateMeta captures context metadata for a namespaced state file.
type StateMeta struct {
	SchemaVersion  int    `yaml:"schemaVersion"`
	ConfigPath     string `yaml:"configPath"`
	ConfigPathHash string `yaml:"configPathHash"`
	CreatedAt      string `yaml:"createdAt"`
	LastUsedAt     string `yaml:"lastUsedAt"`
}

// ErrStateNotFound is returned when no state file exists.
var ErrStateNotFound = errors.New("state not found")

// LoadState loads the thts state from the namespaced state file.
func LoadState() (*State, error) {
	path := StatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
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

	state.ensureMeta()

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

	state.ensureMeta()
	state.Meta.LastUsedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

func (s *State) ensureMeta() {
	canonicalConfigPath := CanonicalConfigPath()
	currentPath := StatePathForConfig(canonicalConfigPath)
	currentHash := statePathHash(currentPath)
	now := time.Now().UTC().Format(time.RFC3339)

	if s.Meta == nil {
		s.Meta = &StateMeta{
			SchemaVersion:  1,
			ConfigPath:     canonicalConfigPath,
			ConfigPathHash: currentHash,
			CreatedAt:      now,
			LastUsedAt:     now,
		}
		return
	}

	if s.Meta.SchemaVersion == 0 {
		s.Meta.SchemaVersion = 1
	}
	if s.Meta.CreatedAt == "" {
		s.Meta.CreatedAt = now
	}

	s.Meta.ConfigPath = canonicalConfigPath
	s.Meta.ConfigPathHash = currentHash
	if s.Meta.LastUsedAt == "" {
		s.Meta.LastUsedAt = now
	}
}

func statePathHash(statePath string) string {
	base := filepath.Base(statePath)
	const prefix = "state-"
	const suffix = ".yaml"
	if len(base) > len(prefix)+len(suffix) && base[:len(prefix)] == prefix {
		return base[len(prefix) : len(base)-len(suffix)]
	}
	return ""
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
