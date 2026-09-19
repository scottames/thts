package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/scottames/thts/internal/agents"
	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/ui"
)

// Only the exact old generated file is safe to delete. Customized settings,
// JSONC, and symlinks remain user-owned, even if an old manifest claims them.
const legacyOpenCodeSettings = `{
  "model": "anthropic/claude-sonnet-4-20250514",
  "permissions": {
    "allow": []
  }
}
`

func isLegacyOpenCodeSettings(agentDir string, manifest *Manifest) (bool, error) {
	if !manifest.SettingsCreated && !slices.Contains(manifest.Files, "opencode.json") {
		return false, nil
	}
	path := filepath.Join(agentDir, "opencode.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, nil
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("expected regular OpenCode settings file: %s", path)
	}
	content, err := os.ReadFile(path)
	return string(content) == legacyOpenCodeSettings, err
}

func migrateOpenCodeSettings(agentDir string, manifest *Manifest) error {
	owned := manifest.SettingsCreated || slices.Contains(manifest.Files, "opencode.json")
	if !owned {
		return nil
	}
	remove, err := isLegacyOpenCodeSettings(agentDir, manifest)
	if err != nil {
		return err
	}
	path := filepath.Join(agentDir, "opencode.json")
	if remove {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Println(ui.Info("Removed obsolete thts-generated OpenCode settings"))
	} else if _, err := os.Lstat(path); err == nil {
		fmt.Println(ui.InfoF("Preserved user-owned settings: %s", path))
		content, _ := os.ReadFile(path)
		var settings struct {
			Permissions map[string]json.RawMessage `json:"permissions"`
		}
		if json.Unmarshal(content, &settings) == nil && settings.Permissions["allow"] != nil {
			fmt.Println(ui.Warning("Preserved obsolete permissions.allow object; migrate it using the OpenCode permissions guide."))
		}
	}
	manifest.SettingsCreated = false
	manifest.Files = removeStringValue(manifest.Files, "opencode.json")
	return nil
}

func reportOpenCodeRuntimeMode(mode config.ComponentMode) {
	switch mode {
	case config.ComponentModeGlobal:
		fmt.Println(ui.Info("OpenCode: using global plugin"))
	case config.ComponentModeDisabled:
		fmt.Println(ui.Info("OpenCode plugin disabled: automatic injection is unavailable. Enable hooks or select shared/on-demand integration."))
	}
}

func reconcileOpenCodeIntegration(projectDir, agentDir string, manifest *Manifest, level IntegrationLevel, mode config.ComponentMode) error {
	// Save after reconciliation, including partial progress on failure, so failed
	// cleanup never loses the remaining ownership record.
	err := reconcileOpenCodeFiles(projectDir, agentDir, manifest, level, mode)
	if saveErr := writeManifest(agentDir, manifest); saveErr != nil {
		return errors.Join(err, fmt.Errorf("save OpenCode migration: %w", saveErr))
	}
	return err
}

func reconcileOpenCodeFiles(projectDir, agentDir string, manifest *Manifest, level IntegrationLevel, mode config.ComponentMode) error {
	if err := migrateOpenCodeSettings(agentDir, manifest); err != nil {
		return err
	}
	keepPlugin := (level == IntegrationHook || level == IntegrationAgentsContentLocal) && mode == config.ComponentModeLocal
	const plugin = "plugins/thts-integration.ts"
	if !keepPlugin && slices.Contains(manifest.Files, filepath.FromSlash(plugin)) {
		if err := os.Remove(filepath.Join(agentDir, plugin)); err != nil && !os.IsNotExist(err) {
			return err
		}
		manifest.Files = removeStringValue(manifest.Files, filepath.FromSlash(plugin))
		cleanEmptyDirs(filepath.Join(agentDir, "plugins"))
	}
	if mod := manifest.Modifications.InstructionsMD; mod != nil && (level != IntegrationAgentsContent || mod.IntegrationType == "config") {
		if err := removeThtsIntegration(mod, agents.AgentOpenCode, projectDir, nil); err != nil && !os.IsNotExist(err) {
			return err
		}
		manifest.Modifications.InstructionsMD = nil
	}
	return migrateLegacyLocalInstructions(projectDir, agentDir, agents.GetConfig(agents.AgentOpenCode), manifest)
}
