package agents

import (
	"path/filepath"
	"slices"
	"testing"

	internalagents "github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
)

func TestGlobalManifestRecordAgentComponentMergesAndRefreshesAgentPaths(t *testing.T) {
	claudePath := filepath.Join(config.GlobalAgentDir("claude"), "skills", "old.md")
	openCodePath := filepath.Join(config.GlobalAgentDir("opencode"), "skills", "old", "SKILL.md")
	piOldPath := filepath.Join(config.GlobalAgentDir("pi"), "skills", "old", "SKILL.md")
	piPath := filepath.Join(config.GlobalAgentDir("pi"), "skills", "thts-integrate", "SKILL.md")

	manifest := &GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {
			Agents: []string{"claude", "opencode", "pi"},
			Files:  []string{claudePath, openCodePath, piOldPath},
		},
	}}

	manifest.RecordAgentComponent("skills", internalagents.AgentPi, []string{piPath, piPath})
	info := manifest.Components["skills"]
	if got, want := info.Agents, []string{"claude", "opencode", "pi"}; !slices.Equal(got, want) {
		t.Fatalf("agents = %v, want %v", got, want)
	}
	if slices.Contains(info.Files, piOldPath) {
		t.Errorf("stale Pi path remains: %v", info.Files)
	}
	if got, want := info.Files, []string{claudePath, openCodePath, piPath}; !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
}

func TestGlobalManifestFindsPiOwnership(t *testing.T) {
	manifest := &GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {
			Agents: []string{"pi"},
			Files:  []string{filepath.Join(config.GlobalAgentDir("pi"), "skills", "thts-integrate", "SKILL.md")},
		},
	}}
	if !manifest.HasAgentComponent("pi", "skills") {
		t.Fatal("Pi global ownership was not found")
	}
}

func TestGlobalManifestDoesNotProveOwnershipForEscapingPath(t *testing.T) {
	piRoot := config.GlobalAgentDir("pi")
	escapingPath := piRoot + string(filepath.Separator) + ".." + string(filepath.Separator) + ".." + string(filepath.Separator) + "outside"
	manifest := &GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"skills": {
			Agents: []string{"pi"},
			Files:  []string{escapingPath},
		},
	}}

	if manifest.HasAgentComponent("pi", "skills") {
		t.Fatalf("escaping manifest path proved Pi ownership: %q", escapingPath)
	}
}

func TestGlobalManifestFindsPiOwnershipAtEffectiveRoot(t *testing.T) {
	piRoot := filepath.Join(t.TempDir(), "pi")
	t.Setenv("PI_CODING_AGENT_DIR", piRoot)
	path := filepath.Join(config.GlobalAgentDir("pi"), "extensions", "thts-integration.ts")
	manifest := &GlobalManifest{Components: map[string]*GlobalComponentInfo{
		"hooks": {Agents: []string{"pi"}, Files: []string{path}},
	}}

	if !filepath.IsAbs(path) {
		t.Fatalf("Pi effective-root path = %q, want absolute", path)
	}
	if !manifest.HasAgentComponent("pi", "hooks") {
		t.Fatal("Pi extension under its effective root did not prove ownership")
	}
}
