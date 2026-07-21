package agents

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	thtsfiles "github.com/scottames/thts"
	internalagents "github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
)

func TestWriteAgentSettingsUsesEmbeddedTemplateAtAgentDestination(t *testing.T) {
	for _, tt := range []struct {
		agentType   internalagents.AgentType
		destination string
	}{
		{agentType: internalagents.AgentCodex, destination: "config.toml"},
		{agentType: internalagents.AgentGemini, destination: "settings.json"},
		{agentType: internalagents.AgentOpenCode, destination: "opencode.json"},
	} {
		t.Run(string(tt.agentType), func(t *testing.T) {
			cfg := internalagents.GetConfig(tt.agentType)
			if cfg.SettingsFile != tt.destination {
				t.Fatalf("settings destination = %q, want %q", cfg.SettingsFile, tt.destination)
			}
			agentDir := t.TempDir()

			if err := writeAgentSettings(agentDir, tt.agentType); err != nil {
				t.Fatalf("writeAgentSettings() error: %v", err)
			}

			got, err := os.ReadFile(filepath.Join(agentDir, cfg.SettingsFile))
			if err != nil {
				t.Fatalf("read destination %s: %v", cfg.SettingsFile, err)
			}
			want := thtsfiles.GetDefaultSettings(cfg.SettingsTemplate)
			if string(got) != want {
				t.Errorf("settings at %s = %q, want embedded default %q", cfg.SettingsFile, got, want)
			}
		})
	}
}

func TestClaudeSettingsRemainDynamic(t *testing.T) {
	cfg := internalagents.GetConfig(internalagents.AgentClaude)
	if cfg.SettingsTemplate != "" {
		t.Fatalf("Claude settings template = %q, want empty for dynamic settings", cfg.SettingsTemplate)
	}

	agentDir := t.TempDir()
	if err := writeAgentSettings(agentDir, internalagents.AgentClaude); err != nil {
		t.Fatalf("writeAgentSettings() error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(agentDir, cfg.SettingsFile))
	if err != nil {
		t.Fatalf("read Claude settings: %v", err)
	}
	if !strings.Contains(string(got), "alwaysThinkingEnabled") {
		t.Errorf("Claude settings = %q, want dynamic Claude default", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	previous := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
		_ = reader.Close()
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(output)
}

func TestResolveAgentComponentMode(t *testing.T) {
	manifest := &GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {
			Agents: []string{"claude", "opencode"},
			Files: []string{
				filepath.Join(config.GlobalAgentDir("claude"), "skills", "thts-integrate.md"),
				filepath.Join(config.GlobalAgentDir("opencode"), "skills", "thts-integrate", "SKILL.md"),
			},
		},
	}}

	legacyGlobal := &config.Config{Agents: &config.AgentsConfig{Skills: config.ComponentModeGlobal}}
	for _, agentType := range []internalagents.AgentType{internalagents.AgentClaude, internalagents.AgentOpenCode} {
		if got := resolveAgentComponentMode(legacyGlobal, manifest, agentType, "skills"); got != config.ComponentModeGlobal {
			t.Errorf("legacy global %s skills = %q, want global", agentType, got)
		}
	}
	if got := resolveAgentComponentMode(legacyGlobal, manifest, internalagents.AgentPi, "skills"); got != config.ComponentModeLocal {
		t.Errorf("legacy global Pi skills = %q, want local without ownership", got)
	}
	pathlessPi := &GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {Agents: []string{"pi"}},
	}}
	if got := resolveAgentComponentMode(legacyGlobal, pathlessPi, internalagents.AgentPi, "skills"); got != config.ComponentModeLocal {
		t.Errorf("legacy global Pi skills with no component path = %q, want local", got)
	}
	manifest.RecordAgentComponent("skills", internalagents.AgentPi, []string{filepath.Join(config.GlobalAgentDir("pi"), "skills", "thts-integrate", "SKILL.md")})
	if got := resolveAgentComponentMode(legacyGlobal, manifest, internalagents.AgentPi, "skills"); got != config.ComponentModeGlobal {
		t.Errorf("legacy global Pi skills after install = %q, want global", got)
	}
	if got := resolveAgentComponentMode(legacyGlobal, nil, internalagents.AgentClaude, "skills"); got != config.ComponentModeLocal {
		t.Errorf("legacy global without manifest = %q, want local", got)
	}

	override := &config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"pi": {Skills: config.ComponentModeGlobal},
	}}}
	if got := resolveAgentComponentMode(override, nil, internalagents.AgentPi, "skills"); got != config.ComponentModeGlobal {
		t.Errorf("Pi override = %q, want global", got)
	}
	if got := resolveAgentComponentMode(override, nil, internalagents.AgentClaude, "skills"); got != config.ComponentModeLocal {
		t.Errorf("Claude default = %q, want local", got)
	}
}

