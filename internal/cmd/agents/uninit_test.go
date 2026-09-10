package agents

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	internalagents "github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
)

func TestGlobalUninitRemovesOnlySelectedAgentOwnership(t *testing.T) {
	stateDir := t.TempDir()
	piDir := filepath.Join(t.TempDir(), "pi")
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("PI_CODING_AGENT_DIR", piDir)

	piPath := filepath.Join(piDir, "skills", "thts-integrate", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(piPath), 0755); err != nil {
		t.Fatalf("create Pi skills directory: %v", err)
	}
	if err := os.WriteFile(piPath, []byte("Pi skill"), 0644); err != nil {
		t.Fatalf("write Pi skill: %v", err)
	}
	claudePath := filepath.Join(config.GlobalAgentDir("claude"), "skills", "thts-existing.md")
	manifest := &GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {Agents: []string{"claude", "pi"}, Files: []string{claudePath, piPath}},
	}}
	if err := SaveGlobalManifest(manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"claude": {Skills: config.ComponentModeGlobal},
		"pi":     {Skills: config.ComponentModeGlobal},
	}}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	previousAgents, previousForce, previousDryRun, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousAgents, previousForce, previousDryRun, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "pi", true, false, true
	if err := runGlobalUninit(nil, nil); err != nil {
		t.Fatalf("runGlobalUninit() error: %v", err)
	}

	if _, err := os.Stat(piPath); !os.IsNotExist(err) {
		t.Fatalf("Pi global path was not removed: %v", err)
	}
	updated, err := LoadGlobalManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if !updated.HasAgentComponent("claude", "skills") || updated.HasAgentComponent("pi", "skills") {
		t.Fatalf("manifest ownership = %+v, want Claude only", updated.Components["skills"])
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if mode, _ := loaded.GetAgentComponentOverride("pi", "skills"); mode != config.ComponentModeLocal {
		t.Errorf("Pi skills mode = %q, want local", mode)
	}
	if mode, _ := loaded.GetAgentComponentOverride("claude", "skills"); mode != config.ComponentModeGlobal {
		t.Errorf("Claude skills mode = %q, want global", mode)
	}
}

func TestGlobalUninitResetsLegacySharedModeWhenLastOwnerRemoved(t *testing.T) {
	stateDir := t.TempDir()
	piDir := filepath.Join(t.TempDir(), "pi")
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("PI_CODING_AGENT_DIR", piDir)

	piPath := filepath.Join(piDir, "skills", "thts-integrate", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(piPath), 0755); err != nil {
		t.Fatalf("create Pi skills directory: %v", err)
	}
	if err := os.WriteFile(piPath, []byte("Pi skill"), 0644); err != nil {
		t.Fatalf("write Pi skill: %v", err)
	}
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {Agents: []string{"pi"}, Files: []string{piPath}},
	}}); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{Skills: config.ComponentModeGlobal}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	previousAgents, previousForce, previousDryRun, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousAgents, previousForce, previousDryRun, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "pi", true, false, true
	if err := runGlobalUninit(nil, nil); err != nil {
		t.Fatalf("runGlobalUninit() error: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if mode := loaded.GetAgentComponentMode("skills"); mode != config.ComponentModeLocal {
		t.Errorf("legacy shared skills mode = %q, want local", mode)
	}
}

