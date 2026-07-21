package profile

import (
	"strings"
	"testing"
)

func TestCreateDefaultAgentsHelpListsAllAgents(t *testing.T) {
	help := createCmd.UsageString()

	for _, agent := range []string{"claude", "codex", "opencode", "gemini", "pi"} {
		if !strings.Contains(help, agent) {
			t.Errorf("--default-agents help = %q, missing %q", help, agent)
		}
	}
}
