package agents

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	internalagents "github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
)

func TestPiWithSettingsSkipsManagedSettings(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	previousWithSettings := initWithSettings
	t.Cleanup(func() { initWithSettings = previousWithSettings })
	initWithSettings = true

	projectDir := t.TempDir()
	plan, err := buildInstallationPlan(projectDir, internalagents.AgentPi, IntegrationHook)
	if err != nil {
		t.Fatalf("buildInstallationPlan() error: %v", err)
	}
	if plan.settingsFile != "" {
		t.Errorf("Pi plan settings file = %q, want none", plan.settingsFile)
	}
	dryRunOutput := captureStdout(t, func() { printInstallationPlan(plan) })
	if !strings.Contains(dryRunOutput, "Pi has no thts-managed settings") {
		t.Errorf("Pi dry-run output = %q, want managed-settings skip notice", dryRunOutput)
	}
	if strings.Contains(dryRunOutput, "settings.json") {
		t.Errorf("Pi dry-run output promises settings.json: %q", dryRunOutput)
	}

	piDir := filepath.Join(projectDir, internalagents.GetConfig(internalagents.AgentPi).RootDir)
	settingsPath := filepath.Join(piDir, "settings.json")
	var initErr error
	initOutput := captureStdout(t, func() {
		initErr = initAgent(projectDir, internalagents.AgentPi, IntegrationHook)
	})
	if initErr != nil {
		t.Fatalf("initAgent() error: %v", initErr)
	}
	if !strings.Contains(initOutput, "does not manage Pi settings") {
		t.Errorf("Pi settings skip output = %q, want informative skip", initOutput)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Errorf("Pi initialization created settings.json: %v", err)
	}

	const userSettings = `{"theme":"dark"}\n`
	if err := os.WriteFile(settingsPath, []byte(userSettings), 0644); err != nil {
		t.Fatalf("write user Pi settings: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationHook); err != nil {
		t.Fatalf("reinitialize Pi agent: %v", err)
	}

	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read user Pi settings: %v", err)
	}
	if string(got) != userSettings {
		t.Errorf("Pi settings = %q, want existing user settings preserved", got)
	}

	manifest, err := loadManifest(piDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.SettingsCreated {
		t.Error("Pi manifest SettingsCreated = true, want false")
	}
	if slices.Contains(manifest.Files, "settings.json") {
		t.Errorf("Pi manifest files = %v, must not include settings.json", manifest.Files)
	}
}

func TestPiHookExtensionLifecycle(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	projectDir := t.TempDir()
	piConfig := internalagents.GetConfig(internalagents.AgentPi)
	piDir := filepath.Join(projectDir, piConfig.RootDir)
	relativeExtension := filepath.Join(piConfig.PluginsDir, "thts-integration.ts")
	extensionPath := filepath.Join(piDir, relativeExtension)

	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationHook); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	if _, err := os.Stat(extensionPath); err != nil {
		t.Fatalf("Pi extension after hook init: %v", err)
	}

	manifest, err := loadManifest(piDir)
	if err != nil {
		t.Fatalf("loadManifest() error: %v", err)
	}
	if countValue(manifest.Files, relativeExtension) != 1 {
		t.Fatalf("manifest files = %v, want one %s", manifest.Files, relativeExtension)
	}

	if err := os.WriteFile(extensionPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("write stale extension: %v", err)
	}
	var refreshErr error
	refreshOutput := captureStdout(t, func() {
		refreshErr = refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentPi})
	})
	if refreshErr != nil {
		t.Fatalf("refreshAgentSetup() error: %v", refreshErr)
	}
	if !strings.Contains(refreshOutput, "Updated 1 extension(s)") {
		t.Errorf("Pi refresh output = %q, want extension terminology", refreshOutput)
	}
	if strings.Contains(strings.ToLower(refreshOutput), "plugin") {
		t.Errorf("Pi refresh output uses plugin terminology: %q", refreshOutput)
	}
	content, err := os.ReadFile(extensionPath)
	if err != nil {
		t.Fatalf("read refreshed extension: %v", err)
	}
	if string(content) == "stale" {
		t.Fatal("refresh did not restore the Pi extension")
	}

	if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentPi}); err != nil {
		t.Fatalf("Uninit() error: %v", err)
	}
	if _, err := os.Stat(extensionPath); !os.IsNotExist(err) {
		t.Fatalf("Pi extension after uninit = %v, want removed", err)
	}
}

