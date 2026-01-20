//go:build integration

package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottames/thts/internal/agents"
	fsutil "github.com/scottames/thts/internal/fs"
)

func TestHookIntegration_Claude(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create .claude directory
	agentDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create agent dir: %v", err)
	}

	// Get agent config
	cfg := agents.GetConfig(agents.AgentClaude)
	if cfg == nil {
		t.Fatal("expected non-nil config for Claude")
	}
	if !cfg.SupportsHooks {
		t.Fatal("expected Claude to support hooks")
	}

	// Create manifest
	manifest := &Manifest{
		Agent:            string(agents.AgentClaude),
		IntegrationLevel: IntegrationHook,
		Files:            []string{},
		Modifications:    ManifestModifications{},
	}

	// Setup hook integration
	if err := setupHookIntegration(tmpDir, agentDir, agents.AgentClaude, cfg, manifest); err != nil {
		t.Fatalf("setupHookIntegration failed: %v", err)
	}

	// Verify hooks were copied
	sessionStartPath := filepath.Join(agentDir, "hooks", "thts-session-start.sh")
	if _, err := os.Stat(sessionStartPath); os.IsNotExist(err) {
		t.Error("expected thts-session-start.sh to exist")
	}

	promptCheckPath := filepath.Join(agentDir, "hooks", "thts-prompt-check.sh")
	if _, err := os.Stat(promptCheckPath); os.IsNotExist(err) {
		t.Error("expected thts-prompt-check.sh to exist")
	}

	// Verify hooks are executable
	info, err := os.Stat(sessionStartPath)
	if err != nil {
		t.Fatalf("failed to stat hook: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("hook should be executable")
	}

	// Verify settings.local.json was created
	settingsPath := filepath.Join(agentDir, "settings.local.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("expected settings.local.json to exist")
	}

	// Verify settings content
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	hooks, ok := settings["hooks"].([]any)
	if !ok {
		t.Fatal("settings should contain hooks array")
	}
	if len(hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(hooks))
	}

	// Verify manifest was updated
	foundSessionStart := false
	foundPromptCheck := false
	for _, f := range manifest.Files {
		if f == "hooks/thts-session-start.sh" {
			foundSessionStart = true
		}
		if f == "hooks/thts-prompt-check.sh" {
			foundPromptCheck = true
		}
	}
	if !foundSessionStart {
		t.Error("manifest should contain hooks/thts-session-start.sh")
	}
	if !foundPromptCheck {
		t.Error("manifest should contain hooks/thts-prompt-check.sh")
	}
	if manifest.Modifications.Hooks == nil {
		t.Error("manifest.Modifications.Hooks should not be nil")
	}
}

func TestHookIntegration_Gemini(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create .gemini directory
	agentDir := filepath.Join(tmpDir, ".gemini")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create agent dir: %v", err)
	}

	// Get agent config
	cfg := agents.GetConfig(agents.AgentGemini)
	if cfg == nil {
		t.Fatal("expected non-nil config for Gemini")
	}
	if !cfg.SupportsHooks {
		t.Fatal("expected Gemini to support hooks")
	}

	// Create manifest
	manifest := &Manifest{
		Agent:            string(agents.AgentGemini),
		IntegrationLevel: IntegrationHook,
		Files:            []string{},
		Modifications:    ManifestModifications{},
	}

	// Setup hook integration
	if err := setupHookIntegration(tmpDir, agentDir, agents.AgentGemini, cfg, manifest); err != nil {
		t.Fatalf("setupHookIntegration failed: %v", err)
	}

	// Verify hooks were copied
	sessionStartPath := filepath.Join(agentDir, "hooks", "thts-session-start.sh")
	if _, err := os.Stat(sessionStartPath); os.IsNotExist(err) {
		t.Error("expected thts-session-start.sh to exist")
	}

	promptCheckPath := filepath.Join(agentDir, "hooks", "thts-prompt-check.sh")
	if _, err := os.Stat(promptCheckPath); os.IsNotExist(err) {
		t.Error("expected thts-prompt-check.sh to exist")
	}

	// Verify settings.local.json was created
	settingsPath := filepath.Join(agentDir, "settings.local.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("expected settings.local.json to exist")
	}
}

