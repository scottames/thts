# tpd User Guide

<!-- mtoc-start -->

- [What tpd Does](#what-tpd-does)
- [Key Concepts](#key-concepts)
  - [Two Places, One Set of Files](#two-places-one-set-of-files)
  - [Where Files Actually Live](#where-files-actually-live)
  - [Editing Through Symlinks](#editing-through-symlinks)
  - [The Searchable Directory](#the-searchable-directory)
- [Getting Started](#getting-started)
  - [First-Time Setup](#first-time-setup)
  - [Initializing a Project](#initializing-a-project)
  - [Your First Notes](#your-first-notes)
- [Directory Organization](#directory-organization)
  - [Where to Put What](#where-to-put-what)
  - [Suggested Structure](#suggested-structure)
- [Syncing](#syncing)
  - [Automatic Sync](#automatic-sync)
  - [Manual Sync](#manual-sync)
  - [Handling Conflicts](#handling-conflicts)
- [Team Collaboration](#team-collaboration)
  - [Sharing a Thoughts Repo](#sharing-a-thoughts-repo)
  - [Discovering Teammates' Notes](#discovering-teammates-notes)
- [Profiles](#profiles)
  - [Creating a Profile](#creating-a-profile)
  - [Using a Profile](#using-a-profile)
  - [Managing Profiles](#managing-profiles)
  - [How Profiles Work](#how-profiles-work)
- [Git Worktrees](#git-worktrees)
  - [Disabling Auto-Sync in Worktrees](#disabling-auto-sync-in-worktrees)
- [Configuration](#configuration)
  - [Viewing Config](#viewing-config)
  - [Editing Config](#editing-config)
  - [Config Options](#config-options)
  - [gitIgnore Options](#gitignore-options)
- [Working with AI Assistants](#working-with-ai-assistants)
  - [Claude Code Integration](#claude-code-integration)
    - [Installing Integration](#installing-integration)
    - [Integration Levels](#integration-levels)
    - [What Gets Installed](#what-gets-installed)
    - [Using the Commands](#using-the-commands)
    - [Session Handoffs](#session-handoffs)
    - [Removing Integration](#removing-integration)
- [Compatibility with HumanLayer](#compatibility-with-humanlayer)
  - [Why Two Tools?](#why-two-tools)
  - [What's Shared](#whats-shared)
  - [Switching Between Tools](#switching-between-tools)
  - [Team Compatibility](#team-compatibility)
  - [Config Compatibility](#config-compatibility)
  - [Command Mapping](#command-mapping)
  - [Differences](#differences)

<!-- mtoc-end -->

## What tpd Does

tpd manages developer notes (architecture decisions, TODOs, investigation logs)
separately from your code repositories while making them accessible in every
project.

**Problems it solves:**

- Notes cluttering code repos or getting lost in random files
- Context switching between projects and losing track of decisions
- Sharing team knowledge without polluting git history
- Making notes searchable by AI coding assistants

## Key Concepts

### Two Places, One Set of Files

tpd uses two locations that can be confusing at first:

|                  | Thoughts Repo             | Thoughts Directory            |
| ---------------- | ------------------------- | ----------------------------- |
| **Example**      | `~/thoughts/`             | `~/src/myproject/thoughts/`   |
| **What it is**   | A real git repo           | Symlinks to the thoughts repo |
| **Created by**   | `tpd setup`               | `tpd init`                    |
| **Git behavior** | Normal commits, push/pull | Git-ignored, never committed  |
| **Contains**     | Your actual files         | Only symlinks + hard links    |

**Your files live in the thoughts repo.** The thoughts directory in each project
is just a window into the relevant parts of that repo.

### Where Files Actually Live

```plaintext
~/thoughts/                              # THOUGHTS REPO - files live here
├── repos/
│   ├── myproject/
│   │   ├── {user}/
│   │   │   └── notes.md                 # ← actual file
│   │   └── shared/
│   │       └── architecture.md          # ← actual file
│   └── another-project/
│       └── ...
└── global/
    ├── {user}/
    │   └── snippets.md                  # ← actual file
    └── shared/
        └── team-standards.md            # ← actual file

~/src/myproject/thoughts/                # THOUGHTS DIRECTORY - symlinks
├── {user}/        → ~/thoughts/repos/myproject/{user}/
├── shared/        → ~/thoughts/repos/myproject/shared/
├── global/        → ~/thoughts/global/
└── searchable/                          # hard links (explained below)
```

### Editing Through Symlinks

When you edit `~/src/myproject/thoughts/{user}/notes.md`, you're actually
editing `~/thoughts/repos/myproject/{user}/notes.md` through the symlink.

This means:

- Changes appear immediately in both locations (same file)
- `tpd sync` commits changes to the thoughts repo
- Your code repo never sees the files (they're git-ignored)

### The Searchable Directory

Many tools (including AI assistants) don't follow symlinks when searching. The
`searchable/` directory contains **hard links** to all your thoughts files,
making them discoverable.

```plaintext
thoughts/searchable/
├── {user}/notes.md           # hard link to actual file
├── shared/architecture.md    # hard link to actual file
└── global/{user}/snippets.md # hard link to actual file
```

**Important:**

- Hard links are the same file (editing one edits both)
- Always reference files by their canonical path (e.g.,
  `thoughts/{user}/notes.md`)
- The searchable directory rebuilds on `tpd sync`

## Getting Started

### First-Time Setup

Run once per machine to configure your thoughts repo:

```bash
tpd setup
```

This prompts for:

- **Thoughts repo path** - Where your notes live (default: `~/thoughts`)
- **Username** - Your identifier for personal notes (default: `$USER`)

The thoughts repo is created as a git repo if it doesn't exist.

### Initializing a Project

In any git repository:

```bash
cd ~/src/myproject
tpd init
```

This:

1. Creates the `thoughts/` directory with symlinks
2. Adds `thoughts/` to `.gitignore`
3. Installs git hooks for protection and auto-sync
4. Creates the project structure in your thoughts repo

**Options:**

```bash
tpd init --name custom-name    # Override project name (default: from git remote)
tpd init --profile work        # Use a specific profile
tpd init --force               # Reinitialize existing setup
```

### Your First Notes

```bash
# Create a note
echo "# Project Architecture" > thoughts/{user}/architecture.md

# Check status
tpd status

# Sync to thoughts repo (also happens automatically on commits)
tpd sync -m "Added architecture notes"
```

## Directory Organization

### Where to Put What

| Location                  | Use For                     | Visibility                |
| ------------------------- | --------------------------- | ------------------------- |
| `thoughts/{user}/`        | Your personal project notes | Just you                  |
| `thoughts/shared/`        | Team project notes          | Everyone with repo access |
| `thoughts/global/{user}/` | Your cross-project notes    | Just you                  |
| `thoughts/global/shared/` | Team cross-project notes    | Everyone with repo access |

### Suggested Structure

```plaintext
thoughts/{user}/                    # Personal project notes
├── todo.md                         # Your task list
├── investigations/
│   └── 2024-01-15-auth-bug.md     # Debugging sessions
└── decisions/
    └── api-design.md              # Your design notes

thoughts/shared/                    # Team project notes
├── architecture.md                 # System design
├── onboarding.md                   # Getting started guide
└── decisions/
    └── 2024-01-10-database.md     # Team decisions (ADRs)

thoughts/global/{user}/             # Your cross-project notes
├── snippets.md                     # Reusable code patterns
└── tools.md                        # Tool configurations

thoughts/global/shared/             # Team cross-project notes
├── coding-standards.md
└── review-checklist.md
```

## Syncing

### Automatic Sync

Git hooks installed by `tpd init` handle syncing:

- **Pre-commit hook** - Prevents accidentally committing `thoughts/` to your
  code repo
- **Post-commit hook** - Syncs thoughts to the thoughts repo after each commit

### Manual Sync

```bash
tpd sync                    # Sync with auto-generated message
tpd sync -m "Updated docs"  # Sync with custom message
```

Sync does:

1. Discovers other users' directories (creates symlinks for teammates)
2. Rebuilds the `searchable/` directory
3. Commits all changes to the thoughts repo
4. Pulls and rebases from remote
5. Pushes to remote

### Handling Conflicts

If sync fails due to conflicts:

```bash
# tpd will print instructions like:
cd ~/thoughts
git status          # See conflicting files
# Fix conflicts manually
git rebase --continue
tpd sync            # Retry
```

## Team Collaboration

### Sharing a Thoughts Repo

Push your thoughts repo to a private remote:

```bash
cd ~/thoughts
git remote add origin git@github.com:yourteam/thoughts.git
git push -u origin main
```

Teammates clone it and point their config to it.

### Discovering Teammates' Notes

When a teammate syncs their notes and you run `tpd sync`, their directories
automatically appear:

```plaintext
thoughts/
├── {user}/          # Your notes
├── alice/           # Alice's notes (auto-discovered)
├── bob/             # Bob's notes (auto-discovered)
├── shared/          # Team notes
└── global/
```

This happens because sync checks for new user directories in the thoughts repo
and creates symlinks for them.

## Profiles

Profiles let you maintain separate thoughts repos for different contexts (work
vs personal, different clients).

### Creating a Profile

```bash
tpd profile create work --repo ~/work-thoughts
```

### Using a Profile

```bash
cd ~/src/work-project
tpd init --profile work    # Uses work profile's thoughts repo
```

### Managing Profiles

```bash
tpd profile list              # List all profiles
tpd profile show work         # Show profile details
tpd profile delete work       # Delete a profile
```

### How Profiles Work

Each profile has its own thoughts repo. When you init a project with a profile,
it maps that project to use the profile's repo.

```json
{
  "profiles": {
    "work": { "thoughtsRepo": "~/work-thoughts" }
  },
  "repoMappings": {
    "/home/{user}/src/work-project": { "profile": "work" }
  }
}
```

## Git Worktrees

tpd supports git worktrees. Each worktree needs its own `tpd init`:

```bash
git worktree add ../feature -b feature
cd ../feature
tpd init    # Sets up thoughts directory for this worktree
```

**How it works:**

- Git hooks install to the common git dir (shared across worktrees)
- Symlinks are per-worktree (each worktree has its own `thoughts/`)
- Project name derives from git remote (same across all worktrees)

### Disabling Auto-Sync in Worktrees

If you don't want post-commit sync in worktrees:

```bash
tpd config --edit
# Set "autoSyncInWorktrees": false
```

## Configuration

### Viewing Config

```bash
tpd config              # Pretty print
tpd config --json       # JSON output
```

### Editing Config

```bash
tpd config --edit       # Opens in $EDITOR
```

### Config Options

```json
{
  "thoughtsRepo": "~/thoughts",
  "reposDir": "repos",
  "globalDir": "global",
  "user": "{user}",
  "autoSyncInWorktrees": true,
  "gitIgnore": "project"
}
```

| Option                | Description                       | Default      |
| --------------------- | --------------------------------- | ------------ |
| `thoughtsRepo`        | Path to thoughts repo             | `~/thoughts` |
| `reposDir`            | Subdirectory for project thoughts | `repos`      |
| `globalDir`           | Subdirectory for global thoughts  | `global`     |
| `user`                | Your username (can't be "global") | `$USER`      |
| `autoSyncInWorktrees` | Auto-sync on commits in worktrees | `true`       |
| `gitIgnore`           | Where to ignore `thoughts/`       | `project`    |

### gitIgnore Options

| Value      | Behavior                      |
| ---------- | ----------------------------- |
| `project`  | Add to project's `.gitignore` |
| `local`    | Add to `.git/info/exclude`    |
| `global`   | Add to `~/.config/git/ignore` |
| `disabled` | Don't add anywhere            |

## Working with AI Assistants

The `searchable/` directory makes your thoughts discoverable by AI tools that
don't follow symlinks.

When working with AI assistants:

- Point them to search in `thoughts/searchable/` for finding content
- Reference files by canonical path (e.g., `thoughts/{user}/notes.md`)
- Run `tpd sync` to update searchable directory before AI sessions

### Claude Code Integration

tpd provides deep integration with Claude Code to give AI assistants awareness
of your thoughts directory and enable session continuity.

#### Installing Integration

```bash
tpd claude init              # Install with default options
tpd claude init -i           # Interactive mode
tpd claude init --with-settings  # Also create settings.json
```

#### Integration Levels

When you run `tpd claude init`, you'll be asked how to activate the integration:

| Level                     | Description                                              | Best For                    |
| ------------------------- | -------------------------------------------------------- | --------------------------- |
| **Always-on (CLAUDE.md)** | Adds `@.claude/tpd-instructions.md` to project CLAUDE.md | Teams sharing Claude config |
| **Always-on (local)**     | Creates `.claude/CLAUDE.local.md` (gitignored)           | Personal always-on          |
| **On-demand only**        | Just installs skill/commands                             | Manual activation           |

#### What Gets Installed

**Files copied to `.claude/`:**

- `tpd-instructions.md` - Teaches Claude about thoughts/ structure and usage
- `skills/tpd-integrate.md` - On-demand activation skill
- `commands/tpd-handoff.md` - Create session handoff documents
- `commands/tpd-resume.md` - Resume from handoff documents
- `agents/thoughts-locator.md` - Find documents in thoughts/
- `agents/thoughts-analyzer.md` - Extract insights from documents

#### Using the Commands

| Command              | Purpose                                         |
| -------------------- | ----------------------------------------------- |
| `/tpd-integrate`     | Activate thoughts/ awareness for current task   |
| `/tpd-handoff`       | Create a handoff document when ending a session |
| `/tpd-resume <path>` | Resume work from a handoff document             |

#### Session Handoffs

Handoffs preserve context across Claude Code sessions:

```bash
# At end of session
/tpd-handoff

# Next session (or different person)
/tpd-resume thoughts/shared/handoffs/2024-01-15_10-30-00_feature-work.md
```

The handoff document captures:

- Current git state (branch, commit, uncommitted changes)
- Tasks completed and in-progress
- Key learnings and gotchas
- Next steps

#### Removing Integration

To remove Claude Code integration from a project:

```bash
tpd claude uninit              # Interactive confirmation
tpd claude uninit --force      # Skip confirmation
tpd claude uninit --dry-run    # Preview what would be removed
```

This removes:

- All tpd files from `.claude/` (instructions, skills, commands, agents)
- The `@.claude/tpd-instructions.md` include from CLAUDE.md (if present)
- Gitignore patterns added by init

The `.claude/` directory itself is preserved if it contains other files.

**Note:** Running `tpd uninit` (to remove thoughts/ integration) also removes
Claude integration automatically, ensuring a clean teardown.

## Compatibility with HumanLayer

`tpd` is a Go reimplementation of the `thoughts` subcommand from
[HumanLayer's CLI](https://github.com/humanlayer/humanlayer) (`humanlayer`). The
two tools are fully interoperable.

### Why Two Tools?

| Tool                  | Best For                                                                    |
| --------------------- | --------------------------------------------------------------------------- |
| `tpd`                 | Standalone binary, no runtime dependencies, Go ecosystem                    |
| `humanlayer thoughts` | Already using HumanLayer, Node.js ecosystem, additional humanlayer features |

### What's Shared

Both tools use identical:

| Component               | Location                                                   |
| ----------------------- | ---------------------------------------------------------- |
| Config file             | `~/.config/humanlayer/humanlayer.json`                     |
| Thoughts repo structure | `~/thoughts/repos/<project>/`, `~/thoughts/global/`        |
| Symlink layout          | `thoughts/{user}/`, `thoughts/shared/`, `thoughts/global/` |
| Searchable directory    | `thoughts/searchable/` with hard links                     |
| Git hooks               | Pre-commit protection, post-commit sync                    |

### Switching Between Tools

You can switch tools at any time without migration:

```bash
# Using tpd
tpd init
tpd sync -m "Some notes"

# Later, using humanlayer (same project, same notes)
humanlayer thoughts sync -m "More notes"

# Back to tpd
tpd status
```

### Team Compatibility

Team members can use different tools:

- Alice uses `tpd` (prefers Go binaries)
- Bob uses `humanlayer thoughts` (already has HumanLayer installed)
- Both share the same thoughts repo
- Notes sync correctly regardless of which tool created them

### Config Compatibility

`tpd` reads from HumanLayer's config location for compatibility:

```plaintext
~/.config/humanlayer/humanlayer.json  # Read by both tools
~/.config/tpd/config.json             # tpd also writes here
```

When you run `tpd setup`, it checks for existing HumanLayer config and uses
those settings if found.

### Command Mapping

| tpd          | humanlayer thoughts          | Description              |
| ------------ | ---------------------------- | ------------------------ |
| `tpd setup`  | `humanlayer thoughts setup`  | First-time configuration |
| `tpd init`   | `humanlayer thoughts init`   | Initialize in a project  |
| `tpd sync`   | `humanlayer thoughts sync`   | Sync to thoughts repo    |
| `tpd status` | `humanlayer thoughts status` | Show status              |
| `tpd uninit` | `humanlayer thoughts uninit` | Remove from project      |

### Differences

`tpd` adds some features not in `humanlayer thoughts`:

- Profile management (`tpd profile create/list/show/delete`)
- Claude Code integration (`tpd claude init/uninit`)
- Cross-platform binaries via goreleaser

The core thoughts workflow is identical.