func TestResolveGlobalAgentSelectionPrefersExplicitAgents(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	if err := config.Save(&config.Config{Profiles: map[string]*config.ProfileConfig{
		"default": {
			Default:       true,
			DefaultAgents: []string{"claude", "opencode"},
		},
	}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	previous := initAgents
	t.Cleanup(func() { initAgents = previous })
	initAgents = "pi"

	selected, err := resolveGlobalAgentSelection()
	if err != nil {
		t.Fatalf("resolveGlobalAgentSelection() error: %v", err)
	}
	if len(selected) != 1 || selected[0] != internalagents.AgentPi {
		t.Fatalf("explicit selection = %v, want [pi]", selected)
	}

	initAgents = ""
	selected, err = resolveGlobalAgentSelection()
	if err != nil {
		t.Fatalf("resolveGlobalAgentSelection() error: %v", err)
	}
	if got, want := internalagents.AgentTypesToStrings(selected), []string{"claude", "opencode"}; !slices.Equal(got, want) {
		t.Fatalf("profile selection = %v, want %v", got, want)
	}
	if err := os.Remove(config.ThtsConfigPath()); err != nil {
		t.Fatalf("remove profile config: %v", err)
	}
	selected, err = resolveGlobalAgentSelection()
	if err != nil {
		t.Fatalf("resolveGlobalAgentSelection() error: %v", err)
	}
	if got, want := selected, internalagents.AllAgentTypes(); !slices.Equal(got, want) {
		t.Fatalf("fallback selection = %v, want %v", got, want)
	}
}

func TestGlobalInitPersistsSuccessfulPiPairsWhenHooksFail(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(t.TempDir(), "pi"))
	if err := config.Save(&config.Config{}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {
			Agents: []string{"claude"},
			Files:  []string{filepath.Join(config.GlobalAgentDir("claude"), "skills", "existing.md")},
		},
	}}); err != nil {
		t.Fatalf("save existing manifest: %v", err)
	}

	previousHooksInstaller := globalHooksInstaller
	globalHooksInstaller = func(string, internalagents.AgentType, *internalagents.AgentConfig) ([]string, error) {
		return nil, errors.New("simulated Pi extension write failure")
	}
	t.Cleanup(func() { globalHooksInstaller = previousHooksInstaller })

	previousAgents, previousGlobal, previousDryRun := initAgents, initGlobal, initDryRun
	t.Cleanup(func() {
		initAgents, initGlobal, initDryRun = previousAgents, previousGlobal, previousDryRun
	})
	initAgents, initGlobal, initDryRun = "pi", "skills,commands,hooks", false

	err := runGlobalInit(nil, nil)
	if err == nil {
		t.Fatal("runGlobalInit() succeeded despite Pi hook installation failure")
	}

	manifest, loadErr := LoadGlobalManifest()
	if loadErr != nil {
		t.Fatalf("load manifest: %v", loadErr)
	}
	for _, component := range []string{"skills", "commands"} {
		if !manifest.HasAgentComponent("pi", component) {
			t.Errorf("Pi %s success was not persisted: %+v", component, manifest.Components)
		}
	}
	if !manifest.HasAgentComponent("claude", "skills") {
		t.Errorf("existing Claude ownership was lost: %+v", manifest.Components["skills"])
	}
	if manifest.HasAgentComponent("pi", "hooks") {
		t.Errorf("failed Pi hooks were marked global: %+v", manifest.Components["hooks"])
	}

	loaded, loadErr := config.Load()
	if loadErr != nil {
		t.Fatalf("load config: %v", loadErr)
	}
	for _, component := range []string{"skills", "commands"} {
		mode, ok := loaded.GetAgentComponentOverride("pi", component)
		if !ok || mode != config.ComponentModeGlobal {
			t.Errorf("Pi %s mode = (%q, %t), want (global, true)", component, mode, ok)
		}
	}
	if _, ok := loaded.GetAgentComponentOverride("pi", "hooks"); ok {
		t.Error("failed Pi hooks received a global override")
	}
}

