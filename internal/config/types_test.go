package config

import (
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if !cfg.AutoSyncInWorktrees {
		t.Error("expected AutoSyncInWorktrees to be true")
	}
	if cfg.Gitignore != ComponentModeLocal {
		t.Errorf("expected Gitignore to be local, got %s", cfg.Gitignore)
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
			name: "no mapping returns default profile",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{},
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
			name: "mapping without profile returns default profile",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo"},
				},
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
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "work"},
				},
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
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "nonexistent"},
				},
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
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/some/repo": {Repo: "my-repo", Profile: "work"},
				},
				Profiles: nil,
			},
			repoPath: "/some/repo",
			want:     nil,
		},
		{
			name: "empty profiles map returns nil",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{},
				Profiles:     map[string]*ProfileConfig{},
			},
			repoPath: "/some/repo",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ResolveProfileForRepo(tt.repoPath)

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

func TestGetDefaultProfile(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		wantProfile *ProfileConfig
		wantName    string
	}{
		{
			name: "returns profile marked as default",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"work": {
						ThoughtsRepo: "~/work",
						Default:      false,
					},
					"personal": {
						ThoughtsRepo: "~/personal",
						Default:      true,
					},
				},
			},
			wantProfile: &ProfileConfig{
				ThoughtsRepo: "~/personal",
				Default:      true,
			},
			wantName: "personal",
		},
		{
			name: "returns nil for empty profiles",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{},
			},
			wantProfile: nil,
			wantName:    "",
		},
		{
			name: "returns nil for nil profiles",
			cfg: &Config{
				Profiles: nil,
			},
			wantProfile: nil,
			wantName:    "",
		},
		{
			name: "returns nil if no default marked",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"work": {
						ThoughtsRepo: "~/work",
						Default:      false,
					},
				},
			},
			wantProfile: nil,
			wantName:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProfile, gotName := tt.cfg.GetDefaultProfile()

			if tt.wantProfile == nil {
				if gotProfile != nil {
					t.Errorf("expected nil profile, got %+v", gotProfile)
				}
				if gotName != "" {
					t.Errorf("expected empty name, got %s", gotName)
				}
				return
			}

			if gotProfile == nil {
				t.Errorf("expected profile, got nil")
				return
			}

			if gotProfile.ThoughtsRepo != tt.wantProfile.ThoughtsRepo {
				t.Errorf("ThoughtsRepo = %s, want %s", gotProfile.ThoughtsRepo, tt.wantProfile.ThoughtsRepo)
			}
			if gotName != tt.wantName {
				t.Errorf("name = %s, want %s", gotName, tt.wantName)
			}
		})
	}
}

