package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteAndLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()

	original := &Manifest{
		IntegrationLevel: IntegrationAlwaysOn,
		Files:            []string{"tpd-instructions.md", "skills/tpd-integrate.md"},
		SettingsCreated:  true,
		Modifications: ManifestModifications{
			ClaudeMD: &ClaudeMDModification{
				Path:    "/path/to/CLAUDE.md",
				Action:  "appended",
				Pattern: "@.claude/tpd-instructions.md",
			},
			Gitignore: &GitignoreModification{
				Patterns: []string{".claude/CLAUDE.local.md"},
			},
		},
	}

	// Write manifest
	if err := writeManifest(tmpDir, original); err != nil {
		t.Fatalf("writeManifest() error: %v", err)
	}

	// Verify file exists
	manifestPath := filepath.Join(tmpDir, ManifestFile)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatal("manifest file not created")
	}

	// Load manifest
	loaded, err := loadManifest(tmpDir)
	if err != nil {
		t.Fatalf("loadManifest() error: %v", err)
	}

	// Verify fields
	if loaded.Version != 1 {
		t.Errorf("expected Version=1, got %d", loaded.Version)
	}
	if loaded.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
	if loaded.IntegrationLevel != IntegrationAlwaysOn {
		t.Errorf("expected IntegrationLevel=%s, got %s", IntegrationAlwaysOn, loaded.IntegrationLevel)
	}
	if len(loaded.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(loaded.Files))
	}
	if !loaded.SettingsCreated {
		t.Error("expected SettingsCreated=true")
	}
	if loaded.Modifications.ClaudeMD == nil {
		t.Error("expected ClaudeMD modification")
	}
	if loaded.Modifications.ClaudeMD.Action != "appended" {
		t.Errorf("expected Action=appended, got %s", loaded.Modifications.ClaudeMD.Action)
	}
}

func TestDetectInstallation(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create some tpd files
	if err := os.WriteFile(filepath.Join(claudeDir, "tpd-instructions.md"), []byte("# Instructions"), 0644); err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(claudeDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "tpd-integrate.md"), []byte("# Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test detection
	manifest := detectInstallation(claudeDir, tmpDir)
	if manifest == nil {
		t.Fatal("expected manifest to be detected")
	}

	if len(manifest.Files) != 2 {
		t.Errorf("expected 2 files detected, got %d: %v", len(manifest.Files), manifest.Files)
	}

	// Verify tpd-instructions.md is detected
	found := false
	for _, f := range manifest.Files {
		if f == "tpd-instructions.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tpd-instructions.md to be detected")
	}
}

func TestDetectInstallationNoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a non-tpd file
	if err := os.WriteFile(filepath.Join(claudeDir, "other.md"), []byte("# Other"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test detection
	manifest := detectInstallation(claudeDir, tmpDir)
	if manifest != nil {
		t.Error("expected no manifest when no tpd files present")
	}
}

func TestRemoveFromClaudeMD(t *testing.T) {
	tmpDir := t.TempDir()
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "include at end with newlines",
			content:  "# Instructions\n\nSome content.\n@.claude/tpd-instructions.md\n",
			expected: "# Instructions\n\nSome content.\n",
		},
		{
			name:     "include in middle",
			content:  "# Instructions\n@.claude/tpd-instructions.md\nMore content.",
			expected: "# Instructions\nMore content.",
		},
		{
			name:     "include at start",
			content:  "@.claude/tpd-instructions.md\n# Instructions",
			expected: "# Instructions",
		},
		{
			name:     "include only",
			content:  "@.claude/tpd-instructions.md\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(claudeMDPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			mod := &ClaudeMDModification{
				Path:    claudeMDPath,
				Pattern: "@.claude/tpd-instructions.md",
			}

			if err := removeFromClaudeMD(mod); err != nil {
				t.Fatalf("removeFromClaudeMD() error: %v", err)
			}

			result, err := os.ReadFile(claudeMDPath)
			if err != nil {
				t.Fatal(err)
			}

			if string(result) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}

func TestIsDirEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty dir
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	isEmpty, err := isDirEmpty(emptyDir)
	if err != nil {
		t.Fatalf("isDirEmpty() error: %v", err)
	}
	if !isEmpty {
		t.Error("expected empty dir to be empty")
	}

	// Non-empty dir
	nonEmptyDir := filepath.Join(tmpDir, "nonempty")
	if err := os.MkdirAll(nonEmptyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonEmptyDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	isEmpty, err = isDirEmpty(nonEmptyDir)
	if err != nil {
		t.Fatalf("isDirEmpty() error: %v", err)
	}
	if isEmpty {
		t.Error("expected non-empty dir to not be empty")
	}
}

func TestUninitWithManifest(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create tpd files
	if err := os.WriteFile(filepath.Join(claudeDir, "tpd-instructions.md"), []byte("# Instructions"), 0644); err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(claudeDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "tpd-integrate.md"), []byte("# Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create manifest
	manifest := &Manifest{
		Version:          1,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		IntegrationLevel: IntegrationOnDemand,
		Files:            []string{"tpd-instructions.md", "skills/tpd-integrate.md"},
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, ManifestFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Run uninit
	if err := Uninit(tmpDir, true); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}

	// Verify files are removed
	if _, err := os.Stat(filepath.Join(claudeDir, "tpd-instructions.md")); !os.IsNotExist(err) {
		t.Error("expected tpd-instructions.md to be removed")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "tpd-integrate.md")); !os.IsNotExist(err) {
		t.Error("expected tpd-integrate.md to be removed")
	}
	if _, err := os.Stat(filepath.Join(claudeDir, ManifestFile)); !os.IsNotExist(err) {
		t.Error("expected manifest to be removed")
	}
}

func TestUninitNoClaudeDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Should return nil when no .claude directory
	if err := Uninit(tmpDir, true); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
}

func TestUninitDetection(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create tpd files without manifest (simulate old installation)
	if err := os.WriteFile(filepath.Join(claudeDir, "tpd-instructions.md"), []byte("# Instructions"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run uninit (should use detection)
	if err := Uninit(tmpDir, true); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}

	// Verify file is removed
	if _, err := os.Stat(filepath.Join(claudeDir, "tpd-instructions.md")); !os.IsNotExist(err) {
		t.Error("expected tpd-instructions.md to be removed via detection")
	}
}

func TestUninitWithClaudeMDModification(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create CLAUDE.md with @include
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	claudeMDContent := "# Instructions\n\nSome content.\n@.claude/tpd-instructions.md\n"
	if err := os.WriteFile(claudeMDPath, []byte(claudeMDContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create tpd files
	if err := os.WriteFile(filepath.Join(claudeDir, "tpd-instructions.md"), []byte("# Instructions"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create manifest with CLAUDE.md modification
	manifest := &Manifest{
		Version:          1,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		IntegrationLevel: IntegrationAlwaysOn,
		Files:            []string{"tpd-instructions.md"},
		Modifications: ManifestModifications{
			ClaudeMD: &ClaudeMDModification{
				Path:    claudeMDPath,
				Action:  "appended",
				Pattern: "@.claude/tpd-instructions.md",
			},
		},
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, ManifestFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Run uninit
	if err := Uninit(tmpDir, true); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}

	// Verify CLAUDE.md still exists but @include is removed
	content, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("CLAUDE.md should still exist: %v", err)
	}
	if strings.Contains(string(content), "@.claude/tpd-instructions.md") {
		t.Error("expected @include to be removed from CLAUDE.md")
	}
	if !strings.Contains(string(content), "# Instructions") {
		t.Error("expected other CLAUDE.md content to be preserved")
	}
}
