# tpd - Thoughts, Plans, and Dreams

A CLI for managing developer notes separately from code repositories while
keeping them accessible in every project.

## Why tpd?

- **Keep notes out of code repos** - Architecture decisions, TODOs, and
  investigation notes don't belong in git history
- **Access notes in every project** - Your thoughts appear as a local
  `thoughts/` directory via symlinks
- **Never lose context** - Notes sync to a central git repo you control
- **Share selectively** - Personal notes stay private, team notes are shared

## Quick Start

```bash
# First-time setup (once per machine)
tpd setup

# Initialize in any git repo
cd ~/src/myproject
tpd init

# Start writing
echo "# Architecture Notes" > thoughts/scotty/architecture.md

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

## How It Works

Your notes live in a **central thoughts repo** (e.g., `~/thoughts/`). When you
run `tpd init` in a project, it creates symlinks so notes appear locally:

```plaintext
~/src/myproject/thoughts/     # Symlinks (git-ignored)
├── scotty/   →  ~/thoughts/repos/myproject/scotty/
├── shared/   →  ~/thoughts/repos/myproject/shared/
└── global/   →  ~/thoughts/global/
```

Editing `thoughts/scotty/notes.md` in your project actually edits the file in
your central thoughts repo. Changes sync automatically on commits.

## Documentation

- [User Guide](docs/guide.md) - Complete documentation
- [Troubleshooting](docs/troubleshooting.md) - Common issues and solutions

## Compatibility

`tpd` is compatible with [HumanLayer](https://github.com/humanlayer/humanlayer)'s
`thoughts` subcommand. You can switch between them - they read/write the same
config format and directory structure.