func TestGlobalUninitPreservesOwnershipWhenSelectedPathCannotBeRemoved(t *testing.T) {
	stateDir := t.TempDir()
	piDir := filepath.Join(t.TempDir(), "pi")
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("PI_CODING_AGENT_DIR", piDir)

	failedPath := filepath.Join(piDir, "skills", "failed")
	if err := os.MkdirAll(failedPath, 0755); err != nil {
		t.Fatalf("create failed removal directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(failedPath, "child"), []byte("keep"), 0644); err != nil {
		t.Fatalf("create failed removal child: %v", err)
	}
	succeededPath := filepath.Join(piDir, "skills", "succeeded")
	if err := os.WriteFile(succeededPath, []byte("remove"), 0644); err != nil {
		t.Fatalf("create successful removal file: %v", err)
	}
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {Agents: []string{"pi"}, Files: []string{failedPath, succeededPath}},
	}}); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"pi": {Skills: config.ComponentModeGlobal},
	}}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	previousAgents, previousForce, previousDryRun, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousAgents, previousForce, previousDryRun, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "pi", true, false, true
	if err := runGlobalUninit(nil, nil); err == nil {
		t.Fatal("runGlobalUninit() succeeded despite failed Pi path removal")
	}

	manifest, err := LoadGlobalManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if !manifest.HasAgentComponent("pi", "skills") || len(manifest.Components["skills"].Files) != 1 || manifest.Components["skills"].Files[0] != failedPath {
		t.Fatalf("failed Pi ownership was removed: %+v", manifest.Components["skills"])
	}
	if _, err := os.Stat(succeededPath); !os.IsNotExist(err) {
		t.Fatalf("successful Pi path was not removed: %v", err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if mode, _ := loaded.GetAgentComponentOverride("pi", "skills"); mode != config.ComponentModeGlobal {
		t.Errorf("Pi skills mode = %q, want global after failed cleanup", mode)
	}
}

func TestGlobalUninitStopsBeforeConfigWhenManifestSaveFails(t *testing.T) {
	stateDir := t.TempDir()
	piDir := filepath.Join(t.TempDir(), "pi")
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("PI_CODING_AGENT_DIR", piDir)

	piPath := filepath.Join(piDir, "skills", "thts-integrate", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(piPath), 0755); err != nil {
		t.Fatalf("create Pi skills directory: %v", err)
	}
	if err := os.WriteFile(piPath, []byte("Pi skill"), 0644); err != nil {
		t.Fatalf("write Pi skill: %v", err)
	}
	claudePath := filepath.Join(config.GlobalAgentDir("claude"), "skills", "thts-integrate.md")
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {Agents: []string{"claude", "pi"}, Files: []string{claudePath, piPath}},
	}}); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"claude": {Skills: config.ComponentModeGlobal},
		"pi":     {Skills: config.ComponentModeGlobal},
	}}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	previousSaveManifest := saveGlobalManifestForUninit
	saveGlobalManifestForUninit = func(*GlobalManifest) error { return errors.New("manifest save failed") }
	t.Cleanup(func() { saveGlobalManifestForUninit = previousSaveManifest })
	previousAgents, previousForce, previousDryRun, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousAgents, previousForce, previousDryRun, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "pi", true, false, true

	if err := runGlobalUninit(nil, nil); err == nil {
		t.Fatal("runGlobalUninit() succeeded despite manifest persistence failure")
	}
	manifest, err := LoadGlobalManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if !manifest.HasAgentComponent("pi", "skills") {
		t.Fatalf("manifest advanced despite failed save: %+v", manifest.Components["skills"])
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if mode, _ := loaded.GetAgentComponentOverride("pi", "skills"); mode != config.ComponentModeGlobal {
		t.Errorf("Pi skills mode = %q, want global when manifest save fails", mode)
	}
}

func TestPiUninitPreservesUnmanagedProjectResources(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	projectDir := t.TempDir()
	piDir := filepath.Join(projectDir, internalagents.GetConfig(internalagents.AgentPi).RootDir)
	userExtension := filepath.Join(piDir, "extensions", "user-extension.ts")

	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationHook); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	if err := os.WriteFile(userExtension, []byte("user extension"), 0644); err != nil {
		t.Fatalf("write user extension: %v", err)
	}
	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentPi}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(piDir, "extensions", "thts-integration.ts")); !os.IsNotExist(err) {
		t.Fatalf("managed Pi extension after uninit = %v, want removed", err)
	}
	if _, err := os.Stat(userExtension); err != nil {
		t.Fatalf("unmanaged Pi extension after uninit: %v", err)
	}
}

func TestBatchUninitDoesNotTransferSharedMarkerToRemovedAgent(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	projectDir := t.TempDir()

	if err := initAgent(projectDir, internalagents.AgentCodex, IntegrationAgentsContent); err != nil {
		t.Fatalf("initialize Codex: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationAgentsContent); err != nil {
		t.Fatalf("initialize Pi: %v", err)
	}

	previousAgents, previousForce, previousDryRun, previousAll, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal = previousAgents, previousForce, previousDryRun, previousAll, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal = "codex,pi", true, false, false, false
	t.Chdir(projectDir)

	if err := runAgentsUninit(nil, nil); err != nil {
		t.Fatalf("runAgentsUninit() error: %v", err)
	}
	for _, path := range []string{
		filepath.Join(projectDir, "AGENTS.md"),
		filepath.Join(projectDir, ".codex", ManifestFile),
		filepath.Join(projectDir, ".pi", ManifestFile),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("path after batch uninit %s = %v, want absent", path, err)
		}
	}
}

