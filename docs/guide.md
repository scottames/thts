# tpd User Guide

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
│   │   ├── scotty/
│   │   │   └── notes.md                 # ← actual file
│   │   └── shared/
│   │       └── architecture.md          # ← actual file
│   └── another-project/
│       └── ...
└── global/
    ├── scotty/
    │   └── snippets.md                  # ← actual file
    └── shared/
        └── team-standards.md            # ← actual file

~/src/myproject/thoughts/                # THOUGHTS DIRECTORY - symlinks
├── scotty/        → ~/thoughts/repos/myproject/scotty/
├── shared/        → ~/thoughts/repos/myproject/shared/
├── global/        → ~/thoughts/global/
└── searchable/                          # hard links (explained below)
```

### Editing Through Symlinks

When you edit `~/src/myproject/thoughts/scotty/notes.md`, you're actually
editing `~/thoughts/repos/myproject/scotty/notes.md` through the symlink.

This means:

- Changes appear immediately in both locations (same file)
- `tpd sync` commits changes to the thoughts repo
- Your code repo never sees the files (they're git-ignored)

### The Searchable Directory

Many tools (including AI assistants) don't follow symlinks when searching.
The `searchable/` directory contains **hard links** to all your thoughts files,
making them discoverable.

```plaintext
thoughts/searchable/
├── scotty/notes.md           # hard link to actual file
├── shared/architecture.md    # hard link to actual file
└── global/scotty/snippets.md # hard link to actual file
```

**Important:**

- Hard links are the same file (editing one edits both)
- Always reference files by their canonical path (e.g., `thoughts/scotty/notes.md`)
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
echo "# Project Architecture" > thoughts/scotty/architecture.md

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
thoughts/scotty/                    # Personal project notes
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

thoughts/global/scotty/             # Your cross-project notes
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
├── scotty/          # Your notes
├── alice/           # Alice's notes (auto-discovered)
├── bob/             # Bob's notes (auto-discovered)
├── shared/          # Team notes
└── global/
```

This happens because sync checks for new user directories in the thoughts repo
and creates symlinks for them.

## Profiles

Profiles let you maintain separate thoughts repos for different contexts
(work vs personal, different clients).

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
    "/home/scotty/src/work-project": { "profile": "work" }
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
  "user": "scotty",
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
- Reference files by canonical path (e.g., `thoughts/scotty/notes.md`)
- Run `tpd sync` to update searchable directory before AI sessions
