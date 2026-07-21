package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAgentType(t *testing.T) {
	tests := []struct {
		input   string
		want    AgentType
		wantErr bool
	}{
		{"claude", AgentClaude, false},
		{"CLAUDE", AgentClaude, false},
		{"  Claude  ", AgentClaude, false},
		{"codex", AgentCodex, false},
		{"CODEX", AgentCodex, false},
		{"opencode", AgentOpenCode, false},
		{"OpenCode", AgentOpenCode, false},
		{"gemini", AgentGemini, false},
		{"GEMINI", AgentGemini, false},
		{"pi", AgentPi, false},
		{"PI", AgentPi, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseAgentType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAgentType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseAgentType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseAgentTypeUnknownListsAllAgents(t *testing.T) {
	_, err := ParseAgentType("invalid")
	if err == nil {
		t.Fatal("ParseAgentType(invalid) returned nil error")
	}

	for _, name := range []string{"claude", "codex", "opencode", "gemini", "pi"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("ParseAgentType(invalid) error = %q, missing %q", err, name)
		}
	}
}

func TestParseAgentTypes(t *testing.T) {
	tests := []struct {
		input   string
		want    []AgentType
		wantErr bool
	}{
		{"claude", []AgentType{AgentClaude}, false},
		{"claude,codex", []AgentType{AgentClaude, AgentCodex}, false},
		{"claude,codex,opencode", []AgentType{AgentClaude, AgentCodex, AgentOpenCode}, false},
		{"claude,claude", []AgentType{AgentClaude}, false}, // deduplication
		{"", nil, false},
		{"  ", nil, false},
		{"invalid", nil, true},
		{"claude,invalid", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseAgentTypes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAgentTypes(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ParseAgentTypes(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseAgentTypes(%q)[%d] = %v, want %v", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	tests := []struct {
		agentType AgentType
		wantDir   string
	}{
		{AgentClaude, ".claude"},
		{AgentCodex, ".codex"},
		{AgentOpenCode, ".opencode"},
		{AgentGemini, ".gemini"},
	}

	for _, tt := range tests {
		t.Run(string(tt.agentType), func(t *testing.T) {
			config := GetConfig(tt.agentType)
			if config == nil {
				t.Fatalf("GetConfig(%q) returned nil", tt.agentType)
			}
			if config.RootDir != tt.wantDir {
				t.Errorf("GetConfig(%q).RootDir = %q, want %q", tt.agentType, config.RootDir, tt.wantDir)
			}
		})
	}
}

func TestAgentConfigProperties(t *testing.T) {
	// Claude-specific properties
	claude := GetConfig(AgentClaude)
	if !claude.SupportsCommands {
		t.Error("Claude should support commands")
	}
	if claude.IntegrationType != "marker" {
		t.Errorf("Claude IntegrationType = %q, want marker", claude.IntegrationType)
	}
	if claude.InstructionTargetFile != "CLAUDE.md" {
		t.Errorf("Claude InstructionTargetFile = %q, want CLAUDE.md", claude.InstructionTargetFile)
	}
	if claude.SkillNeedsDir {
		t.Error("Claude skills should not require subdirectories")
	}
	if claude.InstructionsFile != "" {
		t.Errorf("Claude InstructionsFile = %q, want empty (uses hooks for dynamic injection)", claude.InstructionsFile)
	}
	if claude.CommandsDir != "commands" {
		t.Errorf("Claude CommandsDir = %q, want commands", claude.CommandsDir)
	}
	if claude.CommandsGlobalOnly {
		t.Error("Claude commands should not be global-only")
	}
	if claude.GlobalUsesXDG {
		t.Error("Claude should not use XDG for global config")
	}

	// Codex-specific properties
	codex := GetConfig(AgentCodex)
	if !codex.SupportsCommands {
		t.Error("Codex should support commands (prompts)")
	}
	if codex.CommandsDir != "prompts" {
		t.Errorf("Codex CommandsDir = %q, want prompts", codex.CommandsDir)
	}
	if !codex.CommandsGlobalOnly {
		t.Error("Codex commands should be global-only")
	}
	if codex.GlobalUsesXDG {
		t.Error("Codex should not use XDG for global config")
	}
	if codex.IntegrationType != "marker" {
		t.Errorf("Codex IntegrationType = %q, want marker", codex.IntegrationType)
	}
	if codex.InstructionTargetFile != "AGENTS.md" {
		t.Errorf("Codex InstructionTargetFile = %q, want AGENTS.md", codex.InstructionTargetFile)
	}
	if codex.InstructionsFile != "" {
		t.Errorf("Codex InstructionsFile = %q, want empty (inline in AGENTS.md)", codex.InstructionsFile)
	}
	if !codex.SkillNeedsDir {
		t.Error("Codex skills should require subdirectories")
	}

	// OpenCode-specific properties
	opencode := GetConfig(AgentOpenCode)
	if !opencode.SupportsCommands {
		t.Error("OpenCode should support commands")
	}
	if opencode.CommandsDir != "commands" {
		t.Errorf("OpenCode CommandsDir = %q, want commands", opencode.CommandsDir)
	}
	if opencode.CommandsGlobalOnly {
		t.Error("OpenCode commands should not be global-only")
	}
	if !opencode.GlobalUsesXDG {
		t.Error("OpenCode should use XDG for global config")
	}
	if opencode.IntegrationType != "marker" {
		t.Errorf("OpenCode IntegrationType = %q, want marker", opencode.IntegrationType)
	}
	if opencode.InstructionTargetFile != "AGENTS.md" {
		t.Errorf("OpenCode InstructionTargetFile = %q, want AGENTS.md", opencode.InstructionTargetFile)
	}
	if opencode.InstructionsFile != "" {
		t.Errorf("OpenCode InstructionsFile = %q, want empty (uses plugin for dynamic injection)", opencode.InstructionsFile)
	}
	if opencode.SkillsDir != "skills" {
		t.Errorf("OpenCode SkillsDir = %q, want skills", opencode.SkillsDir)
	}
	if opencode.AgentsDir != "agents" {
		t.Errorf("OpenCode AgentsDir = %q, want agents", opencode.AgentsDir)
	}

	// Gemini-specific properties
	gemini := GetConfig(AgentGemini)
	if !gemini.SupportsCommands {
		t.Error("Gemini should support commands")
	}
	if gemini.CommandsDir != "commands" {
		t.Errorf("Gemini CommandsDir = %q, want commands", gemini.CommandsDir)
	}
	if gemini.CommandsFormat != "toml" {
		t.Errorf("Gemini CommandsFormat = %q, want toml", gemini.CommandsFormat)
	}
	if gemini.AgentsDir != "" {
		t.Errorf("Gemini AgentsDir = %q, want empty (no agents support)", gemini.AgentsDir)
	}
	if gemini.IntegrationType != "marker" {
		t.Errorf("Gemini IntegrationType = %q, want marker", gemini.IntegrationType)
	}
	if gemini.InstructionTargetFile != "AGENTS.md" {
		t.Errorf("Gemini InstructionTargetFile = %q, want AGENTS.md", gemini.InstructionTargetFile)
	}
	if gemini.SettingsContextKey != "contextFileName" {
		t.Errorf("Gemini SettingsContextKey = %q, want contextFileName", gemini.SettingsContextKey)
	}
	if !gemini.SkillNeedsDir {
		t.Error("Gemini skills should require subdirectories")
	}
	if gemini.GlobalUsesXDG {
		t.Error("Gemini should not use XDG for global config")
	}
	if gemini.CommandsGlobalOnly {
		t.Error("Gemini commands should not be global-only")
	}
}

func TestDetectExistingAgents(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "thts-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// No agents initially
	found := DetectExistingAgents(tmpDir)
	if len(found) != 0 {
		t.Errorf("Expected no agents, got %v", found)
	}

	// Create .claude directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}
	found = DetectExistingAgents(tmpDir)
	if len(found) != 1 || found[0] != AgentClaude {
		t.Errorf("Expected [claude], got %v", found)
	}

	// Create .codex directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".codex"), 0755); err != nil {
		t.Fatalf("Failed to create .codex dir: %v", err)
	}
	found = DetectExistingAgents(tmpDir)
	if len(found) != 2 {
		t.Errorf("Expected 2 agents, got %v", found)
	}
}

func TestDetectExistingPi(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Instructions\n"), 0644); err != nil {
		t.Fatalf("failed to create AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "pi"), []byte(""), 0755); err != nil {
		t.Fatalf("failed to create executable: %v", err)
	}

	if found := DetectExistingAgents(tmpDir); len(found) != 0 {
		t.Errorf("DetectExistingAgents() = %v with AGENTS.md and executable only, want none", found)
	}

	if err := os.Mkdir(filepath.Join(tmpDir, ".pi"), 0755); err != nil {
		t.Fatalf("failed to create .pi directory: %v", err)
	}
	if found := DetectExistingAgents(tmpDir); len(found) != 1 || found[0] != AgentPi {
		t.Errorf("DetectExistingAgents() = %v, want [pi]", found)
	}
}

func TestSortAgentTypes(t *testing.T) {
	agents := []AgentType{AgentOpenCode, AgentClaude, AgentCodex}
	SortAgentTypes(agents)

	expected := []AgentType{AgentClaude, AgentCodex, AgentOpenCode}
	for i, a := range agents {
		if a != expected[i] {
			t.Errorf("SortAgentTypes position %d: got %v, want %v", i, a, expected[i])
		}
	}
}

func TestAgentTypesToStrings(t *testing.T) {
	agents := []AgentType{AgentClaude, AgentCodex}
	strings := AgentTypesToStrings(agents)

	if len(strings) != 2 {
		t.Fatalf("Expected 2 strings, got %d", len(strings))
	}
	if strings[0] != "claude" {
		t.Errorf("strings[0] = %q, want claude", strings[0])
	}
	if strings[1] != "codex" {
		t.Errorf("strings[1] = %q, want codex", strings[1])
	}
}

func TestStringsToAgentTypes(t *testing.T) {
	strs := []string{"claude", "opencode"}
	agents, err := StringsToAgentTypes(strs)
	if err != nil {
		t.Fatalf("StringsToAgentTypes failed: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("Expected 2 agents, got %d", len(agents))
	}
	if agents[0] != AgentClaude {
		t.Errorf("agents[0] = %v, want claude", agents[0])
	}
	if agents[1] != AgentOpenCode {
		t.Errorf("agents[1] = %v, want opencode", agents[1])
	}

	// Test error case
	_, err = StringsToAgentTypes([]string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid agent type")
	}
}

func TestAllAgentTypes(t *testing.T) {
	all := AllAgentTypes()
	if len(all) != 5 {
		t.Errorf("Expected 5 agent types, got %d", len(all))
	}
	seen := make(map[AgentType]bool)
	for _, agentType := range all {
		if seen[agentType] {
			t.Errorf("AllAgentTypes() contains duplicate %q", agentType)
		}
		seen[agentType] = true
	}
	// Verify canonical order
	if all[0] != AgentClaude {
		t.Error("First agent should be claude")
	}
	if all[1] != AgentCodex {
		t.Error("Second agent should be codex")
	}
	if all[2] != AgentOpenCode {
		t.Error("Third agent should be opencode")
	}
	if all[3] != AgentGemini {
		t.Error("Fourth agent should be gemini")
	}
	if all[4] != AgentPi {
		t.Error("Fifth agent should be pi")
	}
}

func TestCommandsDirLabel(t *testing.T) {
	tests := []struct {
		agentType AgentType
		want      string
	}{
		{AgentClaude, "commands"},
		{AgentCodex, "prompts"},
		{AgentOpenCode, "commands"},
		{AgentGemini, "commands"},
		{AgentPi, "prompts"},
	}

	for _, tt := range tests {
		t.Run(string(tt.agentType), func(t *testing.T) {
			got := CommandsDirLabel(tt.agentType)
			if got != tt.want {
				t.Errorf("CommandsDirLabel(%q) = %q, want %q", tt.agentType, got, tt.want)
			}
		})
	}
}

// TestAgentCompleteness verifies all registered agents have required configuration.
// This test catches incomplete agent additions - if you add a new agent to AllAgentTypes(),
// this test will fail until you also add the config, label, and parser case.
func TestAgentCompleteness(t *testing.T) {
	for _, agentType := range AllAgentTypes() {
		t.Run(string(agentType), func(t *testing.T) {
			// Must have config
			config := GetConfig(agentType)
			if config == nil {
				t.Errorf("Agent %q registered in AllAgentTypes() but missing from AgentConfigs", agentType)
				return
			}

			// Must have label
			if AgentTypeLabels[agentType] == "" {
				t.Errorf("Agent %q missing from AgentTypeLabels", agentType)
			}

			// Must be parseable
			parsed, err := ParseAgentType(string(agentType))
			if err != nil {
				t.Errorf("Agent %q not handled in ParseAgentType: %v", agentType, err)
			}
			if parsed != agentType {
				t.Errorf("ParseAgentType(%q) = %q, want %q", agentType, parsed, agentType)
			}

			// Config must have required fields
			if config.RootDir == "" {
				t.Errorf("Agent %q config missing RootDir", agentType)
			}
			if config.SkillsDir == "" {
				t.Errorf("Agent %q config missing SkillsDir", agentType)
			}
			// AgentsDir can be empty for agents that don't support the agents feature (e.g., Gemini)
			if config.IntegrationType == "" {
				t.Errorf("Agent %q config missing IntegrationType", agentType)
			}
			if config.SettingsFile == "" {
				t.Errorf("Agent %q config missing SettingsFile", agentType)
			}
			if config.SettingsFormat == "" {
				t.Errorf("Agent %q config missing SettingsFormat", agentType)
			}
		})
	}
}

func TestPiNativeCapabilities(t *testing.T) {
	pi := GetConfig(AgentPi)
	if pi == nil {
		t.Fatal("GetConfig(pi) returned nil")
	}
	if pi.RootDir != ".pi" {
		t.Errorf("Pi RootDir = %q, want .pi", pi.RootDir)
	}
	if !pi.SkillNeedsDir {
		t.Error("Pi skills should require subdirectories")
	}
	if pi.CommandsDir != "prompts" {
		t.Errorf("Pi CommandsDir = %q, want prompts", pi.CommandsDir)
	}
	if pi.AgentsDir != "" {
		t.Errorf("Pi AgentsDir = %q, want empty", pi.AgentsDir)
	}
	if pi.SettingsFile != "settings.json" || pi.SettingsFormat != "json" {
		t.Errorf("Pi settings destination = %s (%s), want settings.json (json)", pi.SettingsFile, pi.SettingsFormat)
	}
	if pi.SettingsTemplate != "" {
		t.Errorf("Pi settings template = %q, want empty", pi.SettingsTemplate)
	}
	if !pi.SupportsHooks || pi.PluginsDir != "extensions" || pi.HooksDir != "" {
		t.Errorf("Pi hooks/plugins = supports %t, hooks %q, plugins %q; want true, empty, extensions", pi.SupportsHooks, pi.HooksDir, pi.PluginsDir)
	}
	if pi.InstructionTargetFile != "AGENTS.md" {
		t.Errorf("Pi InstructionTargetFile = %q, want AGENTS.md", pi.InstructionTargetFile)
	}

	data := GetEmbedTemplateData(AgentPi)
	if data.HasTaskList || data.TaskTracking != "" || data.HasSpawnTasks || data.HasAgentsFeature {
		t.Errorf("Pi template capabilities = %+v, want no task tracking, task spawning, or agents metadata", data)
	}
}
