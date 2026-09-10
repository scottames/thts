package agents

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	internalagents "github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
)

func setupDroidTest(t *testing.T) {
	t.Helper()
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

func TestDroidInitRejectsUnownedResourceCollision(t *testing.T) {
	setupDroidTest(t)
	previousForce := initForce
	t.Cleanup(func() { initForce = previousForce })
	initForce = false

	projectDir := t.TempDir()
	path := filepath.Join(projectDir, ".factory", "droids", "thoughts-locator.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create user droid directory: %v", err)
	}
	const userContent = "user-owned droid\n"
	if err := os.WriteFile(path, []byte(userContent), 0644); err != nil {
		t.Fatalf("write user droid: %v", err)
	}

	err := initAgent(projectDir, internalagents.AgentDroid, IntegrationOnDemand)
	if err == nil || !strings.Contains(err.Error(), "not owned by thts") {
		t.Fatalf("initAgent() error = %v, want unowned-resource error", err)
	}
	if got, readErr := os.ReadFile(path); readErr != nil || string(got) != userContent {
		t.Fatalf("user droid after rejected init = %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(projectDir, ".factory", ManifestFile)); !os.IsNotExist(statErr) {
		t.Fatalf("manifest after rejected init = %v, want absent", statErr)
	}
}

func TestDroidInitCLIPropagatesResourceCollision(t *testing.T) {
	setupDroidTest(t)
	previousAgents, previousForce, previousInteractive, previousDryRun := initAgents, initForce, initInteractive, initDryRun
	t.Cleanup(func() {
		initAgents, initForce, initInteractive, initDryRun = previousAgents, previousForce, previousInteractive, previousDryRun
	})
	initAgents, initForce, initInteractive, initDryRun = "droid", false, false, false

	projectDir := t.TempDir()
	t.Chdir(projectDir)
	path := filepath.Join(projectDir, ".factory", "commands", "thts-resume.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create commands directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("user command\n"), 0644); err != nil {
		t.Fatalf("write user command: %v", err)
	}

	if err := runAgentsInit(nil, nil); err == nil || !strings.Contains(err.Error(), "not owned by thts") {
		t.Fatalf("runAgentsInit() error = %v, want unowned-resource error", err)
	}
}

func TestDroidManifestlessUninitPreservesFactoryResources(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	droidPath := filepath.Join(projectDir, ".factory", "droids", "thoughts-locator.md")
	if err := os.MkdirAll(filepath.Dir(droidPath), 0755); err != nil {
		t.Fatalf("create Factory directory: %v", err)
	}
	const droidContent = "user-owned droid\n"
	if err := os.WriteFile(droidPath, []byte(droidContent), 0644); err != nil {
		t.Fatalf("write Factory droid: %v", err)
	}
	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	const agentsContent = "# Instructions\n\n<!-- thts-start -->\nshared owner\n<!-- thts-end -->\n"
	if err := os.WriteFile(agentsPath, []byte(agentsContent), 0644); err != nil {
		t.Fatalf("write shared instructions: %v", err)
	}

	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	if got, err := os.ReadFile(droidPath); err != nil || string(got) != droidContent {
		t.Fatalf("manifestless droid after uninit = %q, %v", got, err)
	}
	if got, err := os.ReadFile(agentsPath); err != nil || string(got) != agentsContent {
		t.Fatalf("shared instructions after Droid uninit = %q, %v", got, err)
	}
}

func TestDroidWithSettingsSkipsManagedSettings(t *testing.T) {
	setupDroidTest(t)
	previous := initWithSettings
	t.Cleanup(func() { initWithSettings = previous })
	initWithSettings = true

	projectDir := t.TempDir()
	plan, err := buildInstallationPlan(projectDir, internalagents.AgentDroid, IntegrationHook)
	if err != nil {
		t.Fatalf("buildInstallationPlan() error: %v", err)
	}
	if plan.settingsFile != "" || !strings.Contains(plan.settingsSkipNotice, "Droid CLI has no thts-managed settings") {
		t.Fatalf("Droid settings plan = file %q notice %q", plan.settingsFile, plan.settingsSkipNotice)
	}
	if plan.settingsLocalFile != "hooks.json" {
		t.Errorf("Droid hook config plan = %q, want hooks.json", plan.settingsLocalFile)
	}

	droidDir := filepath.Join(projectDir, ".factory")
	if err := os.MkdirAll(droidDir, 0755); err != nil {
		t.Fatalf("create Droid directory: %v", err)
	}
	const userSettings = "{\"theme\":\"dark\"}\n"
	settingsPath := filepath.Join(droidDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(userSettings), 0644); err != nil {
		t.Fatalf("write user settings: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationHook); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	got, err := os.ReadFile(settingsPath)
	if err != nil || string(got) != userSettings {
		t.Fatalf("Droid settings = %q, %v; want preserved", got, err)
	}
	manifest, err := loadManifest(droidDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.SettingsCreated || slices.Contains(manifest.Files, "settings.json") || slices.Contains(manifest.Files, "hooks.json") {
		t.Errorf("Droid manifest owns user configuration: %+v", manifest)
	}
}

func TestDroidHookLifecyclePreservesFactoryConfiguration(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	droidDir := filepath.Join(projectDir, ".factory")
	if err := os.MkdirAll(filepath.Join(droidDir, "droids"), 0755); err != nil {
		t.Fatalf("create Droid directory: %v", err)
	}
	userDroid := filepath.Join(droidDir, "droids", "user-droid.md")
	if err := os.WriteFile(userDroid, []byte("user droid"), 0644); err != nil {
		t.Fatalf("write user droid: %v", err)
	}
	hooksPath := filepath.Join(droidDir, "hooks.json")
	writeDroidHooks(t, hooksPath, map[string]any{
		"SessionStart": []any{droidHookEntry("./user-session-hook.sh")},
		"CustomEvent":  []any{droidHookEntry("./custom-event.sh")},
		"customKey":    map[string]any{"preserve": true},
	})

	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationHook); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	assertDroidProjectLayout(t, droidDir)
	assertDroidHookState(t, hooksPath, true, false)

	manifest, err := loadManifest(droidDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.Modifications.Hooks == nil || manifest.Modifications.Hooks.SettingsFile != "hooks.json" {
		t.Fatalf("Droid hook modification = %+v, want hooks.json", manifest.Modifications.Hooks)
	}
	if slices.Contains(manifest.Files, "hooks.json") {
		t.Fatal("Droid hooks.json must not be tracked as a wholly owned file")
	}

	sessionScript := filepath.Join(droidDir, "hooks", "thts-session-start.sh")
	if err := os.WriteFile(sessionScript, []byte("stale"), 0644); err != nil {
		t.Fatalf("write stale hook: %v", err)
	}
	document := readDroidHooks(t, hooksPath)
	delete(document, "UserPromptSubmit")
	writeDroidHooks(t, hooksPath, document)
	if err := refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("refreshAgentSetup() error: %v", err)
	}
	script, err := os.ReadFile(sessionScript)
	if err != nil || string(script) == "stale" {
		t.Fatalf("refreshed hook = %q, %v", script, err)
	}
	if info, err := os.Stat(sessionScript); err != nil || info.Mode()&0111 == 0 {
		t.Fatalf("session hook executable mode = %v, %v", info, err)
	}
	assertDroidHookState(t, hooksPath, true, false)

	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	if _, err := os.Stat(sessionScript); !os.IsNotExist(err) {
		t.Fatalf("managed hook after uninit = %v, want absent", err)
	}
	if _, err := os.Stat(userDroid); err != nil {
		t.Fatalf("user droid after uninit: %v", err)
	}
	assertDroidHookState(t, hooksPath, false, false)
}

func TestDroidPreexistingEmptyProjectHookConfigIsPreserved(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	droidDir := filepath.Join(projectDir, ".factory")
	if err := os.MkdirAll(droidDir, 0755); err != nil {
		t.Fatalf("create Droid directory: %v", err)
	}
	hooksPath := filepath.Join(droidDir, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte("{}\n"), 0644); err != nil {
		t.Fatalf("write empty hooks config: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationHook); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	if document := readDroidHooks(t, hooksPath); len(document) != 0 {
		t.Fatalf("preserved project hooks config = %+v, want empty", document)
	}
}

func TestDroidLocalHookCleanupFailureRetainsOwnership(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationHook); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	droidDir := filepath.Join(projectDir, ".factory")
	hooksPath := filepath.Join(droidDir, "hooks.json")
	const malformed = "{not-json\n"
	if err := os.WriteFile(hooksPath, []byte(malformed), 0644); err != nil {
		t.Fatalf("write malformed hooks: %v", err)
	}

	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err == nil {
		t.Fatal("Uninit() succeeded with malformed Droid hooks.json")
	}
	for _, path := range []string{
		filepath.Join(droidDir, ManifestFile),
		filepath.Join(droidDir, "hooks", "thts-session-start.sh"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("managed resource %s after failed uninit: %v", path, err)
		}
	}
	if got, err := os.ReadFile(hooksPath); err != nil || string(got) != malformed {
		t.Fatalf("malformed hooks after failed uninit = %q, %v", got, err)
	}
}

func TestDroidFileCleanupFailureRetainsManifest(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationOnDemand); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	droidDir := filepath.Join(projectDir, ".factory")
	managedPath := filepath.Join(droidDir, "commands", "thts-resume.md")
	if err := os.Remove(managedPath); err != nil {
		t.Fatalf("remove managed command: %v", err)
	}
	if err := os.Mkdir(managedPath, 0755); err != nil {
		t.Fatalf("replace managed command with directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(managedPath, "blocker"), []byte("block removal\n"), 0644); err != nil {
		t.Fatalf("write removal blocker: %v", err)
	}

	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err == nil {
		t.Fatal("Uninit() succeeded despite managed-file removal failure")
	}
	manifest, err := loadManifest(droidDir)
	if err != nil {
		t.Fatalf("load retained manifest: %v", err)
	}
	if !slices.Contains(manifest.Files, filepath.Join("commands", "thts-resume.md")) {
		t.Fatalf("retained manifest lost failed path: %+v", manifest.Files)
	}
	if slices.Contains(manifest.Files, filepath.Join("commands", "thts-handoff.md")) {
		t.Fatalf("retained manifest still owns successfully removed path: %+v", manifest.Files)
	}
}

func TestDroidRefreshRejectsUnownedResourceCollision(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationOnDemand); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	droidDir := filepath.Join(projectDir, ".factory")
	relativePath := filepath.Join("droids", "thoughts-analyzer.md")
	path := filepath.Join(droidDir, relativePath)
	manifest, err := loadManifest(droidDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	manifest.Files = slices.DeleteFunc(manifest.Files, func(value string) bool { return value == relativePath })
	if err := writeManifest(droidDir, manifest); err != nil {
		t.Fatalf("write modified manifest: %v", err)
	}
	const userContent = "replacement user droid\n"
	if err := os.WriteFile(path, []byte(userContent), 0644); err != nil {
		t.Fatalf("write user collision: %v", err)
	}

	if err := refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentDroid}); err == nil || !strings.Contains(err.Error(), "not owned by thts") {
		t.Fatalf("refreshAgentSetup() error = %v, want unowned-resource error", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != userContent {
		t.Fatalf("user collision after refresh = %q, %v", got, err)
	}
}

func TestDroidRefreshValidatesHooksBeforeCreatingResources(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationHook); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	droidDir := filepath.Join(projectDir, ".factory")
	relativePath := filepath.Join("commands", "thts-resume.md")
	path := filepath.Join(droidDir, relativePath)
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove managed command: %v", err)
	}
	manifest, err := loadManifest(droidDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	manifest.Files = slices.DeleteFunc(manifest.Files, func(value string) bool { return value == relativePath })
	if err := writeManifest(droidDir, manifest); err != nil {
		t.Fatalf("write modified manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(droidDir, "hooks.json"), []byte("{not-json\n"), 0644); err != nil {
		t.Fatalf("write malformed hooks: %v", err)
	}

	if err := refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentDroid}); err == nil {
		t.Fatal("refreshAgentSetup() succeeded with malformed hooks")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("resource created before hook validation = %v, want absent", err)
	}
}

func TestDroidUninitCLIPropagatesManifestError(t *testing.T) {
	setupDroidTest(t)
	previousAgents, previousForce, previousDryRun, previousAll, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal = previousAgents, previousForce, previousDryRun, previousAll, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitAll, uninitGlobal = "droid", true, false, false, false
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	droidDir := filepath.Join(projectDir, ".factory")
	if err := os.MkdirAll(droidDir, 0755); err != nil {
		t.Fatalf("create Droid directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(droidDir, ManifestFile), []byte("{not-json\n"), 0644); err != nil {
		t.Fatalf("write malformed manifest: %v", err)
	}

	if err := runAgentsUninit(nil, nil); err == nil || !strings.Contains(err.Error(), "invalid manifest") {
		t.Fatalf("runAgentsUninit() error = %v, want manifest parse error", err)
	}
}

func TestDroidForceReinitPreservesCreatedInstructionOwnership(t *testing.T) {
	setupDroidTest(t)
	previousForce := initForce
	t.Cleanup(func() { initForce = previousForce })
	projectDir := t.TempDir()
	initForce = false
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContentLocal); err != nil {
		t.Fatalf("initial initAgent() error: %v", err)
	}
	initForce = true
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContentLocal); err != nil {
		t.Fatalf("forced initAgent() error: %v", err)
	}
	manifest, err := loadManifest(filepath.Join(projectDir, ".factory"))
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.Modifications.InstructionsMD == nil || manifest.Modifications.InstructionsMD.Action != "created" {
		t.Fatalf("instruction ownership after force = %+v, want created", manifest.Modifications.InstructionsMD)
	}
	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".factory", "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("thts-created local instructions after uninit = %v, want absent", err)
	}
}

func TestDroidForceReinitFailureRetainsResourceOwnership(t *testing.T) {
	setupDroidTest(t)
	previousForce := initForce
	t.Cleanup(func() { initForce = previousForce })
	projectDir := t.TempDir()
	initForce = false
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationOnDemand); err != nil {
		t.Fatalf("initial initAgent() error: %v", err)
	}
	droidDir := filepath.Join(projectDir, ".factory")
	relativePath := filepath.Join("commands", "thts-resume.md")
	path := filepath.Join(droidDir, relativePath)
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove managed command: %v", err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("replace managed command with directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "blocker"), []byte("block write\n"), 0644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	initForce = true
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationOnDemand); err == nil {
		t.Fatal("forced initAgent() succeeded despite command write failure")
	}
	manifest, err := loadManifest(droidDir)
	if err != nil {
		t.Fatalf("load retained manifest: %v", err)
	}
	if !slices.Contains(manifest.Files, relativePath) {
		t.Fatalf("manifest lost failed command ownership: %+v", manifest.Files)
	}
}

