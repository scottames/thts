package config

import (
	"sort"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.ThoughtsRepo != "~/thoughts" {
		t.Errorf("expected ThoughtsRepo to be ~/thoughts, got %s", cfg.ThoughtsRepo)
	}
	if cfg.ReposDir != "repos" {
		t.Errorf("expected ReposDir to be repos, got %s", cfg.ReposDir)
	}
	if cfg.GlobalDir != "global" {
		t.Errorf("expected GlobalDir to be global, got %s", cfg.GlobalDir)
	}
	if !cfg.AutoSyncInWorktrees {
		t.Error("expected AutoSyncInWorktrees to be true")
	}
	if cfg.GitIgnore != GitIgnoreProject {
		t.Errorf("expected GitIgnore to be project, got %s", cfg.GitIgnore)
	}
	if cfg.RepoMappings == nil {
		t.Error("expected RepoMappings to be initialized")
	}
	if cfg.Profiles == nil {
		t.Error("expected Profiles to be initialized")
	}
}

func TestResolveProfileForRepo(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		repoPath string
		want     *ResolvedProfile
	}{
		{
			name: "no mapping returns default",
			cfg: &Config{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				RepoMappings: map[string]*RepoMapping{},
			},
			repoPath: "/some/repo",
			want: &ResolvedProfile{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				ProfileName:  "",
			},
		},
		{
			name: "mapping without profile returns default",
			cfg: &Config{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo"},
				},
			},
			repoPath: "/some/repo",
			want: &ResolvedProfile{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				ProfileName:  "",
			},
		},
		{
			name: "mapping with profile returns profile config",
			cfg: &Config{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "work"},
				},
				Profiles: map[string]*ProfileConfig{
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
			name: "mapping with missing profile returns default",
			cfg: &Config{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "nonexistent"},
				},
				Profiles: map[string]*ProfileConfig{},
			},
			repoPath: "/some/repo",
			want: &ResolvedProfile{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				ProfileName:  "",
			},
		},
		{
			name: "nil profiles map with profile specified returns default",
			cfg: &Config{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "work"},
				},
				Profiles: nil,
			},
			repoPath: "/some/repo",
			want: &ResolvedProfile{
				ThoughtsRepo: "~/thoughts",
				ReposDir:     "repos",
				GlobalDir:    "global",
				ProfileName:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ResolveProfileForRepo(tt.repoPath)

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

func TestValidateProfile(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		profileName string
		want        bool
	}{
		{
			name: "profile exists",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"work": {ThoughtsRepo: "~/work"},
				},
			},
			profileName: "work",
			want:        true,
		},
		{
			name: "profile does not exist",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"work": {ThoughtsRepo: "~/work"},
				},
			},
			profileName: "personal",
			want:        false,
		},
		{
			name: "nil profiles map",
			cfg: &Config{
				Profiles: nil,
			},
			profileName: "work",
			want:        false,
		},
		{
			name: "empty profiles map",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{},
			},
			profileName: "work",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ValidateProfile(tt.profileName)
			if got != tt.want {
				t.Errorf("ValidateProfile(%q) = %v, want %v", tt.profileName, got, tt.want)
			}
		})
	}
}

func TestRepoMappingGetRepoName(t *testing.T) {
	tests := []struct {
		name    string
		mapping *RepoMapping
		want    string
	}{
		{
			name:    "nil receiver",
			mapping: nil,
			want:    "",
		},
		{
			name:    "empty repo",
			mapping: &RepoMapping{Repo: ""},
			want:    "",
		},
		{
			name:    "normal repo",
			mapping: &RepoMapping{Repo: "my-project"},
			want:    "my-project",
		},
		{
			name:    "repo with profile",
			mapping: &RepoMapping{Repo: "my-project", Profile: "work"},
			want:    "my-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mapping.GetRepoName()
			if got != tt.want {
				t.Errorf("GetRepoName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeProfileName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "alphanumeric passthrough",
			input: "myprofile123",
			want:  "myprofile123",
		},
		{
			name:  "allows underscore and hyphen",
			input: "my_profile-name",
			want:  "my_profile-name",
		},
		{
			name:  "replaces spaces",
			input: "my profile",
			want:  "my_profile",
		},
		{
			name:  "replaces special characters",
			input: "my.profile@name!",
			want:  "my_profile_name_",
		},
		{
			name:  "replaces slashes",
			input: "path/to/profile",
			want:  "path_to_profile",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "mixed case preserved",
			input: "MyProfile",
			want:  "MyProfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeProfileName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeProfileName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCountReposUsingProfile(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		profileName string
		want        int
	}{
		{
			name: "zero repos",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{},
			},
			profileName: "work",
			want:        0,
		},
		{
			name: "no matching repos",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "personal"},
					"/repo2": {Repo: "repo2"},
				},
			},
			profileName: "work",
			want:        0,
		},
		{
			name: "one matching repo",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "work"},
					"/repo2": {Repo: "repo2", Profile: "personal"},
				},
			},
			profileName: "work",
			want:        1,
		},
		{
			name: "multiple matching repos",
			cfg: &Config{
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
			cfg: &Config{
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
			got := tt.cfg.CountReposUsingProfile(tt.profileName)
			if got != tt.want {
				t.Errorf("CountReposUsingProfile(%q) = %d, want %d", tt.profileName, got, tt.want)
			}
		})
	}
}

func TestGetReposUsingProfile(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		profileName string
		want        []string
	}{
		{
			name: "no repos",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{},
			},
			profileName: "work",
			want:        nil,
		},
		{
			name: "no matching repos",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "personal"},
				},
			},
			profileName: "work",
			want:        nil,
		},
		{
			name: "multiple matching repos",
			cfg: &Config{
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
			got := tt.cfg.GetReposUsingProfile(tt.profileName)

			// Sort both slices for comparison since map iteration order is not guaranteed
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

func TestDeleteProfile(t *testing.T) {
	t.Run("deletes existing profile", func(t *testing.T) {
		cfg := &Config{
			Profiles: map[string]*ProfileConfig{
				"work":     {ThoughtsRepo: "~/work"},
				"personal": {ThoughtsRepo: "~/personal"},
			},
		}

		cfg.DeleteProfile("work")

		if _, exists := cfg.Profiles["work"]; exists {
			t.Error("expected work profile to be deleted")
		}
		if _, exists := cfg.Profiles["personal"]; !exists {
			t.Error("expected personal profile to remain")
		}
	})

	t.Run("no-op for missing profile", func(t *testing.T) {
		cfg := &Config{
			Profiles: map[string]*ProfileConfig{
				"work": {ThoughtsRepo: "~/work"},
			},
		}

		// Should not panic
		cfg.DeleteProfile("nonexistent")

		if _, exists := cfg.Profiles["work"]; !exists {
			t.Error("expected work profile to remain")
		}
	})

	t.Run("no-op for nil profiles map", func(t *testing.T) {
		cfg := &Config{
			Profiles: nil,
		}

		// Should not panic
		cfg.DeleteProfile("any")
	})
}
