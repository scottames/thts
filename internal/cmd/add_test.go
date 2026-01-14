package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottames/thts/internal/config"
)

func TestValidateAddOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    *AddOptions
		wantErr string
	}{
		{
			name:    "valid empty options",
			opts:    &AddOptions{},
			wantErr: "",
		},
		{
			name: "valid with category",
			opts: &AddOptions{
				Category: "notes",
				Title:    "Test note",
			},
			wantErr: "",
		},
		{
			name: "repo and profile mutually exclusive",
			opts: &AddOptions{
				RepoPath:    "/some/repo",
				ProfileName: "work",
			},
			wantErr: "--repo and --profile are mutually exclusive",
		},
		{
			name: "shared and personal mutually exclusive",
			opts: &AddOptions{
				ForceShared: true,
				ForceUser:   true,
			},
			wantErr: "--shared and --personal are mutually exclusive",
		},
		{
			name: "valid with shared flag",
			opts: &AddOptions{
				ForceShared: true,
				Category:    "notes",
			},
			wantErr: "",
		},
		{
			name: "valid with personal flag",
			opts: &AddOptions{
				ForceUser: true,
				Category:  "notes",
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAddOptions(tt.opts)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateAddOptions() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("validateAddOptions() error = nil, want %q", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("validateAddOptions() error = %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestParseCategoryPath(t *testing.T) {
	tests := []struct {
		input      string
		wantCat    string
		wantSubCat string
	}{
		{"", "", ""},
		{"notes", "notes", ""},
		{"plans/active", "plans", "active"},
		{"plans/active/nested", "plans", "active/nested"}, // only splits on first /
		{"research", "research", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cat, subCat := parseCategoryPath(tt.input)
			if cat != tt.wantCat {
				t.Errorf("parseCategoryPath(%q) category = %q, want %q", tt.input, cat, tt.wantCat)
			}
			if subCat != tt.wantSubCat {
				t.Errorf("parseCategoryPath(%q) subCategory = %q, want %q", tt.input, subCat, tt.wantSubCat)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"API Design Decisions", "api-design-decisions"},
		{"  Leading/Trailing  ", "leadingtrailing"},
		{"Special!@#$%Characters", "specialcharacters"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"Under_Score_Test", "under-score-test"},
		{"UPPERCASE", "uppercase"},
		{"", "untitled"},
		{"---", "untitled"},
		{"a", "a"},
		// Long title should be truncated to 50 chars
		{"This is a very long title that should be truncated to fit within the maximum allowed length for filenames", "this-is-a-very-long-title-that-should-be-truncated"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateFilename(t *testing.T) {
	tests := []struct {
		title      string
		wantPrefix string // We can't test the exact date, but we can check the pattern
		wantSuffix string
	}{
		{"My Note", "-my-note.md", ".md"},
		{"", "-untitled.md", ".md"},
		{"API Design", "-api-design.md", ".md"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := generateFilename(tt.title)
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("generateFilename(%q) = %q, want suffix %q", tt.title, got, tt.wantSuffix)
			}
			// Check date format (YYYY-MM-DD-)
			if len(got) < 11 || got[4] != '-' || got[7] != '-' {
				t.Errorf("generateFilename(%q) = %q, doesn't start with date format", tt.title, got)
			}
		})
	}
}

func TestResolveScopePath(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.Config
		opts      *AddOptions
		wantScope string
	}{
		{
			name: "force shared overrides everything",
			cfg: &config.Config{
				User:         "testuser",
				DefaultScope: config.ScopeUser,
			},
			opts: &AddOptions{
				ForceShared: true,
				Category:    "notes",
			},
			wantScope: "shared",
		},
		{
			name: "force user overrides everything",
			cfg: &config.Config{
				User:         "testuser",
				DefaultScope: config.ScopeShared,
			},
			opts: &AddOptions{
				ForceUser: true,
				Category:  "notes",
			},
			wantScope: "testuser",
		},
		{
			name: "category scope shared takes precedence over config default",
			cfg: &config.Config{
				User:         "testuser",
				DefaultScope: config.ScopeUser,
				Categories: map[string]*config.Category{
					"research": {
						Description: "Research",
						Scope:       config.CategoryScopeShared,
					},
				},
			},
			opts: &AddOptions{
				Category: "research",
			},
			wantScope: "shared",
		},
		{
			name: "category scope user takes precedence over config default",
			cfg: &config.Config{
				User:         "testuser",
				DefaultScope: config.ScopeShared,
				Categories: map[string]*config.Category{
					"private": {
						Description: "Private notes",
						Scope:       config.CategoryScopeUser,
					},
				},
			},
			opts: &AddOptions{
				Category: "private",
			},
			wantScope: "testuser",
		},
		{
			name: "category scope both uses config default (user)",
			cfg: &config.Config{
				User:         "testuser",
				DefaultScope: config.ScopeUser,
				Categories: map[string]*config.Category{
					"notes": {
						Description: "Notes",
						Scope:       config.CategoryScopeBoth,
					},
				},
			},
			opts: &AddOptions{
				Category: "notes",
			},
			wantScope: "testuser",
		},
		{
			name: "category scope both uses config default (shared)",
			cfg: &config.Config{
				User:         "testuser",
				DefaultScope: config.ScopeShared,
				Categories: map[string]*config.Category{
					"notes": {
						Description: "Notes",
						Scope:       config.CategoryScopeBoth,
					},
				},
			},
			opts: &AddOptions{
				Category: "notes",
			},
			wantScope: "shared",
		},
		{
			name: "unknown category uses config default",
			cfg: &config.Config{
				User:         "testuser",
				DefaultScope: config.ScopeUser,
			},
			opts: &AddOptions{
				Category: "unknown",
			},
			wantScope: "testuser",
		},
		{
			name: "no category uses config default",
			cfg: &config.Config{
				User:         "testuser",
				DefaultScope: config.ScopeShared,
			},
			opts:      &AddOptions{},
			wantScope: "shared",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveScopePath(tt.cfg, tt.opts)
			if got != tt.wantScope {
				t.Errorf("resolveScopePath() = %q, want %q", got, tt.wantScope)
			}
		})
	}
}

func TestBuildTargetPath(t *testing.T) {
	tests := []struct {
		name   string
		target *AddTarget
		want   string
	}{
		{
			name: "repo target with category",
			target: &AddTarget{
				ThoughtsDir: "/home/user/project/thoughts",
				ScopePath:   "shared",
				Category:    "notes",
				IsGlobal:    false,
			},
			want: "/home/user/project/thoughts/shared/notes",
		},
		{
			name: "repo target with subcategory",
			target: &AddTarget{
				ThoughtsDir: "/home/user/project/thoughts",
				ScopePath:   "testuser",
				Category:    "plans",
				SubCategory: "active",
				IsGlobal:    false,
			},
			want: "/home/user/project/thoughts/testuser/plans/active",
		},
		{
			name: "global target with category",
			target: &AddTarget{
				ThoughtsDir: "/home/user/thoughts/global",
				ScopePath:   "shared",
				Category:    "research",
				IsGlobal:    true,
			},
			want: "/home/user/thoughts/global/shared/research",
		},
		{
			name: "global target with subcategory",
			target: &AddTarget{
				ThoughtsDir: "/home/user/thoughts/global",
				ScopePath:   "testuser",
				Category:    "plans",
				SubCategory: "complete",
				IsGlobal:    true,
			},
			want: "/home/user/thoughts/global/testuser/plans/complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTargetPath(tt.target)
			if got != tt.want {
				t.Errorf("buildTargetPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnsureTargetDir(t *testing.T) {
	// Create a temp directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		targetPath     string
		wantCreatedLen int // Number of directories expected to be created
		wantErr        bool
	}{
		{
			name:           "create nested directories",
			targetPath:     filepath.Join(tmpDir, "a", "b", "c"),
			wantCreatedLen: 3,
			wantErr:        false,
		},
		{
			name:           "directory already exists",
			targetPath:     tmpDir, // Already exists
			wantCreatedLen: 0,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := ensureTargetDir(tt.targetPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureTargetDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(created) != tt.wantCreatedLen {
				t.Errorf("ensureTargetDir() created %d dirs, want %d", len(created), tt.wantCreatedLen)
			}
			// Verify directory exists
			if _, err := os.Stat(tt.targetPath); os.IsNotExist(err) {
				t.Errorf("ensureTargetDir() directory was not created: %s", tt.targetPath)
			}
		})
	}
}

func TestGetTemplateContent(t *testing.T) {
	// Create temp directory with .templates/
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test template
	testTemplate := "---\ndate: test\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(templatesDir, "note.md"), []byte(testTemplate), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		thoughtsDir  string
		templateName string
		wantContains string
	}{
		{
			name:         "existing template",
			thoughtsDir:  tmpDir,
			templateName: "note.md",
			wantContains: "# Test",
		},
		{
			name:         "non-existent template returns default",
			thoughtsDir:  tmpDir,
			templateName: "nonexistent.md",
			wantContains: "date:",
		},
		{
			name:         "non-existent dir returns default",
			thoughtsDir:  "/nonexistent/path",
			templateName: "note.md",
			wantContains: "date:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTemplateContent(tt.thoughtsDir, tt.templateName)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("getTemplateContent() = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}

func TestResolveAddTarget_DefaultCategory(t *testing.T) {
	// Test that empty category defaults to "notes"
	tmpDir := t.TempDir()

	// Create a minimal valid thoughts setup
	thoughtsDir := filepath.Join(tmpDir, "thoughts")
	if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create shared symlink (just a regular dir for testing)
	sharedDir := filepath.Join(thoughtsDir, "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Make it a symlink (remove dir first, ignore error as it may not exist)
	if err := os.Remove(sharedDir); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Symlink(tmpDir, sharedDir); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		User:         "testuser",
		DefaultScope: config.ScopeUser,
		Profiles: map[string]*config.ProfileConfig{
			"default": {
				ThoughtsRepo: tmpDir,
				GlobalDir:    "global",
				Default:      true,
			},
		},
	}

	// Create global dir
	globalDir := filepath.Join(tmpDir, "global", "testuser")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}

	opts := &AddOptions{
		// No category specified
		ProfileName: "default", // Use profile to avoid git repo detection
	}

	target, err := resolveAddTarget(cfg, opts)
	if err != nil {
		t.Fatalf("resolveAddTarget() error = %v", err)
	}

	if target.Category != "notes" {
		t.Errorf("resolveAddTarget() category = %q, want %q", target.Category, "notes")
	}
}
