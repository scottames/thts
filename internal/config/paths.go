package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
// Respects THTS_CONFIG_PATH environment variable if set.
func ThtsConfigPath() string {
	if path := os.Getenv("THTS_CONFIG_PATH"); path != "" {
		return ExpandPath(path)
	}
	return filepath.Join(XDGConfigHome(), "thts", "config.yaml")
}

// CanonicalConfigPath returns the realpath of the active thts config path.
// It expands ~ and resolves symlinks when possible.
func CanonicalConfigPath() string {
	return canonicalPath(ThtsConfigPath())
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

// DefaultUser returns the default username.
// Respects THTS_USER environment variable if set, otherwise falls back to $USER.
func DefaultUser() string {
	if user := os.Getenv("THTS_USER"); user != "" {
		return user
	}
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

// LegacyStatePath returns the legacy single-file thts state path.
func LegacyStatePath() string {
	return filepath.Join(XDGStateHome(), "thts", "state.yaml")
}

// StatePathForConfig returns the namespaced state path for a specific config path.
func StatePathForConfig(configPath string) string {
	canonicalConfigPath := canonicalPath(configPath)
	hash := sha256.Sum256([]byte(canonicalConfigPath))
	hashStr := hex.EncodeToString(hash[:])
	filename := fmt.Sprintf("state-%s.yaml", hashStr)
	return filepath.Join(XDGStateHome(), "thts", filename)
}

// StatePath returns the namespaced thts state file for the active config path.
func StatePath() string {
	return StatePathForConfig(CanonicalConfigPath())
}

// GlobalManifestPath returns the path to the global agent manifest file.
func GlobalManifestPath() string {
	return filepath.Join(XDGStateHome(), "thts", "global-manifest.json")
}

func canonicalPath(path string) string {
	expandedPath := ExpandPath(path)
	resolvedPath, err := filepath.EvalSymlinks(expandedPath)
	if err == nil {
		return resolvedPath
	}
	return expandedPath
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
	case "gemini":
		return filepath.Join(home, ".gemini")
	}
	return ""
}

// GlobalGitignorePath returns the path to the global gitignore file.
func GlobalGitignorePath() string {
	return filepath.Join(XDGConfigHome(), "git", "ignore")
}