func TestHookIntegration_OpenCode(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create .opencode directory
	agentDir := filepath.Join(tmpDir, ".opencode")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create agent dir: %v", err)
	}

	// Get agent config
	cfg := agents.GetConfig(agents.AgentOpenCode)
	if cfg == nil {
		t.Fatal("expected non-nil config for OpenCode")
	}
	if !cfg.SupportsHooks {
		t.Fatal("expected OpenCode to support hooks")
	}

	// Create manifest
	manifest := &Manifest{
		Agent:            string(agents.AgentOpenCode),
		IntegrationLevel: IntegrationHook,
		Files:            []string{},
		Modifications:    ManifestModifications{},
	}

	// Setup hook integration
	if err := setupHookIntegration(tmpDir, agentDir, agents.AgentOpenCode, cfg, manifest); err != nil {
		t.Fatalf("setupHookIntegration failed: %v", err)
	}

	// Verify plugin was copied (OpenCode uses plugins, not hooks)
	pluginPath := filepath.Join(agentDir, "plugins", "thts-integration.ts")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Error("expected thts-integration.ts plugin to exist")
	}

	// Verify manifest was updated
	found := false
	for _, f := range manifest.Files {
		if f == "plugins/thts-integration.ts" {
			found = true
			break
		}
	}
	if !found {
		t.Error("manifest should contain plugins/thts-integration.ts")
	}
}

func TestHookIntegration_CodexFallback(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create .codex directory
	agentDir := filepath.Join(tmpDir, ".codex")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create agent dir: %v", err)
	}

	// Get agent config
	cfg := agents.GetConfig(agents.AgentCodex)
	if cfg == nil {
		t.Fatal("expected non-nil config for Codex")
	}
	if cfg.SupportsHooks {
		t.Fatal("expected Codex to NOT support hooks")
	}

	// Create manifest
	manifest := &Manifest{
		Agent:            string(agents.AgentCodex),
		IntegrationLevel: IntegrationHook,
		Files:            []string{},
		Modifications:    ManifestModifications{},
	}

	// Setup hook integration (should fall back)
	if err := setupHookIntegration(tmpDir, agentDir, agents.AgentCodex, cfg, manifest); err != nil {
		t.Fatalf("setupHookIntegration failed: %v", err)
	}

	// Verify level was changed to agents-content
	if manifest.IntegrationLevel != IntegrationAgentsContent {
		t.Errorf("expected IntegrationLevel to be agents-content, got %s", manifest.IntegrationLevel)
	}
}

