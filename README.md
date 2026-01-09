# tpd - Thoughts, Plans, and Dreams

A CLI for managing developer notes separately from code repositories while
keeping them accessible in every project.

<!-- mtoc-start -->

- [Why tpd?](#why-tpd)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [How It Works](#how-it-works)
- [Claude Code Integration](#claude-code-integration)
- [Documentation](#documentation)
- [Attribution](#attribution)
- [Compatibility with HumanLayer](#compatibility-with-humanlayer)
- [License](#license)

<!-- mtoc-end -->

## Why tpd?

- **Keep notes out of code repos** - Architecture decisions, TODOs, and
  investigation notes don't belong in git history
- **Access notes in every project** - Your thoughts appear as a local
  `thoughts/` directory via symlinks
- **Never lose context** - Notes sync to a central git repo you control
- **Share selectively** - Personal notes stay private, team notes are shared

<!-- prettier-ignore-start -->
> [!INFO]
> `tbd` is a Go reimplementation of the `thoughts` subcommand from
> [HumanLayer's CLI](https://github.com/humanlayer/humanlayer) (`humanlayer`).
> See [Compatibility with HumanLayer](#compatibility-with-humanlayer) for more
> information.

<!-- prettier-ignore-end -->

## Quick Start

```bash
# First-time setup (once per machine)
tpd setup

# Initialize in any git repo
cd ~/src/myproject
tpd init

# Start writing
echo "# Architecture Notes" > thoughts/$user/architecture.md

# Sync happens automatically on commits, or manually:
tpd sync -m "Added architecture notes"
```

## Commands

| Command                       | Description                         |
| ----------------------------- | ----------------------------------- |
| `tpd setup`                   | First-time configuration            |
| `tpd init [--profile <name>]` | Initialize thoughts in current repo |
| `tpd sync [-m <message>]`     | Sync thoughts to central repo       |
| `tpd status`                  | Show thoughts status                |
| `tpd uninit`                  | Remove thoughts from current repo   |
| `tpd config [--edit]`         | View/edit configuration             |
| `tpd profile create <name>`   | Create a profile                    |
| `tpd profile list`            | List profiles                       |
| `tpd profile show <name>`     | Show profile details                |
| `tpd profile delete <name>`   | Delete a profile                    |
| `tpd claude init`             | Install Claude Code integration     |
| `tpd claude uninit`           | Remove Claude Code integration      |

## How It Works

Your notes live in a **central thoughts repo** (e.g., `~/thoughts/`). When you
run `tpd init` in a project, it creates symlinks so notes appear locally:

```plaintext
~/src/myproject/thoughts/     # Symlinks (git-ignored)
├── {user}/   →  ~/thoughts/repos/myproject/{user}/
├── shared/   →  ~/thoughts/repos/myproject/shared/
└── global/   →  ~/thoughts/global/
```

Editing `thoughts/{user}/notes.md` in your project actually edits the file in
your central thoughts repo. Changes sync automatically on commits.

## Claude Code Integration

tpd integrates with Claude Code to give AI assistants awareness of your thoughts
directory:

```bash
tpd claude init              # Install integration files
tpd claude init -i           # Interactive mode (select files/options)
tpd claude uninit            # Remove integration (run tpd claude uninit --help for options)
```

This installs:

- `/tpd-integrate` - Skill to activate thoughts/ awareness for current task
- `/tpd-handoff` - Create session handoff documents
- `/tpd-resume` - Resume from handoff documents
- Specialized agents for searching/analyzing thoughts

See [User Guide](docs/guide.md#claude-code-integration) for integration options.

## Documentation

- [User Guide](docs/guide.md) - Complete documentation
- [Troubleshooting](docs/troubleshooting.md) - Common issues and solutions

## Attribution

This project is inspired by and based on the `thoughts` subcommand from
[HumanLayer](https://github.com/humanlayer/humanlayer) by the HumanLayer
Authors.

The original implementation provided the design, directory structure, and config
format that `tpd` replicates for compatibility. Thanks to the HumanLayer team
for creating and open-sourcing this workflow.

## Compatibility with HumanLayer

`tpd` is a Go reimplementation of
[HumanLayer](https://github.com/humanlayer/humanlayer)'s `humanlayer thoughts`
subcommand. **You can switch between them freely** - they use the same:

- Config file format (`~/.config/humanlayer/humanlayer.json`)
- Directory structure (`~/thoughts/repos/<project>/`)
- Symlink layout in projects
- Git hooks

This means:

- Use `tpd` on machines where you prefer a standalone binary
- Use `humanlayer thoughts` where you already have HumanLayer installed
- Team members can use whichever tool they prefer
- Your notes work with both tools simultaneously

See the [Compatibility Guide](docs/guide.md#compatibility-with-humanlayer) for
details.

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
