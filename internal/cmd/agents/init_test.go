package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottames/thts/internal/config"
)

func TestAdjustHeaderLevels(t *testing.T) {
	t.Run("increments headers by offset", func(t *testing.T) {
		input := "# Title\n## Section\n### Subsection"
		expected := "## Title\n### Section\n#### Subsection"

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("preserves non-header lines", func(t *testing.T) {
		input := "# Title\nSome text\n## Section\nMore text"
		expected := "## Title\nSome text\n### Section\nMore text"

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("skips headers inside fenced code blocks", func(t *testing.T) {
		input := "# Title\n```markdown\n# This is inside code\n## Also inside\n```\n## Real Section"
		expected := "## Title\n```markdown\n# This is inside code\n## Also inside\n```\n### Real Section"

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("handles tilde fenced code blocks", func(t *testing.T) {
		input := "# Title\n~~~\n# Inside tilde block\n~~~\n## Section"
		expected := "## Title\n~~~\n# Inside tilde block\n~~~\n### Section"

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("zero offset returns unchanged", func(t *testing.T) {
		input := "# Title\n## Section"

		result := adjustHeaderLevels(input, 0)
		if result != input {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, input)
		}
	})

	t.Run("negative offset returns unchanged", func(t *testing.T) {
		input := "# Title\n## Section"

		result := adjustHeaderLevels(input, -1)
		if result != input {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, input)
		}
	})

	t.Run("handles offset of 2", func(t *testing.T) {
		input := "# Title\n## Section"
		expected := "### Title\n#### Section"

		result := adjustHeaderLevels(input, 2)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		result := adjustHeaderLevels("", 1)
		if result != "" {
			t.Errorf("adjustHeaderLevels() = %q, want empty string", result)
		}
	})

	t.Run("handles multiple code blocks", func(t *testing.T) {
		input := strings.Join([]string{
			"# Title",
			"```",
			"# code header",
			"```",
			"## Section",
			"```go",
			"// # comment that looks like header",
			"```",
			"### Subsection",
		}, "\n")

		expected := strings.Join([]string{
			"## Title",
			"```",
			"# code header",
			"```",
			"### Section",
			"```go",
			"// # comment that looks like header",
			"```",
			"#### Subsection",
		}, "\n")

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() =\n%s\nwant\n%s", result, expected)
		}
	})
}

func TestDetectExistingAgentManifests(t *testing.T) {
	t.Run("detects manifests in project directories", func(t *testing.T) {
		// Create temp directory
		tmpDir, err := os.MkdirTemp("", "thts-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		// Create .claude directory with manifest
		claudeDir := filepath.Join(tmpDir, ".claude")
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			t.Fatalf("failed to create .claude dir: %v", err)
		}
		manifestPath := filepath.Join(claudeDir, ManifestFile)
		if err := os.WriteFile(manifestPath, []byte(`{"version":1,"agent":"claude"}`), 0644); err != nil {
			t.Fatalf("failed to write manifest: %v", err)
		}

		// Detect
		detected := detectExistingAgentManifests(tmpDir)

		if len(detected) != 1 {
			t.Errorf("expected 1 detected agent, got %d", len(detected))
		}
	})

	t.Run("returns empty for no manifests", func(t *testing.T) {
		// Create temp directory
		tmpDir, err := os.MkdirTemp("", "thts-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		detected := detectExistingAgentManifests(tmpDir)

		if len(detected) != 0 {
			t.Errorf("expected 0 detected agents, got %d", len(detected))
		}
	})
}

func TestBuildInstructionsData(t *testing.T) {
	t.Run("builds data from config with categories", func(t *testing.T) {
		cfg := &config.Config{
			User: "testuser",
			Categories: map[string]*config.Category{
				"notes": {
					Description: "Quick notes",
					Scope:       config.CategoryScopeShared,
				},
				"research": {
					Description: "Research findings",
					Trigger:     "After research phase",
					Scope:       config.CategoryScopeShared,
				},
			},
		}

		data := buildInstructionsData(cfg)

		if data.User != "testuser" {
			t.Errorf("expected user 'testuser', got %q", data.User)
		}

		if len(data.Categories) != 2 {
			t.Errorf("expected 2 categories, got %d", len(data.Categories))
		}

		// Check that categories are present
		found := make(map[string]bool)
		for _, cat := range data.Categories {
			found[cat.Name] = true
		}

		if !found["notes"] {
			t.Error("expected 'notes' category")
		}
		if !found["research"] {
			t.Error("expected 'research' category")
		}
	})

	t.Run("flattens sub-categories", func(t *testing.T) {
		cfg := &config.Config{
			User: "testuser",
			Categories: map[string]*config.Category{
				"plans": {
					Description: "Implementation plans",
					Scope:       config.CategoryScopeShared,
					SubCategories: map[string]*config.SubCategory{
						"active": {
							Description: "Active plans",
						},
						"complete": {
							Description: "Completed plans",
						},
					},
				},
			},
		}

		data := buildInstructionsData(cfg)

		// Should have 3 entries: plans, plans/active, plans/complete
		if len(data.Categories) != 3 {
			t.Errorf("expected 3 categories (including sub-categories), got %d", len(data.Categories))
		}

		// Check paths
		paths := make(map[string]bool)
		for _, cat := range data.Categories {
			paths[cat.Name] = true
		}

		if !paths["plans"] {
			t.Error("expected 'plans' category")
		}
		if !paths["plans/active"] {
			t.Error("expected 'plans/active' sub-category")
		}
		if !paths["plans/complete"] {
			t.Error("expected 'plans/complete' sub-category")
		}
	})

	t.Run("uses default categories when config has none", func(t *testing.T) {
		cfg := &config.Config{
			User: "testuser",
			// No categories set - should use defaults
		}

		data := buildInstructionsData(cfg)

		// Should have default categories
		if len(data.Categories) == 0 {
			t.Error("expected default categories to be used when config has none")
		}
	})
}

func TestBuildLocationString(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		scope    config.CategoryScope
		expected string
	}{
		{
			name:     "shared scope",
			path:     "notes",
			scope:    config.CategoryScopeShared,
			expected: "`thoughts/shared/notes/`",
		},
		{
			name:     "user scope",
			path:     "tickets",
			scope:    config.CategoryScopeUser,
			expected: "`thoughts/{user}/tickets/`",
		},
		{
			name:     "both scope",
			path:     "research",
			scope:    config.CategoryScopeBoth,
			expected: "`thoughts/shared/research/` or `thoughts/{user}/research/`",
		},
		{
			name:     "nested path",
			path:     "plans/active",
			scope:    config.CategoryScopeShared,
			expected: "`thoughts/shared/plans/active/`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLocationString(tt.path, tt.scope)
			if result != tt.expected {
				t.Errorf("buildLocationString(%q, %v) = %q, want %q",
					tt.path, tt.scope, result, tt.expected)
			}
		})
	}
}