func TestProjectPlansAndInitUseAgentSpecificGlobalModes(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"pi": {
			Skills:   config.ComponentModeGlobal,
			Commands: config.ComponentModeGlobal,
			Hooks:    config.ComponentModeGlobal,
		},
	}}}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills":   {Agents: []string{"pi"}},
		"commands": {Agents: []string{"pi"}},
		"hooks":    {Agents: []string{"pi"}},
	}}); err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	projectDir := t.TempDir()
	piPlan, err := buildInstallationPlan(projectDir, internalagents.AgentPi, IntegrationHook)
	if err != nil {
		t.Fatalf("build Pi plan: %v", err)
	}
	if len(piPlan.skillFiles)+len(piPlan.commandFiles)+len(piPlan.hookFiles)+len(piPlan.pluginFiles) != 0 {
		t.Errorf("Pi local plan includes global resources: %+v", piPlan)
	}
	claudePlan, err := buildInstallationPlan(projectDir, internalagents.AgentClaude, IntegrationHook)
	if err != nil {
		t.Fatalf("build Claude plan: %v", err)
	}
	if len(claudePlan.skillFiles) == 0 || len(claudePlan.commandFiles) == 0 || len(claudePlan.hookFiles) == 0 {
		t.Errorf("Claude local plan omitted resources: %+v", claudePlan)
	}

	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationHook); err != nil {
		t.Fatalf("init Pi agent: %v", err)
	}
	piDir := filepath.Join(projectDir, internalagents.GetConfig(internalagents.AgentPi).RootDir)
	for _, path := range []string{
		filepath.Join(piDir, "skills", "thts-integrate", "SKILL.md"),
		filepath.Join(piDir, "prompts", "thts-handoff.md"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Pi project resource exists despite global ownership: %s", path)
		}
	}
}

func TestDisabledHooksMatchEmptyLocalPlan(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"claude": {Hooks: config.ComponentModeDisabled},
		"pi":     {Hooks: config.ComponentModeDisabled},
	}}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	projectDir := t.TempDir()
	for _, agentType := range []internalagents.AgentType{internalagents.AgentClaude, internalagents.AgentPi} {
		plan, err := buildInstallationPlan(projectDir, agentType, IntegrationHook)
		if err != nil {
			t.Fatalf("build %s plan: %v", agentType, err)
		}
		if len(plan.hookFiles) != 0 || len(plan.pluginFiles) != 0 || plan.settingsLocalFile != "" || plan.hooksSettingsModified {
			t.Errorf("%s disabled hook plan includes local integration: %+v", agentType, plan)
		}

		if err := initAgent(projectDir, agentType, IntegrationHook); err != nil {
			t.Fatalf("init %s agent: %v", agentType, err)
		}
		agentConfig := internalagents.GetConfig(agentType)
		agentDir := filepath.Join(projectDir, agentConfig.RootDir)
		for _, dir := range []string{agentConfig.HooksDir, agentConfig.PluginsDir} {
			if dir == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(agentDir, dir)); !os.IsNotExist(err) {
				t.Errorf("%s disabled hook mode created %s", agentType, dir)
			}
		}
		if _, err := os.Stat(filepath.Join(agentDir, "settings.local.json")); !os.IsNotExist(err) {
			t.Errorf("%s disabled hook mode created settings.local.json", agentType)
		}
		manifest, err := loadManifest(agentDir)
		if err != nil {
			t.Fatalf("load %s manifest: %v", agentType, err)
		}
		if manifest.Modifications.Hooks != nil {
			t.Errorf("%s disabled hook mode recorded hook settings: %+v", agentType, manifest.Modifications.Hooks)
		}
	}
}