func TestBatchUninitContinuesAfterManifestAnalysisError(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	projectDir := t.TempDir()
	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationOnDemand); err != nil {
		t.Fatalf("initialize Pi: %v", err)
	}
	droidDir := filepath.Join(projectDir, ".factory")
	if err := os.MkdirAll(droidDir, 0755); err != nil {
		t.Fatalf("create Droid directory: %v", err)
	}
	const malformed = "{not-json\n"
	if err := os.WriteFile(filepath.Join(droidDir, ManifestFile), []byte(malformed), 0644); err != nil {
		t.Fatalf("write malformed Droid manifest: %v", err)
	}

	previousAgents, previousForce, previousDryRun, previousAll, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal = previousAgents, previousForce, previousDryRun, previousAll, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal = "pi,droid", true, false, false, false
	t.Chdir(projectDir)

	err := runAgentsUninit(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "analyze droid") || !strings.Contains(err.Error(), "invalid manifest") {
		t.Fatalf("runAgentsUninit() error = %v, want Droid analysis error", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".pi", ManifestFile)); !os.IsNotExist(err) {
		t.Fatalf("Pi manifest after mixed batch uninit = %v, want absent", err)
	}
	data, readErr := os.ReadFile(filepath.Join(droidDir, ManifestFile))
	if readErr != nil || string(data) != malformed {
		t.Fatalf("Droid manifest after mixed batch uninit = %q, %v", data, readErr)
	}
}

func TestGeminiHookUninitRemovesCreatedSettings(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	projectDir := t.TempDir()
	if err := initAgent(projectDir, internalagents.AgentGemini, IntegrationHook); err != nil {
		t.Fatalf("initialize Gemini: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentGemini, IntegrationHook); err != nil {
		t.Fatalf("reinitialize Gemini: %v", err)
	}
	settingsPath := filepath.Join(projectDir, ".gemini", "settings.local.json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("Gemini settings after init: %v", err)
	}

	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentGemini}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("Gemini settings after uninit = %v, want absent", err)
	}
	if data, err := os.ReadFile(filepath.Join(projectDir, ".gitignore")); err == nil && strings.Contains(string(data), ".gemini/settings.local.json") {
		t.Fatalf("Gemini settings ignore remained after reinit and uninit: %s", data)
	}
}

func TestGeminiHookUninitRestoresEnablementWithoutHooksObject(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	projectDir := t.TempDir()
	geminiDir := filepath.Join(projectDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("create Gemini directory: %v", err)
	}
	settingsPath := filepath.Join(geminiDir, "settings.local.json")
	if err := os.WriteFile(settingsPath, []byte("{\"theme\":\"dark\",\"tools\":{\"enableHooks\":false}}\n"), 0644); err != nil {
		t.Fatalf("write preexisting settings: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentGemini, IntegrationHook); err != nil {
		t.Fatalf("initialize Gemini: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\"theme\":\"dark\",\"tools\":{\"enableHooks\":true}}\n"), 0644); err != nil {
		t.Fatalf("remove hooks object: %v", err)
	}
	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentGemini}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("preexisting settings after uninit: %v", err)
	}
	const expected = "{\n  \"theme\": \"dark\",\n  \"tools\": {\n    \"enableHooks\": false\n  }\n}\n"
	if string(data) != expected {
		t.Fatalf("preexisting settings after uninit = %s, want %s", data, expected)
	}
}

func TestGeminiHookUninitRestoresPreexistingEnablement(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	projectDir := t.TempDir()
	geminiDir := filepath.Join(projectDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("create Gemini directory: %v", err)
	}
	settingsPath := filepath.Join(geminiDir, "settings.local.json")
	const original = "{\"hooks\":{\"enabled\":false},\"theme\":\"dark\",\"tools\":{\"enableHooks\":false}}\n"
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("write preexisting settings: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentGemini, IntegrationHook); err != nil {
		t.Fatalf("initialize Gemini: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentGemini, IntegrationHook); err != nil {
		t.Fatalf("reinitialize Gemini: %v", err)
	}
	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentGemini}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("preexisting settings after uninit: %v", err)
	}
	const expected = "{\n  \"hooks\": {\n    \"enabled\": false\n  },\n  \"theme\": \"dark\",\n  \"tools\": {\n    \"enableHooks\": false\n  }\n}\n"
	if string(data) != expected {
		t.Fatalf("preexisting settings after uninit = %s, want %s", data, expected)
	}
}