func TestDroidFailedLocalInitRecordsMarkerAndRefreshesGitignore(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, ".gitignore"), 0755); err != nil {
		t.Fatalf("create blocking .gitignore directory: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContentLocal); err == nil {
		t.Fatal("initAgent() succeeded despite unusable .gitignore")
	}
	droidDir := filepath.Join(projectDir, ".factory")
	manifest, err := loadManifest(droidDir)
	if err != nil {
		t.Fatalf("load partial manifest: %v", err)
	}
	if manifest.Modifications.InstructionsMD == nil {
		t.Fatal("partial manifest did not retain local instruction marker ownership")
	}
	if err := os.Remove(filepath.Join(projectDir, ".gitignore")); err != nil {
		t.Fatalf("remove blocking .gitignore directory: %v", err)
	}
	if err := refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("refreshAgentSetup() error: %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(projectDir, ".gitignore"))
	if err != nil || !strings.Contains(string(gitignore), ".factory/AGENTS.md") {
		t.Fatalf("refreshed .gitignore = %q, %v", gitignore, err)
	}
}

func TestDroidLocalInstructionsWithPreexistingMarkerAreNotOwned(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	droidDir := filepath.Join(projectDir, ".factory")
	if err := os.MkdirAll(droidDir, 0755); err != nil {
		t.Fatalf("create Droid directory: %v", err)
	}
	localInstructions := filepath.Join(droidDir, "AGENTS.md")
	const userContent = "# User instructions\n\n<!-- thts-start -->\nuser-managed content\n<!-- thts-end -->\n"
	if err := os.WriteFile(localInstructions, []byte(userContent), 0644); err != nil {
		t.Fatalf("write preexisting marker: %v", err)
	}

	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContentLocal); err != nil {
		t.Fatalf("initialize Droid: %v", err)
	}
	manifest, err := loadManifest(droidDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.Modifications.InstructionsMD != nil {
		t.Fatalf("manifest claimed preexisting instructions: %+v", manifest.Modifications.InstructionsMD)
	}
	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("uninitialize Droid: %v", err)
	}
	if got, err := os.ReadFile(localInstructions); err != nil || string(got) != userContent {
		t.Fatalf("preexisting instructions after uninit = %q, %v", got, err)
	}
}

