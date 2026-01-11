// Package thtsfiles provides embedded Claude Code integration files for thts.
// This package exists at the repo root to enable go:embed access to
// instructions/, skills/, commands/, and agents/ directories.
package thtsfiles

import "embed"

// Instructions contains embedded instruction markdown files.
//
//go:embed instructions/*.md
var Instructions embed.FS

// Skills contains embedded skill markdown files.
//
//go:embed skills/*.md
var Skills embed.FS

// Commands contains embedded command markdown files.
//
//go:embed commands/*.md
var Commands embed.FS

// Agents contains embedded agent markdown files.
//
//go:embed agents/*.md
var Agents embed.FS

// DefaultSettingsJSON provides a default settings.json template.
var DefaultSettingsJSON = `{
  "permissions": {
    "allow": []
  },
  "enableAllProjectMcpServers": false
}
`