func TestGlobalGeminiHookUninitPreservesPreexistingSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("create Gemini directory: %v", err)
	}
	settingsPath := filepath.Join(geminiDir, "settings.json")
	const original = "{\"hooks\":{\"enabled\":false},\"theme\":\"dark\",\"tools\":{\"enableHooks\":false}}\n"
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("write preexisting settings: %v", err)
	}
	manifest := NewGlobalManifest()
	if _, err := installGlobalComponent("hooks", []internalagents.AgentType{internalagents.AgentGemini}, manifest); err != nil {
		t.Fatalf("installGlobalComponent() error: %v", err)
	}
	if !slices.Contains(manifest.Components["hooks"].PreexistingFiles, settingsPath) {
		t.Fatalf("preexisting settings not recorded: %+v", manifest.Components["hooks"])
	}
	if err := SaveGlobalManifest(manifest); err != nil {
		t.Fatalf("SaveGlobalManifest() error: %v", err)
	}

	previousAgents, previousForce, previousDryRun, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousAgents, previousForce, previousDryRun, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "gemini", true, false, true
	if err := runGlobalUninit(nil, nil); err != nil {
		t.Fatalf("runGlobalUninit() error: %v", err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("preexisting settings after uninit: %v", err)
	}
	const expected = "{\n  \"hooks\": {\n    \"enabled\": false\n  },\n  \"theme\": \"dark\",\n  \"tools\": {\n    \"enableHooks\": false\n  }\n}\n"
	if string(data) != expected {
		t.Fatalf("preexisting settings after uninit = %s, want %s", data, expected)
	}
}

func TestGlobalGeminiHookUninitPreservesLegacyTrackedSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0755); err != nil {
		t.Fatalf("create Gemini directory: %v", err)
	}
	settingsPath := filepath.Join(geminiDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{\"hooks\":{\"enabled\":true},\"theme\":\"dark\",\"tools\":{\"enableHooks\":true}}\n"), 0644); err != nil {
		t.Fatalf("write legacy settings: %v", err)
	}
	manifest := NewGlobalManifest()
	manifest.Components["hooks"] = &GlobalComponentInfo{
		Agents: []string{"gemini"},
		Files:  []string{settingsPath},
	}
	if err := SaveGlobalManifest(manifest); err != nil {
		t.Fatalf("SaveGlobalManifest() error: %v", err)
	}

	previousAgents, previousForce, previousDryRun, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousAgents, previousForce, previousDryRun, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "gemini", true, false, true
	if err := runGlobalUninit(nil, nil); err != nil {
		t.Fatalf("runGlobalUninit() error: %v", err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("legacy settings after uninit: %v", err)
	}
	if !strings.Contains(string(data), `"theme": "dark"`) {
		t.Fatalf("legacy settings after uninit = %s", data)
	}
}

func TestIsPathSafeForRemoval(t *testing.T) {
	baseDir := "/home/user/project/.claude"

	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected bool
	}{
		// Invalid paths that should return false
		{
			name:     "empty string",
			path:     "",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "current dir",
			path:     ".",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "parent escape",
			path:     "..",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "parent escape with path",
			path:     "../secrets",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "absolute path unix",
			path:     "/etc/passwd",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "root path",
			path:     "/",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "hidden parent traversal",
			path:     "foo/../../etc/passwd",
			baseDir:  baseDir,
			expected: false,
		},

		// Valid paths that should return true
		{
			name:     "simple file",
			path:     "AGENTS.md",
			baseDir:  baseDir,
			expected: true,
		},
		{
			name:     "nested file",
			path:     "skills/foo.md",
			baseDir:  baseDir,
			expected: true,
		},
		{
			name:     "deeply nested file",
			path:     "commands/thts-handoff.md",
			baseDir:  baseDir,
			expected: true,
		},
		{
			name:     "file with dots in name",
			path:     "settings.local.json",
			baseDir:  baseDir,
			expected: true,
		},
		{
			name:     "directory in path",
			path:     "skills/thts-integrate/SKILL.md",
			baseDir:  baseDir,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathSafeForRemoval(tt.path, tt.baseDir)
			if result != tt.expected {
				t.Errorf("isPathSafeForRemoval(%q, %q) = %v, want %v",
					tt.path, tt.baseDir, result, tt.expected)
			}
		})
	}
}

func TestIsPathSafeForRemoval_EdgeCases(t *testing.T) {
	t.Run("path that becomes base dir after join", func(t *testing.T) {
		// filepath.Join("/base", "") returns "/base"
		// This should be caught and return false
		result := isPathSafeForRemoval("", "/home/user/.claude")
		if result {
			t.Error("expected false for empty path that resolves to base dir")
		}
	})

	t.Run("path with embedded null traversal", func(t *testing.T) {
		// Paths containing ".." somewhere in the middle
		result := isPathSafeForRemoval("foo/../../../etc/passwd", "/home/user/.claude")
		if result {
			t.Error("expected false for path with embedded traversal escaping base")
		}
	})

	t.Run("sibling traversal stays within base", func(t *testing.T) {
		// foo/../bar stays within base dir
		result := isPathSafeForRemoval("foo/../bar", "/home/user/.claude")
		if !result {
			t.Error("expected true for sibling traversal that stays within base")
		}
	})
}
