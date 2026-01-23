package config

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"go.yaml.in/yaml/v3"
)

// setupTestXDGState sets up a temporary XDG state directory and returns cleanup function
func setupTestXDGState(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "thts-state-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	originalXDG := os.Getenv("XDG_STATE_HOME")
	if err := os.Setenv("XDG_STATE_HOME", dir); err != nil {
		t.Fatalf("failed to set XDG_STATE_HOME: %v", err)
	}

	return dir, func() {
		if originalXDG != "" {
			_ = os.Setenv("XDG_STATE_HOME", originalXDG)
		} else {
			_ = os.Unsetenv("XDG_STATE_HOME")
		}
		_ = os.RemoveAll(dir)
	}
}

func TestStatePath(t *testing.T) {
	xdgDir, cleanup := setupTestXDGState(t)
	defer cleanup()

	path := StatePath()
	expected := filepath.Join(xdgDir, "thts", "state.yaml")
	if path != expected {
		t.Errorf("StatePath() = %q, want %q", path, expected)
	}
}

func TestLoadState(t *testing.T) {
	t.Run("loads state from file", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDGState(t)
		defer cleanup()

		// Create state directory
		stateDir := filepath.Join(xdgDir, "thts")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatalf("failed to create state dir: %v", err)
		}

		state := &State{
			RepoMappings: map[string]*RepoMapping{
				"/path/to/repo": {Repo: "myrepo", Profile: "work"},
			},
		}

		data, _ := yaml.Marshal(state)
		if err := os.WriteFile(filepath.Join(stateDir, "state.yaml"), data, 0644); err != nil {
			t.Fatalf("failed to write state: %v", err)
		}

		loaded, err := LoadState()
		if err != nil {
			t.Fatalf("LoadState() error: %v", err)
		}

		if len(loaded.RepoMappings) != 1 {
			t.Errorf("RepoMappings length = %d, want 1", len(loaded.RepoMappings))
		}

		mapping := loaded.RepoMappings["/path/to/repo"]
		if mapping == nil {
			t.Fatal("expected mapping to exist")
		}
		if mapping.Repo != "myrepo" {
			t.Errorf("Repo = %q, want %q", mapping.Repo, "myrepo")
		}
		if mapping.Profile != "work" {
			t.Errorf("Profile = %q, want %q", mapping.Profile, "work")
		}
	})

	t.Run("returns error if state not found", func(t *testing.T) {
		_, cleanup := setupTestXDGState(t)
		defer cleanup()

		_, err := LoadState()
		if err != ErrStateNotFound {
			t.Errorf("LoadState() error = %v, want ErrStateNotFound", err)
		}
	})

	t.Run("initializes nil maps", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDGState(t)
		defer cleanup()

		// Create state with no repoMappings key
		stateDir := filepath.Join(xdgDir, "thts")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatalf("failed to create state dir: %v", err)
		}

		// Empty YAML file
		if err := os.WriteFile(filepath.Join(stateDir, "state.yaml"), []byte(""), 0644); err != nil {
			t.Fatalf("failed to write state: %v", err)
		}

		loaded, err := LoadState()
		if err != nil {
			t.Fatalf("LoadState() error: %v", err)
		}

		if loaded.RepoMappings == nil {
			t.Error("RepoMappings should be initialized")
		}
	})
}

func TestLoadStateOrDefault(t *testing.T) {
	t.Run("returns loaded state when exists", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDGState(t)
		defer cleanup()

		stateDir := filepath.Join(xdgDir, "thts")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatalf("failed to create state dir: %v", err)
		}

		state := &State{
			RepoMappings: map[string]*RepoMapping{
				"/path": {Repo: "testrepo"},
			},
		}
		data, _ := yaml.Marshal(state)
		if err := os.WriteFile(filepath.Join(stateDir, "state.yaml"), data, 0644); err != nil {
			t.Fatalf("failed to write state: %v", err)
		}

		loaded := LoadStateOrDefault()
		if len(loaded.RepoMappings) != 1 {
			t.Errorf("expected 1 mapping, got %d", len(loaded.RepoMappings))
		}
	})

	t.Run("returns empty state when not found", func(t *testing.T) {
		_, cleanup := setupTestXDGState(t)
		defer cleanup()

		loaded := LoadStateOrDefault()
		if loaded.RepoMappings == nil {
			t.Error("RepoMappings should be initialized")
		}
		if len(loaded.RepoMappings) != 0 {
			t.Errorf("expected 0 mappings, got %d", len(loaded.RepoMappings))
		}
	})
}