func TestSetDefaultProfile(t *testing.T) {
	t.Run("sets default profile", func(t *testing.T) {
		cfg := &Config{
			Profiles: map[string]*ProfileConfig{
				"personal": {ThoughtsRepo: "~/personal", Default: true},
				"work":     {ThoughtsRepo: "~/work", Default: false},
			},
		}

		result := cfg.SetDefaultProfile("work")

		if !result {
			t.Error("expected SetDefaultProfile to return true")
		}
		if cfg.Profiles["personal"].Default {
			t.Error("expected personal profile Default to be false")
		}
		if !cfg.Profiles["work"].Default {
			t.Error("expected work profile Default to be true")
		}
	})

	t.Run("returns false for non-existent profile", func(t *testing.T) {
		cfg := &Config{
			Profiles: map[string]*ProfileConfig{
				"personal": {ThoughtsRepo: "~/personal", Default: true},
			},
		}

		result := cfg.SetDefaultProfile("nonexistent")

		if result {
			t.Error("expected SetDefaultProfile to return false")
		}
		if !cfg.Profiles["personal"].Default {
			t.Error("expected personal profile Default to remain true")
		}
	})

	t.Run("returns false for nil profiles", func(t *testing.T) {
		cfg := &Config{Profiles: nil}

		result := cfg.SetDefaultProfile("work")

		if result {
			t.Error("expected SetDefaultProfile to return false")
		}
	})
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

func TestCountReposUsingProfileWithImplicit(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		profileName string
		want        ProfileUsageCounts
	}{
		{
			name: "zero repos",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{},
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
				},
			},
			profileName: "default",
			want:        ProfileUsageCounts{Explicit: 0, Implicit: 0, Total: 0},
		},
		{
			name: "explicit only",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "work"},
					"/repo2": {Repo: "repo2", Profile: "work"},
				},
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
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1"},
					"/repo2": {Repo: "repo2"},
					"/repo3": {Repo: "repo3", Profile: "work"},
				},
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
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1", Profile: "default"},
					"/repo2": {Repo: "repo2"},
					"/repo3": {Repo: "repo3"},
					"/repo4": {Repo: "repo4", Profile: "work"},
				},
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
					"work":    {ThoughtsRepo: "~/work"},
				},
			},
			profileName: "default",
			want:        ProfileUsageCounts{Explicit: 1, Implicit: 2, Total: 3},
		},
		{
			name: "non-default profile has no implicit",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": {Repo: "repo1"},
					"/repo2": {Repo: "repo2", Profile: "work"},
				},
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
					"work":    {ThoughtsRepo: "~/work"},
				},
			},
			profileName: "work",
			want:        ProfileUsageCounts{Explicit: 1, Implicit: 0, Total: 1},
		},
		{
			name: "handles nil mapping",
			cfg: &Config{
				RepoMappings: map[string]*RepoMapping{
					"/repo1": nil,
					"/repo2": {Repo: "repo2"},
				},
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
				},
			},
			profileName: "default",
			want:        ProfileUsageCounts{Explicit: 0, Implicit: 1, Total: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.CountReposUsingProfileWithImplicit(tt.profileName)
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

func TestGetDefaultScope(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want Scope
	}{
		{
			name: "returns configured scope",
			cfg:  &Config{DefaultScope: ScopeShared},
			want: ScopeShared,
		},
		{
			name: "returns user when configured as user",
			cfg:  &Config{DefaultScope: ScopeUser},
			want: ScopeUser,
		},
		{
			name: "defaults to user when empty",
			cfg:  &Config{DefaultScope: ""},
			want: ScopeUser,
		},
		{
			name: "defaults to user for new config",
			cfg:  &Config{},
			want: ScopeUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetDefaultScope()
			if got != tt.want {
				t.Errorf("GetDefaultScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCategories(t *testing.T) {
	t.Run("returns default categories when none configured", func(t *testing.T) {
		cfg := &Config{}

		categories := cfg.GetCategories()

		if categories == nil {
			t.Fatal("expected categories, got nil")
		}
		for _, name := range []string{"notes", "plans", "research", "decisions", "handoffs"} {
			if _, exists := categories[name]; !exists {
				t.Errorf("expected category %q to exist", name)
			}
		}
	})

	t.Run("returns default categories when empty map", func(t *testing.T) {
		cfg := &Config{Categories: map[string]*Category{}}

		categories := cfg.GetCategories()

		if categories == nil {
			t.Fatal("expected categories, got nil")
		}
		if _, exists := categories["notes"]; !exists {
			t.Error("expected category 'notes' to exist")
		}
	})

	t.Run("returns custom categories when configured", func(t *testing.T) {
		cfg := &Config{
			Categories: map[string]*Category{
				"custom": {Description: "Custom category"},
			},
		}

		categories := cfg.GetCategories()

		if categories == nil {
			t.Fatal("expected categories, got nil")
		}
		if _, exists := categories["custom"]; !exists {
			t.Error("expected category 'custom' to exist")
		}
		if _, exists := categories["notes"]; exists {
			t.Error("expected category 'notes' to not exist (custom replaces defaults)")
		}
	})
}

func TestGetCategoriesForProfile(t *testing.T) {
	t.Run("returns profile categories when set", func(t *testing.T) {
		cfg := &Config{
			Categories: map[string]*Category{
				"global": {Description: "Global category"},
			},
			Profiles: map[string]*ProfileConfig{
				"work": {
					ThoughtsRepo: "~/work",
					Categories: map[string]*Category{
						"tickets": {Description: "Work tickets"},
					},
				},
			},
		}

		categories := cfg.GetCategoriesForProfile("work")

		if categories == nil {
			t.Fatal("expected categories, got nil")
		}
		if _, exists := categories["tickets"]; !exists {
			t.Error("expected category 'tickets' to exist")
		}
		if _, exists := categories["global"]; exists {
			t.Error("expected category 'global' to not exist (profile overrides)")
		}
	})

	t.Run("returns global categories when profile has none", func(t *testing.T) {
		cfg := &Config{
			Categories: map[string]*Category{
				"global": {Description: "Global category"},
			},
			Profiles: map[string]*ProfileConfig{
				"work": {
					ThoughtsRepo: "~/work",
					// No categories set
				},
			},
		}

		categories := cfg.GetCategoriesForProfile("work")

		if categories == nil {
			t.Fatal("expected categories, got nil")
		}
		if _, exists := categories["global"]; !exists {
			t.Error("expected category 'global' to exist")
		}
	})

	t.Run("returns global categories when profile has empty map", func(t *testing.T) {
		cfg := &Config{
			Categories: map[string]*Category{
				"global": {Description: "Global category"},
			},
			Profiles: map[string]*ProfileConfig{
				"work": {
					ThoughtsRepo: "~/work",
					Categories:   map[string]*Category{},
				},
			},
		}

		categories := cfg.GetCategoriesForProfile("work")

		if categories == nil {
			t.Fatal("expected categories, got nil")
		}
		if _, exists := categories["global"]; !exists {
			t.Error("expected category 'global' to exist")
		}
	})

	t.Run("returns default categories when profile not found", func(t *testing.T) {
		cfg := &Config{}

		categories := cfg.GetCategoriesForProfile("nonexistent")

		if categories == nil {
			t.Fatal("expected categories, got nil")
		}
		if _, exists := categories["notes"]; !exists {
			t.Error("expected category 'notes' to exist")
		}
	})

	t.Run("returns defaults when no global categories and profile has none", func(t *testing.T) {
		cfg := &Config{
			Profiles: map[string]*ProfileConfig{
				"work": {ThoughtsRepo: "~/work"},
			},
		}

		categories := cfg.GetCategoriesForProfile("work")

		if categories == nil {
			t.Fatal("expected categories, got nil")
		}
		if _, exists := categories["notes"]; !exists {
			t.Error("expected category 'notes' to exist")
		}
	})
}

func TestGetCategory(t *testing.T) {
	t.Run("returns category when exists", func(t *testing.T) {
		cfg := &Config{
			Categories: map[string]*Category{
				"custom": {Description: "Custom category"},
			},
		}

		cat := cfg.GetCategory("custom")

		if cat == nil {
			t.Fatal("expected category, got nil")
		}
		if cat.Description != "Custom category" {
			t.Errorf("Description = %q, want %q", cat.Description, "Custom category")
		}
	})

	t.Run("returns nil when category not found", func(t *testing.T) {
		cfg := &Config{
			Categories: map[string]*Category{
				"custom": {Description: "Custom category"},
			},
		}

		cat := cfg.GetCategory("nonexistent")

		if cat != nil {
			t.Errorf("expected nil, got %+v", cat)
		}
	})

	t.Run("returns default category when no custom categories", func(t *testing.T) {
		cfg := &Config{}

		cat := cfg.GetCategory("notes")

		if cat == nil {
			t.Fatal("expected category, got nil")
		}
		if cat.Description != "Quick notes, gotchas, learnings" {
			t.Errorf("Description = %q, want %q", cat.Description, "Quick notes, gotchas, learnings")
		}
	})
}

func TestGetTemplate(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *Config
		categoryName    string
		subCategoryName string
		want            string
	}{
		{
			name:            "returns sub-category template when set",
			categoryName:    "plans",
			subCategoryName: "complete",
			cfg: &Config{
				Categories: map[string]*Category{
					"plans": {
						Description: "Plans",
						Template:    "plan.md",
						SubCategories: map[string]*SubCategory{
							"complete": {
								Description: "Complete plans",
								Template:    "complete-plan.md",
							},
						},
					},
				},
			},
			want: "complete-plan.md",
		},
		{
			name:            "falls back to category template when sub-category has none",
			categoryName:    "plans",
			subCategoryName: "active",
			cfg: &Config{
				Categories: map[string]*Category{
					"plans": {
						Description: "Plans",
						Template:    "plan.md",
						SubCategories: map[string]*SubCategory{
							"active": {
								Description: "Active plans",
								// No template
							},
						},
					},
				},
			},
			want: "plan.md",
		},
		{
			name:            "falls back to category template when sub-category not found",
			categoryName:    "plans",
			subCategoryName: "nonexistent",
			cfg: &Config{
				Categories: map[string]*Category{
					"plans": {
						Description: "Plans",
						Template:    "plan.md",
					},
				},
			},
			want: "plan.md",
		},
		{
			name:            "returns category template without sub-category",
			categoryName:    "plans",
			subCategoryName: "",
			cfg: &Config{
				Categories: map[string]*Category{
					"plans": {
						Description: "Plans",
						Template:    "plan.md",
					},
				},
			},
			want: "plan.md",
		},
		{
			name:            "falls back to defaultTemplate when category has none",
			categoryName:    "notes",
			subCategoryName: "",
			cfg: &Config{
				DefaultTemplate: "my-default.md",
				Categories: map[string]*Category{
					"notes": {
						Description: "Notes",
						// No template
					},
				},
			},
			want: "my-default.md",
		},
		{
			name:            "falls back to default.md when no templates configured",
			categoryName:    "notes",
			subCategoryName: "",
			cfg: &Config{
				Categories: map[string]*Category{
					"notes": {
						Description: "Notes",
					},
				},
			},
			want: "default.md",
		},
		{
			name:            "falls back to default.md when category not found",
			categoryName:    "nonexistent",
			subCategoryName: "",
			cfg:             &Config{},
			want:            "default.md",
		},
		{
			name:            "uses default categories and their templates",
			categoryName:    "research",
			subCategoryName: "",
			cfg:             &Config{},
			want:            "research.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetTemplate(tt.categoryName, tt.subCategoryName)
			if got != tt.want {
				t.Errorf("GetTemplate(%q, %q) = %q, want %q", tt.categoryName, tt.subCategoryName, got, tt.want)
			}
		})
	}
}

func TestDefaultCategories(t *testing.T) {
	categories := DefaultCategories()

	if categories == nil {
		t.Fatal("expected categories, got nil")
	}

	// Check all expected categories exist
	expectedCategories := []string{"research", "plans", "handoffs", "decisions", "notes"}
	for _, name := range expectedCategories {
		if _, exists := categories[name]; !exists {
			t.Errorf("expected category %q to exist", name)
		}
	}

	// Verify descriptions are set
	for name, cat := range categories {
		if cat.Description == "" {
			t.Errorf("category %q should have description", name)
		}
	}

	// Verify specific templates are set correctly
	if categories["research"].Template != "research.md" {
		t.Errorf("research.Template = %q, want %q", categories["research"].Template, "research.md")
	}
	if categories["plans"].Template != "plan.md" {
		t.Errorf("plans.Template = %q, want %q", categories["plans"].Template, "plan.md")
	}
	if categories["decisions"].Template != "decision.md" {
		t.Errorf("decisions.Template = %q, want %q", categories["decisions"].Template, "decision.md")
	}
	if categories["notes"].Template != "note.md" {
		t.Errorf("notes.Template = %q, want %q", categories["notes"].Template, "note.md")
	}
	if categories["handoffs"].Template != "" {
		t.Errorf("handoffs.Template = %q, want empty", categories["handoffs"].Template)
	}
}

func TestCategoryWithSubCategories(t *testing.T) {
	cfg := &Config{
		Categories: map[string]*Category{
			"plans": {
				Description: "Plans",
				Template:    "plan.md",
				SubCategories: map[string]*SubCategory{
					"todo": {
						Description: "Plans not yet started",
					},
					"active": {
						Description: "Currently being implemented",
					},
					"complete": {
						Description: "Finished plans",
						Trigger:     "When implementation is verified complete",
						Template:    "complete-plan.md",
					},
				},
			},
		},
	}

	cat := cfg.GetCategory("plans")
	if cat == nil {
		t.Fatal("expected category, got nil")
	}
	if cat.SubCategories == nil {
		t.Fatal("expected sub-categories, got nil")
	}
	if len(cat.SubCategories) != 3 {
		t.Errorf("len(SubCategories) = %d, want 3", len(cat.SubCategories))
	}

	// Verify sub-category properties
	complete := cat.SubCategories["complete"]
	if complete == nil {
		t.Fatal("expected sub-category 'complete', got nil")
	}
	if complete.Description != "Finished plans" {
		t.Errorf("Description = %q, want %q", complete.Description, "Finished plans")
	}
	if complete.Trigger != "When implementation is verified complete" {
		t.Errorf("Trigger = %q, want %q", complete.Trigger, "When implementation is verified complete")
	}
	if complete.Template != "complete-plan.md" {
		t.Errorf("Template = %q, want %q", complete.Template, "complete-plan.md")
	}

	// Verify template resolution
	if got := cfg.GetTemplate("plans", "complete"); got != "complete-plan.md" {
		t.Errorf("GetTemplate(plans, complete) = %q, want %q", got, "complete-plan.md")
	}
	if got := cfg.GetTemplate("plans", "todo"); got != "plan.md" {
		t.Errorf("GetTemplate(plans, todo) = %q, want %q", got, "plan.md")
	}
	if got := cfg.GetTemplate("plans", ""); got != "plan.md" {
		t.Errorf("GetTemplate(plans, \"\") = %q, want %q", got, "plan.md")
	}
}

func TestCategoryGetScope(t *testing.T) {
	tests := []struct {
		name string
		cat  *Category
		want CategoryScope
	}{
		{
			name: "returns shared when set",
			cat:  &Category{Description: "Test", Scope: CategoryScopeShared},
			want: CategoryScopeShared,
		},
		{
			name: "returns user when set",
			cat:  &Category{Description: "Test", Scope: CategoryScopeUser},
			want: CategoryScopeUser,
		},
		{
			name: "returns both when set",
			cat:  &Category{Description: "Test", Scope: CategoryScopeBoth},
			want: CategoryScopeBoth,
		},
		{
			name: "defaults to shared when empty",
			cat:  &Category{Description: "Test", Scope: ""},
			want: CategoryScopeShared,
		},
		{
			name: "defaults to shared for new category",
			cat:  &Category{Description: "Test"},
			want: CategoryScopeShared,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cat.GetScope()
			if got != tt.want {
				t.Errorf("GetScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubCategoryGetScope(t *testing.T) {
	tests := []struct {
		name        string
		sub         *SubCategory
		parentScope CategoryScope
		want        CategoryScope
	}{
		{
			name:        "returns own scope when set",
			sub:         &SubCategory{Description: "Test", Scope: CategoryScopeUser},
			parentScope: CategoryScopeShared,
			want:        CategoryScopeUser,
		},
		{
			name:        "inherits parent scope when empty",
			sub:         &SubCategory{Description: "Test", Scope: ""},
			parentScope: CategoryScopeShared,
			want:        CategoryScopeShared,
		},
		{
			name:        "inherits parent scope when not set",
			sub:         &SubCategory{Description: "Test"},
			parentScope: CategoryScopeBoth,
			want:        CategoryScopeBoth,
		},
		{
			name:        "can override parent shared with user",
			sub:         &SubCategory{Description: "Test", Scope: CategoryScopeUser},
			parentScope: CategoryScopeShared,
			want:        CategoryScopeUser,
		},
		{
			name:        "can override parent user with shared",
			sub:         &SubCategory{Description: "Test", Scope: CategoryScopeShared},
			parentScope: CategoryScopeUser,
			want:        CategoryScopeShared,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sub.GetScope(tt.parentScope)
			if got != tt.want {
				t.Errorf("GetScope(%v) = %v, want %v", tt.parentScope, got, tt.want)
			}
		})
	}
}

func TestDefaultCategoriesScope(t *testing.T) {
	categories := DefaultCategories()

	// Verify scope values are set correctly for defaults
	expectedScopes := map[string]CategoryScope{
		"research":  CategoryScopeShared,
		"plans":     CategoryScopeShared,
		"handoffs":  CategoryScopeShared,
		"decisions": CategoryScopeShared,
		"notes":     CategoryScopeBoth,
	}

	for name, expectedScope := range expectedScopes {
		cat, exists := categories[name]
		if !exists {
			t.Errorf("expected category %q to exist", name)
			continue
		}
		if cat.Scope != expectedScope {
			t.Errorf("category %q: Scope = %v, want %v", name, cat.Scope, expectedScope)
		}
	}
}

// TestConfigTemplateCoversAllFields ensures the YAML template documents all Config fields.
// This test prevents drift between the Config struct and the template.
func TestConfigTemplateCoversAllFields(t *testing.T) {
	// Parse the embedded template
	var templateConfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(ConfigTemplate), &templateConfig); err != nil {
		t.Fatalf("failed to parse ConfigTemplate: %v", err)
	}

	// Expected top-level keys from Config struct (based on yaml tags)
	// These are the configurable fields users should know about
	expectedKeys := []string{
		"user",
		"autoSyncInWorktrees",
		"gitignore",
		"defaultScope",
		"defaultTemplate",
		"sync",
		"agents",
		"hooks",
		"categories",
		"profiles",
	}

	// Check that all expected keys are documented in the template
	// Note: repoMappings is intentionally excluded as it's shown commented out
	// and is an advanced feature
	var missing []string
	for _, key := range expectedKeys {
		if _, exists := templateConfig[key]; !exists {
			// Also check if it's in a comment (starts with #)
			if !strings.Contains(ConfigTemplate, key+":") {
				missing = append(missing, key)
			}
		}
	}

	if len(missing) > 0 {
		t.Errorf("ConfigTemplate is missing documentation for fields: %v\n"+
			"Update config_template.yaml to document these fields.", missing)
	}

	// Verify template can be parsed as a valid Config
	var cfg Config
	if err := yaml.Unmarshal([]byte(ConfigTemplate), &cfg); err != nil {
		t.Errorf("ConfigTemplate is not valid Config YAML: %v", err)
	}
}

func TestFullDefaults(t *testing.T) {
	cfg := FullDefaults()

	// Verify base defaults are included
	if !cfg.AutoSyncInWorktrees {
		t.Error("expected AutoSyncInWorktrees to be true")
	}
	if cfg.Gitignore != ComponentModeLocal {
		t.Errorf("expected Gitignore to be local, got %s", cfg.Gitignore)
	}

	// Verify expanded defaults
	if cfg.DefaultScope != ScopeUser {
		t.Errorf("expected DefaultScope to be user, got %s", cfg.DefaultScope)
	}
	if cfg.DefaultTemplate != "default.md" {
		t.Errorf("expected DefaultTemplate to be default.md, got %s", cfg.DefaultTemplate)
	}

	// Verify Sync config
	if cfg.Sync == nil {
		t.Fatal("expected Sync to be set")
	}
	if cfg.Sync.Mode != SyncModeFull {
		t.Errorf("expected Sync.Mode to be full, got %s", cfg.Sync.Mode)
	}

	// Verify Agents config
	if cfg.Agents == nil {
		t.Fatal("expected Agents to be set")
	}
	if cfg.Agents.Skills != ComponentModeLocal {
		t.Errorf("expected Agents.Skills to be local, got %s", cfg.Agents.Skills)
	}
	if cfg.Agents.Commands != ComponentModeLocal {
		t.Errorf("expected Agents.Commands to be local, got %s", cfg.Agents.Commands)
	}
	if cfg.Agents.Agents != ComponentModeLocal {
		t.Errorf("expected Agents.Agents to be local, got %s", cfg.Agents.Agents)
	}

	// Verify Hooks config
	if cfg.Hooks == nil {
		t.Fatal("expected Hooks to be set")
	}
	if len(cfg.Hooks.Keywords) == 0 {
		t.Error("expected Hooks.Keywords to have values")
	}

	// Verify Categories
	if cfg.Categories == nil {
		t.Fatal("expected Categories to be set")
	}
	if len(cfg.Categories) == 0 {
		t.Error("expected Categories to have values")
	}

	// Verify Profiles (inherited from Defaults)
	if cfg.Profiles == nil {
		t.Fatal("expected Profiles to be set")
	}
	if _, exists := cfg.Profiles["default"]; !exists {
		t.Error("expected default profile to exist")
	}

	// Verify Sync config has commit message templates
	if cfg.Sync.CommitMessage == "" {
		t.Error("expected Sync.CommitMessage to be set")
	}
	if cfg.Sync.CommitMessageHook == "" {
		t.Error("expected Sync.CommitMessageHook to be set")
	}
}

func TestGetCommitMessage(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		profileName string
		want        string
	}{
		{
			name: "returns default when no config set",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
				},
			},
			profileName: "default",
			want:        DefaultCommitMessage(),
		},
		{
			name: "returns global commitMessage when set",
			cfg: &Config{
				Sync: &SyncConfig{
					CommitMessage: "Global: {{.Repo}}",
				},
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
				},
			},
			profileName: "default",
			want:        "Global: {{.Repo}}",
		},
		{
			name: "profile commitMessage overrides global",
			cfg: &Config{
				Sync: &SyncConfig{
					CommitMessage: "Global: {{.Repo}}",
				},
				Profiles: map[string]*ProfileConfig{
					"work": {
						ThoughtsRepo: "~/work",
						Sync: &SyncConfig{
							CommitMessage: "Work: {{.Repo}}",
						},
					},
				},
			},
			profileName: "work",
			want:        "Work: {{.Repo}}",
		},
		{
			name: "falls back to global when profile has no override",
			cfg: &Config{
				Sync: &SyncConfig{
					CommitMessage: "Global: {{.Repo}}",
				},
				Profiles: map[string]*ProfileConfig{
					"work": {
						ThoughtsRepo: "~/work",
						// No Sync override
					},
				},
			},
			profileName: "work",
			want:        "Global: {{.Repo}}",
		},
		{
			name: "falls back to global when profile Sync is empty",
			cfg: &Config{
				Sync: &SyncConfig{
					CommitMessage: "Global: {{.Repo}}",
				},
				Profiles: map[string]*ProfileConfig{
					"work": {
						ThoughtsRepo: "~/work",
						Sync:         &SyncConfig{}, // Empty sync config
					},
				},
			},
			profileName: "work",
			want:        "Global: {{.Repo}}",
		},
		{
			name: "returns default when profile not found",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{},
			},
			profileName: "nonexistent",
			want:        DefaultCommitMessage(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetCommitMessage(tt.profileName)
			if got != tt.want {
				t.Errorf("GetCommitMessage(%q) = %q, want %q", tt.profileName, got, tt.want)
			}
		})
	}
}