func TestDroidSharedMarkerOwnershipTransfersToAnotherAgent(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContent); err != nil {
		t.Fatalf("initialize Droid: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationAgentsContent); err != nil {
		t.Fatalf("initialize Pi: %v", err)
	}
	rootInstructions := filepath.Join(projectDir, "AGENTS.md")
	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("uninitialize Droid: %v", err)
	}
	if content, err := os.ReadFile(rootInstructions); err != nil || !strings.Contains(string(content), ThtsMarkerStart) {
		t.Fatalf("shared marker after Droid uninit = %q, %v", content, err)
	}
	piManifest, err := loadManifest(filepath.Join(projectDir, ".pi"))
	if err != nil || piManifest.Modifications.InstructionsMD == nil {
		t.Fatalf("Pi did not receive shared marker ownership: %+v, %v", piManifest, err)
	}
	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentPi}); err != nil {
		t.Fatalf("uninitialize Pi: %v", err)
	}
	if _, err := os.Stat(rootInstructions); !os.IsNotExist(err) {
		t.Fatalf("shared instructions after final owner uninit = %v, want absent", err)
	}
}

func TestDroidSharedMarkerOwnershipTransfersFromNestedDirectory(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	if err := exec.Command("git", "init", projectDir).Run(); err != nil {
		t.Fatalf("initialize repository: %v", err)
	}
	nestedDir := filepath.Join(projectDir, "nested")
	if err := os.Mkdir(nestedDir, 0755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	if err := initAgent(nestedDir, internalagents.AgentDroid, IntegrationAgentsContent); err != nil {
		t.Fatalf("initialize Droid: %v", err)
	}
	if err := initAgent(nestedDir, internalagents.AgentPi, IntegrationAgentsContent); err != nil {
		t.Fatalf("initialize Pi: %v", err)
	}
	rootInstructions := filepath.Join(projectDir, "AGENTS.md")
	if err := Uninit(nestedDir, true, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("uninitialize Droid: %v", err)
	}
	if content, err := os.ReadFile(rootInstructions); err != nil || !strings.Contains(string(content), ThtsMarkerStart) {
		t.Fatalf("shared marker after Droid uninit = %q, %v", content, err)
	}
	piManifest, err := loadManifest(filepath.Join(nestedDir, ".pi"))
	if err != nil || piManifest.Modifications.InstructionsMD == nil {
		t.Fatalf("Pi did not receive shared marker ownership: %+v, %v", piManifest, err)
	}
}

func TestDroidRefreshRemovesLocalResourcesWhenComponentsBecomeGlobal(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationOnDemand); err != nil {
		t.Fatalf("initialize Droid: %v", err)
	}
	droidDir := filepath.Join(projectDir, ".factory")
	userDroid := filepath.Join(droidDir, "droids", "user-droid.md")
	if err := os.WriteFile(userDroid, []byte("user droid\n"), 0644); err != nil {
		t.Fatalf("write user droid: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"droid": {
			Skills:   config.ComponentModeGlobal,
			Commands: config.ComponentModeGlobal,
			Agents:   config.ComponentModeGlobal,
		},
	}}}); err != nil {
		t.Fatalf("save global component modes: %v", err)
	}

	if err := refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentDroid}); err != nil {
		t.Fatalf("refresh Droid: %v", err)
	}
	for _, path := range []string{
		filepath.Join(droidDir, "skills", "thts-integrate", "SKILL.md"),
		filepath.Join(droidDir, "commands", "thts-handoff.md"),
		filepath.Join(droidDir, "droids", "thoughts-locator.md"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("local resource after global transition %s = %v, want absent", path, err)
		}
	}
	if _, err := os.Stat(userDroid); err != nil {
		t.Fatalf("user droid after global transition: %v", err)
	}
}

