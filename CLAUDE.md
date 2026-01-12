# thts - Thoughts, Plans, and Dreams

Go CLI for managing developer thoughts/notes across repositories. Replicates
HumanLayer's `thoughts` subcommand with full feature compatibility.

## Commands

| Command              | Description                                         |
| -------------------- | --------------------------------------------------- |
| `thts setup`         | Initial setup - configure thoughts repo location    |
| `thts init`          | Initialize thoughts in current git repo             |
| `thts sync`          | Sync thoughts to central repo                       |
| `thts status`        | Show thoughts status                                |
| `thts uninit`        | Remove thoughts from current repo                   |
| `thts config`        | View/edit configuration                             |
| `thts profile`       | Manage profiles (create/list/show/delete)           |
| `thts agents init`   | Install agent integration (claude, codex, opencode) |
| `thts agents uninit` | Remove agent integration from project               |

## Project Structure

```plaintext
cmd/thts/          # Entry point
internal/
  agents/          # Agent type registry and detection
  cmd/             # Cobra commands
  config/          # Config loading/saving, paths, types
  fs/              # Filesystem utilities (symlinks, gitignore)
  git/             # Git operations, hooks
  thts/            # Searchable directory (hard links)
instructions/      # Embedded: AGENTS.md (shared instructions)
skills/            # Embedded: skills per agent (claude/codex/opencode)
commands/          # Embedded: thts-handoff.md, thts-resume.md (claude only)
agents/            # Embedded: thoughts-locator.md, thoughts-analyzer.md
embed.go           # Go embed declarations for above
```

## UI Standards

Terminal output uses the `internal/ui` package for consistency.

### Message Types

| Function       | Symbol | Color  | Usage               |
| -------------- | ------ | ------ | ------------------- |
| `ui.Success()` | `✓`    | Green  | Operation completed |
| `ui.Info()`    | `ℹ`    | Blue   | Informational       |
| `ui.Warning()` | `⚠`    | Yellow | Non-fatal issues    |
| `ui.Error()`   | `✗`    | Red    | Fatal errors        |

### Headers and Sections

- `ui.Header("Title")` - Bordered box for major sections
- `ui.SubHeader("Section:")` - Bold text for subsections

### Tables

- `ui.NewTable("Col1", "Col2")` - For structured data display
- `ui.KeyValueTable(rows)` - For key-value pairs

### Text Styling

- `ui.Accent(text)` - Cyan, for paths and commands
- `ui.Muted(text)` - Gray, for secondary info
- `ui.Bullet(text)` - Indented bullet point

## Agent Integration Files

Files in `instructions/`, `skills/`, `commands/`, `agents/` are embedded in the
binary and copied to agent directories by `thts agents init`.

### Supported Agents

| Agent    | Directory    | Skill Format        | Settings File   |
| -------- | ------------ | ------------------- | --------------- |
| Claude   | `.claude/`   | `skills/*.md`       | `settings.json` |
| Codex    | `.codex/`    | `skills/*/SKILL.md` | `config.toml`   |
| OpenCode | `.opencode/` | `skill/*/SKILL.md`  | `opencode.json` |

### Embedded Files

| File                   | Purpose                       | Agent Support |
| ---------------------- | ----------------------------- | ------------- |
| `AGENTS.md`            | Shared thoughts/ instructions | All           |
| `thts-integrate`       | On-demand activation skill    | All           |
| `thts-handoff.md`      | Session handoff command       | Claude only   |
| `thts-resume.md`       | Resume from handoff command   | Claude only   |
| `thoughts-locator.md`  | Find documents agent          | All           |
| `thoughts-analyzer.md` | Analyze documents agent       | All           |

## Reference

Based on [HumanLayer CLI](https://github.com/humanlayer/humanlayer)'s `thoughts`
subcommand.

**IMPORTANT**: `thts` should always be feature-compatible with humanlayer's
`thoughts` subcommand so users can switch between them.

## Development

### Linting

Uses [trunk](https://trunk.io) for linting. **All checks must pass to consider
any work "complete".**

```bash
trunk check        # Run all linters
trunk fmt          # Auto-format
trunk check --fix  # Auto-fix where possible
```

### Building

```bash
go build -o thts ./cmd/thts
```

### Testing

Tests should not be an afterthought or skipped.

```bash
go test ./...                                    # Unit tests
go test -tags=integration ./internal/cmd/...     # Integration tests
go test -tags=integration ./...                  # All tests
```

**Coverage targets:** config/git >70%, fs/thts >60% (all met)

## Documentation

**Any relevant docs should be updated or created to consider any work
"complete".**

User-facing docs live in `docs/`:

- `README.md` - Entry point, quick start
- `docs/guide.md` - Complete user guide
- `docs/troubleshooting.md` - Common issues and solutions

### Documentation Style

- **Minimal and concise** - Easy to scan and find what you need
- **No emojis** - Keep it professional
- **Include examples** - Shell commands with expected behavior
- **Consistent terminology**:
  - "thoughts repo" = central `~/thoughts/` git repository
  - "thoughts directory" = project's `thoughts/` symlinks

### Key Concept to Clarify

The distinction between the **thoughts repo** and **thoughts directory** is
easily confused. Always be explicit:

|            | Thoughts Repo                   | Thoughts Directory                 |
| ---------- | ------------------------------- | ---------------------------------- |
| Location   | `~/thoughts/`                   | `~/src/project/thoughts/`          |
| What it is | Real git repo with actual files | Symlinks pointing to thoughts repo |
| Created by | `thts setup`                    | `thts init`                        |

Editing a file in the thoughts directory actually edits the file in the thoughts
repo through the symlink. This should be called out when relevant.

## thts

### Dogfooding thts

We're dogfooding `thts` in this repo - capture any bugs/issues/learnings to
thoughts/shared/notes/2026-01-11-thts-meta-testing.md

<!-- thts-start -->

@.claude/thts-instructions.md

<!-- thts-end -->
