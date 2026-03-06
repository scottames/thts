# Contributing to thts

Thanks for your interest in contributing to thts!

## Getting Started

### Prerequisites

- Go 1.26+
- [trunk](https://trunk.io) for linting
- Git

### Building

```bash
go build -o thts ./cmd/thts
```

### Testing

```bash
go test ./...                                # Unit tests
go test -tags=integration ./internal/cmd/... # Integration tests
go test -tags=integration ./...              # All tests
```

### Linting

All checks must pass before submitting:

```bash
trunk check       # Run all linters
trunk fmt         # Auto-format
trunk check --fix # Auto-fix where possible
```

## Submitting Changes

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes with tests
4. Ensure `trunk check` and `go test ./...` pass
5. Submit a pull request

### Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```text
feat: add new feature
fix: resolve bug in sync
chore: update dependencies
docs: improve user guide
refactor: simplify config loading
test: test improvements
```

### Pull Requests

- Keep PRs focused on a single change
- Include tests for new functionality
- Update documentation if behavior changes

## Compatibility

thts aims to stay mostly compatible with
[HumanLayer's `thoughts` subcommand](https://github.com/humanlayer/humanlayer).
Changes to directory structure, symlink layout, or config fallback behavior
should preserve this compatibility.

## Platform Support

Currently supported: Linux and macOS (amd64/arm64). Windows is not supported.
