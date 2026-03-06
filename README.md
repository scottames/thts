# thts

A CLI for storing developer thoughts, plans, and dreams in a central repo while
keeping them accessible in any project. Integrates with AI coding agents (Claude
Code, Codex, OpenCode), giving them persistent memory for research, plans, and
context across sessions.

<!-- prettier-ignore-start -->
> [!WARNING]
> This project incorporates code, comments, and documentation
> generated or assisted by artificial intelligence tools (such as Claude or
> ChatGPT). All content is actively reviewed and modified by project maintainers
> before inclusion. Use at your own risk.
<!-- prettier-ignore-end -->

<!-- mtoc-start -->

- [How It Works](#how-it-works)
- [Why thts?](#why-thts)
- [Installation](#installation)
  - [Go Install](#go-install)
  - [Binary Downloads](#binary-downloads)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Documentation](#documentation)
- [AI Agent Integration](#ai-agent-integration)
- [Attribution](#attribution)
- [Compatibility with HumanLayer](#compatibility-with-humanlayer)
  - [Config handling](#config-handling)
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

<!-- prettier-ignore-start -->
>[!TIP]
> The _central thoughts repo_ can live inside an
> [Obsidian](https://obsidian.md/) vault. It's all markdown at the end of the
> day.
<!-- prettier-ignore-end -->

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
  - AI agents will automatically, if [configured](#ai-agent-integration), use
    `thts` to keep track of research, notes, plans, etc.

<!-- prettier-ignore-start -->
> [!NOTE]
> `thts` is a Go reimplementation of the `thoughts` subcommand from
> [HumanLayer's CLI](https://github.com/humanlayer/humanlayer) (`humanlayer`).
> See [Compatibility with HumanLayer](#compatibility-with-humanlayer) for more
> information.

<!-- prettier-ignore-end -->

## Installation

### Go Install

```bash
go install github.com/scottames/thts/cmd/thts@latest
```

### Binary Downloads

Pre-built binaries for Linux and macOS (amd64/arm64) are available on the
[GitHub Releases](https://github.com/scottames/thts/releases) page.

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

| Command                        | Description                                                |
| ------------------------------ | ---------------------------------------------------------- |
| `thts setup`                   | First-time configuration                                   |
| `thts init [--profile <name>]` | Initialize thoughts in current repo (uses default profile) |
| `thts sync [-m <message>]`     | Sync thoughts to central repo                              |
| `thts status`                  | Show thoughts status                                       |
| `thts uninit`                  | Remove thoughts from current repo                          |
| `thts config [--edit]`         | View/edit configuration                                    |
| `thts profile create <name>`   | Create a profile                                           |
| `thts profile list`            | List profiles                                              |
| `thts profile show <name>`     | Show profile details                                       |
| `thts profile delete <name>`   | Delete a profile                                           |
| `thts init agents`             | Install AI agent integration                               |
| `thts uninit agents`           | Remove AI agent integration                                |

## Documentation

- [User Guide](docs/guide.md) - Complete documentation
- [Troubleshooting](docs/troubleshooting.md) - Common issues and solutions

## AI Agent Integration

`thts` integrates with AI coding agents to give them awareness of your thoughts
directory. Supports Claude Code, OpenAI Codex, and OpenCode.

```bash
thts init agents              # Install for detected agents
thts init agents -i           # Interactive mode
thts init agents --global     # Install to global config directories
thts uninit agents            # Remove integration
```

This installs skills, commands, and agents for thoughts/ integration including:

- `/thts-integrate` - Activate thoughts/ awareness for current task
- `/thts-handoff`, `/thts-resume` - Session handoff and resume
- Specialized agents for searching/analyzing thoughts

See [User Guide](docs/guide.md#ai-agent-integration) for details.

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
subcommand. They share:

- Directory structure (`~/thoughts/repos/<project>/`)
- Symlink layout in projects
- Git hooks

### Config handling

`thts` uses its own config at `~/.config/thts/config.yaml` (YAML format). It can
read HumanLayer's config (`~/.config/humanlayer/humanlayer.json`) as a fallback,
but never writes to it. This means:

- Migrating from HumanLayer to `thts` is seamless - existing config is read
  automatically
- `thts` writes its own config, so HumanLayer won't see changes made via `thts`
- Team members can use whichever tool they prefer on the same thoughts repo

See the [Compatibility Guide](docs/guide.md#compatibility-with-humanlayer) for
details.

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
