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

```
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