func TestPiIntegrationLevels(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	piConfig := internalagents.GetConfig(internalagents.AgentPi)

	t.Run("shared updates only root AGENTS", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := initAgent(projectDir, internalagents.AgentPi, IntegrationAgentsContent); err != nil {
			t.Fatalf("initAgent() error: %v", err)
		}
		rootInstructions, err := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
		if err != nil || !strings.Contains(string(rootInstructions), ThtsMarkerStart) {
			t.Fatalf("root AGENTS.md = %q, %v; want thts marker", rootInstructions, err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, piConfig.RootDir, piConfig.PluginsDir, "thts-integration.ts")); !os.IsNotExist(err) {
			t.Fatalf("Pi extension in shared mode = %v, want absent", err)
		}
	})

	t.Run("local uses an ignored extension", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := initAgent(projectDir, internalagents.AgentPi, IntegrationAgentsContentLocal); err != nil {
			t.Fatalf("initAgent() error: %v", err)
		}
		if err := updateGitignoreForAgents(projectDir, []internalagents.AgentType{internalagents.AgentPi}); err != nil {
			t.Fatalf("updateGitignoreForAgents() error: %v", err)
		}
		extensionPath := filepath.Join(projectDir, piConfig.RootDir, piConfig.PluginsDir, "thts-integration.ts")
		if _, err := os.Stat(extensionPath); err != nil {
			t.Fatalf("Pi extension in local mode: %v", err)
		}
		gitignore, err := os.ReadFile(filepath.Join(projectDir, ".gitignore"))
		if err != nil || !strings.Contains(string(gitignore), ".pi/*/thts-*") {
			t.Fatalf(".gitignore = %q, %v; want Pi managed extension pattern", gitignore, err)
		}
	})

	t.Run("on-demand has no runtime adapter", func(t *testing.T) {
		projectDir := t.TempDir()
		if err := initAgent(projectDir, internalagents.AgentPi, IntegrationOnDemand); err != nil {
			t.Fatalf("initAgent() error: %v", err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, "AGENTS.md")); !os.IsNotExist(err) {
			t.Fatalf("root AGENTS.md in on-demand mode = %v, want absent", err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, piConfig.RootDir, piConfig.PluginsDir, "thts-integration.ts")); !os.IsNotExist(err) {
			t.Fatalf("Pi extension in on-demand mode = %v, want absent", err)
		}
	})
}

func TestPiLocalContentUsesGlobalRuntimeAdapter(t *testing.T) {
	stateDir := t.TempDir()
	piRoot := filepath.Join(t.TempDir(), "pi")
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("PI_CODING_AGENT_DIR", piRoot)

	globalExtension := filepath.Join(piRoot, "extensions", "thts-integration.ts")
	if err := os.MkdirAll(filepath.Dir(globalExtension), 0755); err != nil {
		t.Fatalf("create global Pi extensions: %v", err)
	}
	if err := os.WriteFile(globalExtension, []byte("global extension"), 0644); err != nil {
		t.Fatalf("write global Pi extension: %v", err)
	}
	if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"hooks": {Agents: []string{"pi"}, Files: []string{globalExtension}},
	}}); err != nil {
		t.Fatalf("save global manifest: %v", err)
	}
	if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
		"pi": {Hooks: config.ComponentModeGlobal},
	}}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	projectDir := t.TempDir()
	plan, err := buildInstallationPlan(projectDir, internalagents.AgentPi, IntegrationAgentsContentLocal)
	if err != nil {
		t.Fatalf("buildInstallationPlan() error: %v", err)
	}
	if len(plan.pluginFiles) != 0 || plan.instructionsFile != "" || len(plan.gitignorePatterns) != 0 {
		t.Fatalf("Pi local-content plan with global adapter = %+v, want no local adapter or instructions", plan)
	}

	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationAgentsContentLocal); err != nil {
		t.Fatalf("initAgent() error: %v", err)
	}
	piDir := filepath.Join(projectDir, ".pi")
	for _, path := range []string{filepath.Join(piDir, "extensions", "thts-integration.ts"), filepath.Join(piDir, "AGENTS.local.md")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("Pi local-content resource %s = %v, want absent", path, err)
		}
	}
}

