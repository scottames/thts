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

// Gitignore marker constants.
const (
	// GitignoreMarkerStart is the start marker for thts-managed gitignore patterns.
	GitignoreMarkerStart = "# thts-agent-start"
	// GitignoreMarkerEnd is the end marker for thts-managed gitignore patterns.
	GitignoreMarkerEnd = "# thts-agent-end"
)

// AddGitignoreMarkerBlock adds patterns between markers in the gitignore file.
// If a marker block already exists, it updates the patterns within it.
// Returns the list of patterns that were added.
func AddGitignoreMarkerBlock(repoPath string, patterns []string) ([]string, error) {
	ignorePath := filepath.Join(repoPath, ".gitignore")

	// Read existing content
	content, err := os.ReadFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	text := string(content)

	// Build the marker block
	var blockLines []string
	blockLines = append(blockLines, GitignoreMarkerStart)
	blockLines = append(blockLines, patterns...)
	blockLines = append(blockLines, GitignoreMarkerEnd)
	block := strings.Join(blockLines, "\n")

	// Check if marker block already exists
	startIdx := strings.Index(text, GitignoreMarkerStart)
	endIdx := strings.Index(text, GitignoreMarkerEnd)

	var newContent string
	if startIdx != -1 && endIdx != -1 {
		// Update existing block
		endIdx += len(GitignoreMarkerEnd)
		// Include trailing newline if present
		if endIdx < len(text) && text[endIdx] == '\n' {
			endIdx++
		}
		newContent = text[:startIdx] + block + "\n" + text[endIdx:]
	} else {
		// Add new block at end
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		newContent = text + block + "\n"
	}

	if err := os.WriteFile(ignorePath, []byte(newContent), 0644); err != nil {
		return nil, err
	}

	return patterns, nil
}

// RemoveGitignoreMarkerBlock removes the marker block from the gitignore file.
// Returns the patterns that were removed.
func RemoveGitignoreMarkerBlock(repoPath string) ([]string, error) {
	ignorePath := filepath.Join(repoPath, ".gitignore")

	content, err := os.ReadFile(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	text := string(content)
	startIdx := strings.Index(text, GitignoreMarkerStart)
	endIdx := strings.Index(text, GitignoreMarkerEnd)

	// No markers found
	if startIdx == -1 && endIdx == -1 {
		return nil, nil
	}

	// Corrupted markers
	if startIdx == -1 || endIdx == -1 {
		return nil, nil // Silently ignore corrupted state
	}

	// Extract patterns that were in the block
	blockStart := startIdx + len(GitignoreMarkerStart)
	if blockStart < len(text) && text[blockStart] == '\n' {
		blockStart++
	}
	blockContent := text[blockStart:endIdx]
	var patterns []string
	for _, line := range strings.Split(blockContent, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	// Calculate removal range
	endIdx += len(GitignoreMarkerEnd)
	// Include trailing newline
	if endIdx < len(text) && text[endIdx] == '\n' {
		endIdx++
	}

	newContent := text[:startIdx] + text[endIdx:]

	// Clean up multiple blank lines
	for strings.Contains(newContent, "\n\n\n") {
		newContent = strings.ReplaceAll(newContent, "\n\n\n", "\n\n")
	}

	if err := os.WriteFile(ignorePath, []byte(newContent), 0644); err != nil {
		return nil, err
	}

	return patterns, nil
}

// HasGitignoreMarkerBlock checks if the gitignore file has a thts marker block.
func HasGitignoreMarkerBlock(repoPath string) bool {
	ignorePath := filepath.Join(repoPath, ".gitignore")
	content, err := os.ReadFile(ignorePath)
	if err != nil {
		return false
	}
	text := string(content)
	return strings.Contains(text, GitignoreMarkerStart) && strings.Contains(text, GitignoreMarkerEnd)
}

// GetGitignoreMarkerPatterns returns the patterns in the marker block.
func GetGitignoreMarkerPatterns(repoPath string) []string {
	ignorePath := filepath.Join(repoPath, ".gitignore")
	content, err := os.ReadFile(ignorePath)
	if err != nil {
		return nil
	}

	text := string(content)
	startIdx := strings.Index(text, GitignoreMarkerStart)
	endIdx := strings.Index(text, GitignoreMarkerEnd)

	if startIdx == -1 || endIdx == -1 {
		return nil
	}

	blockStart := startIdx + len(GitignoreMarkerStart)
	if blockStart < len(text) && text[blockStart] == '\n' {
		blockStart++
	}
	blockContent := text[blockStart:endIdx]

	var patterns []string
	for _, line := range strings.Split(blockContent, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	return patterns
}