func TestDroidIntegrationLevels(t *testing.T) {
	setupDroidTest(t)

	t.Run("hook", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationHook); err != nil {
			t.Fatalf("init hook: %v", err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, ".factory", "hooks.json")); err != nil {
			t.Fatalf("hook configuration: %v", err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, "AGENTS.md")); !os.IsNotExist(err) {
			t.Fatalf("root AGENTS.md in hook mode = %v, want absent", err)
		}
	})

	t.Run("shared content", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContent); err != nil {
			t.Fatalf("init shared content: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
		if err != nil || !strings.Contains(string(content), ThtsMarkerStart) {
			t.Fatalf("root AGENTS.md = %q, %v", content, err)
		}
	})

	t.Run("local content preserves user instructions", func(t *testing.T) {
		projectDir := t.TempDir()
		localPath := filepath.Join(projectDir, ".factory", "AGENTS.md")
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			t.Fatalf("create local instructions directory: %v", err)
		}
		if err := os.WriteFile(localPath, []byte("# Factory Instructions\n\nKeep this.\n"), 0644); err != nil {
			t.Fatalf("write user instructions: %v", err)
		}
		if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContentLocal); err != nil {
			t.Fatalf("init local content: %v", err)
		}
		content, err := os.ReadFile(localPath)
		if err != nil || !strings.Contains(string(content), "Keep this.") || !strings.Contains(string(content), ThtsMarkerStart) {
			t.Fatalf("local AGENTS.md = %q, %v", content, err)
		}
		gitignore, err := os.ReadFile(filepath.Join(projectDir, ".gitignore"))
		if err != nil || !strings.Contains(string(gitignore), ".factory/AGENTS.md") {
			t.Fatalf(".gitignore = %q, %v", gitignore, err)
		}
	})

	t.Run("on demand", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationOnDemand); err != nil {
			t.Fatalf("init on demand: %v", err)
		}
		for _, path := range []string{
			filepath.Join(projectDir, "AGENTS.md"),
			filepath.Join(projectDir, ".factory", "AGENTS.md"),
			filepath.Join(projectDir, ".factory", "hooks.json"),
			filepath.Join(projectDir, ".factory", "hooks"),
		} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("on-demand activation artifact %s = %v, want absent", path, err)
			}
		}
	})
}