func TestPiGlobalRuntimeAdapterRemovesPreviousLocalExtension(t *testing.T) {
	piConfig := internalagents.GetConfig(internalagents.AgentPi)
	relativeExtension := filepath.Join(piConfig.PluginsDir, "thts-integration.ts")

	for _, tt := range []struct {
		name    string
		initial IntegrationLevel
		apply   func(projectDir string) error
	}{
		{name: "reinit hook", initial: IntegrationHook, apply: func(projectDir string) error {
			return initAgent(projectDir, internalagents.AgentPi, IntegrationHook)
		}},
		{name: "reinit local content", initial: IntegrationAgentsContentLocal, apply: func(projectDir string) error {
			return initAgent(projectDir, internalagents.AgentPi, IntegrationAgentsContentLocal)
		}},
		{name: "refresh hook", initial: IntegrationHook, apply: func(projectDir string) error {
			return refreshAgentSetup(projectDir, []internalagents.AgentType{internalagents.AgentPi})
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateDir := t.TempDir()
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			piRoot := filepath.Join(t.TempDir(), "pi")
			t.Setenv("XDG_STATE_HOME", stateDir)
			t.Setenv("THTS_CONFIG_PATH", configPath)
			t.Setenv("PI_CODING_AGENT_DIR", piRoot)

			projectDir := t.TempDir()
			piDir := filepath.Join(projectDir, piConfig.RootDir)
			extensionPath := filepath.Join(piDir, relativeExtension)
			unrelatedExtension := filepath.Join(piDir, piConfig.PluginsDir, "user-extension.ts")
			if err := initAgent(projectDir, internalagents.AgentPi, tt.initial); err != nil {
				t.Fatalf("initial init: %v", err)
			}
			if _, err := os.Stat(extensionPath); err != nil {
				t.Fatalf("initial local extension: %v", err)
			}
			if err := os.WriteFile(unrelatedExtension, []byte("user extension"), 0644); err != nil {
				t.Fatalf("write unrelated extension: %v", err)
			}

			globalExtension := filepath.Join(piRoot, piConfig.PluginsDir, "thts-integration.ts")
			if err := os.MkdirAll(filepath.Dir(globalExtension), 0755); err != nil {
				t.Fatalf("create global extension dir: %v", err)
			}
			if err := os.WriteFile(globalExtension, []byte("global extension"), 0644); err != nil {
				t.Fatalf("write global extension: %v", err)
			}
			if err := SaveGlobalManifest(&GlobalManifest{Components: map[string]*GlobalComponentInfo{
				"hooks": {Agents: []string{"pi"}, Files: []string{globalExtension}},
			}}); err != nil {
				t.Fatalf("save global manifest: %v", err)
			}
			if err := config.Save(&config.Config{Agents: &config.AgentsConfig{PerAgent: map[string]*config.AgentComponentModes{
				"pi": {Hooks: config.ComponentModeGlobal},
			}}}); err != nil {
				t.Fatalf("save config: %v", err)
			}

			if err := tt.apply(projectDir); err != nil {
				t.Fatalf("apply global runtime adapter mode: %v", err)
			}
			if _, err := os.Stat(extensionPath); !os.IsNotExist(err) {
				t.Fatalf("local Pi extension after global adapter transition = %v, want removed", err)
			}
			if _, err := os.Stat(unrelatedExtension); err != nil {
				t.Fatalf("unrelated extension after global adapter transition: %v", err)
			}

			manifest, err := loadManifest(piDir)
			if err != nil {
				t.Fatalf("load manifest: %v", err)
			}
			if slices.Contains(manifest.Files, relativeExtension) {
				t.Fatalf("manifest still owns local extension: %v", manifest.Files)
			}
		})
	}
}

