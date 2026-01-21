package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// FilterByAgents returns components filtered to only include files for requested agents.
// If requested is empty, returns all components unchanged (current behavior for removing all).
// Returns a deep copy with filtered files and agents lists.
func (m *GlobalManifest) FilterByAgents(requested []string) map[string]*GlobalComponentInfo {
	if len(requested) == 0 {
		return m.Components
	}

	requestedSet := make(map[string]bool)
	for _, a := range requested {
		requestedSet[a] = true
	}

	filtered := make(map[string]*GlobalComponentInfo)
	for name, info := range m.Components {
		// Filter agents to only requested ones
		var filteredAgents []string
		for _, a := range info.Agents {
			if requestedSet[a] {
				filteredAgents = append(filteredAgents, a)
			}
		}
		if len(filteredAgents) == 0 {
			continue
		}

		// Filter files to only those belonging to requested agents
		filteredFiles := filterFilesByAgents(info.Files, requestedSet)
		if len(filteredFiles) == 0 {
			continue
		}

		filtered[name] = &GlobalComponentInfo{
			Agents:    filteredAgents,
			Files:     filteredFiles,
			Gitignore: info.Gitignore,
		}
	}
	return filtered
}

// filterFilesByAgents filters files to only those belonging to agents in the set.
// Determines agent ownership by checking if file path contains the agent's global directory.
func filterFilesByAgents(files []string, agentSet map[string]bool) []string {
	var result []string
	for _, f := range files {
		agent := getAgentFromPath(f)
		if agent != "" && agentSet[agent] {
			result = append(result, f)
		}
	}
	return result
}

// getAgentFromPath determines which agent a file belongs to based on its path.
func getAgentFromPath(filePath string) string {
	// Check each known agent's global directory
	agentTypes := []string{"claude", "codex", "opencode", "gemini"}
	for _, agent := range agentTypes {
		globalDir := config.GlobalAgentDir(agent)
		if globalDir == "" {
			continue
		}
		// Check if file is under this agent's directory
		if strings.HasPrefix(filePath, globalDir+string(filepath.Separator)) || filePath == globalDir {
			return agent
		}
	}
	return ""
}

// RemoveAgents removes the specified agents and their files from all components.
// Components with no remaining agents are deleted.
// Returns true if the manifest is now empty.
func (m *GlobalManifest) RemoveAgents(toRemove []string) bool {
	removeSet := make(map[string]bool)
	for _, a := range toRemove {
		removeSet[a] = true
	}

	// Build set of agents to keep (inverse of removeSet)
	keepSet := make(map[string]bool)
	for _, info := range m.Components {
		for _, a := range info.Agents {
			if !removeSet[a] {
				keepSet[a] = true
			}
		}
	}

	for name, info := range m.Components {
		// Remove specified agents from the agents list
		info.Agents = removeFromAgentSlice(info.Agents, removeSet)

		// Remove files belonging to the removed agents (keep files for remaining agents)
		info.Files = filterFilesByAgents(info.Files, keepSet)

		// Delete component if no agents remain
		if len(info.Agents) == 0 {
			delete(m.Components, name)
		}
	}
	return m.IsEmpty()
}

// removeFromAgentSlice removes agents in removeSet from the slice.
func removeFromAgentSlice(agents []string, removeSet map[string]bool) []string {
	var result []string
	for _, a := range agents {
		if !removeSet[a] {
			result = append(result, a)
		}
	}
	return result
}

// GetFilesForAgents returns files from components that include any of the specified agents.
// If agents is empty, returns all files.
func (m *GlobalManifest) GetFilesForAgents(agents []string) []string {
	filtered := m.FilterByAgents(agents)
	var files []string
	for _, comp := range filtered {
		files = append(files, comp.Files...)
	}
	return files
}
