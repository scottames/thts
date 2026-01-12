package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// setupTestXDG sets up a temporary XDG config directory and returns cleanup function
func setupTestXDG(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "thts-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		t.Fatalf("failed to set XDG_CONFIG_HOME: %v", err)
	}

	return dir, func() {
		if originalXDG != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", originalXDG)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
		_ = os.RemoveAll(dir)
	}
}

func TestLoad(t *testing.T) {
	t.Run("loads from thts path", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create thts config directory
		thtsDir := filepath.Join(xdgDir, "thts")
		if err := os.MkdirAll(thtsDir, 0755); err != nil {
			t.Fatalf("failed to create thts dir: %v", err)
		}

		cfg := &Config{
			User: "testuser",
			Profiles: map[string]*ProfileConfig{
				"personal": {
					ThoughtsRepo: "~/my-thoughts",
					ReposDir:     "repos",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		data, _ := yaml.Marshal(cfg)
		if err := os.WriteFile(filepath.Join(thtsDir, "config.yaml"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		profile, name := loaded.GetDefaultProfile()
		if profile == nil {
			t.Fatal("expected default profile to exist")
		}
		if name != "personal" {
			t.Errorf("profile name = %q, want personal", name)
		}
		if profile.ThoughtsRepo != "~/my-thoughts" {
			t.Errorf("ThoughtsRepo = %q, want ~/my-thoughts", profile.ThoughtsRepo)
		}
		if loaded.User != "testuser" {
			t.Errorf("User = %q, want testuser", loaded.User)
		}
	})

	t.Run("falls back to HumanLayer path", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create HumanLayer config (not thts)
		hlDir := filepath.Join(xdgDir, "humanlayer")
		if err := os.MkdirAll(hlDir, 0755); err != nil {
			t.Fatalf("failed to create humanlayer dir: %v", err)
		}

		hlConfig := map[string]interface{}{
			"thoughts": map[string]interface{}{
				"thoughtsRepo": "~/hl-thoughts",
				"reposDir":     "repos",
				"globalDir":    "global",
				"user":         "hluser",
			},
		}

		data, _ := json.Marshal(hlConfig)
		if err := os.WriteFile(filepath.Join(hlDir, "humanlayer.json"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		// HumanLayer config gets translated to a "default" profile
		profile, name := loaded.GetDefaultProfile()
		if profile == nil {
			t.Fatal("expected default profile to exist")
		}
		if name != "default" {
			t.Errorf("profile name = %q, want default", name)
		}
		if profile.ThoughtsRepo != "~/hl-thoughts" {
			t.Errorf("ThoughtsRepo = %q, want ~/hl-thoughts", profile.ThoughtsRepo)
		}
		if loaded.User != "hluser" {
			t.Errorf("User = %q, want hluser", loaded.User)
		}
	})

	t.Run("prefers thts over HumanLayer", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create both configs
		thtsDir := filepath.Join(xdgDir, "thts")
		if err := os.MkdirAll(thtsDir, 0755); err != nil {
			t.Fatalf("failed to create thts dir: %v", err)
		}

		hlDir := filepath.Join(xdgDir, "humanlayer")
		if err := os.MkdirAll(hlDir, 0755); err != nil {
			t.Fatalf("failed to create humanlayer dir: %v", err)
		}

		thtsCfg := &Config{
			User: "thtsuser",
			Profiles: map[string]*ProfileConfig{
				"personal": {
					ThoughtsRepo: "~/thts-thoughts",
					ReposDir:     "repos",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}
		data, _ := yaml.Marshal(thtsCfg)
		if err := os.WriteFile(filepath.Join(thtsDir, "config.yaml"), data, 0644); err != nil {
			t.Fatalf("failed to write thts config: %v", err)
		}

		hlConfig := map[string]interface{}{
			"thoughts": map[string]interface{}{
				"thoughtsRepo": "~/hl-thoughts",
				"user":         "hluser",
			},
		}
		data, _ = json.Marshal(hlConfig)
		if err := os.WriteFile(filepath.Join(hlDir, "humanlayer.json"), data, 0644); err != nil {
			t.Fatalf("failed to write hl config: %v", err)
		}

		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		profile, _ := loaded.GetDefaultProfile()
		if profile.ThoughtsRepo != "~/thts-thoughts" {
			t.Error("should prefer thts config over HumanLayer")
		}
	})

	t.Run("returns error if neither exists", func(t *testing.T) {
		_, cleanup := setupTestXDG(t)
		defer cleanup()

		_, err := Load()
		if err != ErrConfigNotFound {
			t.Errorf("Load() error = %v, want ErrConfigNotFound", err)
		}
	})

	t.Run("returns error for HumanLayer without thoughts key", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create HumanLayer config without thoughts key
		hlDir := filepath.Join(xdgDir, "humanlayer")
		if err := os.MkdirAll(hlDir, 0755); err != nil {
			t.Fatalf("failed to create humanlayer dir: %v", err)
		}

		hlConfig := map[string]interface{}{
			"someOtherKey": "value",
		}

		data, _ := json.Marshal(hlConfig)
		if err := os.WriteFile(filepath.Join(hlDir, "humanlayer.json"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := Load()
		if err != ErrConfigNotFound {
			t.Errorf("Load() error = %v, want ErrConfigNotFound", err)
		}
	})

	t.Run("initializes nil maps", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create config with minimal fields
		thtsDir := filepath.Join(xdgDir, "thts")
		if err := os.MkdirAll(thtsDir, 0755); err != nil {
			t.Fatalf("failed to create thts dir: %v", err)
		}

		cfg := `user: test
profiles:
  default:
    thoughtsRepo: ~/thoughts
    default: true
`
		if err := os.WriteFile(filepath.Join(thtsDir, "config.yaml"), []byte(cfg), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		if loaded.RepoMappings == nil {
			t.Error("RepoMappings should be initialized")
		}
		if loaded.Profiles == nil {
			t.Error("Profiles should be initialized")
		}
	})

	t.Run("translates humanlayer profiles", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create HumanLayer config with profiles
		hlDir := filepath.Join(xdgDir, "humanlayer")
		if err := os.MkdirAll(hlDir, 0755); err != nil {
			t.Fatalf("failed to create humanlayer dir: %v", err)
		}

		hlConfig := map[string]interface{}{
			"thoughts": map[string]interface{}{
				"thoughtsRepo": "~/thoughts",
				"reposDir":     "repos",
				"globalDir":    "global",
				"user":         "testuser",
				"profiles": map[string]interface{}{
					"work": map[string]interface{}{
						"thoughtsRepo": "~/work-thoughts",
						"reposDir":     "work-repos",
						"globalDir":    "work-global",
					},
				},
			},
		}

		data, _ := json.Marshal(hlConfig)
		if err := os.WriteFile(filepath.Join(hlDir, "humanlayer.json"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		// Should have default profile from top-level fields
		defaultProfile, name := loaded.GetDefaultProfile()
		if defaultProfile == nil {
			t.Fatal("expected default profile to exist")
		}
		if name != "default" {
			t.Errorf("default profile name = %q, want default", name)
		}
		if defaultProfile.ThoughtsRepo != "~/thoughts" {
			t.Errorf("default ThoughtsRepo = %q, want ~/thoughts", defaultProfile.ThoughtsRepo)
		}

		// Should have work profile
		workProfile, exists := loaded.Profiles["work"]
		if !exists {
			t.Fatal("expected work profile to exist")
		}
		if workProfile.ThoughtsRepo != "~/work-thoughts" {
			t.Errorf("work ThoughtsRepo = %q, want ~/work-thoughts", workProfile.ThoughtsRepo)
		}
	})
}

func TestSave(t *testing.T) {
	t.Run("creates config dir if missing", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		cfg := &Config{
			User:                "testuser",
			AutoSyncInWorktrees: true,
			Gitignore:           ComponentModeLocal,
			RepoMappings:        make(map[string]*RepoMapping),
			Profiles: map[string]*ProfileConfig{
				"personal": {
					ThoughtsRepo: "~/thoughts",
					ReposDir:     "repos",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}

		if err := Save(cfg); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		// Verify directory was created
		thtsDir := filepath.Join(xdgDir, "thts")
		if _, err := os.Stat(thtsDir); err != nil {
			t.Error("thts config directory should be created")
		}

		// Verify file exists
		configPath := filepath.Join(thtsDir, "config.yaml")
		if _, err := os.Stat(configPath); err != nil {
			t.Error("config file should be created")
		}
	})

	t.Run("writes valid YAML", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		cfg := &Config{
			User:                "testuser",
			AutoSyncInWorktrees: true,
			Gitignore:           ComponentModeLocal,
			RepoMappings: map[string]*RepoMapping{
				"/path/to/repo": {Repo: "myrepo", Profile: "work"},
			},
			Profiles: map[string]*ProfileConfig{
				"personal": {
					ThoughtsRepo: "~/thoughts",
					ReposDir:     "repos",
					GlobalDir:    "global",
					Default:      true,
				},
				"work": {
					ThoughtsRepo: "~/work-thoughts",
					ReposDir:     "repos",
					GlobalDir:    "global",
				},
			},
		}

		if err := Save(cfg); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		// Read and parse
		configPath := filepath.Join(xdgDir, "thts", "config.yaml")
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read saved config: %v", err)
		}

		var loaded Config
		if err := yaml.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("saved YAML is invalid: %v", err)
		}

		if loaded.User != "testuser" {
			t.Errorf("User = %q, want testuser", loaded.User)
		}
		if len(loaded.RepoMappings) != 1 {
			t.Errorf("RepoMappings length = %d, want 1", len(loaded.RepoMappings))
		}
		if len(loaded.Profiles) != 2 {
			t.Errorf("Profiles length = %d, want 2", len(loaded.Profiles))
		}

		profile, name := loaded.GetDefaultProfile()
		if profile == nil {
			t.Fatal("expected default profile to exist")
		}
		if name != "personal" {
			t.Errorf("default profile name = %q, want personal", name)
		}
		if profile.ThoughtsRepo != "~/thoughts" {
			t.Errorf("ThoughtsRepo = %q, want ~/thoughts", profile.ThoughtsRepo)
		}
	})

	t.Run("overwrites existing config", func(t *testing.T) {
		_, cleanup := setupTestXDG(t)
		defer cleanup()

		// Save initial config
		cfg1 := &Config{
			User:         "user1",
			RepoMappings: make(map[string]*RepoMapping),
			Profiles: map[string]*ProfileConfig{
				"personal": {
					ThoughtsRepo: "~/thoughts-v1",
					ReposDir:     "repos",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}
		if err := Save(cfg1); err != nil {
			t.Fatalf("first Save() error: %v", err)
		}

		// Save updated config
		cfg2 := &Config{
			User:         "user2",
			RepoMappings: make(map[string]*RepoMapping),
			Profiles: map[string]*ProfileConfig{
				"personal": {
					ThoughtsRepo: "~/thoughts-v2",
					ReposDir:     "repos",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}
		if err := Save(cfg2); err != nil {
			t.Fatalf("second Save() error: %v", err)
		}

		// Verify updated config
		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		profile, _ := loaded.GetDefaultProfile()
		if profile.ThoughtsRepo != "~/thoughts-v2" {
			t.Errorf("ThoughtsRepo = %q, want ~/thoughts-v2", profile.ThoughtsRepo)
		}
		if loaded.User != "user2" {
			t.Errorf("User = %q, want user2", loaded.User)
		}
	})
}

func TestExists(t *testing.T) {
	t.Run("true with thts config", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		thtsDir := filepath.Join(xdgDir, "thts")
		if err := os.MkdirAll(thtsDir, 0755); err != nil {
			t.Fatalf("failed to create thts dir: %v", err)
		}

		cfg := &Config{
			Profiles: map[string]*ProfileConfig{
				"personal": {ThoughtsRepo: "~/thoughts", Default: true},
			},
		}
		data, _ := yaml.Marshal(cfg)
		if err := os.WriteFile(filepath.Join(thtsDir, "config.yaml"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		if !Exists() {
			t.Error("Exists() should return true with thts config")
		}
	})

	t.Run("true with HumanLayer config", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		hlDir := filepath.Join(xdgDir, "humanlayer")
		if err := os.MkdirAll(hlDir, 0755); err != nil {
			t.Fatalf("failed to create humanlayer dir: %v", err)
		}

		hlConfig := map[string]interface{}{
			"thoughts": map[string]interface{}{
				"thoughtsRepo": "~/thoughts",
			},
		}
		data, _ := json.Marshal(hlConfig)
		if err := os.WriteFile(filepath.Join(hlDir, "humanlayer.json"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		if !Exists() {
			t.Error("Exists() should return true with HumanLayer config")
		}
	})

	t.Run("false without thoughts key in HumanLayer", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		hlDir := filepath.Join(xdgDir, "humanlayer")
		if err := os.MkdirAll(hlDir, 0755); err != nil {
			t.Fatalf("failed to create humanlayer dir: %v", err)
		}

		hlConfig := map[string]interface{}{
			"someOtherKey": "value",
		}
		data, _ := json.Marshal(hlConfig)
		if err := os.WriteFile(filepath.Join(hlDir, "humanlayer.json"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		if Exists() {
			t.Error("Exists() should return false without thoughts key")
		}
	})

	t.Run("false with neither", func(t *testing.T) {
		_, cleanup := setupTestXDG(t)
		defer cleanup()

		if Exists() {
			t.Error("Exists() should return false with no config")
		}
	})
}

func TestLoadOrDefault(t *testing.T) {
	t.Run("returns loaded config when exists", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		thtsDir := filepath.Join(xdgDir, "thts")
		if err := os.MkdirAll(thtsDir, 0755); err != nil {
			t.Fatalf("failed to create thts dir: %v", err)
		}

		cfg := &Config{
			User: "customuser",
			Profiles: map[string]*ProfileConfig{
				"personal": {
					ThoughtsRepo: "~/custom-thoughts",
					ReposDir:     "repos",
					GlobalDir:    "global",
					Default:      true,
				},
			},
		}
		data, _ := yaml.Marshal(cfg)
		if err := os.WriteFile(filepath.Join(thtsDir, "config.yaml"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		loaded := LoadOrDefault()

		profile, _ := loaded.GetDefaultProfile()
		if profile.ThoughtsRepo != "~/custom-thoughts" {
			t.Errorf("ThoughtsRepo = %q, want ~/custom-thoughts", profile.ThoughtsRepo)
		}
		if loaded.User != "customuser" {
			t.Errorf("User = %q, want customuser", loaded.User)
		}
	})

	t.Run("returns defaults when config not found", func(t *testing.T) {
		_, cleanup := setupTestXDG(t)
		defer cleanup()

		// No config files created, should return defaults
		loaded := LoadOrDefault()

		defaults := Defaults()
		defaultProfile, _ := defaults.GetDefaultProfile()
		loadedProfile, _ := loaded.GetDefaultProfile()

		if loadedProfile.ThoughtsRepo != defaultProfile.ThoughtsRepo {
			t.Errorf("ThoughtsRepo = %q, want %q", loadedProfile.ThoughtsRepo, defaultProfile.ThoughtsRepo)
		}
		if loadedProfile.ReposDir != defaultProfile.ReposDir {
			t.Errorf("ReposDir = %q, want %q", loadedProfile.ReposDir, defaultProfile.ReposDir)
		}
		if loadedProfile.GlobalDir != defaultProfile.GlobalDir {
			t.Errorf("GlobalDir = %q, want %q", loadedProfile.GlobalDir, defaultProfile.GlobalDir)
		}
	})
}