func TestDroidModeTransitionsRemoveOnlyManagedActivation(t *testing.T) {
	setupDroidTest(t)
	projectDir := t.TempDir()
	droidDir := filepath.Join(projectDir, ".factory")
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationHook); err != nil {
		t.Fatalf("init hook: %v", err)
	}
	userHook := filepath.Join(droidDir, "hooks", "user-hook.sh")
	if err := os.WriteFile(userHook, []byte("user hook"), 0644); err != nil {
		t.Fatalf("write user hook: %v", err)
	}
	localInstructions := filepath.Join(droidDir, "AGENTS.md")
	if err := os.WriteFile(localInstructions, []byte("# User Factory Instructions\n"), 0644); err != nil {
		t.Fatalf("write user local instructions: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContentLocal); err != nil {
		t.Fatalf("hook to local transition: %v", err)
	}
	if _, err := os.Stat(filepath.Join(droidDir, "hooks", "thts-session-start.sh")); !os.IsNotExist(err) {
		t.Fatalf("managed hook after transition = %v, want absent", err)
	}
	if _, err := os.Stat(userHook); err != nil {
		t.Fatalf("user hook after transition: %v", err)
	}
	if _, err := os.Stat(filepath.Join(droidDir, "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("empty thts-created hooks.json after transition = %v, want absent", err)
	}
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationAgentsContent); err != nil {
		t.Fatalf("local to shared transition: %v", err)
	}
	localContent, err := os.ReadFile(localInstructions)
	if err != nil || string(localContent) != "# User Factory Instructions\n" {
		t.Fatalf("user local AGENTS.md after transition = %q, %v", localContent, err)
	}
	rootContent, err := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
	if err != nil || !strings.Contains(string(rootContent), ThtsMarkerStart) {
		t.Fatalf("shared AGENTS.md = %q, %v", rootContent, err)
	}
	if err := initAgent(projectDir, internalagents.AgentDroid, IntegrationOnDemand); err != nil {
		t.Fatalf("shared to on-demand transition: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("thts-created shared AGENTS.md after transition = %v, want absent", err)
	}
}

func TestDroidGlobalLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDroidTest(t)
	droidRoot := filepath.Join(home, ".factory")
	piRoot := filepath.Join(home, ".pi-test")
	t.Setenv("PI_CODING_AGENT_DIR", piRoot)
	piSkill := filepath.Join(piRoot, "skills", "thts-integrate", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(piSkill), 0755); err != nil {
		t.Fatalf("create existing Pi skill directory: %v", err)
	}
	if err := os.WriteFile(piSkill, []byte("Pi skill"), 0644); err != nil {
		t.Fatalf("write existing Pi skill: %v", err)
	}
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {Agents: []string{"pi"}, Files: []string{piSkill}},
	}}); err != nil {
		t.Fatalf("save existing Pi manifest: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"pi": {Skills: config.ComponentModeGlobal},
	}}}); err != nil {
		t.Fatalf("save existing Pi config: %v", err)
	}
	if err := os.MkdirAll(droidRoot, 0755); err != nil {
		t.Fatalf("create global Droid root: %v", err)
	}
	hooksPath := filepath.Join(droidRoot, "hooks.json")
	writeDroidHooks(t, hooksPath, map[string]any{"SessionStart": []any{droidHookEntry("/user/hook.sh")}})
	settingsPath := filepath.Join(droidRoot, "settings.json")
	const settings = "{\"theme\":\"dark\"}\n"
	if err := os.WriteFile(settingsPath, []byte(settings), 0644); err != nil {
		t.Fatalf("write global settings: %v", err)
	}

	previousAgents, previousGlobal, previousDryRun := initAgents, initGlobal, initDryRun
	t.Cleanup(func() { initAgents, initGlobal, initDryRun = previousAgents, previousGlobal, previousDryRun })
	initAgents, initGlobal, initDryRun = "droid", "all", false
	if err := runGlobalInit(nil, nil); err != nil {
		t.Fatalf("runGlobalInit() error: %v", err)
	}
	for _, path := range []string{
		filepath.Join(droidRoot, "skills", "thts-integrate", "SKILL.md"),
		filepath.Join(droidRoot, "commands", "thts-handoff.md"),
		filepath.Join(droidRoot, "droids", "thoughts-locator.md"),
		filepath.Join(droidRoot, "hooks", "thts-session-start.sh"),
		hooksPath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("global Droid resource %s: %v", path, err)
		}
	}
	assertDroidHookState(t, hooksPath, true, true)
	if got, err := os.ReadFile(settingsPath); err != nil || string(got) != settings {
		t.Fatalf("global settings = %q, %v; want preserved", got, err)
	}
	manifest, err := LoadGlobalManifest()
	if err != nil {
		t.Fatalf("LoadGlobalManifest() error: %v", err)
	}
	for _, component := range []string{"skills", "commands", "agents", "hooks"} {
		if !manifest.HasAgentComponent("droid", component) {
			t.Errorf("Droid does not own global %s: %+v", component, manifest.Components)
		}
	}
	if !manifest.HasAgentComponent("pi", "skills") {
		t.Fatalf("Droid global init removed Pi ownership: %+v", manifest.Components)
	}
	if err := runGlobalInit(nil, nil); err != nil {
		t.Fatalf("repeated runGlobalInit() error: %v", err)
	}
	manifest, err = LoadGlobalManifest()
	if err != nil || !slices.Contains(manifest.Components["hooks"].Files, hooksPath) {
		t.Fatalf("repeated global init lost hook config ownership: %+v, %v", manifest, err)
	}

	userDroid := filepath.Join(droidRoot, "droids", "user-droid.md")
	if err := os.WriteFile(userDroid, []byte("user droid"), 0644); err != nil {
		t.Fatalf("write global user droid: %v", err)
	}
	previousUninitAgents, previousForce, previousUninitDryRun, previousUninitGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousUninitAgents, previousForce, previousUninitDryRun, previousUninitGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "droid", true, false, true
	if err := runGlobalUninit(nil, nil); err != nil {
		t.Fatalf("runGlobalUninit() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(droidRoot, "skills", "thts-integrate", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("global managed skill after uninit = %v, want absent", err)
	}
	if _, err := os.Stat(userDroid); err != nil {
		t.Fatalf("global user droid after uninit: %v", err)
	}
	assertDroidHookState(t, hooksPath, false, true)
	if got, err := os.ReadFile(settingsPath); err != nil || string(got) != settings {
		t.Fatalf("global settings after uninit = %q, %v; want preserved", got, err)
	}
	manifest, err = LoadGlobalManifest()
	if err != nil || !manifest.HasAgentComponent("pi", "skills") || manifest.HasAgentComponent("droid", "skills") {
		t.Fatalf("targeted Droid uninit manifest = %+v, %v", manifest, err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config after Droid uninit: %v", err)
	}
	if mode, _ := loaded.GetAgentComponentOverride("pi", "skills"); mode != config.ComponentModeGlobal {
		t.Errorf("Pi skills mode = %q, want global after Droid uninit", mode)
	}
}

func TestDroidGlobalInitRejectsUnownedResourceCollision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDroidTest(t)
	previousAgents, previousGlobal, previousDryRun, previousForce := initAgents, initGlobal, initDryRun, initForce
	t.Cleanup(func() {
		initAgents, initGlobal, initDryRun, initForce = previousAgents, previousGlobal, previousDryRun, previousForce
	})
	initAgents, initGlobal, initDryRun, initForce = "droid", "agents", false, false

	path := filepath.Join(home, ".factory", "droids", "thoughts-analyzer.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create global droids directory: %v", err)
	}
	const userContent = "user-owned global droid\n"
	if err := os.WriteFile(path, []byte(userContent), 0644); err != nil {
		t.Fatalf("write global droid: %v", err)
	}

	err := runGlobalInit(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not owned by thts") {
		t.Fatalf("runGlobalInit() error = %v, want unowned-resource error", err)
	}
	if got, readErr := os.ReadFile(path); readErr != nil || string(got) != userContent {
		t.Fatalf("global user droid after rejected init = %q, %v", got, readErr)
	}
	manifest, loadErr := LoadGlobalManifest()
	if loadErr != nil {
		t.Fatalf("LoadGlobalManifest() error: %v", loadErr)
	}
	if manifest != nil && manifest.HasAgentComponent("droid", "agents") {
		t.Fatalf("global manifest claimed rejected droid resource: %+v", manifest)
	}
}

