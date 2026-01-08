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

// TPDConfigPath returns the path to the tpd config file.
func TPDConfigPath() string {
	return filepath.Join(XDGConfigHome(), "tpd", "config.json")
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
