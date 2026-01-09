package tpd

import (
	"fmt"
	"os"
	"path/filepath"
)

// GenerateClaudeMD generates the content for a CLAUDE.md file in the thoughts directory.
// This file documents the thoughts directory structure for Claude Code.
func GenerateClaudeMD(projectName, user string) string {
	return fmt.Sprintf(`# Thoughts Directory Structure

This directory contains developer thoughts and notes for the %s repository.
Managed by `+"`tpd`"+` - do not commit to the code repository.

## Structure

- `+"`%s/`"+` - Your personal notes for this repository
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
- The `+"`searchable/`"+` directory is rebuilt on `+"`tpd sync`"+`

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

- `+"`tpd sync`"+` - Manually sync changes to thoughts repository
- `+"`tpd status`"+` - Show sync status
`, projectName, user, user, user, user, user)
}

// WriteClaudeMD writes the CLAUDE.md file to the thoughts directory.
// Returns true if the file was created, false if it already exists.
func WriteClaudeMD(thoughtsDir, projectName, user string) (bool, error) {
	claudePath := filepath.Join(thoughtsDir, "CLAUDE.md")

	// Check if file already exists
	if _, err := os.Stat(claudePath); err == nil {
		return false, nil
	}

	content := GenerateClaudeMD(projectName, user)
	if err := os.WriteFile(claudePath, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("failed to write CLAUDE.md: %w", err)
	}

	return true, nil
}