func TestPiTransitionsRemoveOnlyManifestOwnedLocalExtension(t *testing.T) {
	for _, tt := range []struct {
		name string
		from IntegrationLevel
		to   IntegrationLevel
	}{
		{name: "hook to on-demand", from: IntegrationHook, to: IntegrationOnDemand},
		{name: "local adapter to on-demand", from: IntegrationAgentsContentLocal, to: IntegrationOnDemand},
		{name: "hook to shared", from: IntegrationHook, to: IntegrationAgentsContent},
		{name: "local adapter to shared", from: IntegrationAgentsContentLocal, to: IntegrationAgentsContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
			t.Setenv("XDG_STATE_HOME", t.TempDir())

			projectDir := t.TempDir()
			piDir := filepath.Join(projectDir, ".pi")
			extensionPath := filepath.Join(piDir, "extensions", "thts-integration.ts")
			unrelatedExtension := filepath.Join(piDir, "extensions", "user-extension.ts")
			if err := initAgent(projectDir, internalagents.AgentPi, tt.from); err != nil {
				t.Fatalf("init %s: %v", tt.from, err)
			}
			if err := os.WriteFile(unrelatedExtension, []byte("user extension"), 0644); err != nil {
				t.Fatalf("write unrelated extension: %v", err)
			}

			if err := initAgent(projectDir, internalagents.AgentPi, tt.to); err != nil {
				t.Fatalf("transition to %s: %v", tt.to, err)
			}
			if _, err := os.Stat(extensionPath); !os.IsNotExist(err) {
				t.Fatalf("managed extension after transition = %v, want removed", err)
			}
			if _, err := os.Stat(unrelatedExtension); err != nil {
				t.Fatalf("unrelated extension after transition: %v", err)
			}

			manifest, err := loadManifest(piDir)
			if err != nil {
				t.Fatalf("load transitioned manifest: %v", err)
			}
			if slices.Contains(manifest.Files, filepath.Join("extensions", "thts-integration.ts")) {
				t.Fatalf("transitioned manifest still owns extension: %v", manifest.Files)
			}

			if err := Uninit(projectDir, true, []internalagents.AgentType{internalagents.AgentPi}); err != nil {
				t.Fatalf("Uninit() error: %v", err)
			}
			if _, err := os.Stat(unrelatedExtension); err != nil {
				t.Fatalf("unrelated extension after uninit: %v", err)
			}
		})
	}
}

func TestPiPlanUsesExtensionTerminology(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))

	plan, err := buildInstallationPlan(t.TempDir(), internalagents.AgentPi, IntegrationHook)
	if err != nil {
		t.Fatalf("buildInstallationPlan() error: %v", err)
	}
	output := captureStdout(t, func() { printInstallationPlan(plan) })
	if !strings.Contains(output, "Extensions") {
		t.Errorf("Pi plan = %q, want extension terminology", output)
	}
	if strings.Contains(output, "Plugins") || strings.Contains(output, "Hooks") {
		t.Errorf("Pi plan uses implementation terminology: %q", output)
	}
}

func TestPiExtensionCopyUsesSingularTerminology(t *testing.T) {
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var initErr error
	output := captureStdout(t, func() { initErr = initAgent(t.TempDir(), internalagents.AgentPi, IntegrationHook) })
	if initErr != nil {
		t.Fatalf("initAgent() error: %v", initErr)
	}
	if !strings.Contains(output, "Copied 1 extension(s)") {
		t.Errorf("Pi initialization output = %q, want singular extension terminology", output)
	}
	if strings.Contains(output, "extensions(s)") {
		t.Errorf("Pi initialization output has plural extension wording: %q", output)
	}
}

