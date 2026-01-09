package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tpdfiles "github.com/scottames/tpd"
)

func TestIsValidModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"haiku", true},
		{"sonnet", true},
		{"opus", true},
		{"Opus", false}, // case-sensitive
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := isValidModel(tt.model)
			if got != tt.want {
				t.Errorf("isValidModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	slice := []string{"commands", "agents", "settings"}

	tests := []struct {
		item string
		want bool
	}{
		{"commands", true},
		{"agents", true},
		{"settings", true},
		{"other", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.item, func(t *testing.T) {
			got := contains(slice, tt.item)
			if got != tt.want {
				t.Errorf("contains(%q) = %v, want %v", tt.item, got, tt.want)
			}
		})
	}
}

func TestListEmbeddedFiles(t *testing.T) {
	// Test listing command files
	commands, err := listEmbeddedFiles(tpdfiles.Commands, "commands")
	if err != nil {
		t.Fatalf("listEmbeddedFiles(commands) error: %v", err)
	}
	if len(commands) == 0 {
		t.Error("expected at least one command file, got none")
	}
	// Check that tpd-handoff.md is included
	found := false
	for _, f := range commands {
		if f == "tpd-handoff.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected tpd-handoff.md in commands, got: %v", commands)
	}

	// Test listing agent files
	agents, err := listEmbeddedFiles(tpdfiles.Agents, "agents")
	if err != nil {
		t.Fatalf("listEmbeddedFiles(agents) error: %v", err)
	}
	if len(agents) == 0 {
		t.Error("expected at least one agent file, got none")
	}
	// Check that thoughts-locator.md is included
	found = false
	for _, f := range agents {
		if f == "thoughts-locator.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected thoughts-locator.md in agents, got: %v", agents)
	}
}

func TestWriteSettings(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name              string
		alwaysThinking    bool
		maxThinkingTokens int
		model             ModelType
	}{
		{
			name:              "defaults",
			alwaysThinking:    true,
			maxThinkingTokens: 32000,
			model:             ModelOpus,
		},
		{
			name:              "custom values",
			alwaysThinking:    false,
			maxThinkingTokens: 16000,
			model:             ModelSonnet,
		},
		{
			name:              "haiku with max tokens",
			alwaysThinking:    true,
			maxThinkingTokens: 64000,
			model:             ModelHaiku,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(claudeDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatal(err)
			}

			err := writeSettings(testDir, tt.alwaysThinking, tt.maxThinkingTokens, tt.model)
			if err != nil {
				t.Fatalf("writeSettings() error: %v", err)
			}

			// Read and verify the settings file
			settingsPath := filepath.Join(testDir, "settings.json")
			content, err := os.ReadFile(settingsPath)
			if err != nil {
				t.Fatalf("failed to read settings.json: %v", err)
			}

			var settings map[string]any
			if err := json.Unmarshal(content, &settings); err != nil {
				t.Fatalf("failed to unmarshal settings: %v", err)
			}

			// Check model
			if model, ok := settings["model"].(string); !ok || model != string(tt.model) {
				t.Errorf("expected model %q, got %v", tt.model, settings["model"])
			}

			// Check alwaysThinkingEnabled
			if tt.alwaysThinking {
				if enabled, ok := settings["alwaysThinkingEnabled"].(bool); !ok || !enabled {
					t.Errorf("expected alwaysThinkingEnabled true, got %v", settings["alwaysThinkingEnabled"])
				}
			} else {
				// When false, the key should not be present
				if _, ok := settings["alwaysThinkingEnabled"]; ok {
					t.Error("expected alwaysThinkingEnabled to be absent when false")
				}
			}

			// Check env settings
			env, ok := settings["env"].(map[string]any)
			if !ok {
				t.Fatal("expected env map in settings")
			}
			if env["CLAUDE_BASH_MAINTAIN_WORKING_DIR"] != "1" {
				t.Errorf("expected CLAUDE_BASH_MAINTAIN_WORKING_DIR='1', got %v", env["CLAUDE_BASH_MAINTAIN_WORKING_DIR"])
			}
		})
	}
}

func TestCopyEmbeddedCategory(t *testing.T) {
	tmpDir := t.TempDir()

	// Test copying all commands
	targetDir := filepath.Join(tmpDir, "commands")
	copied, skipped, copiedFiles, err := copyEmbeddedCategory(tpdfiles.Commands, "commands", targetDir, nil)
	if err != nil {
		t.Fatalf("copyEmbeddedCategory() error: %v", err)
	}
	if copied == 0 {
		t.Error("expected at least one file copied")
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped when copying all, got %d", skipped)
	}
	if len(copiedFiles) != copied {
		t.Errorf("expected copiedFiles length %d to match copied count %d", len(copiedFiles), copied)
	}

	// Verify the file exists
	handoffPath := filepath.Join(targetDir, "tpd-handoff.md")
	if _, err := os.Stat(handoffPath); os.IsNotExist(err) {
		t.Error("expected tpd-handoff.md to be copied")
	}

	// Test copying selected files only
	targetDir2 := filepath.Join(tmpDir, "commands2")
	copied2, skipped2, copiedFiles2, err := copyEmbeddedCategory(
		tpdfiles.Commands, "commands", targetDir2,
		[]string{"tpd-handoff.md"},
	)
	if err != nil {
		t.Fatalf("copyEmbeddedCategory() with selection error: %v", err)
	}
	if copied2 != 1 {
		t.Errorf("expected 1 file copied, got %d", copied2)
	}
	if len(copiedFiles2) != 1 || copiedFiles2[0] != "tpd-handoff.md" {
		t.Errorf("expected copiedFiles2 to be ['tpd-handoff.md'], got %v", copiedFiles2)
	}
	// All others should be skipped
	allFiles, _ := listEmbeddedFiles(tpdfiles.Commands, "commands")
	expectedSkipped := len(allFiles) - 1
	if skipped2 != expectedSkipped {
		t.Errorf("expected %d skipped, got %d", expectedSkipped, skipped2)
	}

	// Test with empty selection (skip all)
	targetDir3 := filepath.Join(tmpDir, "commands3")
	copied3, skipped3, copiedFiles3, err := copyEmbeddedCategory(
		tpdfiles.Commands, "commands", targetDir3,
		[]string{},
	)
	if err != nil {
		t.Fatalf("copyEmbeddedCategory() with empty selection error: %v", err)
	}
	if copied3 != 0 {
		t.Errorf("expected 0 files copied with empty selection, got %d", copied3)
	}
	if len(copiedFiles3) != 0 {
		t.Errorf("expected no files in copiedFiles3, got %v", copiedFiles3)
	}
	if skipped3 != len(allFiles) {
		t.Errorf("expected %d skipped, got %d", len(allFiles), skipped3)
	}
}