func TestDroidFailedGlobalHookInitDoesNotClaimUncreatedResources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDroidTest(t)
	previousAgents, previousGlobal, previousDryRun, previousForce := initAgents, initGlobal, initDryRun, initForce
	t.Cleanup(func() {
		initAgents, initGlobal, initDryRun, initForce = previousAgents, previousGlobal, previousDryRun, previousForce
	})
	initAgents, initGlobal, initDryRun, initForce = "droid", "hooks", false, false
	droidRoot := filepath.Join(home, ".factory")
	if err := os.MkdirAll(droidRoot, 0755); err != nil {
		t.Fatalf("create global Droid root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(droidRoot, "hooks.json"), []byte("{not-json\n"), 0644); err != nil {
		t.Fatalf("write malformed hook config: %v", err)
	}

	if err := runGlobalInit(nil, nil); err == nil {
		t.Fatal("runGlobalInit() succeeded with malformed hook configuration")
	}
	manifest, err := LoadGlobalManifest()
	if err != nil {
		t.Fatalf("LoadGlobalManifest() error: %v", err)
	}
	if manifest != nil && manifest.HasAgentComponent("droid", "hooks") {
		t.Fatalf("global manifest claimed uncreated hooks: %+v", manifest.Components)
	}
	for _, name := range []string{"thts-session-start.sh", "thts-prompt-check.sh"} {
		if _, err := os.Stat(filepath.Join(droidRoot, "hooks", name)); !os.IsNotExist(err) {
			t.Fatalf("hook script after failed validation %s = %v, want absent", name, err)
		}
	}
}

func TestDroidPartialGlobalHookInstallRetainsScriptOwnership(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDroidTest(t)
	previousAgents, previousGlobal, previousDryRun, previousForce := initAgents, initGlobal, initDryRun, initForce
	t.Cleanup(func() {
		initAgents, initGlobal, initDryRun, initForce = previousAgents, previousGlobal, previousDryRun, previousForce
	})
	initAgents, initGlobal, initDryRun, initForce = "droid", "hooks", false, false
	droidRoot := filepath.Join(home, ".factory")
	if err := os.MkdirAll(droidRoot, 0755); err != nil {
		t.Fatalf("create global Droid root: %v", err)
	}
	hooksPath := filepath.Join(droidRoot, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte("{}\n"), 0444); err != nil {
		t.Fatalf("write read-only hooks config: %v", err)
	}

	if err := runGlobalInit(nil, nil); err == nil {
		t.Fatal("runGlobalInit() succeeded despite unwritable hooks.json")
	}
	manifest, err := LoadGlobalManifest()
	if err != nil {
		t.Fatalf("LoadGlobalManifest() error: %v", err)
	}
	for _, name := range []string{"thts-session-start.sh", "thts-prompt-check.sh"} {
		path := filepath.Join(droidRoot, "hooks", name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("partially installed script %s: %v", path, err)
		}
		if manifest == nil || !slices.Contains(manifest.Components["hooks"].Files, path) {
			t.Fatalf("partial global manifest does not own %s: %+v", path, manifest)
		}
	}
	if manifest != nil && slices.Contains(manifest.Components["hooks"].Files, hooksPath) {
		t.Fatalf("partial global manifest claimed unmodified hook config: %+v", manifest.Components["hooks"])
	}
}

func TestDroidPreexistingEmptyGlobalHookConfigIsPreserved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDroidTest(t)
	previousAgents, previousGlobal, previousDryRun, previousForce := initAgents, initGlobal, initDryRun, initForce
	t.Cleanup(func() {
		initAgents, initGlobal, initDryRun, initForce = previousAgents, previousGlobal, previousDryRun, previousForce
	})
	initAgents, initGlobal, initDryRun, initForce = "droid", "hooks", false, false
	droidRoot := filepath.Join(home, ".factory")
	if err := os.MkdirAll(droidRoot, 0755); err != nil {
		t.Fatalf("create global Droid root: %v", err)
	}
	hooksPath := filepath.Join(droidRoot, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte("{}\n"), 0644); err != nil {
		t.Fatalf("write empty global hooks config: %v", err)
	}
	if err := runGlobalInit(nil, nil); err != nil {
		t.Fatalf("runGlobalInit() error: %v", err)
	}
	previousUninitAgents, previousUninitForce, previousUninitDryRun, previousUninitGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousUninitAgents, previousUninitForce, previousUninitDryRun, previousUninitGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "droid", true, false, true
	if err := runGlobalUninit(nil, nil); err != nil {
		t.Fatalf("runGlobalUninit() error: %v", err)
	}
	if document := readDroidHooks(t, hooksPath); len(document) != 0 {
		t.Fatalf("preserved global hooks config = %+v, want empty", document)
	}
}