func TestSaveState(t *testing.T) {
	t.Run("creates state dir if missing", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDGState(t)
		defer cleanup()

		state := &State{
			RepoMappings: map[string]*RepoMapping{
				"/test/repo": {Repo: "test", Profile: "default"},
			},
		}

		if err := SaveState(state); err != nil {
			t.Fatalf("SaveState() error: %v", err)
		}

		// Verify directory was created
		stateDir := filepath.Join(xdgDir, "thts")
		if _, err := os.Stat(stateDir); err != nil {
			t.Error("state directory should be created")
		}

		// Verify file exists
		statePath := filepath.Join(stateDir, "state.yaml")
		if _, err := os.Stat(statePath); err != nil {
			t.Error("state file should be created")
		}
	})

	t.Run("writes valid YAML", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDGState(t)
		defer cleanup()

		state := &State{
			RepoMappings: map[string]*RepoMapping{
				"/path/to/repo": {Repo: "myrepo", Profile: "work"},
			},
		}

		if err := SaveState(state); err != nil {
			t.Fatalf("SaveState() error: %v", err)
		}

		// Read and parse
		statePath := filepath.Join(xdgDir, "thts", "state.yaml")
		data, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("failed to read saved state: %v", err)
		}

		var loaded State
		if err := yaml.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("saved YAML is invalid: %v", err)
		}

		if len(loaded.RepoMappings) != 1 {
			t.Errorf("RepoMappings length = %d, want 1", len(loaded.RepoMappings))
		}
	})

	t.Run("overwrites existing state", func(t *testing.T) {
		_, cleanup := setupTestXDGState(t)
		defer cleanup()

		// Save initial state
		state1 := &State{
			RepoMappings: map[string]*RepoMapping{
				"/repo1": {Repo: "repo1"},
			},
		}
		if err := SaveState(state1); err != nil {
			t.Fatalf("first SaveState() error: %v", err)
		}

		// Save updated state
		state2 := &State{
			RepoMappings: map[string]*RepoMapping{
				"/repo2": {Repo: "repo2"},
			},
		}
		if err := SaveState(state2); err != nil {
			t.Fatalf("second SaveState() error: %v", err)
		}

		// Verify updated state
		loaded, err := LoadState()
		if err != nil {
			t.Fatalf("LoadState() error: %v", err)
		}

		if loaded.RepoMappings["/repo2"] == nil {
			t.Error("expected /repo2 mapping")
		}
		if loaded.RepoMappings["/repo1"] != nil {
			t.Error("expected /repo1 mapping to be removed")
		}
	})
}

func TestStateResolveProfileForRepo(t *testing.T) {
	tests := []struct {
		name     string
		state    *State
		cfg      *Config
		repoPath string
		want     *ResolvedProfile
	}{
		{
			name: "no mapping returns default profile",
			state: &State{
				RepoMappings: map[string]*RepoMapping{},
			},
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"personal": {
						ThoughtsRepo: "~/thoughts",
						ReposDir:     "repos",
						GlobalDir:    "global",
						Default:      true,
					},
				},
			},
			repoPath: "/some/repo",
			want: &ResolvedProfile{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				ProfileName:  "personal",
			},
		},
		{
			name: "mapping with profile returns profile config",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "work"},
				},
			},
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"personal": {
						ThoughtsRepo: "~/thoughts",
						ReposDir:     "repos",
						GlobalDir:    "global",
						Default:      true,
					},
					"work": {
						ThoughtsRepo: "~/work-thoughts",
						ReposDir:     "projects",
						GlobalDir:    "shared",
					},
				},
			},
			repoPath: "/some/repo",
			want: &ResolvedProfile{
				ThoughtsRepo: "~/work-thoughts",
				ReposDir:     "projects",
				GlobalDir:    "shared",
				ProfileName:  "work",
			},
		},
		{
			name: "mapping with missing profile returns default profile",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "nonexistent"},
				},
			},
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"personal": {
						ThoughtsRepo: "~/thoughts",
						ReposDir:     "repos",
						GlobalDir:    "global",
						Default:      true,
					},
				},
			},
			repoPath: "/some/repo",
			want: &ResolvedProfile{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				ProfileName:  "personal",
			},
		},
		{
			name: "nil profiles map returns nil",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "work"},
				},
			},
			cfg: &Config{
				Profiles: nil,
			},
			repoPath: "/some/repo",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.ResolveProfileForRepo(tt.cfg, tt.repoPath)

			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Errorf("expected %+v, got nil", tt.want)
				return
			}

			if got.ThoughtsRepo != tt.want.ThoughtsRepo {
				t.Errorf("ThoughtsRepo = %s, want %s", got.ThoughtsRepo, tt.want.ThoughtsRepo)
			}
			if got.ReposDir != tt.want.ReposDir {
				t.Errorf("ReposDir = %s, want %s", got.ReposDir, tt.want.ReposDir)
			}
			if got.GlobalDir != tt.want.GlobalDir {
				t.Errorf("GlobalDir = %s, want %s", got.GlobalDir, tt.want.GlobalDir)
			}
			if got.ProfileName != tt.want.ProfileName {
				t.Errorf("ProfileName = %s, want %s", got.ProfileName, tt.want.ProfileName)
			}
		})
	}
}

