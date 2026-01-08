package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrConfigNotFound is returned when no config file exists.
var ErrConfigNotFound = errors.New("config not found")

// humanLayerConfigFile represents the HumanLayer config file structure.
// We only care about the 'thoughts' key.
type humanLayerConfigFile struct {
	Thoughts *Config `json:"thoughts,omitempty"`
}

// Load loads the tpd configuration.
// It first tries to load from ~/.config/tpd/config.json,
// then falls back to ~/.config/humanlayer/humanlayer.json.
func Load() (*Config, error) {
	// Try tpd config first
	tpdPath := TPDConfigPath()
	if cfg, err := loadFromPath(tpdPath); err == nil {
		return cfg, nil
	}

	// Fall back to HumanLayer config
	hlPath := HumanLayerConfigPath()
	if cfg, err := loadFromHumanLayerPath(hlPath); err == nil {
		return cfg, nil
	}

	return nil, ErrConfigNotFound
}

// loadFromPath loads config directly from a path (tpd format).
func loadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
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

	cfg := hlCfg.Thoughts

	// Initialize maps if nil
	if cfg.RepoMappings == nil {
		cfg.RepoMappings = make(map[string]*RepoMapping)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]*ProfileConfig)
	}

	return cfg, nil
}

// Save saves the configuration to ~/.config/tpd/config.json.
func Save(cfg *Config) error {
	path := TPDConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Exists checks if a config file exists (either tpd or HumanLayer).
func Exists() bool {
	if _, err := os.Stat(TPDConfigPath()); err == nil {
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
