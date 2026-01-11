package thts

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
