package agents

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	thtsfiles "github.com/scottames/thts"
	internalagents "github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
)

func setupOpenCodeTest(t *testing.T) {
	t.Helper()
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldAgents, oldDryRun, oldSettings, oldRefresh, oldForce := initAgents, initDryRun, initWithSettings, initRefresh, initForce
	initAgents, initDryRun, initWithSettings, initRefresh, initForce = "", false, false, false, false
	t.Cleanup(func() {
		initAgents, initDryRun, initWithSettings, initRefresh, initForce = oldAgents, oldDryRun, oldSettings, oldRefresh, oldForce
	})
}

func TestOpenCodeSettingsAreUserOwned(t *testing.T) {
	setupOpenCodeTest(t)
	initWithSettings = true
	for _, content := range []string{"", `{"model":"user/model"}`, "// user config\n{}"} {
		t.Run(content, func(t *testing.T) {
			project := t.TempDir()
			agentDir := filepath.Join(project, ".opencode")
			if err := os.MkdirAll(agentDir, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(agentDir, "opencode.json")
			if strings.HasPrefix(content, "//") {
				path += "c"
			}
			if content != "" {
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := initAgent(project, internalagents.AgentOpenCode, IntegrationHook); err != nil {
				t.Fatal(err)
			}
			manifest, err := loadManifest(agentDir)
			if err != nil || manifest.SettingsCreated || slices.Contains(manifest.Files, "opencode.json") {
				t.Fatalf("unexpected settings ownership: %+v, %v", manifest, err)
			}
			if err := refreshAgentSetup(project, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
				t.Fatal(err)
			}
			if err := Uninit(project, true, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if content == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("settings unexpectedly created: %v", err)
				}
			} else if err != nil || string(got) != content {
				t.Fatalf("user settings changed: %q, %v", got, err)
			}
		})
	}
}

func TestOpenCodeLegacySettingsMigration(t *testing.T) {
	for _, operation := range []string{"refresh", "reinit", "uninit"} {
		for _, customized := range []bool{false, true} {
			t.Run(operation+map[bool]string{false: "/exact", true: "/custom"}[customized], func(t *testing.T) {
				setupOpenCodeTest(t)
				project := t.TempDir()
				if err := initAgent(project, internalagents.AgentOpenCode, IntegrationHook); err != nil {
					t.Fatal(err)
				}
				agentDir := filepath.Join(project, ".opencode")
				manifest, err := loadManifest(agentDir)
				if err != nil {
					t.Fatal(err)
				}
				manifest.SettingsCreated = true
				manifest.Files = append(manifest.Files, "opencode.json")
				if err := writeManifest(agentDir, manifest); err != nil {
					t.Fatal(err)
				}
				content := legacyOpenCodeSettings
				if customized {
					content = strings.Replace(content, "claude-sonnet-4-20250514", "custom-model", 1)
				}
				path := filepath.Join(agentDir, "opencode.json")
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				switch operation {
				case "refresh":
					err = refreshAgentSetup(project, []internalagents.AgentType{internalagents.AgentOpenCode})
				case "reinit":
					err = initAgent(project, internalagents.AgentOpenCode, IntegrationHook)
				case "uninit":
					err = Uninit(project, true, []internalagents.AgentType{internalagents.AgentOpenCode})
				}
				if err != nil {
					t.Fatal(err)
				}
				if operation != "uninit" {
					updated, err := loadManifest(agentDir)
					if err != nil || updated.SettingsCreated || slices.Contains(updated.Files, "opencode.json") {
						t.Fatalf("settings ownership retained: %+v, %v", updated, err)
					}
					if err := Uninit(project, true, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
						t.Fatal(err)
					}
				}
				got, err := os.ReadFile(path)
				if customized {
					if err != nil || string(got) != content {
						t.Fatalf("custom settings changed: %q, %v", got, err)
					}
				} else if !os.IsNotExist(err) {
					t.Fatalf("legacy settings retained: %v", err)
				}
			})
		}
	}
}

func TestOpenCodeSettingsSymlinkAndCleanupFailure(t *testing.T) {
	setupOpenCodeTest(t)
	agentDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(target, []byte(legacyOpenCodeSettings), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(agentDir, "opencode.json")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{SettingsCreated: true, Files: []string{"opencode.json"}}
	if err := migrateOpenCodeSettings(agentDir, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("settings symlink removed: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	manifest.SettingsCreated = true
	manifest.Files = []string{"opencode.json"}
	if err := migrateOpenCodeSettings(agentDir, manifest); err == nil {
		t.Fatal("expected cleanup error for non-file")
	}
	if !manifest.SettingsCreated || !slices.Contains(manifest.Files, "opencode.json") {
		t.Fatal("failed settings cleanup lost ownership")
	}
}

func TestOpenCodePluginTransitions(t *testing.T) {
	for _, previous := range []IntegrationLevel{IntegrationHook, IntegrationAgentsContentLocal} {
		for _, target := range []IntegrationLevel{IntegrationHook, IntegrationAgentsContentLocal, IntegrationAgentsContent, IntegrationOnDemand} {
			for _, mode := range []config.ComponentMode{config.ComponentModeLocal, config.ComponentModeGlobal, config.ComponentModeDisabled} {
				t.Run(string(previous)+"/"+string(target)+"/"+string(mode), func(t *testing.T) {
					setupOpenCodeTest(t)
					project := t.TempDir()
					if err := initAgent(project, internalagents.AgentOpenCode, previous); err != nil {
						t.Fatal(err)
					}
					if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
						"opencode": {Hooks: mode},
					}}}); err != nil {
						t.Fatal(err)
					}
					if err := initAgent(project, internalagents.AgentOpenCode, target); err != nil {
						t.Fatal(err)
					}
					if err := refreshAgentSetup(project, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
						t.Fatal(err)
					}
					agentDir := filepath.Join(project, ".opencode")
					_, err := os.Stat(filepath.Join(agentDir, "plugins", "thts-integration.ts"))
					want := mode == config.ComponentModeLocal && (target == IntegrationHook || target == IntegrationAgentsContentLocal)
					if (err == nil) != want {
						t.Fatalf("plugin exists = %t, want %t (%v)", err == nil, want, err)
					}
					if _, err := os.Stat(filepath.Join(agentDir, "AGENTS.local.md")); !os.IsNotExist(err) {
						t.Fatalf("unsupported local fallback exists: %v", err)
					}
				})
			}
		}
	}
}

