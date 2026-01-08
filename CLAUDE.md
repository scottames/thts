# tpd - Thoughts, Plans, and Dreams

Go CLI for managing developer thoughts/notes across repositories. Replicates
HumanLayer's `thoughts` subcommand with full feature compatibility.

## Commands

| Command       | Description                                      |
| ------------- | ------------------------------------------------ |
| `tpd setup`   | Initial setup - configure thoughts repo location |
| `tpd init`    | Initialize thoughts in current git repo          |
| `tpd sync`    | Sync thoughts to central repo                    |
| `tpd status`  | Show thoughts status                             |
| `tpd uninit`  | Remove thoughts from current repo                |
| `tpd config`  | View/edit configuration                          |
| `tpd profile` | Manage profiles (create/list/show/delete)        |

## Project Structure

```plaintext
cmd/tpd/           # Entry point
internal/
  cmd/             # Cobra commands
  config/          # Config loading/saving, paths, types
  fs/              # Filesystem utilities (symlinks, gitignore)
  git/             # Git operations, hooks
  tpd/             # Searchable directory (hard links)
```

## Reference

Based on [HumanLayer CLI](https://github.com/humanlayer/humanlayer)'s `thoughts`
subcommand.

**IMPORTANT**: `tpd` should always be feature-compatible with humanlayer's
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
go build -o tpd ./cmd/tpd
```

### Testing

Tests should not be an afterthought or skipped.

```bash
go test ./...                                    # Unit tests
go test -tags=integration ./internal/cmd/...     # Integration tests
go test -tags=integration ./...                  # All tests
```

**Coverage targets:** config/git >70%, fs/tpd >60% (all met)

## Documentation

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
| Created by | `tpd setup`                     | `tpd init`                         |

Editing a file in the thoughts directory actually edits the file in the thoughts
repo through the symlink. This should be called out when relevant.
