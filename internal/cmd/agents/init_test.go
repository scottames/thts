package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalagents "github.com/scottames/thts/internal/agents"
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

func TestCopyPluginsToManifest_TracksPluginOnce(t *testing.T) {
	projectDir := t.TempDir()
	cfg := internalagents.GetConfig(internalagents.AgentOpenCode)
	agentDir := filepath.Join(projectDir, cfg.RootDir)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("create agent directory: %v", err)
	}
	manifest := &Manifest{
		Agent: string(internalagents.AgentOpenCode),
		Files: []string{filepath.Join("skills", "thts-integrate", "SKILL.md")},
	}

	_, err := copyPluginsToManifest(
		agentDir,
		internalagents.AgentOpenCode,
		cfg,
		manifest,
	)
	if err != nil {
		t.Fatalf("copyPluginsToManifest() error: %v", err)
	}

	relativePlugin := filepath.Join("plugins", "thts-integration.ts")
	if _, err := os.Stat(filepath.Join(agentDir, relativePlugin)); err != nil {
		t.Fatalf("expected local plugin: %v", err)
	}
	if countValue(manifest.Files, relativePlugin) == 0 {
		t.Fatalf("manifest files = %v, missing %s", manifest.Files, relativePlugin)
	}

	if _, err := copyPluginsToManifest(agentDir, internalagents.AgentOpenCode, cfg, manifest); err != nil {
		t.Fatalf("second copyPluginsToManifest() error: %v", err)
	}
	if count := countValue(manifest.Files, relativePlugin); count != 1 {
		t.Fatalf("plugin appears %d times in manifest, want 1", count)
	}
}

func TestBuildInstallationPlan_OpenCodeLocalIgnoresOnlyPlugin(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))

	plan, err := buildInstallationPlan(t.TempDir(), internalagents.AgentOpenCode, IntegrationAgentsContentLocal)
	if err != nil {
		t.Fatalf("buildInstallationPlan() error: %v", err)
	}

	relativePlugin := filepath.Join("plugins", "thts-integration.ts")
	if len(plan.pluginFiles) != 1 || plan.pluginFiles[0] != relativePlugin {
		t.Fatalf("plugin files = %v, want [%s]", plan.pluginFiles, relativePlugin)
	}
	if len(plan.gitignorePatterns) != 0 {
		t.Fatalf("gitignore patterns = %v, want no redundant exact plugin pattern", plan.gitignorePatterns)
	}
	if plan.instructionsFile != "" {
		t.Fatalf("instructions file = %q, want empty", plan.instructionsFile)
	}
}

func TestOpenCodeLocalPluginLifecycle(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	projectDir := t.TempDir()
	cfg := internalagents.GetConfig(internalagents.AgentOpenCode)
	agentDir := filepath.Join(projectDir, cfg.RootDir)
	relativePlugin := filepath.Join(cfg.PluginsDir, "thts-integration.ts")
	pluginPath := filepath.Join(agentDir, relativePlugin)

	if err := initAgent(projectDir, internalagents.AgentOpenCode, IntegrationAgentsContentLocal); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	if err := updateGitignoreForAgents(projectDir, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
		t.Fatalf("updateGitignoreForAgents() error: %v", err)
	}

	manifest, err := loadManifest(agentDir)
	if err != nil {
		t.Fatalf("loadManifest() error: %v", err)
	}
	if manifest.IntegrationLevel != IntegrationAgentsContentLocal {
		t.Fatalf("integration level = %q, want %q", manifest.IntegrationLevel, IntegrationAgentsContentLocal)
	}
	if countValue(manifest.Files, relativePlugin) != 1 {
		t.Fatalf("manifest files = %v, want one %s", manifest.Files, relativePlugin)
	}
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatalf("expected plugin after init: %v", err)
	}

	if err := os.WriteFile(pluginPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("write stale plugin: %v", err)
	}
	if err := refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
		t.Fatalf("refreshAgentSetup() error: %v", err)
	}
	plugin, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("read refreshed plugin: %v", err)
	}
	if string(plugin) == "stale" {
		t.Error("refresh did not restore embedded plugin")
	}
	manifest, err = loadManifest(agentDir)
	if err != nil {
		t.Fatalf("load refreshed manifest: %v", err)
	}
	if countValue(manifest.Files, relativePlugin) != 1 {
		t.Fatalf("refreshed manifest files = %v, want one %s", manifest.Files, relativePlugin)
	}

	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	if _, err := os.Stat(pluginPath); !os.IsNotExist(err) {
		t.Fatalf("plugin still exists after uninit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentDir, ManifestFile)); !os.IsNotExist(err) {
		t.Fatalf("manifest still exists after uninit: %v", err)
	}
}

func TestRefreshOpenCodeLocalPluginMigratesLegacyInstructions(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	projectDir := t.TempDir()
	cfg := internalagents.GetConfig(internalagents.AgentOpenCode)
	agentDir := filepath.Join(projectDir, cfg.RootDir)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("create agent directory: %v", err)
	}

	legacyFile := filepath.Join(agentDir, "AGENTS.local.md")
	if err := os.WriteFile(legacyFile, []byte("# Local Agent Instructions\n\n@thts-instructions.md\n"), 0644); err != nil {
		t.Fatalf("write legacy instructions: %v", err)
	}
	legacyPattern := filepath.Join(cfg.RootDir, "AGENTS.local.md")
	if err := os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(legacyPattern+"\n"), 0644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	manifest := &Manifest{
		Agent:            string(internalagents.AgentOpenCode),
		IntegrationLevel: IntegrationAgentsContentLocal,
		Modifications: ManifestModifications{
			Gitignore: &GitignoreModification{Patterns: []string{legacyPattern}},
		},
	}
	if err := writeManifest(agentDir, manifest); err != nil {
		t.Fatalf("write legacy manifest: %v", err)
	}

	if err := refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
		t.Fatalf("refreshAgentSetup() error: %v", err)
	}
	if _, err := os.Stat(legacyFile); !os.IsNotExist(err) {
		t.Fatalf("legacy instructions still exist: %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(projectDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if strings.Contains(string(gitignore), legacyPattern) {
		t.Errorf("legacy gitignore pattern still present: %s", gitignore)
	}

	manifest, err = loadManifest(agentDir)
	if err != nil {
		t.Fatalf("load migrated manifest: %v", err)
	}
	relativePlugin := filepath.Join(cfg.PluginsDir, "thts-integration.ts")
	if countValue(manifest.Files, relativePlugin) != 1 {
		t.Fatalf("migrated manifest files = %v, want one %s", manifest.Files, relativePlugin)
	}
}

func countValue(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}