func TestOpenCodeRefreshRemovesOwnedPluginOnly(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(map[bool]string{false: "unowned", true: "owned"}[owned], func(t *testing.T) {
			setupOpenCodeTest(t)
			project := t.TempDir()
			if err := initAgent(project, internalagents.AgentOpenCode, IntegrationAgentsContentLocal); err != nil {
				t.Fatal(err)
			}
			agentDir := filepath.Join(project, ".opencode")
			manifest, err := loadManifest(agentDir)
			if err != nil {
				t.Fatal(err)
			}
			if !owned {
				manifest.Files = removeStringValue(manifest.Files, "plugins/thts-integration.ts")
				if err := writeManifest(agentDir, manifest); err != nil {
					t.Fatal(err)
				}
			}
			if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
				"opencode": {Hooks: config.ComponentModeGlobal},
			}}}); err != nil {
				t.Fatal(err)
			}
			if err := refreshAgentSetup(project, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
				t.Fatal(err)
			}
			_, err = os.Stat(filepath.Join(agentDir, "plugins", "thts-integration.ts"))
			if (err == nil) == owned {
				t.Fatalf("plugin removal did not honor ownership: %v", err)
			}
		})
	}
}

func TestRefreshCLISelectionAndDryRun(t *testing.T) {
	setupOpenCodeTest(t)
	project := t.TempDir()
	t.Chdir(project)
	for _, agent := range []internalagents.AgentType{internalagents.AgentOpenCode, internalagents.AgentPi} {
		if err := initAgent(project, agent, IntegrationHook); err != nil {
			t.Fatal(err)
		}
	}
	plugin := filepath.Join(project, ".opencode", "plugins", "thts-integration.ts")
	pi := filepath.Join(project, ".pi", "extensions", "thts-integration.ts")
	for _, path := range []string{plugin, pi} {
		if err := os.WriteFile(path, []byte("stale"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	initAgents, initRefresh, initDryRun = "opencode", true, true
	if err := runAgentsInit(InitCmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{plugin, pi} {
		if got, err := os.ReadFile(path); err != nil || string(got) != "stale" {
			t.Fatalf("dry-run wrote %s: %q, %v", path, got, err)
		}
	}
	initDryRun = false
	if err := runAgentsInit(InitCmd, nil); err != nil {
		t.Fatal(err)
	}
	want, err := thtsfiles.OpenCodePlugins.ReadFile("embedded/plugins/opencode/thts-integration.ts")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(plugin); err != nil || string(got) != string(want) {
		t.Fatalf("plugin not refreshed: %v", err)
	}
	if got, err := os.ReadFile(pi); err != nil || string(got) != "stale" {
		t.Fatalf("refresh touched unselected Pi: %q, %v", got, err)
	}
}

func TestOpenCodeRemovalRetainsFailedCleanup(t *testing.T) {
	setupOpenCodeTest(t)
	project := t.TempDir()
	if err := initAgent(project, internalagents.AgentOpenCode, IntegrationHook); err != nil {
		t.Fatal(err)
	}
	agentDir := filepath.Join(project, ".opencode")
	plugin := filepath.Join(agentDir, "plugins", "thts-integration.ts")
	if err := os.Remove(plugin); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(plugin, 0755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(plugin, "blocker")
	if err := os.WriteFile(blocker, []byte("preserve"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Uninit(project, true, []internalagents.AgentType{internalagents.AgentOpenCode}); err == nil {
		t.Fatal("expected failed cleanup")
	}
	manifest, err := loadManifest(agentDir)
	if err != nil || !slices.Equal(manifest.Files, []string{"plugins/thts-integration.ts"}) {
		t.Fatalf("failed cleanup lost ownership or retained removed paths: %+v, %v", manifest, err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	if err := Uninit(project, true, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCodeRefreshMigratesLegacyConfigInstructions(t *testing.T) {
	setupOpenCodeTest(t)
	project := t.TempDir()
	agentDir := filepath.Join(project, ".opencode")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(project, "opencode.json")
	if err := os.WriteFile(settings, []byte(`{"model":"user/model","instructions":[".opencode/thts-instructions.md","keep.md"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{
		Agent: "opencode", IntegrationLevel: IntegrationAgentsContent,
		Modifications: ManifestModifications{InstructionsMD: &InstructionsMDModification{
			Path: settings, Action: "modified", IntegrationType: "config", ConfigKey: "instructions",
		}},
	}
	if err := writeManifest(agentDir, manifest); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := refreshAgentSetup(project, []internalagents.AgentType{internalagents.AgentOpenCode}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(settings)
	if err != nil || strings.Contains(string(got), "thts-instructions.md") || !strings.Contains(string(got), "keep.md") || !strings.Contains(string(got), "user/model") {
		t.Fatalf("config migration changed user values or retained obsolete entry: %s, %v", got, err)
	}
	policy, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil || strings.Count(string(policy), ThtsMarkerStart) != 1 {
		t.Fatalf("shared policy not migrated: %s, %v", policy, err)
	}
	if err := initAgent(project, internalagents.AgentOpenCode, IntegrationOnDemand); err != nil {
		t.Fatal(err)
	}
	policy, err = os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err == nil && strings.Contains(string(policy), ThtsMarkerStart) {
		t.Fatal("on-demand transition retained shared policy")
	}
}