func TestPiGlobalLifecycleUsesEffectiveRoot(t *testing.T) {
	stateDir := t.TempDir()
	piRoot := filepath.Join(t.TempDir(), "effective-pi-root")
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("PI_CODING_AGENT_DIR", piRoot)

	previousAgents, previousGlobal, previousDryRun := initAgents, initGlobal, initDryRun
	t.Cleanup(func() { initAgents, initGlobal, initDryRun = previousAgents, previousGlobal, previousDryRun })
	initAgents, initGlobal, initDryRun = "pi", "all", false
	if err := runGlobalInit(nil, nil); err != nil {
		t.Fatalf("runGlobalInit() error: %v", err)
	}

	extensionPath := filepath.Join(piRoot, "extensions", "thts-integration.ts")
	if _, err := os.Stat(extensionPath); err != nil {
		t.Fatalf("global Pi extension: %v", err)
	}
	manifest, err := LoadGlobalManifest()
	if err != nil {
		t.Fatalf("LoadGlobalManifest() error: %v", err)
	}
	if _, exists := manifest.Components["agents"]; exists {
		t.Fatalf("Pi global manifest unexpectedly contains agents: %+v", manifest.Components["agents"])
	}
	for _, name := range []string{"thoughts-locator.md", "thoughts-analyzer.md"} {
		if _, err := os.Stat(filepath.Join(piRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("Pi global agent resource %s = %v, want absent", name, err)
		}
	}
	for component, info := range manifest.Components {
		if !manifest.HasAgentComponent("pi", component) {
			t.Errorf("Pi does not own global %s: %+v", component, info)
		}
		for _, path := range info.Files {
			if !filepath.IsAbs(path) || !strings.HasPrefix(path, piRoot+string(filepath.Separator)) {
				t.Errorf("global manifest path = %q, want absolute path under %q", path, piRoot)
			}
		}
	}

	projectDir := t.TempDir()
	if _, err := os.Stat(filepath.Join(projectDir, ".pi")); !os.IsNotExist(err) {
		t.Fatalf("targeted global init created project Pi directory: %v", err)
	}
	if err := initAgent(projectDir, internalagents.AgentPi, IntegrationHook); err != nil {
		t.Fatalf("project initAgent() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".pi", "extensions", "thts-integration.ts")); !os.IsNotExist(err) {
		t.Fatalf("project extension duplicated a global adapter: %v", err)
	}

	unrelatedPath := filepath.Join(piRoot, "extensions", "user-extension.ts")
	if err := os.WriteFile(unrelatedPath, []byte("user extension"), 0644); err != nil {
		t.Fatalf("write unrelated Pi extension: %v", err)
	}
	previousUninitAgents, previousForce, previousUninitDryRun, previousUninitGlobal := uninitAgents, uninitForce, uninitDryRun, uninitGlobal
	t.Cleanup(func() {
		uninitAgents, uninitForce, uninitDryRun, uninitGlobal = previousUninitAgents, previousForce, previousUninitDryRun, previousUninitGlobal
	})
	uninitAgents, uninitForce, uninitDryRun, uninitGlobal = "pi", true, false, true
	if err := runGlobalUninit(nil, nil); err != nil {
		t.Fatalf("runGlobalUninit() error: %v", err)
	}
	if _, err := os.Stat(extensionPath); !os.IsNotExist(err) {
		t.Fatalf("global Pi extension after uninit = %v, want removed", err)
	}
	if _, err := os.Stat(unrelatedPath); err != nil {
		t.Fatalf("unrelated global Pi extension after uninit: %v", err)
	}
}

func TestPiGlobalDryRunCreatesNoFiles(t *testing.T) {
	piRoot := filepath.Join(t.TempDir(), "pi")
	t.Setenv("PI_CODING_AGENT_DIR", piRoot)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("THTS_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))

	previousAgents, previousGlobal, previousDryRun := initAgents, initGlobal, initDryRun
	t.Cleanup(func() { initAgents, initGlobal, initDryRun = previousAgents, previousGlobal, previousDryRun })
	initAgents, initGlobal, initDryRun = "pi", "all", true
	if err := runGlobalInit(nil, nil); err != nil {
		t.Fatalf("runGlobalInit() error: %v", err)
	}
	if _, err := os.Stat(piRoot); !os.IsNotExist(err) {
		t.Fatalf("global dry-run created Pi root: %v", err)
	}
	if _, err := os.Stat(config.GlobalManifestPath()); !os.IsNotExist(err) {
		t.Fatalf("global dry-run created manifest: %v", err)
	}
}
