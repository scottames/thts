package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupTestXDG sets up a temporary XDG config directory and returns cleanup function
func setupTestXDG(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "tpd-config-test-*")
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
	t.Run("loads from tpd path", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create tpd config
		tpdDir := filepath.Join(xdgDir, "tpd")
		if err := os.MkdirAll(tpdDir, 0755); err != nil {
			t.Fatalf("failed to create tpd dir: %v", err)
		}

		cfg := &Config{
			ThoughtsRepo: "~/my-thoughts",
			ReposDir:     "repos",
			GlobalDir:    "global",
			User:         "testuser",
		}

		data, _ := json.Marshal(cfg)
		if err := os.WriteFile(filepath.Join(tpdDir, "config.json"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		if loaded.ThoughtsRepo != "~/my-thoughts" {
			t.Errorf("ThoughtsRepo = %q, want ~/my-thoughts", loaded.ThoughtsRepo)
		}
		if loaded.User != "testuser" {
			t.Errorf("User = %q, want testuser", loaded.User)
		}
	})

	t.Run("falls back to HumanLayer path", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create HumanLayer config (not tpd)
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

		if loaded.ThoughtsRepo != "~/hl-thoughts" {
			t.Errorf("ThoughtsRepo = %q, want ~/hl-thoughts", loaded.ThoughtsRepo)
		}
		if loaded.User != "hluser" {
			t.Errorf("User = %q, want hluser", loaded.User)
		}
	})

	t.Run("prefers tpd over HumanLayer", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		// Create both configs
		tpdDir := filepath.Join(xdgDir, "tpd")
		if err := os.MkdirAll(tpdDir, 0755); err != nil {
			t.Fatalf("failed to create tpd dir: %v", err)
		}

		hlDir := filepath.Join(xdgDir, "humanlayer")
		if err := os.MkdirAll(hlDir, 0755); err != nil {
			t.Fatalf("failed to create humanlayer dir: %v", err)
		}

		tpdCfg := &Config{ThoughtsRepo: "~/tpd-thoughts", User: "tpduser"}
		data, _ := json.Marshal(tpdCfg)
		if err := os.WriteFile(filepath.Join(tpdDir, "config.json"), data, 0644); err != nil {
			t.Fatalf("failed to write tpd config: %v", err)
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

		if loaded.ThoughtsRepo != "~/tpd-thoughts" {
			t.Error("should prefer tpd config over HumanLayer")
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

		// Create config with null maps
		tpdDir := filepath.Join(xdgDir, "tpd")
		if err := os.MkdirAll(tpdDir, 0755); err != nil {
			t.Fatalf("failed to create tpd dir: %v", err)
		}

		cfg := `{"thoughtsRepo": "~/thoughts", "user": "test"}`
		if err := os.WriteFile(filepath.Join(tpdDir, "config.json"), []byte(cfg), 0644); err != nil {
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
}

func TestSave(t *testing.T) {
	t.Run("creates config dir if missing", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		cfg := &Config{
			ThoughtsRepo:        "~/thoughts",
			ReposDir:            "repos",
			GlobalDir:           "global",
			User:                "testuser",
			AutoSyncInWorktrees: true,
			GitIgnore:           GitIgnoreProject,
			RepoMappings:        make(map[string]*RepoMapping),
			Profiles:            make(map[string]*ProfileConfig),
		}

		if err := Save(cfg); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		// Verify directory was created
		tpdDir := filepath.Join(xdgDir, "tpd")
		if _, err := os.Stat(tpdDir); err != nil {
			t.Error("tpd config directory should be created")
		}

		// Verify file exists
		configPath := filepath.Join(tpdDir, "config.json")
		if _, err := os.Stat(configPath); err != nil {
			t.Error("config file should be created")
		}
	})

	t.Run("writes valid JSON", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		cfg := &Config{
			ThoughtsRepo:        "~/thoughts",
			ReposDir:            "repos",
			GlobalDir:           "global",
			User:                "testuser",
			AutoSyncInWorktrees: true,
			GitIgnore:           GitIgnoreProject,
			RepoMappings: map[string]*RepoMapping{
				"/path/to/repo": {Repo: "myrepo", Profile: "work"},
			},
			Profiles: map[string]*ProfileConfig{
				"work": {ThoughtsRepo: "~/work-thoughts", ReposDir: "repos", GlobalDir: "global"},
			},
		}

		if err := Save(cfg); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		// Read and parse
		configPath := filepath.Join(xdgDir, "tpd", "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read saved config: %v", err)
		}

		var loaded Config
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("saved JSON is invalid: %v", err)
		}

		if loaded.ThoughtsRepo != "~/thoughts" {
			t.Errorf("ThoughtsRepo = %q, want ~/thoughts", loaded.ThoughtsRepo)
		}
		if loaded.User != "testuser" {
			t.Errorf("User = %q, want testuser", loaded.User)
		}
		if len(loaded.RepoMappings) != 1 {
			t.Errorf("RepoMappings length = %d, want 1", len(loaded.RepoMappings))
		}
		if len(loaded.Profiles) != 1 {
			t.Errorf("Profiles length = %d, want 1", len(loaded.Profiles))
		}
	})

	t.Run("overwrites existing config", func(t *testing.T) {
		_, cleanup := setupTestXDG(t)
		defer cleanup()

		// Save initial config
		cfg1 := &Config{
			ThoughtsRepo: "~/thoughts-v1",
			User:         "user1",
			RepoMappings: make(map[string]*RepoMapping),
			Profiles:     make(map[string]*ProfileConfig),
		}
		if err := Save(cfg1); err != nil {
			t.Fatalf("first Save() error: %v", err)
		}

		// Save updated config
		cfg2 := &Config{
			ThoughtsRepo: "~/thoughts-v2",
			User:         "user2",
			RepoMappings: make(map[string]*RepoMapping),
			Profiles:     make(map[string]*ProfileConfig),
		}
		if err := Save(cfg2); err != nil {
			t.Fatalf("second Save() error: %v", err)
		}

		// Verify updated config
		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		if loaded.ThoughtsRepo != "~/thoughts-v2" {
			t.Errorf("ThoughtsRepo = %q, want ~/thoughts-v2", loaded.ThoughtsRepo)
		}
		if loaded.User != "user2" {
			t.Errorf("User = %q, want user2", loaded.User)
		}
	})
}

