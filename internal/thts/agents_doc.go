package thts

import (
	"fmt"
	"os"
	"path/filepath"
)

// GenerateThoughtsAgentsMD generates the content for thoughts/AGENTS.md.
// This file documents the thoughts directory structure for Claude Code.
func GenerateThoughtsAgentsMD(projectName, user string) string {
	return fmt.Sprintf(`# Thoughts Directory Structure

This directory contains developer thoughts and notes for the %s repository.
Managed by `+"`thts`"+` - do not commit to the code repository.

## Structure

Directories are named by **user**, not by repository. The repository name (%s)
determines where files are stored in the central thoughts repo.

- `+"`%s/`"+` - Your personal notes (named after your username)
- `+"`shared/`"+` - Team-shared notes for this repository
- `+"`global/`"+` - Cross-repository thoughts
  - `+"`%s/`"+` - Your personal cross-repo notes
  - `+"`shared/`"+` - Team-shared cross-repo notes
- `+"`searchable/`"+` - Hard links for searching (auto-generated)

## Searching in Thoughts

The `+"`searchable/`"+` directory contains hard links to all thoughts files. This
allows search tools to find content without following symlinks.

**IMPORTANT**:

- Files in `+"`searchable/`"+` are hard links (editing either updates both)
- Always reference files by canonical path (e.g., `+"`thoughts/%s/todo.md`"+`)
- The `+"`searchable/`"+` directory is rebuilt on `+"`thts sync`"+`

## Usage

Create markdown files to document:

- Architecture decisions
- Design notes
- TODO items
- Investigation results
- Meeting notes

Quick access:

- `+"`thoughts/%s/`"+` for repo-specific notes (most common)
- `+"`thoughts/global/%s/`"+` for cross-repo notes

## Commands

- `+"`thts sync`"+` - Manually sync changes to thoughts repository
- `+"`thts status`"+` - Show sync status
`, projectName, projectName, user, user, user, user, user)
}

// WriteThoughtsAgentsMD writes AGENTS.md in thoughts/ and ensures CLAUDE.md points to it.
// Returns true if AGENTS.md was created, false if it already exists.
func WriteThoughtsAgentsMD(thoughtsDir, projectName, user string) (bool, error) {
	agentsPath := filepath.Join(thoughtsDir, "AGENTS.md")
	created := false

	// Create AGENTS.md if needed
	if _, err := os.Stat(agentsPath); err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("failed to stat AGENTS.md: %w", err)
		}

		content := GenerateThoughtsAgentsMD(projectName, user)
		if err := os.WriteFile(agentsPath, []byte(content), 0644); err != nil {
			return false, fmt.Errorf("failed to write AGENTS.md: %w", err)
		}
		created = true
	}

	if err := EnsureThoughtsClaudeSymlink(thoughtsDir); err != nil {
		return false, err
	}

	return created, nil
}

// EnsureThoughtsClaudeSymlink ensures thoughts/CLAUDE.md is a symlink to AGENTS.md.
func EnsureThoughtsClaudeSymlink(thoughtsDir string) error {
	claudePath := filepath.Join(thoughtsDir, "CLAUDE.md")

	if _, err := os.Lstat(claudePath); err == nil {
		if err := os.Remove(claudePath); err != nil {
			return fmt.Errorf("failed to replace CLAUDE.md with symlink: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat CLAUDE.md: %w", err)
	}

	if err := os.Symlink("AGENTS.md", claudePath); err != nil {
		return fmt.Errorf("failed to create CLAUDE.md symlink: %w", err)
	}

	return nil
}
