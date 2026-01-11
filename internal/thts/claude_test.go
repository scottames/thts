package thts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateClaudeMD(t *testing.T) {
	t.Run("includes project name", func(t *testing.T) {
		content := GenerateClaudeMD("my-project", "testuser")

		if !strings.Contains(content, "my-project") {
			t.Error("content should include project name")
		}
	})

	t.Run("includes user name in multiple places", func(t *testing.T) {
		content := GenerateClaudeMD("my-project", "alice")

		// User should appear in structure section
		if !strings.Contains(content, "`alice/`") {
			t.Error("content should include user directory")
		}

		// User should appear in examples
		if !strings.Contains(content, "thoughts/alice/") {
			t.Error("content should include user in example paths")
		}
	})

	t.Run("includes thts commands", func(t *testing.T) {
		content := GenerateClaudeMD("my-project", "testuser")

		if !strings.Contains(content, "`thts sync`") {
			t.Error("content should mention thts sync command")
		}

		if !strings.Contains(content, "`thts status`") {
			t.Error("content should mention thts status command")
		}
	})

	t.Run("documents directory structure", func(t *testing.T) {
		content := GenerateClaudeMD("my-project", "testuser")

		required := []string{
			"`shared/`",
			"`global/`",
			"`searchable/`",
		}

		for _, req := range required {
			if !strings.Contains(content, req) {
				t.Errorf("content should include %s", req)
			}
		}
	})
}

func TestWriteClaudeMD(t *testing.T) {
	t.Run("creates file when it does not exist", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "thts-claude-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		created, err := WriteClaudeMD(dir, "test-project", "testuser")
		if err != nil {
			t.Fatalf("WriteClaudeMD failed: %v", err)
		}

		if !created {
			t.Error("expected file to be created")
		}

		// Verify file exists
		claudePath := filepath.Join(dir, "CLAUDE.md")
		if _, err := os.Stat(claudePath); os.IsNotExist(err) {
			t.Error("CLAUDE.md should exist")
		}

		// Verify content
		content, err := os.ReadFile(claudePath)
		if err != nil {
			t.Fatalf("failed to read CLAUDE.md: %v", err)
		}

		if !strings.Contains(string(content), "test-project") {
			t.Error("file should contain project name")
		}
	})

	t.Run("does not overwrite existing file", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "thts-claude-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		// Create existing file with custom content
		claudePath := filepath.Join(dir, "CLAUDE.md")
		originalContent := "# Custom CLAUDE.md\n\nThis is custom content."
		if err := os.WriteFile(claudePath, []byte(originalContent), 0644); err != nil {
			t.Fatalf("failed to create existing file: %v", err)
		}

		// Try to write again
		created, err := WriteClaudeMD(dir, "test-project", "testuser")
		if err != nil {
			t.Fatalf("WriteClaudeMD failed: %v", err)
		}

		if created {
			t.Error("should not report file as created when it already exists")
		}

		// Verify original content preserved
		content, err := os.ReadFile(claudePath)
		if err != nil {
			t.Fatalf("failed to read CLAUDE.md: %v", err)
		}

		if string(content) != originalContent {
			t.Error("should not overwrite existing file")
		}
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		_, err := WriteClaudeMD("/nonexistent/path", "test-project", "testuser")
		if err == nil {
			t.Error("expected error for non-existent directory")
		}
	})
}
