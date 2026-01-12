package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/scottames/thts/internal/config"
)

// GlobalManifest tracks globally installed thts agent files.
type GlobalManifest struct {
	Version     int                             `json:"version"`
	InstalledAt string                          `json:"installedAt"`
	Components  map[string]*GlobalComponentInfo `json:"components"`
}

// GlobalComponentInfo tracks files for a component.
type GlobalComponentInfo struct {
	Agents    []string             `json:"agents"`              // e.g., ["claude", "codex", "opencode"]
	Files     []string             `json:"files"`               // Absolute paths to installed files
	Gitignore *GlobalGitignoreInfo `json:"gitignore,omitempty"` // Only for gitignore component
}

// GlobalGitignoreInfo tracks gitignore modifications.
type GlobalGitignoreInfo struct {
	File     string   `json:"file"`     // Path to global gitignore
	Patterns []string `json:"patterns"` // Patterns added
}

// NewGlobalManifest creates a new empty manifest.
func NewGlobalManifest() *GlobalManifest {
	return &GlobalManifest{
		Version:     1,
		InstalledAt: time.Now().Format(time.RFC3339),
		Components:  make(map[string]*GlobalComponentInfo),
	}
}

// LoadGlobalManifest loads the global manifest from disk.
// Returns nil if the manifest doesn't exist.
func LoadGlobalManifest() (*GlobalManifest, error) {
	path := config.GlobalManifestPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var manifest GlobalManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// SaveGlobalManifest saves the manifest to disk.
func SaveGlobalManifest(m *GlobalManifest) error {
	path := config.GlobalManifestPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// DeleteGlobalManifest removes the manifest file.
func DeleteGlobalManifest() error {
	path := config.GlobalManifestPath()
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// AddComponent adds or updates a component in the manifest.
func (m *GlobalManifest) AddComponent(name string, info *GlobalComponentInfo) {
	if m.Components == nil {
		m.Components = make(map[string]*GlobalComponentInfo)
	}
	m.Components[name] = info
}

// RemoveComponent removes a component from the manifest.
func (m *GlobalManifest) RemoveComponent(name string) {
	if m.Components != nil {
		delete(m.Components, name)
	}
}

// HasComponent checks if a component is in the manifest.
func (m *GlobalManifest) HasComponent(name string) bool {
	if m.Components == nil {
		return false
	}
	_, ok := m.Components[name]
	return ok
}

// GetAllFiles returns all files across all components.
func (m *GlobalManifest) GetAllFiles() []string {
	var files []string
	for _, comp := range m.Components {
		files = append(files, comp.Files...)
	}
	return files
}

// IsEmpty returns true if no components are installed.
func (m *GlobalManifest) IsEmpty() bool {
	return len(m.Components) == 0
}
