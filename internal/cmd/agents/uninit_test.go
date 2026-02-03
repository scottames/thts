package agents

import (
	"testing"
)

func TestIsPathSafeForRemoval(t *testing.T) {
	baseDir := "/home/user/project/.claude"

	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected bool
	}{
		// Invalid paths that should return false
		{
			name:     "empty string",
			path:     "",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "current dir",
			path:     ".",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "parent escape",
			path:     "..",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "parent escape with path",
			path:     "../secrets",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "absolute path unix",
			path:     "/etc/passwd",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "root path",
			path:     "/",
			baseDir:  baseDir,
			expected: false,
		},
		{
			name:     "hidden parent traversal",
			path:     "foo/../../etc/passwd",
			baseDir:  baseDir,
			expected: false,
		},

		// Valid paths that should return true
		{
			name:     "simple file",
			path:     "AGENTS.md",
			baseDir:  baseDir,
			expected: true,
		},
		{
			name:     "nested file",
			path:     "skills/foo.md",
			baseDir:  baseDir,
			expected: true,
		},
		{
			name:     "deeply nested file",
			path:     "commands/thts-handoff.md",
			baseDir:  baseDir,
			expected: true,
		},
		{
			name:     "file with dots in name",
			path:     "settings.local.json",
			baseDir:  baseDir,
			expected: true,
		},
		{
			name:     "directory in path",
			path:     "skills/thts-integrate/SKILL.md",
			baseDir:  baseDir,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathSafeForRemoval(tt.path, tt.baseDir)
			if result != tt.expected {
				t.Errorf("isPathSafeForRemoval(%q, %q) = %v, want %v",
					tt.path, tt.baseDir, result, tt.expected)
			}
		})
	}
}

func TestIsPathSafeForRemoval_EdgeCases(t *testing.T) {
	t.Run("path that becomes base dir after join", func(t *testing.T) {
		// filepath.Join("/base", "") returns "/base"
		// This should be caught and return false
		result := isPathSafeForRemoval("", "/home/user/.claude")
		if result {
			t.Error("expected false for empty path that resolves to base dir")
		}
	})

	t.Run("path with embedded null traversal", func(t *testing.T) {
		// Paths containing ".." somewhere in the middle
		result := isPathSafeForRemoval("foo/../../../etc/passwd", "/home/user/.claude")
		if result {
			t.Error("expected false for path with embedded traversal escaping base")
		}
	})

	t.Run("sibling traversal stays within base", func(t *testing.T) {
		// foo/../bar stays within base dir
		result := isPathSafeForRemoval("foo/../bar", "/home/user/.claude")
		if !result {
			t.Error("expected true for sibling traversal that stays within base")
		}
	})
}
