package cmd

import (
	"encoding/json"
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
		{"", ""},    // empty input returns empty (caller handles this)
		{"---", ""}, // no alphanumeric chars returns empty
		{"!!!", ""}, // only special chars returns empty
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
		wantSuffix string
		wantOK     bool
	}{
		{"My Note", "-my-note.md", true},
		{"API Design", "-api-design.md", true},
		{"", "", false},    // empty title is invalid
		{"---", "", false}, // title with no alphanumeric chars is invalid
		{"!!!", "", false}, // title with only special chars is invalid
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got, ok := generateFilename(tt.title)
			if ok != tt.wantOK {
				t.Errorf("generateFilename(%q) ok = %v, want %v", tt.title, ok, tt.wantOK)
				return
			}
			if !tt.wantOK {
				return // Don't check the filename if we expect failure
			}
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
		wantFound    bool
	}{
		{
			name:         "existing template",
			thoughtsDir:  tmpDir,
			templateName: "note.md",
			wantContains: "# Test",
			wantFound:    true,
		},
		{
			name:         "non-existent template returns default",
			thoughtsDir:  tmpDir,
			templateName: "nonexistent.md",
			wantContains: "date:",
			wantFound:    false,
		},
		{
			name:         "non-existent dir returns default",
			thoughtsDir:  "/nonexistent/path",
			templateName: "note.md",
			wantContains: "date:",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := getTemplateContent(tt.thoughtsDir, tt.templateName)
			if found != tt.wantFound {
				t.Errorf("getTemplateContent() found = %v, want %v", found, tt.wantFound)
			}
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

func TestValidateAddOptions_ContentMutualExclusion(t *testing.T) {
	tests := []struct {
		name    string
		opts    *AddOptions
		wantErr string
	}{
		{
			name: "content only is valid",
			opts: &AddOptions{
				Content: "some content",
			},
			wantErr: "",
		},
		{
			name: "from only is valid",
			opts: &AddOptions{
				FromFile: "file.md",
			},
			wantErr: "",
		},
		{
			name: "stdin only is valid",
			opts: &AddOptions{
				FromStdin: true,
			},
			wantErr: "",
		},
		{
			name: "no-edit only is valid",
			opts: &AddOptions{
				NoEdit: true,
			},
			wantErr: "",
		},
		{
			name: "content and from mutually exclusive",
			opts: &AddOptions{
				Content:  "some content",
				FromFile: "file.md",
			},
			wantErr: "content argument, --from, and --stdin are mutually exclusive",
		},
		{
			name: "content and stdin mutually exclusive",
			opts: &AddOptions{
				Content:   "some content",
				FromStdin: true,
			},
			wantErr: "content argument, --from, and --stdin are mutually exclusive",
		},
		{
			name: "from and stdin mutually exclusive",
			opts: &AddOptions{
				FromFile:  "file.md",
				FromStdin: true,
			},
			wantErr: "content argument, --from, and --stdin are mutually exclusive",
		},
		{
			name: "all three mutually exclusive",
			opts: &AddOptions{
				Content:   "some content",
				FromFile:  "file.md",
				FromStdin: true,
			},
			wantErr: "content argument, --from, and --stdin are mutually exclusive",
		},
		{
			name: "no-edit with content is valid",
			opts: &AddOptions{
				Content: "some content",
				NoEdit:  true,
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

func TestResolveContent_InlineContent(t *testing.T) {
	opts := &AddOptions{
		Content: "# My Thought\n\nSome content here.",
	}

	content, shouldOpenEditor, err := resolveContent(opts, "/tmp", "default.md")
	if err != nil {
		t.Fatalf("resolveContent() error = %v", err)
	}

	if content != "# My Thought\n\nSome content here." {
		t.Errorf("resolveContent() content = %q, want inline content", content)
	}

	if shouldOpenEditor {
		t.Error("resolveContent() shouldOpenEditor = true, want false for inline content")
	}
}

func TestResolveContent_FromFile(t *testing.T) {
	// Create a temp file with content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	testContent := "# From File\n\nFile content here."
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	opts := &AddOptions{
		FromFile: testFile,
	}

	content, shouldOpenEditor, err := resolveContent(opts, "/tmp", "default.md")
	if err != nil {
		t.Fatalf("resolveContent() error = %v", err)
	}

	if content != testContent {
		t.Errorf("resolveContent() content = %q, want file content", content)
	}

	if shouldOpenEditor {
		t.Error("resolveContent() shouldOpenEditor = true, want false for file content")
	}
}

func TestResolveContent_FromFile_NotFound(t *testing.T) {
	opts := &AddOptions{
		FromFile: "/nonexistent/path/file.md",
	}

	_, _, err := resolveContent(opts, "/tmp", "default.md")
	if err == nil {
		t.Fatal("resolveContent() error = nil, want error for missing file")
	}

	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("resolveContent() error = %q, want 'file not found' error", err.Error())
	}
}

func TestResolveContent_Template(t *testing.T) {
	// Create temp directory with .templates/
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test template
	testTemplate := "---\ndate: test\n---\n\n# Template Content\n"
	if err := os.WriteFile(filepath.Join(templatesDir, "note.md"), []byte(testTemplate), 0644); err != nil {
		t.Fatal(err)
	}

	opts := &AddOptions{} // No content flags

	content, shouldOpenEditor, err := resolveContent(opts, tmpDir, "note.md")
	if err != nil {
		t.Fatalf("resolveContent() error = %v", err)
	}

	if !strings.Contains(content, "# Template Content") {
		t.Errorf("resolveContent() content = %q, want template content", content)
	}

	if !shouldOpenEditor {
		t.Error("resolveContent() shouldOpenEditor = false, want true for template")
	}
}

func TestResolveContent_NoEdit(t *testing.T) {
	opts := &AddOptions{
		NoEdit: true,
	}

	_, shouldOpenEditor, err := resolveContent(opts, "/tmp", "default.md")
	if err != nil {
		t.Fatalf("resolveContent() error = %v", err)
	}

	if shouldOpenEditor {
		t.Error("resolveContent() shouldOpenEditor = true, want false with --no-edit")
	}
}

func TestResolveSyncContext(t *testing.T) {
	t.Run("profile flag specified", func(t *testing.T) {
		cfg := &config.Config{
			User: "testuser",
			Profiles: map[string]*config.ProfileConfig{
				"work": {
					ThoughtsRepo: "/home/user/work-thoughts",
					GlobalDir:    "global",
				},
				"default": {
					ThoughtsRepo: "/home/user/thoughts",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		opts := &AddOptions{
			ProfileName: "work",
		}

		ctx, err := resolveSyncContext(cfg, opts)
		if err != nil {
			t.Fatalf("resolveSyncContext() error = %v", err)
		}

		if ctx.RepoPath != "/home/user/work-thoughts" {
			t.Errorf("resolveSyncContext().RepoPath = %q, want %q", ctx.RepoPath, "/home/user/work-thoughts")
		}
		if ctx.ProfileName != "work" {
			t.Errorf("resolveSyncContext().ProfileName = %q, want %q", ctx.ProfileName, "work")
		}
	})

	t.Run("profile flag invalid profile", func(t *testing.T) {
		cfg := &config.Config{
			User: "testuser",
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					ThoughtsRepo: "/home/user/thoughts",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		opts := &AddOptions{
			ProfileName: "nonexistent",
		}

		_, err := resolveSyncContext(cfg, opts)
		if err == nil {
			t.Error("resolveSyncContext() error = nil, want error for invalid profile")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("resolveSyncContext() error = %q, want 'not found' error", err.Error())
		}
	})

	t.Run("falls back to default profile", func(t *testing.T) {
		cfg := &config.Config{
			User: "testuser",
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					ThoughtsRepo: "/home/user/thoughts",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		opts := &AddOptions{} // No profile, no repo

		ctx, err := resolveSyncContext(cfg, opts)
		if err != nil {
			t.Fatalf("resolveSyncContext() error = %v", err)
		}

		if ctx.RepoPath != "/home/user/thoughts" {
			t.Errorf("resolveSyncContext().RepoPath = %q, want %q", ctx.RepoPath, "/home/user/thoughts")
		}
		if ctx.ProfileName != "default" {
			t.Errorf("resolveSyncContext().ProfileName = %q, want %q", ctx.ProfileName, "default")
		}
	})

	t.Run("no default profile configured", func(t *testing.T) {
		cfg := &config.Config{
			User:     "testuser",
			Profiles: map[string]*config.ProfileConfig{},
		}

		opts := &AddOptions{}

		_, err := resolveSyncContext(cfg, opts)
		if err == nil {
			t.Error("resolveSyncContext() error = nil, want error for no default profile")
		}
		if !strings.Contains(err.Error(), "no default profile") {
			t.Errorf("resolveSyncContext() error = %q, want 'no default profile' error", err.Error())
		}
	})
}

func TestValidateAddOptions_JSONQuietMutualExclusion(t *testing.T) {
	tests := []struct {
		name    string
		opts    *AddOptions
		wantErr string
	}{
		{
			name: "json only is valid",
			opts: &AddOptions{
				JSON: true,
			},
			wantErr: "",
		},
		{
			name: "quiet only is valid",
			opts: &AddOptions{
				Quiet: true,
			},
			wantErr: "",
		},
		{
			name: "json and quiet mutually exclusive",
			opts: &AddOptions{
				JSON:  true,
				Quiet: true,
			},
			wantErr: "--json and --quiet are mutually exclusive",
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

func TestExecuteAdd(t *testing.T) {
	t.Run("successful add with template", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create thoughts directory structure with template
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		templatesDir := filepath.Join(thoughtsDir, ".templates")
		sharedDir := filepath.Join(thoughtsDir, "shared")
		if err := os.MkdirAll(templatesDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create shared as a symlink (required for valid setup)
		targetDir := filepath.Join(tmpDir, "shared-target")
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(targetDir, sharedDir); err != nil {
			t.Fatal(err)
		}

		// Create a template
		templateContent := "---\ndate: test\n---\n\n# Test Template\n"
		if err := os.WriteFile(filepath.Join(templatesDir, "note.md"), []byte(templateContent), 0644); err != nil {
			t.Fatal(err)
		}

		cfg := &config.Config{
			User:         "testuser",
			DefaultScope: config.ScopeShared,
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					ThoughtsRepo: tmpDir,
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		// Create global dir for fallback
		globalDir := filepath.Join(tmpDir, "global", "shared")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatal(err)
		}

		opts := &AddOptions{
			Title:       "Test Note",
			Category:    "notes",
			ProfileName: "default",
		}

		result, target, err := executeAdd(cfg, opts)
		if err != nil {
			t.Fatalf("executeAdd() error = %v", err)
		}

		// Verify result
		if result.FilePath == "" {
			t.Error("executeAdd() FilePath is empty")
		}
		if !strings.HasSuffix(result.FilePath, "-test-note.md") {
			t.Errorf("executeAdd() FilePath = %q, want suffix '-test-note.md'", result.FilePath)
		}
		if result.TemplateUsed != "note.md" {
			t.Errorf("executeAdd() TemplateUsed = %q, want 'note.md'", result.TemplateUsed)
		}

		// Verify target
		if target.Category != "notes" {
			t.Errorf("executeAdd() target.Category = %q, want 'notes'", target.Category)
		}

		// Verify file was created
		if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
			t.Errorf("executeAdd() file was not created: %s", result.FilePath)
		}
	})

	t.Run("tracks directory creation", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create minimal thoughts setup
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		sharedDir := filepath.Join(thoughtsDir, "shared")
		targetDir := filepath.Join(tmpDir, "shared-target")
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(targetDir, sharedDir); err != nil {
			t.Fatal(err)
		}

		cfg := &config.Config{
			User:         "testuser",
			DefaultScope: config.ScopeShared,
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					ThoughtsRepo: tmpDir,
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		// Create global dir
		globalDir := filepath.Join(tmpDir, "global", "shared")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatal(err)
		}

		opts := &AddOptions{
			Title:       "Test Note",
			Category:    "newcategory/subcategory", // New nested category
			ProfileName: "default",
			Content:     "test content",
		}

		result, _, err := executeAdd(cfg, opts)
		if err != nil {
			t.Fatalf("executeAdd() error = %v", err)
		}

		// Should have created directories
		if len(result.DirsCreated) == 0 {
			t.Error("executeAdd() DirsCreated is empty, expected directories to be created")
		}
	})

	t.Run("error on invalid title", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.Config{
			User:         "testuser",
			DefaultScope: config.ScopeShared,
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					ThoughtsRepo: tmpDir,
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		// Create global dir
		globalDir := filepath.Join(tmpDir, "global", "shared")
		if err := os.MkdirAll(globalDir, 0755); err != nil {
			t.Fatal(err)
		}

		opts := &AddOptions{
			Title:       "---", // Invalid title (no alphanumeric chars)
			ProfileName: "default",
		}

		_, _, err := executeAdd(cfg, opts)
		if err == nil {
			t.Error("executeAdd() error = nil, want error for invalid title")
		}
		if !strings.Contains(err.Error(), "does not produce a valid filename") {
			t.Errorf("executeAdd() error = %q, want 'does not produce a valid filename' error", err.Error())
		}
	})
}

func TestAddResultJSONTags(t *testing.T) {
	// Verify AddResult can be marshaled to JSON with expected field names
	result := &AddResult{
		FilePath:       "/path/to/file.md",
		DirsCreated:    []string{"/path/to", "/path/to/dir"},
		TemplateUsed:   "note.md",
		OpenedInEditor: true,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	jsonStr := string(data)

	// Check for snake_case field names
	expectedFields := []string{
		`"file_path"`,
		`"dirs_created"`,
		`"template_used"`,
		`"opened_in_editor"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON output missing field %s, got: %s", field, jsonStr)
		}
	}

	// Verify it can be unmarshaled back
	var decoded AddResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.FilePath != result.FilePath {
		t.Errorf("Roundtrip FilePath = %q, want %q", decoded.FilePath, result.FilePath)
	}
	if decoded.OpenedInEditor != result.OpenedInEditor {
		t.Errorf("Roundtrip OpenedInEditor = %v, want %v", decoded.OpenedInEditor, result.OpenedInEditor)
	}
}