func TestDroidGlobalUninitRestoresOwnershipWhenConfigSaveFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDroidTest(t)
	path := filepath.Join(home, ".factory", "skills", "thts-integrate", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create global skill directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("managed skill\n"), 0644); err != nil {
		t.Fatalf("write global skill: %v", err)
	}
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {Agents: []string{"droid"}, Files: []string{path}},
	}}); err != nil {
		t.Fatalf("save global manifest: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"droid": {Skills: config.ComponentModeGlobal},
	}}}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	previousSaveConfig := saveConfigForGlobalUninit
	saveConfigForGlobalUninit = func(*config.Config) error { return os.ErrPermission }
	t.Cleanup(func() { saveConfigForGlobalUninit = previousSaveConfig })
	previousAgents, previousForce, previousDryRun, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousAgents, previousForce, previousDryRun, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "droid", true, false, true

	if err := runGlobalUninit(nil, nil); err == nil {
		t.Fatal("runGlobalUninit() succeeded despite config save failure")
	}
	manifest, err := LoadGlobalManifest()
	if err != nil || manifest == nil || !manifest.HasAgentComponent("droid", "skills") {
		t.Fatalf("global ownership after config failure = %+v, %v", manifest, err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if mode, _ := loaded.GetAgentComponentOverride("droid", "skills"); mode != config.ComponentModeGlobal {
		t.Fatalf("Droid skills mode after config failure = %q, want global", mode)
	}
}

func TestDroidGlobalHookCleanupFailureRetainsOwnership(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDroidTest(t)
	droidRoot := filepath.Join(home, ".factory")
	if err := os.MkdirAll(droidRoot, 0755); err != nil {
		t.Fatalf("create Droid root: %v", err)
	}
	hooksPath := filepath.Join(droidRoot, "hooks.json")
	const malformed = "{not-json\n"
	if err := os.WriteFile(hooksPath, []byte(malformed), 0644); err != nil {
		t.Fatalf("write malformed hooks: %v", err)
	}
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"hooks": {Agents: []string{"droid"}, Files: []string{hooksPath}},
	}}); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"droid": {Hooks: config.ComponentModeGlobal},
	}}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	previousAgents, previousForce, previousDryRun, previousGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousAgents, previousForce, previousDryRun, previousGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "droid", true, false, true
	if err := runGlobalUninit(nil, nil); err == nil {
		t.Fatal("runGlobalUninit() succeeded with malformed Droid hooks.json")
	}
	data, err := os.ReadFile(hooksPath)
	if err != nil || string(data) != malformed {
		t.Fatalf("malformed hooks after cleanup = %q, %v", data, err)
	}
	manifest, err := LoadGlobalManifest()
	if err != nil || !manifest.HasAgentComponent("droid", "hooks") {
		t.Fatalf("Droid hook ownership after failed cleanup = %+v, %v", manifest, err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if mode, _ := loaded.GetAgentComponentOverride("droid", "hooks"); mode != config.ComponentModeGlobal {
		t.Errorf("Droid hooks mode = %q, want global after failed cleanup", mode)
	}
}

func TestDroidGlobalDryRunCreatesNoFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDroidTest(t)
	previousAgents, previousGlobal, previousDryRun := initAgents, initGlobal, initDryRun
	t.Cleanup(func() { initAgents, initGlobal, initDryRun = previousAgents, previousGlobal, previousDryRun })
	initAgents, initGlobal, initDryRun = "droid", "all", true
	if err := runGlobalInit(nil, nil); err != nil {
		t.Fatalf("runGlobalInit() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".factory")); !os.IsNotExist(err) {
		t.Fatalf("global dry-run created .factory: %v", err)
	}
	if _, err := os.Stat(config.GlobalManifestPath()); !os.IsNotExist(err) {
		t.Fatalf("global dry-run created manifest: %v", err)
	}
}

func assertDroidProjectLayout(t *testing.T, droidDir string) {
	t.Helper()
	for _, relative := range []string{
		filepath.Join("skills", "thts-integrate", "SKILL.md"),
		filepath.Join("commands", "thts-handoff.md"),
		filepath.Join("commands", "thts-resume.md"),
		filepath.Join("droids", "thoughts-locator.md"),
		filepath.Join("droids", "thoughts-analyzer.md"),
		filepath.Join("hooks", "thts-session-start.sh"),
		filepath.Join("hooks", "thts-prompt-check.sh"),
		"hooks.json",
	} {
		if _, err := os.Stat(filepath.Join(droidDir, relative)); err != nil {
			t.Errorf("Droid resource %s: %v", relative, err)
		}
	}
}

func droidHookEntry(command string) map[string]any {
	return map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command}}}
}