func TestNormalizeIntegrationLevel(t *testing.T) {
	tests := []struct {
		input    IntegrationLevel
		expected IntegrationLevel
	}{
		{IntegrationAlwaysOn, IntegrationAgentsContent},
		{IntegrationLocalOnly, IntegrationAgentsContentLocal},
		{IntegrationHook, IntegrationHook},
		{IntegrationOnDemand, IntegrationOnDemand},
		{IntegrationAgentsContent, IntegrationAgentsContent},
		{IntegrationAgentsContentLocal, IntegrationAgentsContentLocal},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := normalizeIntegrationLevel(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeIntegrationLevel(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMergeHooksIntoSettings_PreservesExisting(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create .claude directory with existing settings
	agentDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create agent dir: %v", err)
	}

	// Write existing settings with custom hooks
	existingSettings := map[string]any{
		"hooks": []any{
			map[string]any{
				"event":   "SessionStart",
				"command": "./custom-hook.sh",
			},
		},
		"someOtherSetting": "value",
	}
	data, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "settings.local.json"), data, 0644); err != nil {
		t.Fatalf("failed to write settings: %v", err)
	}

	// Get agent config
	cfg := agents.GetConfig(agents.AgentClaude)
	if cfg == nil {
		t.Fatal("expected non-nil config for Claude")
	}

	// Merge hooks
	settingsPath, modified, err := mergeHooksIntoSettings(agentDir, agents.AgentClaude, cfg, false)
	if err != nil {
		t.Fatalf("mergeHooksIntoSettings failed: %v", err)
	}
	if !modified {
		t.Error("expected settings to be modified")
	}
	if settingsPath == "" {
		t.Error("expected non-empty settings path")
	}

	// Read back and verify
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	// Should preserve other settings
	if settings["someOtherSetting"] != "value" {
		t.Error("expected someOtherSetting to be preserved")
	}

	// Should have 3 hooks now (1 custom + 2 thts)
	hooks, ok := settings["hooks"].([]any)
	if !ok {
		t.Fatal("settings should contain hooks array")
	}
	if len(hooks) != 3 {
		t.Errorf("expected 3 hooks (1 custom + 2 thts), got %d", len(hooks))
	}
}

func TestFilterOutThtsHooks(t *testing.T) {
	thtsNames := []string{
		"./.claude/hooks/thts-session-start.sh",
		"./.claude/hooks/thts-prompt-check.sh",
	}

	hooks := []any{
		map[string]any{"event": "SessionStart", "command": "./.claude/hooks/thts-session-start.sh"},
		map[string]any{"event": "SessionStart", "command": "./custom-hook.sh"},
		map[string]any{"event": "UserPromptSubmit", "command": "./.claude/hooks/thts-prompt-check.sh"},
	}

	filtered := filterOutThtsHooks(hooks, thtsNames)

	// Should only have the custom hook
	if len(filtered) != 1 {
		t.Errorf("expected 1 hook after filtering, got %d", len(filtered))
	}
	hookMap, ok := filtered[0].(map[string]any)
	if !ok {
		t.Fatal("expected hook to be a map")
	}
	if hookMap["command"] != "./custom-hook.sh" {
		t.Errorf("expected custom hook to remain, got %v", hookMap["command"])
	}
}

func TestGetThtsHookNames(t *testing.T) {
	// Claude should have hook names (project-level)
	claudeNames := getThtsHookNames(agents.AgentClaude, false)
	if len(claudeNames) != 2 {
		t.Errorf("expected 2 hook names for Claude, got %d", len(claudeNames))
	}

	// Gemini should have hook names (project-level)
	geminiNames := getThtsHookNames(agents.AgentGemini, false)
	if len(geminiNames) != 2 {
		t.Errorf("expected 2 hook names for Gemini, got %d", len(geminiNames))
	}

	// Codex should not have hook names (no hook support)
	codexNames := getThtsHookNames(agents.AgentCodex, false)
	if codexNames != nil {
		t.Errorf("expected nil hook names for Codex, got %v", codexNames)
	}

	// OpenCode should not have hook names (uses plugins instead)
	openCodeNames := getThtsHookNames(agents.AgentOpenCode, false)
	if openCodeNames != nil {
		t.Errorf("expected nil hook names for OpenCode, got %v", openCodeNames)
	}
}

func TestGetThtsHookNames_Global(t *testing.T) {
	// Global hook names should use absolute paths
	claudeNames := getThtsHookNames(agents.AgentClaude, true)
	if len(claudeNames) != 2 {
		t.Errorf("expected 2 global hook names for Claude, got %d", len(claudeNames))
	}

	// Verify paths are absolute (start with /)
	for _, name := range claudeNames {
		if !filepath.IsAbs(name) {
			t.Errorf("expected absolute path for global hook, got %s", name)
		}
		if !strings.Contains(name, ".claude/hooks/thts-") {
			t.Errorf("expected hook path to contain '.claude/hooks/thts-', got %s", name)
		}
	}

	// Gemini global hooks
	geminiNames := getThtsHookNames(agents.AgentGemini, true)
	if len(geminiNames) != 2 {
		t.Errorf("expected 2 global hook names for Gemini, got %d", len(geminiNames))
	}
	for _, name := range geminiNames {
		if !filepath.IsAbs(name) {
			t.Errorf("expected absolute path for global hook, got %s", name)
		}
	}
}

func TestBuildHooksConfig_Global(t *testing.T) {
	cfg := agents.GetConfig(agents.AgentClaude)
	if cfg == nil {
		t.Fatal("expected non-nil config for Claude")
	}

	// Build hooks config for global
	hooksConfig := buildHooksConfig(agents.AgentClaude, cfg, true)
	if len(hooksConfig) != 2 {
		t.Errorf("expected 2 hooks for Claude, got %d", len(hooksConfig))
	}

	// Verify commands are absolute paths
	for _, hook := range hooksConfig {
		hookMap, ok := hook.(map[string]any)
		if !ok {
			t.Fatal("expected hook to be a map")
		}
		cmd, ok := hookMap["command"].(string)
		if !ok {
			t.Fatal("expected command to be a string")
		}
		if !filepath.IsAbs(cmd) {
			t.Errorf("expected absolute path in global hook command, got %s", cmd)
		}
	}

	// Build hooks config for project (should be relative)
	projectConfig := buildHooksConfig(agents.AgentClaude, cfg, false)
	for _, hook := range projectConfig {
		hookMap := hook.(map[string]any)
		cmd := hookMap["command"].(string)
		if filepath.IsAbs(cmd) {
			t.Errorf("expected relative path in project hook command, got %s", cmd)
		}
		// Relative paths should contain .claude/hooks but not start with /
		if !strings.Contains(cmd, ".claude/hooks/thts-") {
			t.Errorf("expected project hook command to contain '.claude/hooks/thts-', got %s", cmd)
		}
	}
}

func TestInstallGlobalHooks(t *testing.T) {
	// Create temp directory simulating global agent dir
	tmpDir := t.TempDir()

	cfg := agents.GetConfig(agents.AgentClaude)
	if cfg == nil {
		t.Fatal("expected non-nil config for Claude")
	}

	// Install global hooks
	files, err := installGlobalHooks(tmpDir, agents.AgentClaude, cfg)
	if err != nil {
		t.Fatalf("installGlobalHooks failed: %v", err)
	}

	// Should have installed hook files + settings.local.json
	if len(files) < 2 {
		t.Errorf("expected at least 2 installed files, got %d", len(files))
	}

	// Verify hook scripts exist and are executable
	sessionStartPath := filepath.Join(tmpDir, "hooks", "thts-session-start.sh")
	if !fsutil.Exists(sessionStartPath) {
		t.Error("expected thts-session-start.sh to exist")
	}
	info, err := os.Stat(sessionStartPath)
	if err != nil {
		t.Fatalf("failed to stat hook: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("expected hook to be executable")
	}

	// Verify settings.local.json was created with hooks
	settingsPath := filepath.Join(tmpDir, "settings.local.json")
	if !fsutil.Exists(settingsPath) {
		t.Error("expected settings.local.json to exist")
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	hooks, ok := settings["hooks"].([]any)
	if !ok {
		t.Fatal("expected settings to contain hooks array")
	}
	if len(hooks) != 2 {
		t.Errorf("expected 2 hooks in settings, got %d", len(hooks))
	}

	// Verify hooks have absolute paths
	for _, hook := range hooks {
		hookMap := hook.(map[string]any)
		cmd := hookMap["command"].(string)
		if !filepath.IsAbs(cmd) {
			t.Errorf("expected absolute path in global settings hook, got %s", cmd)
		}
	}
}

func TestParseGlobalComponents_IncludesHooks(t *testing.T) {
	// "all" should include hooks
	components, err := parseGlobalComponents("all")
	if err != nil {
		t.Fatalf("parseGlobalComponents failed: %v", err)
	}

	hasHooks := false
	for _, c := range components {
		if c == "hooks" {
			hasHooks = true
			break
		}
	}
	if !hasHooks {
		t.Error("expected 'all' to include 'hooks' component")
	}

	// Explicit hooks should work
	components, err = parseGlobalComponents("hooks")
	if err != nil {
		t.Fatalf("parseGlobalComponents failed: %v", err)
	}
	if len(components) != 1 || components[0] != "hooks" {
		t.Errorf("expected ['hooks'], got %v", components)
	}

	// Comma-separated with hooks should work
	components, err = parseGlobalComponents("skills,hooks")
	if err != nil {
		t.Fatalf("parseGlobalComponents failed: %v", err)
	}
	if len(components) != 2 {
		t.Errorf("expected 2 components, got %d", len(components))
	}
}
