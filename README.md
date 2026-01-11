# thts - Thoughts, Plans, and Dreams

A CLI for managing developer notes separately from code repositories while
keeping them accessible in every project.

<!-- mtoc-start -->

- [How It Works](#how-it-works)
- [Why thts?](#why-thts)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Documentation](#documentation)
- [Claude Code Integration](#claude-code-integration)
- [Attribution](#attribution)
- [Compatibility with HumanLayer](#compatibility-with-humanlayer)
- [License](#license)

<!-- mtoc-end -->

## How It Works

Your notes live in a _central thoughts repo_ (e.g., `~/thoughts/`). When you run
`thts init` in a project, it creates symlinks so notes appear locally:

```plaintext
~/src/myproject/thoughts/     # Symlinks (git-ignored)
├── {user}/   →  ~/thoughts/repos/myproject/{user}/
├── shared/   →  ~/thoughts/repos/myproject/shared/
└── global/   →  ~/thoughts/global/
```

Editing `thoughts/{user}/notes.md` in your project actually edits the file in
your central thoughts repo. Changes sync automatically on commits.

## Why thts?

`thts` stores thoughts, plans, dreams, research, etc. in one central repo and
links it to existing repos so that they can be shared across projects and with
teams without versioning them in every repo independently.

- Access notes in every project
  - "thoughts" appear as a local `thoughts/` directory via symlinks in each
    enabled repo/project
- Never lose context
  - Notes sync to a central git repo and can be queried from anywhere
- Share with a team
  - By design notes can be given a personal, project, or team scope
- Automatic LLM integration
  - Claude will automatically, if [configured](#claude-code-integration) use
    `thts` to keep track of research, notes, plans, etc.

<!-- prettier-ignore-start -->
>[!INFO]
> `thts` is a Go reimplementation of the `thoughts` subcommand from
> [HumanLayer's CLI](https://github.com/humanlayer/humanlayer) (`humanlayer`).
> See [Compatibility with HumanLayer](#compatibility-with-humanlayer) for more
> information.

<!-- prettier-ignore-end -->

## Quick Start

```bash
# First-time setup (once per machine)
thts setup

# Initialize in any git repo
cd ~/src/myproject
thts init

# Start writing
echo "# Architecture Notes" > thoughts/$user/architecture.md

# Sync with central remote repo
#  - If integrated with Claude, Claude will be instructed to do so automatically
thts sync -m "Added architecture notes"
```

## Commands

| Command                        | Description                         |
| ------------------------------ | ----------------------------------- |
| `thts setup`                   | First-time configuration            |
| `thts init [--profile <name>]` | Initialize thoughts in current repo |
| `thts sync [-m <message>]`     | Sync thoughts to central repo       |
| `thts status`                  | Show thoughts status                |
| `thts uninit`                  | Remove thoughts from current repo   |
| `thts config [--edit]`         | View/edit configuration             |
| `thts profile create <name>`   | Create a profile                    |
| `thts profile list`            | List profiles                       |
| `thts profile show <name>`     | Show profile details                |
| `thts profile delete <name>`   | Delete a profile                    |
| `thts claude init`             | Install Claude Code integration     |
| `thts claude uninit`           | Remove Claude Code integration      |

## Documentation

- [User Guide](docs/guide.md) - Complete documentation
- [Troubleshooting](docs/troubleshooting.md) - Common issues and solutions

## Claude Code Integration

thts integrates with Claude Code to give AI assistants awareness of your thoughts
directory:

```bash
thts claude init              # Install integration files
thts claude init -i           # Interactive mode (select files/options)
thts claude uninit            # Remove integration (run thts claude uninit --help for options)
```

This installs:

- `/thts-integrate` - Skill to activate thoughts/ awareness for current task
- `/thts-handoff` - Create session handoff documents
- `/thts-resume` - Resume from handoff documents
- Specialized agents for searching/analyzing thoughts

See [User Guide](docs/guide.md#claude-code-integration) for integration options.

## Attribution

This project is inspired by and based on the `thoughts` subcommand from
[HumanLayer](https://github.com/humanlayer/humanlayer) by the HumanLayer
Authors.

The original implementation provided the design, directory structure, and config
format that `thts` replicates for compatibility. Thanks to the HumanLayer team
for creating and open-sourcing this workflow.

## Compatibility with HumanLayer

`thts` is a Go reimplementation of
[HumanLayer](https://github.com/humanlayer/humanlayer)'s `humanlayer thoughts`
subcommand. **You can switch between them freely** - they use the same:

- Config file format (`~/.config/humanlayer/humanlayer.json`)
- Directory structure (`~/thoughts/repos/<project>/`)
- Symlink layout in projects
- Git hooks

This means:

- Use `thts` on machines where you prefer a standalone binary
- Use `humanlayer thoughts` where you already have HumanLayer installed
- Team members can use whichever tool they prefer
- Your notes work with both tools simultaneously

See the [Compatibility Guide](docs/guide.md#compatibility-with-humanlayer) for
details.

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