func writeDroidHooks(t *testing.T, path string, document map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("marshal hooks: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write hooks: %v", err)
	}
}

func readDroidHooks(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hooks: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse hooks: %v", err)
	}
	return document
}

func assertDroidHookState(t *testing.T, path string, wantThts, global bool) {
	t.Helper()
	document := readDroidHooks(t, path)
	if _, wrapped := document["hooks"]; wrapped {
		t.Fatal("Droid hooks.json contains an invalid outer hooks key")
	}
	if _, ok := document["CustomEvent"]; ok {
		if _, ok := document["customKey"]; !ok {
			t.Fatal("unrelated custom JSON key was removed")
		}
	}
	commands := getThtsHookNames(internalagents.AgentDroid, global)
	found := findThtsHookCommands(document, getThtsHookEventNames(internalagents.AgentDroid), commands)
	if wantThts && len(found) != 2 {
		t.Fatalf("thts Droid hook commands = %v, want exactly two", found)
	}
	if !wantThts && len(found) != 0 {
		t.Fatalf("thts Droid hook commands = %v, want none", found)
	}
	if wantThts && !global {
		for _, command := range commands {
			if !strings.HasPrefix(command, "\"$FACTORY_PROJECT_DIR\"/.factory/hooks/") {
				t.Errorf("project Droid hook command = %q, want FACTORY_PROJECT_DIR", command)
			}
		}
	}
}