func TestLocalContentPlanMatchesPluginHookMode(t *testing.T) {
	for _, tt := range []struct {
		name             string
		hooksMode        config.ComponentMode
		wantPlugin       bool
		wantInstructions bool
	}{
		{name: "local hooks install plugin", hooksMode: config.ComponentModeLocal, wantPlugin: true},
		{name: "global hooks use local instructions", hooksMode: config.ComponentModeGlobal, wantInstructions: true},
		{name: "disabled hooks use local instructions", hooksMode: config.ComponentModeDisabled, wantInstructions: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
				"opencode": {Hooks: tt.hooksMode},
			}}}); err != nil {
				t.Fatalf("save config: %v", err)
			}

			projectDir := t.TempDir()
			plan, err := buildInstallationPlan(projectDir, internalagents.AgentOpenCode, IntegrationAgentsContentLocal)
			if err != nil {
				t.Fatalf("build plan: %v", err)
			}
			if got := len(plan.pluginFiles) > 0; got != tt.wantPlugin {
				t.Errorf("planned plugin = %t, want %t: %+v", got, tt.wantPlugin, plan)
			}
			if got := plan.instructionsFile != ""; got != tt.wantInstructions {
				t.Errorf("planned instructions = %t, want %t: %+v", got, tt.wantInstructions, plan)
			}
			if got := len(plan.gitignorePatterns) > 0; got != tt.wantInstructions {
				t.Errorf("planned gitignore = %t, want %t: %+v", got, tt.wantInstructions, plan)
			}

			if err := initAgent(projectDir, internalagents.AgentOpenCode, IntegrationAgentsContentLocal); err != nil {
				t.Fatalf("init agent: %v", err)
			}
			agentDir := filepath.Join(projectDir, internalagents.GetConfig(internalagents.AgentOpenCode).RootDir)
			pluginPath := filepath.Join(agentDir, "plugins", "thts-integration.ts")
			if _, err := os.Stat(pluginPath); (err == nil) != tt.wantPlugin {
				t.Errorf("plugin exists = %t, want %t (%v)", err == nil, tt.wantPlugin, err)
			}
			instructionsPath := filepath.Join(agentDir, "AGENTS.local.md")
			if _, err := os.Stat(instructionsPath); (err == nil) != tt.wantInstructions {
				t.Errorf("local instructions exist = %t, want %t (%v)", err == nil, tt.wantInstructions, err)
			}
			gitignorePath := filepath.Join(projectDir, ".gitignore")
			gitignore, err := os.ReadFile(gitignorePath)
			if tt.wantInstructions {
				if err != nil || !strings.Contains(string(gitignore), filepath.Join(".opencode", "AGENTS.local.md")) {
					t.Errorf("local instruction gitignore missing: %v", err)
				}
			} else if err == nil && strings.Contains(string(gitignore), filepath.Join(".opencode", "AGENTS.local.md")) {
				t.Error("plugin plan unexpectedly wrote local instruction gitignore")
			}
		})
	}
}

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
