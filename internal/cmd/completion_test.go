package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scottames/thts/internal/config"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func TestCompleteCategories(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		toComplete     string
		wantCategories []string
		wantDirective  cobra.ShellCompDirective
	}{
		{
			name:       "returns top-level categories with no input",
			config:     nil, // uses defaults
			toComplete: "",
			wantCategories: []string{
				"decisions",
				"handoffs",
				"notes",
				"plans",
				"research",
			},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:       "returns top-level categories with partial match",
			config:     nil,
			toComplete: "no",
			wantCategories: []string{
				"notes",
			},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns sub-categories when input ends with slash",
			config: &config.Config{
				Categories: map[string]*config.Category{
					"plans": {
						Description: "Plans",
						SubCategories: map[string]*config.SubCategory{
							"active":   {Description: "Active plans"},
							"complete": {Description: "Complete plans"},
							"todo":     {Description: "Todo plans"},
						},
					},
				},
			},
			toComplete: "plans/",
			wantCategories: []string{
				"plans/active",
				"plans/complete",
				"plans/todo",
			},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "returns sub-categories with partial match after slash",
			config: &config.Config{
				Categories: map[string]*config.Category{
					"plans": {
						Description: "Plans",
						SubCategories: map[string]*config.SubCategory{
							"active":   {Description: "Active plans"},
							"complete": {Description: "Complete plans"},
							"todo":     {Description: "Todo plans"},
						},
					},
				},
			},
			toComplete: "plans/co",
			wantCategories: []string{
				"plans/complete",
			},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:           "returns empty for non-existent category",
			config:         nil,
			toComplete:     "nonexistent/",
			wantCategories: []string{},
			wantDirective:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:           "returns empty for category without sub-categories",
			config:         nil,
			toComplete:     "notes/",
			wantCategories: []string{},
			wantDirective:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name: "uses custom categories from config",
			config: &config.Config{
				Categories: map[string]*config.Category{
					"tickets": {Description: "Jira tickets"},
					"journal": {Description: "Daily journal"},
				},
			},
			toComplete: "",
			wantCategories: []string{
				"journal",
				"tickets",
			},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test config
			configDir := setupTestConfig(t, tt.config)
			t.Setenv("XDG_CONFIG_HOME", configDir)

			got, directive := CompleteCategories(nil, nil, tt.toComplete)

			if directive != tt.wantDirective {
				t.Errorf("CompleteCategories() directive = %v, want %v", directive, tt.wantDirective)
			}

			if !slicesEqual(got, tt.wantCategories) {
				t.Errorf("CompleteCategories() = %v, want %v", got, tt.wantCategories)
			}
		})
	}
}

func TestCompleteCategoriesWithProfile(t *testing.T) {
	cfg := &config.Config{
		User: "testuser",
		Categories: map[string]*config.Category{
			"global-notes": {Description: "Global notes"},
		},
		Profiles: map[string]*config.ProfileConfig{
			"work": {
				ThoughtsRepo: "~/work-thoughts",
				Default:      true,
				Categories: map[string]*config.Category{
					"tickets": {Description: "Jira tickets"},
					"sprints": {
						Description: "Sprint planning",
						SubCategories: map[string]*config.SubCategory{
							"current":  {Description: "Current sprint"},
							"backlog":  {Description: "Backlog items"},
							"archived": {Description: "Archived sprints"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		profileName    string
		toComplete     string
		wantCategories []string
	}{
		{
			name:        "profile categories override global",
			profileName: "work",
			toComplete:  "",
			wantCategories: []string{
				"sprints",
				"tickets",
			},
		},
		{
			name:        "profile sub-categories work",
			profileName: "work",
			toComplete:  "sprints/",
			wantCategories: []string{
				"sprints/archived",
				"sprints/backlog",
				"sprints/current",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir := setupTestConfig(t, cfg)
			t.Setenv("XDG_CONFIG_HOME", configDir)

			got, _ := CompleteCategoriesForProfile(nil, nil, tt.toComplete, tt.profileName)

			if !slicesEqual(got, tt.wantCategories) {
				t.Errorf("CompleteCategoriesForProfile() = %v, want %v", got, tt.wantCategories)
			}
		})
	}
}

// setupTestConfig creates a temporary config directory with the given config.
// If cfg is nil, no config file is created (uses defaults).
func setupTestConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()

	configDir := t.TempDir()
	thtsDir := filepath.Join(configDir, "thts")
	if err := os.MkdirAll(thtsDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	if cfg != nil {
		configPath := filepath.Join(thtsDir, "config.yaml")
		data, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatalf("failed to marshal config: %v", err)
		}
		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
	}

	return configDir
}

// slicesEqual compares two string slices for equality (order matters).
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
