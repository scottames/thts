package fs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/scottames/thts/internal/config"
)

// AddToGitignore adds a pattern to the appropriate gitignore file.
// Returns true if the pattern was added, false if it already existed or was disabled.
func AddToGitignore(repoPath, pattern string, location config.GitIgnoreMode) (bool, error) {
	if location == config.GitIgnoreDisabled {
		return false, nil
	}

	var ignorePath string

	switch location {
	case config.GitIgnoreProject:
		ignorePath = filepath.Join(repoPath, ".gitignore")
	case config.GitIgnoreLocal:
		ignorePath = filepath.Join(repoPath, ".git", "info", "exclude")
	case config.GitIgnoreGlobal:
		ignorePath = getGlobalGitignorePath()
	default:
		return false, nil
	}

	// Read existing content
	content, err := os.ReadFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	// Check if pattern already exists
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == pattern {
			return false, nil // Already exists
		}
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(ignorePath), 0755); err != nil {
		return false, err
	}

	// Build new content
	var newContent string
	if len(content) > 0 {
		// Ensure file ends with newline before adding pattern
		existingContent := string(content)
		if !strings.HasSuffix(existingContent, "\n") {
			existingContent += "\n"
		}
		newContent = existingContent + pattern + "\n"
	} else {
		newContent = pattern + "\n"
	}

	if err := os.WriteFile(ignorePath, []byte(newContent), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// RemoveFromGitignore removes a pattern from the gitignore file.
// Returns true if the pattern was removed.
func RemoveFromGitignore(repoPath, pattern string, location config.GitIgnoreMode) (bool, error) {
	if location == config.GitIgnoreDisabled {
		return false, nil
	}

	var ignorePath string

	switch location {
	case config.GitIgnoreProject:
		ignorePath = filepath.Join(repoPath, ".gitignore")
	case config.GitIgnoreLocal:
		ignorePath = filepath.Join(repoPath, ".git", "info", "exclude")
	case config.GitIgnoreGlobal:
		ignorePath = getGlobalGitignorePath()
	default:
		return false, nil
	}

	content, err := os.ReadFile(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	found := false

	for _, line := range lines {
		if strings.TrimSpace(line) == pattern {
			found = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !found {
		return false, nil
	}

	// Write back without trailing empty lines
	result := strings.Join(newLines, "\n")
	result = strings.TrimRight(result, "\n") + "\n"

	return true, os.WriteFile(ignorePath, []byte(result), 0644)
}

// getGlobalGitignorePath returns the path to the global gitignore file.
func getGlobalGitignorePath() string {
	// Check XDG config home first
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "git", "ignore")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "git", "ignore")
}
