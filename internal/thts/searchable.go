package thts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	thtsfs "github.com/scottames/thts/internal/fs"
)

// SearchableResult holds statistics from creating the searchable directory.
type SearchableResult struct {
	LinkedCount     int
	SkippedCount    int
	CrossFilesystem bool
}

// CreateSearchableDir creates a searchable/ directory with hard links to all files.
// Hard links allow search tools to find content without following symlinks.
// Returns the number of files linked and whether any files were skipped due to
// cross-filesystem issues.
func CreateSearchableDir(thoughtsDir string) (*SearchableResult, error) {
	searchDir := filepath.Join(thoughtsDir, "searchable")
	result := &SearchableResult{}

	// Remove existing searchable directory if it exists
	if thtsfs.Exists(searchDir) {
		if err := thtsfs.RemoveAll(searchDir); err != nil {
			return nil, fmt.Errorf("failed to remove existing searchable directory: %w", err)
		}
	}

	// Create new searchable directory
	if err := os.MkdirAll(searchDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create searchable directory: %w", err)
	}

	// Find all files following symlinks
	files, err := findFilesFollowingSymlinks(thoughtsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find files: %w", err)
	}

	// Create hard links
	for _, relPath := range files {
		sourcePath := filepath.Join(thoughtsDir, relPath)
		targetPath := filepath.Join(searchDir, relPath)

		// Create directory structure
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			result.SkippedCount++
			continue
		}

		// Resolve symlink to get the real file path
		realSourcePath, err := filepath.EvalSymlinks(sourcePath)
		if err != nil {
			result.SkippedCount++
			continue
		}

		// Create hard link to the real file
		if err := os.Link(realSourcePath, targetPath); err != nil {
			// Check if it's a cross-filesystem error
			if isCrossFilesystemError(err) {
				result.CrossFilesystem = true
			}
			result.SkippedCount++
			continue
		}

		result.LinkedCount++
	}

	return result, nil
}

// findFilesFollowingSymlinks recursively finds all files through symlinks.
// It skips:
// - Files/directories starting with '.'
// - CLAUDE.md files
// - The searchable/ directory itself
func findFilesFollowingSymlinks(dir string) ([]string, error) {
	var files []string
	visited := make(map[string]bool)

	err := walkFollowingSymlinks(dir, dir, visited, &files)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// walkFollowingSymlinks walks a directory tree following symlinks while avoiding cycles.
func walkFollowingSymlinks(root, dir string, visited map[string]bool, files *[]string) error {
	// Resolve symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil // Skip unresolvable paths
	}

	// Check for cycles
	if visited[realPath] {
		return nil
	}
	visited[realPath] = true

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // Skip unreadable directories
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)

		// Skip dotfiles and special files
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Skip CLAUDE.md
		if name == "CLAUDE.md" {
			continue
		}

		// Skip searchable directory itself
		if name == "searchable" && dir == root {
			continue
		}

		// Get file info (following symlinks)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue // Skip broken symlinks
		}

		if info.IsDir() {
			// Recurse into directory
			if err := walkFollowingSymlinks(root, fullPath, visited, files); err != nil {
				continue
			}
		} else if info.Mode().IsRegular() {
			// Add file to list (relative to root)
			relPath, err := filepath.Rel(root, fullPath)
			if err != nil {
				continue
			}
			*files = append(*files, relPath)
		}
	}

	return nil
}

// isCrossFilesystemError checks if an error is due to cross-filesystem hard link attempt.
func isCrossFilesystemError(err error) bool {
	// Check for EXDEV (cross-device link)
	var linkErr *fs.PathError
	if errors.As(err, &linkErr) {
		if errno, ok := linkErr.Err.(syscall.Errno); ok {
			return errno == syscall.EXDEV
		}
	}
	// Also check directly
	if errno, ok := err.(syscall.Errno); ok {
		return errno == syscall.EXDEV
	}
	return false
}
