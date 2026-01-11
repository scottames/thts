package agents

import (
	"os"
	"path/filepath"
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
	if !claude.SymlinkToAgents {
		t.Error("Claude should symlink CLAUDE.md to AGENTS.md")
	}
	if claude.SkillNeedsDir {
		t.Error("Claude skills should not require subdirectories")
	}
	if claude.InstructionsFile != "CLAUDE.md" {
		t.Errorf("Claude InstructionsFile = %q, want CLAUDE.md", claude.InstructionsFile)
	}

	// Codex-specific properties
	codex := GetConfig(AgentCodex)
	if codex.SupportsCommands {
		t.Error("Codex should not support commands")
	}
	if codex.SymlinkToAgents {
		t.Error("Codex should not symlink to AGENTS.md")
	}
	if !codex.SkillNeedsDir {
		t.Error("Codex skills should require subdirectories")
	}

	// OpenCode-specific properties
	opencode := GetConfig(AgentOpenCode)
	if opencode.SupportsCommands {
		t.Error("OpenCode should not support commands")
	}
	if opencode.SkillsDir != "skill" {
		t.Errorf("OpenCode SkillsDir = %q, want skill", opencode.SkillsDir)
	}
	if opencode.AgentsDir != "agent" {
		t.Errorf("OpenCode AgentsDir = %q, want agent", opencode.AgentsDir)
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
	if len(all) != 3 {
		t.Errorf("Expected 3 agent types, got %d", len(all))
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
}