func TestStateCountReposUsingProfile(t *testing.T) {
	tests := []struct {
		name        string
		state       *State
		profileName string
		want        int
	}{
		{
			name: "zero repos",
			state: &State{
				RepoMappings: map[string]*RepoMapping{},
			},
			profileName: "work",
			want:        0,
		},
		{
			name: "no matching repos",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "personal"},
					"/repo2": {Repo: "repo2"},
				},
			},
			profileName: "work",
			want:        0,
		},
		{
			name: "multiple matching repos",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "work"},
					"/repo2": {Repo: "repo2", Profile: "work"},
					"/repo3": {Repo: "repo3", Profile: "personal"},
				},
			},
			profileName: "work",
			want:        2,
		},
		{
			name: "handles nil mapping",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": nil,
					"/repo2": {Repo: "repo2", Profile: "work"},
				},
			},
			profileName: "work",
			want:        1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.CountReposUsingProfile(tt.profileName)
			if got != tt.want {
				t.Errorf("CountReposUsingProfile(%q) = %d, want %d", tt.profileName, got, tt.want)
			}
		})
	}
}

func TestStateGetReposUsingProfile(t *testing.T) {
	tests := []struct {
		name        string
		state       *State
		profileName string
		want        []string
	}{
		{
			name: "no repos",
			state: &State{
				RepoMappings: map[string]*RepoMapping{},
			},
			profileName: "work",
			want:        nil,
		},
		{
			name: "multiple matching repos",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "work"},
					"/repo2": {Repo: "repo2", Profile: "work"},
					"/repo3": {Repo: "repo3", Profile: "personal"},
				},
			},
			profileName: "work",
			want:        []string{"/repo1", "/repo2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.GetReposUsingProfile(tt.profileName)

			// Sort both slices for comparison
			sort.Strings(got)
			sort.Strings(tt.want)

			if len(got) != len(tt.want) {
				t.Errorf("GetReposUsingProfile(%q) returned %d repos, want %d", tt.profileName, len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GetReposUsingProfile(%q)[%d] = %q, want %q", tt.profileName, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestStateCountReposUsingProfileWithImplicit(t *testing.T) {
	tests := []struct {
		name        string
		state       *State
		cfg         *Config
		profileName string
		want        ProfileUsageCounts
	}{
		{
			name: "zero repos",
			state: &State{
				RepoMappings: map[string]*RepoMapping{},
			},
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
				},
			},
			profileName: "default",
			want:        ProfileUsageCounts{Explicit: 0, Implicit: 0, Total: 0},
		},
		{
			name: "explicit only",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "work"},
					"/repo2": {Repo: "repo2", Profile: "work"},
				},
			},
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
					"work":    {ThoughtsRepo: "~/work"},
				},
			},
			profileName: "work",
			want:        ProfileUsageCounts{Explicit: 2, Implicit: 0, Total: 2},
		},
		{
			name: "implicit only - default profile",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1"},
					"/repo2": {Repo: "repo2"},
					"/repo3": {Repo: "repo3", Profile: "work"},
				},
			},
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
					"work":    {ThoughtsRepo: "~/work"},
				},
			},
			profileName: "default",
			want:        ProfileUsageCounts{Explicit: 0, Implicit: 2, Total: 2},
		},
		{
			name: "mixed explicit and implicit",
			state: &State{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "default"},
					"/repo2": {Repo: "repo2"},
					"/repo3": {Repo: "repo3"},
					"/repo4": {Repo: "repo4", Profile: "work"},
				},
			},
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
					"work":    {ThoughtsRepo: "~/work"},
				},
			},
			profileName: "default",
			want:        ProfileUsageCounts{Explicit: 1, Implicit: 2, Total: 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.CountReposUsingProfileWithImplicit(tt.cfg, tt.profileName)
			if got.Explicit != tt.want.Explicit {
				t.Errorf("Explicit = %d, want %d", got.Explicit, tt.want.Explicit)
			}
			if got.Implicit != tt.want.Implicit {
				t.Errorf("Implicit = %d, want %d", got.Implicit, tt.want.Implicit)
			}
			if got.Total != tt.want.Total {
				t.Errorf("Total = %d, want %d", got.Total, tt.want.Total)
			}
		})
	}
}
