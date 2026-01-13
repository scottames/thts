package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// ErrConfigNotFound is returned when no config file exists.
var ErrConfigNotFound = errors.New("config not found")

// humanLayerThoughtsConfig represents HumanLayer's thoughts config structure.
// It has top-level ThoughtsRepo/ReposDir/GlobalDir which we translate to profiles.
type humanLayerThoughtsConfig struct {
	ThoughtsRepo string                    `json:"thoughtsRepo"`
	ReposDir     string                    `json:"reposDir"`
	GlobalDir    string                    `json:"globalDir"`
	User         string                    `json:"user"`
	RepoMappings map[string]*RepoMapping   `json:"repoMappings,omitempty"`
	Profiles     map[string]*ProfileConfig `json:"profiles,omitempty"`
}

// humanLayerConfigFile represents the HumanLayer config file structure.
type humanLayerConfigFile struct {
	Thoughts *humanLayerThoughtsConfig `json:"thoughts,omitempty"`
}

// Load loads the thts configuration.
// It first tries to load from ~/.config/thts/config.yaml,
// then falls back to ~/.config/humanlayer/humanlayer.json (translated to profiles).
func Load() (*Config, error) {
	// Try thts config first
	thtsPath := ThtsConfigPath()
	if cfg, err := loadFromPath(thtsPath); err == nil {
		return cfg, nil
	}

	// Fall back to HumanLayer config (translate their format to ours)
	hlPath := HumanLayerConfigPath()
	if cfg, err := loadFromHumanLayerPath(hlPath); err == nil {
		return cfg, nil
	}

	return nil, ErrConfigNotFound
}

// loadFromPath loads config directly from a path (thts YAML format).
func loadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Initialize maps if nil
	if cfg.RepoMappings == nil {
		cfg.RepoMappings = make(map[string]*RepoMapping)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]*ProfileConfig)
	}

	return &cfg, nil
}

// loadFromHumanLayerPath loads config from HumanLayer's config file.
// It translates their top-level config format to our profiles-only format.
func loadFromHumanLayerPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var hlCfg humanLayerConfigFile
	if err := json.Unmarshal(data, &hlCfg); err != nil {
		return nil, fmt.Errorf("failed to parse HumanLayer config: %w", err)
	}

	if hlCfg.Thoughts == nil {
		return nil, ErrConfigNotFound
	}

	hl := hlCfg.Thoughts

	// Create our config with profiles-only structure
	cfg := &Config{
		User:         hl.User,
		RepoMappings: hl.RepoMappings,
		Profiles:     make(map[string]*ProfileConfig),
	}

	// Initialize maps if nil
	if cfg.RepoMappings == nil {
		cfg.RepoMappings = make(map[string]*RepoMapping)
	}

	// Translate top-level config to a "default" profile
	if hl.ThoughtsRepo != "" {
		cfg.Profiles["default"] = &ProfileConfig{
			ThoughtsRepo: hl.ThoughtsRepo,
			ReposDir:     hl.ReposDir,
			GlobalDir:    hl.GlobalDir,
			Default:      true,
		}
	}

	// Copy any existing profiles (they won't have Default set)
	for name, profile := range hl.Profiles {
		if _, exists := cfg.Profiles[name]; !exists {
			cfg.Profiles[name] = profile
		}
	}

	return cfg, nil
}

// Save saves the configuration to ~/.config/thts/config.yaml.
func Save(cfg *Config) error {
	path := ThtsConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Exists checks if a config file exists (either thts or HumanLayer).
func Exists() bool {
	if _, err := os.Stat(ThtsConfigPath()); err == nil {
		return true
	}
	if _, err := os.Stat(HumanLayerConfigPath()); err == nil {
		// Check if it has thoughts config
		if _, err := loadFromHumanLayerPath(HumanLayerConfigPath()); err == nil {
			return true
		}
	}
	return false
}

// LoadOrDefault loads the config or returns defaults if not found.
func LoadOrDefault() *Config {
	cfg, err := Load()
	if err != nil {
		return Defaults()
	}
	return cfg
}