func TestGetCommitMessageHook(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		profileName string
		want        string
	}{
		{
			name: "returns default when no config set",
			cfg: &Config{
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
				},
			},
			profileName: "default",
			want:        DefaultCommitMessageHook(),
		},
		{
			name: "returns global commitMessageHook when set",
			cfg: &Config{
				Sync: &SyncConfig{
					CommitMessageHook: "Global hook: {{.CommitMessage}}",
				},
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
				},
			},
			profileName: "default",
			want:        "Global hook: {{.CommitMessage}}",
		},
		{
			name: "profile commitMessageHook overrides global",
			cfg: &Config{
				Sync: &SyncConfig{
					CommitMessageHook: "Global hook: {{.CommitMessage}}",
				},
				Profiles: map[string]*ProfileConfig{
					"work": {
						ThoughtsRepo: "~/work",
						Sync: &SyncConfig{
							CommitMessageHook: "Work hook: {{.CommitMessage}}",
						},
					},
				},
			},
			profileName: "work",
			want:        "Work hook: {{.CommitMessage}}",
		},
		{
			name: "falls back to global when profile has no override",
			cfg: &Config{
				Sync: &SyncConfig{
					CommitMessageHook: "Global hook: {{.CommitMessage}}",
				},
				Profiles: map[string]*ProfileConfig{
					"work": {
						ThoughtsRepo: "~/work",
					},
				},
			},
			profileName: "work",
			want:        "Global hook: {{.CommitMessage}}",
		},
		{
			name: "commitMessage and commitMessageHook resolve independently",
			cfg: &Config{
				Sync: &SyncConfig{
					CommitMessage: "Global sync",
					// No commitMessageHook set - should use default
				},
				Profiles: map[string]*ProfileConfig{
					"default": {ThoughtsRepo: "~/thoughts", Default: true},
				},
			},
			profileName: "default",
			want:        DefaultCommitMessageHook(), // Not "Global sync"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetCommitMessageHook(tt.profileName)
			if got != tt.want {
				t.Errorf("GetCommitMessageHook(%q) = %q, want %q", tt.profileName, got, tt.want)
			}
		})
	}
}
