package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands ~ to home directory and resolves to absolute path.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// ContractPath replaces home directory with ~ for display.
func ContractPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// XDGConfigHome returns the XDG config home directory.
func XDGConfigHome() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config")
}

// ThtsConfigPath returns the path to the thts config file.
func ThtsConfigPath() string {
	return filepath.Join(XDGConfigHome(), "thts", "config.yaml")
}

// HumanLayerConfigPath returns the path to the HumanLayer config file.
func HumanLayerConfigPath() string {
	return filepath.Join(XDGConfigHome(), "humanlayer", "humanlayer.json")
}

// DefaultThoughtsRepo returns the default thoughts repository path.
func DefaultThoughtsRepo() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/thoughts"
	}
	return filepath.Join(home, "thoughts")
}

// DefaultUser returns the default username from $USER environment variable.
func DefaultUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "user"
}

// XDGStateHome returns the XDG state home directory.
func XDGStateHome() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return xdg
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state")
}

// GlobalManifestPath returns the path to the global agent manifest file.
func GlobalManifestPath() string {
	return filepath.Join(XDGStateHome(), "thts", "global-manifest.json")
}

// GlobalAgentDir returns the global directory for an agent type.
// These are the user-level config directories where global skills/commands/agents are installed.
// Note: OpenCode uses XDG (~/.config/opencode/), others use home directory dot-folders.
func GlobalAgentDir(agentType string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch agentType {
	case "claude":
		return filepath.Join(home, ".claude")
	case "codex":
		return filepath.Join(home, ".codex")
	case "opencode":
		// OpenCode uses XDG for global config
		return filepath.Join(XDGConfigHome(), "opencode")
	}
	return ""
}

// GlobalGitignorePath returns the path to the global gitignore file.
func GlobalGitignorePath() string {
	return filepath.Join(XDGConfigHome(), "git", "ignore")
}
