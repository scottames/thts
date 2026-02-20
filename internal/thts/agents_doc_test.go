package thts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateThoughtsAgentsMD(t *testing.T) {
	t.Run("includes project name", func(t *testing.T) {
		content := GenerateThoughtsAgentsMD("my-project", "testuser")

		if !strings.Contains(content, "my-project") {
			t.Error("content should include project name")
		}
	})

	t.Run("includes user name in multiple places", func(t *testing.T) {
		content := GenerateThoughtsAgentsMD("my-project", "alice")

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
		content := GenerateThoughtsAgentsMD("my-project", "testuser")

		if !strings.Contains(content, "`thts sync`") {
			t.Error("content should mention thts sync command")
		}

		if !strings.Contains(content, "`thts status`") {
			t.Error("content should mention thts status command")
		}
	})

	t.Run("documents directory structure", func(t *testing.T) {
		content := GenerateThoughtsAgentsMD("my-project", "testuser")

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

func TestWriteThoughtsAgentsMD(t *testing.T) {
	t.Run("creates AGENTS.md and CLAUDE.md symlink when missing", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "thts-claude-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		created, err := WriteThoughtsAgentsMD(dir, "test-project", "testuser")
		if err != nil {
			t.Fatalf("WriteThoughtsAgentsMD failed: %v", err)
		}

		if !created {
			t.Error("expected file to be created")
		}

		agentsPath := filepath.Join(dir, "AGENTS.md")
		if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
			t.Error("AGENTS.md should exist")
		}

		claudePath := filepath.Join(dir, "CLAUDE.md")
		info, err := os.Lstat(claudePath)
		if err != nil {
			t.Fatalf("failed to stat CLAUDE.md: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("CLAUDE.md should be a symlink")
		}
		target, err := os.Readlink(claudePath)
		if err != nil {
			t.Fatalf("failed to read CLAUDE.md symlink: %v", err)
		}
		if target != "AGENTS.md" {
			t.Fatalf("CLAUDE.md symlink target = %q, want AGENTS.md", target)
		}

		// Verify content through symlink
		content, err := os.ReadFile(claudePath)
		if err != nil {
			t.Fatalf("failed to read CLAUDE.md content: %v", err)
		}
		if !strings.Contains(string(content), "test-project") {
			t.Error("file should contain project name")
		}
	})

	t.Run("does not overwrite existing AGENTS.md", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "thts-claude-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		// Create existing file with custom content
		agentsPath := filepath.Join(dir, "AGENTS.md")
		originalContent := "# Custom AGENTS.md\n\nThis is custom content."
		if err := os.WriteFile(agentsPath, []byte(originalContent), 0644); err != nil {
			t.Fatalf("failed to create existing file: %v", err)
		}

		// Try to write again
		created, err := WriteThoughtsAgentsMD(dir, "test-project", "testuser")
		if err != nil {
			t.Fatalf("WriteThoughtsAgentsMD failed: %v", err)
		}

		if created {
			t.Error("should not report file as created when it already exists")
		}

		// Verify original content preserved
		content, err := os.ReadFile(agentsPath)
		if err != nil {
			t.Fatalf("failed to read AGENTS.md: %v", err)
		}

		if string(content) != originalContent {
			t.Error("should not overwrite existing file")
		}

		claudePath := filepath.Join(dir, "CLAUDE.md")
		info, err := os.Lstat(claudePath)
		if err != nil {
			t.Fatalf("failed to stat CLAUDE.md: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("CLAUDE.md should be a symlink")
		}
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		_, err := WriteThoughtsAgentsMD("/nonexistent/path", "test-project", "testuser")
		if err == nil {
			t.Error("expected error for non-existent directory")
		}
	})

	t.Run("replaces existing CLAUDE.md file with symlink", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "thts-claude-test-*")
		if err != nil {
			t.Fatalf("failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		claudePath := filepath.Join(dir, "CLAUDE.md")
		if err := os.WriteFile(claudePath, []byte("legacy"), 0644); err != nil {
			t.Fatalf("failed to create CLAUDE.md: %v", err)
		}

		created, err := WriteThoughtsAgentsMD(dir, "test-project", "testuser")
		if err != nil {
			t.Fatalf("WriteThoughtsAgentsMD failed: %v", err)
		}
		if !created {
			t.Error("expected created=true when AGENTS.md was created")
		}

		info, err := os.Lstat(claudePath)
		if err != nil {
			t.Fatalf("failed to stat CLAUDE.md: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("CLAUDE.md should be replaced with a symlink")
		}
	})
}