func TestExists(t *testing.T) {
	t.Run("true with tpd config", func(t *testing.T) {
		xdgDir, cleanup := setupTestXDG(t)
		defer cleanup()

		tpdDir := filepath.Join(xdgDir, "tpd")
		if err := os.MkdirAll(tpdDir, 0755); err != nil {
			t.Fatalf("failed to create tpd dir: %v", err)
		}

		cfg := &Config{ThoughtsRepo: "~/thoughts"}
		data, _ := json.Marshal(cfg)
		if err := os.WriteFile(filepath.Join(tpdDir, "config.json"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		if !Exists() {
			t.Error("Exists() should return true with tpd config")
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

		tpdDir := filepath.Join(xdgDir, "tpd")
		if err := os.MkdirAll(tpdDir, 0755); err != nil {
			t.Fatalf("failed to create tpd dir: %v", err)
		}

		cfg := &Config{
			ThoughtsRepo: "~/custom-thoughts",
			User:         "customuser",
		}
		data, _ := json.Marshal(cfg)
		if err := os.WriteFile(filepath.Join(tpdDir, "config.json"), data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		loaded := LoadOrDefault()

		if loaded.ThoughtsRepo != "~/custom-thoughts" {
			t.Errorf("ThoughtsRepo = %q, want ~/custom-thoughts", loaded.ThoughtsRepo)
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
		if loaded.ThoughtsRepo != defaults.ThoughtsRepo {
			t.Errorf("ThoughtsRepo = %q, want %q", loaded.ThoughtsRepo, defaults.ThoughtsRepo)
		}
		if loaded.ReposDir != defaults.ReposDir {
			t.Errorf("ReposDir = %q, want %q", loaded.ReposDir, defaults.ReposDir)
		}
		if loaded.GlobalDir != defaults.GlobalDir {
			t.Errorf("GlobalDir = %q, want %q", loaded.GlobalDir, defaults.GlobalDir)
		}
	})
}
