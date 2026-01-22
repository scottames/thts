# thts User Guide

<!-- mtoc-start -->

- [What thts Does](#what-thts-does)
- [Key Concepts](#key-concepts)
  - [Two Places, One Set of Files](#two-places-one-set-of-files)
  - [Where Files Actually Live](#where-files-actually-live)
  - [Editing Through Symlinks](#editing-through-symlinks)
  - [The Searchable Directory](#the-searchable-directory)
- [Getting Started](#getting-started)
  - [First-Time Setup](#first-time-setup)
  - [Initializing a Project](#initializing-a-project)
  - [Your First Notes](#your-first-notes)
  - [Adding Thoughts with `thts add`](#adding-thoughts-with-thts-add)
- [Directory Organization](#directory-organization)
  - [Where to Put What](#where-to-put-what)
  - [Suggested Structure](#suggested-structure)
- [Syncing](#syncing)
  - [Automatic Sync](#automatic-sync)
  - [Manual Sync](#manual-sync)
  - [Opening in Editor](#opening-in-editor)
  - [Sync Modes](#sync-modes)
  - [Commit Message Templates](#commit-message-templates)
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
  - [Environment Variables](#environment-variables)
  - [gitIgnore Options](#gitignore-options)
- [Shell Completion](#shell-completion)
  - [Loading Completions](#loading-completions)
- [Working with AI Assistants](#working-with-ai-assistants)
  - [AI Agent Integration](#ai-agent-integration)
    - [Supported Agents](#supported-agents)
    - [Installing Integration](#installing-integration)
    - [Global vs Project Configuration](#global-vs-project-configuration)
    - [Prerequisites](#prerequisites)
    - [Integration Levels](#integration-levels)
    - [Customizing Hook Keywords](#customizing-hook-keywords)
    - [What Gets Installed](#what-gets-installed)
    - [Using the Commands](#using-the-commands)
    - [Session Handoffs](#session-handoffs)
    - [Removing Integration](#removing-integration)
- [Compatibility with HumanLayer](#compatibility-with-humanlayer)
  - [Why Two Tools?](#why-two-tools)
  - [What's Shared](#whats-shared)
  - [Config Handling](#config-handling)
  - [Team Compatibility](#team-compatibility)
  - [Command Mapping](#command-mapping)

<!-- mtoc-end -->

## What thts Does

thts manages developer notes (architecture decisions, TODOs, investigation logs)
separately from your code repositories while making them accessible in every
project.

**Problems it solves:**

- Notes cluttering code repos or getting lost in random files
- Context switching between projects and losing track of decisions
- Sharing team knowledge without polluting git history
- Making notes searchable by AI coding assistants

## Key Concepts

### Two Places, One Set of Files

thts uses two locations that can be confusing at first:

|                  | Thoughts Repo             | Thoughts Directory            |
| ---------------- | ------------------------- | ----------------------------- |
| **Example**      | `~/thoughts/`             | `~/src/myproject/thoughts/`   |
| **What it is**   | A real git repo           | Symlinks to the thoughts repo |
| **Created by**   | `thts setup`              | `thts init`                   |
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
- `thts sync` commits changes to the thoughts repo
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
- The searchable directory rebuilds on `thts sync`

## Getting Started

### First-Time Setup

Run once per machine to configure your thoughts repo:

```bash
thts setup
```

This prompts for:

- **Thoughts repo path** - Where your notes live (default: `~/thoughts`)
- **Username** - Your identifier for personal notes (default: `$USER`)

The thoughts repo is created as a git repo if it doesn't exist.

### Initializing a Project

In any git repository:

```bash
cd ~/src/myproject
thts init
```

This:

1. Creates the `thoughts/` directory with symlinks
2. Adds `thoughts/` to `.gitignore`
3. Installs git hooks for protection and auto-sync
4. Creates the project structure in your thoughts repo

**Options:**

```bash
thts init --name custom-name    # Override project name (default: from git remote)
thts init --profile work        # Use a specific profile
thts init --force               # Reinitialize existing setup
```

**Profile assignment:** When you run `thts init`, the current default profile is
explicitly assigned to the repository. This "locks in" the profile so that if
you later change your default profile, existing repositories keep their original
assignment. To see which profile a repository uses, run `thts status`.

### Your First Notes

```bash
# Create a note
echo "# Project Architecture" > thoughts/{user}/architecture.md

# Check status
thts status

# Sync to thoughts repo (also happens automatically on commits)
thts sync -m "Added architecture notes"
```

### Adding Thoughts with `thts add`

The `thts add` command creates properly formatted thought files with templates:

```bash
thts add -t "API design decisions"           # Creates in notes/ (default category)
thts add -t "Feature roadmap" --in plans     # Creates in plans/
thts add -t "Sprint work" --in plans/active  # Creates in plans/active/ subcategory
```

**What it does:**

1. Creates a file named `YYYY-MM-DD-slugified-title.md`
2. Populates it with the appropriate template from `thoughts/.templates/`
3. Opens the file in your editor

**Scope control:**

```bash
thts add -t "Team gotcha" --in notes --shared    # Write to shared/
thts add -t "My todo" --in notes --personal      # Write to {user}/
```

Without flags, scope is determined by:

1. The category's configured scope (e.g., `research` defaults to `shared`)
2. Your `defaultScope` config setting (defaults to `user`)

**Sub-category scope inheritance:**

Sub-categories inherit their parent category's scope unless explicitly
overridden. If a category has `scope: both` (allowing either shared or user),
sub-categories also inherit `both` and will use your `defaultScope` setting.

```yaml
categories:
  plans:
    scope: both # Can be shared or user
    subCategories:
      active: # Inherits "both" from parent
        description: "Active plans"
      complete:
        scope: shared # Explicitly set to shared
        description: "Completed plans"
```

To ensure sub-categories always go to a specific location, set their scope
explicitly rather than relying on inheritance.

**Target selection:**

```bash
thts add -t "Note" --in notes                      # Current repo (or default global)
thts add -t "Work note" --profile work --in notes  # Work profile's global dir
thts add -t "X" --repo ~/other-project --in notes  # Another repo's thoughts dir
```

Target resolution order:

1. `--repo` flag: use that repo's thoughts directory
2. `--profile` flag: use that profile's global thoughts directory
3. Current git repo: use current repo's thoughts directory (if thts initialized)
4. Otherwise: use default profile's global directory

**Content input modes:**

By default, `thts add` opens your editor with the template. For scripted or
piped usage, you can provide content directly:

```bash
# Inline content (positional argument)
thts add -t "memory-issue" "TODO: investigate memory leak" --in notes

# From an existing file
thts add -t "imported plan" --from draft.md --in plans

# From stdin (piped) - auto-detected, no flag needed
echo "Quick note about the bug" | thts add -t "bug-note" --in notes
cat meeting-notes.txt | thts add -t "meeting-2026-01-15" --in notes

# Create file without opening editor
thts add -t "placeholder" --no-edit --in notes
```

Piped input is automatically detected - no need to specify `--stdin` explicitly.
Content sources are mutually exclusive (positional content, `--from`, stdin).
When using any of these, the editor is skipped automatically. Use `--no-edit`
with the default template behavior to create a file without opening the editor.

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
thoughts/{user}/                   # Personal project notes
├── todo.md                        # Your task list
├── investigations/
│   └── 2024-01-15-auth-bug.md     # Debugging sessions
└── decisions/
    └── api-design.md              # Your design notes

thoughts/shared/                   # Team project notes
├── architecture.md                # System design
├── onboarding.md                  # Getting started guide
└── decisions/
    └── 2024-01-10-database.md     # Team decisions (ADRs)

thoughts/global/{user}/            # Your cross-project notes
├── snippets.md                    # Reusable code patterns
└── tools.md                       # Tool configurations

thoughts/global/shared/            # Team cross-project notes
├── coding-standards.md
└── review-checklist.md
```

## Syncing

### Automatic Sync

Git hooks installed by `thts init` handle syncing:

- **Pre-commit hook** - Prevents accidentally committing `thoughts/` to your
  code repo
- **Post-commit hook** - Syncs thoughts to the thoughts repo after each commit

### Manual Sync

```bash
thts sync                    # Sync with auto-generated message
thts sync -m "Updated docs"  # Sync with custom message
```

### Opening in Editor

Open your thoughts directory directly in your editor:

```bash
thts edit                    # Open ./thoughts/ (or default profile if not in repo)
thts edit --profile work     # Open specific profile's thoughts repo
```

**Editor resolution:** Uses config `editor` field, then `$VISUAL`, then
`$EDITOR`. Errors if none set.

Configure a default editor in `~/.config/thts/config.yaml`:

```yaml
editor: nvim
```

Sync does:

1. Discovers other users' directories (creates symlinks for teammates)
2. Rebuilds the `searchable/` directory
3. Commits all changes to the thoughts repo
4. Pulls and rebases from remote
5. Pushes to remote

### Sync Modes

Control remote operations with the `--mode` flag or config:

```bash
thts sync --mode=full    # Pull and push (default)
thts sync --mode=pull    # Pull only, skip push
thts sync --mode=local   # No remote operations
```

| Mode    | Pull | Push | Use Case                                  |
| ------- | ---- | ---- | ----------------------------------------- |
| `full`  | Yes  | Yes  | Normal operation (default)                |
| `pull`  | Yes  | No   | Stay updated, batch pushes for later      |
| `local` | No   | No   | Offline/airplane mode, avoid auth prompts |

Set a default mode in config:

```yaml
sync:
  mode: local
```

When push is skipped, you'll see a warning if there are unpushed commits:

```plaintext
! 2 commits not pushed (local mode)
  Run 'thts sync --mode=full' or 'git push' in ~/thoughts to push
```

### Commit Message Templates

Customize the commit messages used when syncing thoughts using Go text/template
syntax:

```yaml
sync:
  mode: full
  commitMessage: '[{{.Profile}}] {{.Repo}} - {{.Date.Format "2006-01-02"}}'
  commitMessageHook: "Auto-sync ({{.Repo}}): {{.CommitMessage}}"
```

**Available variables:**

| Variable             | Description                                   |
| -------------------- | --------------------------------------------- |
| `{{.Date}}`          | Current time (use `.Format "..."` for custom) |
| `{{.Repo}}`          | Repository name                               |
| `{{.Profile}}`       | Active profile name                           |
| `{{.User}}`          | Your username from config                     |
| `{{.CommitMessage}}` | Triggering commit message (hook only)         |

**Two templates:**

- `commitMessage` - Used for manual sync (`thts sync`)
- `commitMessageHook` - Used for post-commit hook auto-sync
  - Note: the git hook lives in each repo that `thts init` is run on, it
    triggers `thts sync` with a reference to the commit message that triggered
    the git hook

**Per-profile overrides:**

```yaml
profiles:
  work:
    thoughtsRepo: ~/work-thoughts
    sync:
      commitMessage: "[work] {{.Repo}} sync"
      commitMessageHook: "[work] {{.CommitMessage}}"
```

Profile settings override global settings. If not set, defaults are used:

- `commitMessage`:
  `sync: {{.Date.Format "2006-01-02T15:04:05Z07:00"}}`
- `commitMessageHook`: `sync(auto): {{.CommitMessage}}`

### Handling Conflicts

If sync fails due to conflicts:

```bash
# thts will print instructions like:
cd ~/thoughts
git status          # See conflicting files
# Fix conflicts manually
git rebase --continue
thts sync            # Retry
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

When a teammate syncs their notes and you run `thts sync`, their directories
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
thts profile create work --repo ~/work-thoughts
```

### Using a Profile

```bash
cd ~/src/work-project
thts init --profile work    # Uses work profile's thoughts repo
```

### Managing Profiles

```bash
thts profile list              # List all profiles
thts profile show work         # Show profile details
thts profile delete work       # Delete a profile
```

### How Profiles Work

Each profile has its own thoughts repo. When you init a project with a profile,
it maps that project to use the profile's repo.

```yaml
profiles:
  work:
    thoughtsRepo: ~/work-thoughts
repoMappings:
  /home/{user}/src/work-project:
    profile: work
```

## Git Worktrees

thts supports git worktrees. Each worktree needs its own `thts init`:

```bash
git worktree add ../feature -b feature
cd ../feature
thts init    # Sets up thoughts directory for this worktree
```

**How it works:**

- Git hooks install to the common git dir (shared across worktrees)
- Symlinks are per-worktree (each worktree has its own `thoughts/`)
- Project name derives from git remote (same across all worktrees)

### Disabling Auto-Sync in Worktrees

If you don't want post-commit sync in worktrees:

```bash
thts config --edit
# Set "autoSyncInWorktrees": false
```

## Configuration

### Viewing Config

```bash
thts config              # Pretty print
thts config --json       # JSON output
```

### Editing Config

```bash
thts config --edit       # Opens in $EDITOR
```

### Config Options

```yaml
thoughtsRepo: ~/thoughts
reposDir: repos
globalDir: global
user: "{user}"
editor: nvim
autoSyncInWorktrees: true
gitIgnore: project
```

| Option                   | Description                          | Default                      |
| ------------------------ | ------------------------------------ | ---------------------------- |
| `thoughtsRepo`           | Path to thoughts repo                | `~/thoughts`                 |
| `reposDir`               | Subdirectory for project thoughts    | `repos`                      |
| `globalDir`              | Subdirectory for global thoughts     | `global`                     |
| `user`                   | Your username (can't be "global")    | `$USER`                      |
| `editor`                 | Editor for `thts edit`               | `$EDITOR`                    |
| `autoSyncInWorktrees`    | Auto-sync on commits in worktrees    | `true`                       |
| `gitIgnore`              | Where to ignore `thoughts/`          | `project`                    |
| `sync.mode`              | Sync mode: full, pull, or local      | `full`                       |
| `sync.commitMessage`     | Template for manual sync messages    | (see Commit Message section) |
| `sync.commitMessageHook` | Template for hook auto-sync messages | (see Commit Message section) |

### Environment Variables

Environment variables override config file settings. Useful for CI/CD, scripting,
or temporary overrides without modifying config.

| Variable           | Description                       | Overrides             |
| ------------------ | --------------------------------- | --------------------- |
| `THTS_CONFIG_PATH` | Custom path to config file        | Default config path   |
| `THTS_USER`        | Username for thoughts directories | `user` in config      |
| `THTS_PROFILE`     | Default profile to use            | `--profile` flag      |
| `THTS_SYNC_MODE`   | Sync mode (full, pull, local)     | `sync.mode` in config |

**Resolution order** (highest to lowest priority):

1. CLI flag (e.g., `--profile`, `--mode`)
2. Environment variable
3. Config file
4. Default value

**Examples:**

```bash
# Use a different config file
THTS_CONFIG_PATH=~/alt-config.yaml thts status

# Override username for this session
THTS_USER=teammate thts sync

# Use work profile without --profile flag
THTS_PROFILE=work thts init

# Force local-only sync (no network)
THTS_SYNC_MODE=local thts sync
```

**Scripting example:**

```bash
# CI/CD: sync without remote operations
export THTS_SYNC_MODE=local
export THTS_USER=ci-bot
thts sync
```

### gitIgnore Options

| Value      | Behavior                      |
| ---------- | ----------------------------- |
| `project`  | Add to project's `.gitignore` |
| `local`    | Add to `.git/info/exclude`    |
| `global`   | Add to `~/.config/git/ignore` |
| `disabled` | Don't add anywhere            |

## Shell Completion

Generate shell completion scripts for tab completion of commands and flags:

```bash
thts completion bash   # Bash
thts completion zsh    # Zsh
thts completion fish   # Fish
```

### Loading Completions

**Fish:**

```bash
thts completion fish | source
# Persist:
thts completion fish > ~/.config/fish/completions/thts.fish
```

**Bash:**

```bash
source <(thts completion bash)
# Persist:
thts completion bash > /etc/bash_completion.d/thts
```

**Zsh:**

```bash
source <(thts completion zsh)
# Persist (ensure directory is in fpath):
thts completion zsh > "${fpath[1]}/_thts"
```

Completions include dynamic values like profile names and agent types.

## Working with AI Assistants

The `searchable/` directory makes your thoughts discoverable by AI tools that
don't follow symlinks.

When working with AI assistants:

- Point them to search in `thoughts/searchable/` for finding content
- Reference files by canonical path (e.g., `thoughts/{user}/notes.md`)
- Run `thts sync` to update searchable directory before AI sessions

### AI Agent Integration

thts provides deep integration with AI coding agents to give them awareness of
your thoughts directory and enable session continuity.

#### Supported Agents

| Agent       | Project Dir  | Skills Dir          | Commands Dir      | Global Path           |
| ----------- | ------------ | ------------------- | ----------------- | --------------------- |
| Claude Code | `.claude/`   | `skills/`           | `commands/`       | `~/.claude/`          |
| Codex CLI   | `.codex/`    | `skills/*/SKILL.md` | `prompts/` (glob) | `~/.codex/`           |
| OpenCode    | `.opencode/` | `skill/*/SKILL.md`  | `command/`        | `~/.config/opencode/` |

**Key differences:**

- **Codex "prompts"**: Codex calls commands "prompts". They are global-only and
  invoked as `/prompts:<name>` (e.g., `/prompts:thts-handoff`).
- **OpenCode XDG**: OpenCode uses XDG for global config (`~/.config/opencode/`)
  rather than a dot-folder in home.

#### Installing Integration

```bash
thts agents init              # Install for detected agents
thts agents init -i           # Interactive mode
thts agents init --agents claude,codex  # Specify agents
thts agents init --with-settings  # Also create settings files
```

#### Global vs Project Configuration

By default, `thts agents init` installs to project directories (`.claude/`,
`.codex/`, `.opencode/`). You can also install globally:

```bash
thts agents init --global all              # Install everything globally
thts agents init --global skills,commands  # Install specific components
```

**Global paths:**

| Agent    | Global Path           |
| -------- | --------------------- |
| Claude   | `~/.claude/`          |
| Codex    | `~/.codex/`           |
| OpenCode | `~/.config/opencode/` |

**When to use global:**

- Skills/commands you want available in all projects
- Codex prompts (they only work globally)
- When you don't want to modify project files

#### Prerequisites

Hook-based integration requires:

- **jq** - JSON parser for hook scripts (required)
- **yq** - YAML parser for custom keyword configuration (optional)

Install on common systems:

```bash
# macOS
brew install jq yq

# Ubuntu/Debian
sudo apt install jq
pip install yq

# Fedora
sudo dnf install jq yq
```

#### Integration Levels

When you run `thts agents init`, you'll be asked how to activate the
integration:

| Level                        | Config Value           | Description                                | Best For                     |
| ---------------------------- | ---------------------- | ------------------------------------------ | ---------------------------- |
| **Hook-based (recommended)** | `hook`                 | Loads instructions on keyword detection    | Clean CLAUDE.md, low context |
| **Always-on (shared)**       | `agents-content`       | Adds include to project's instruction file | Teams sharing agent config   |
| **Always-on (local)**        | `agents-content-local` | Creates local-only instruction file        | Personal always-on           |
| **On-demand only**           | `on-demand`            | Just installs skill/commands               | Manual activation            |

**Hook-based integration** (default) keeps your CLAUDE.md/AGENTS.md clean by:

1. Injecting a minimal bootstrap (~6 lines) at session start
2. Loading full instructions (~200 lines) only when keywords are detected
3. Keywords include: research, plan, decision, thoughts, handoff, notes, etc.

**Note:** Codex does not support hooks and will automatically fall back to
always-on mode with a warning.

#### Customizing Hook Keywords

The keywords that trigger full instruction loading can be customized in
`~/.config/thts/config.yaml`:

```yaml
hooks:
  keywords:
    - research
    - plan
    - decision
    - thoughts
    - handoff
    # Add your own keywords...
```

Default keywords: research, plan, decision, thoughts, handoff, notes, save,
document, capture, findings, learnings, gotchas, ADR, architecture, resume, wrap
up, end session

#### What Gets Installed

**Files copied to agent directories:**

- `thts-instructions.md` - Teaches the agent about thoughts/ structure
- `skills/thts-integrate.*` - On-demand activation skill
- `commands/thts-handoff.md` - Create session handoff documents
- `commands/thts-resume.md` - Resume from handoff documents
- `agents/thoughts-locator.md` - Find documents in thoughts/
- `agents/thoughts-analyzer.md` - Extract insights from documents

**Note:** Directory names vary by agent (see table above).

#### Using the Commands

| Command               | Purpose                                         |
| --------------------- | ----------------------------------------------- |
| `/thts-integrate`     | Activate thoughts/ awareness for current task   |
| `/thts-handoff`       | Create a handoff document when ending a session |
| `/thts-resume <path>` | Resume work from a handoff document             |

**Codex note:** Use `/prompts:thts-handoff` instead of `/thts-handoff`.

#### Session Handoffs

Handoffs preserve context across sessions:

```bash
# At end of session
/thts-handoff

# Next session (or different person)
/thts-resume thoughts/shared/handoffs/2024-01-15_10-30-00_feature-work.md
```

The handoff document captures:

- Current git state (branch, commit, uncommitted changes)
- Tasks completed and in-progress
- Key learnings and gotchas
- Next steps

#### Removing Integration

To remove agent integration from a project:

```bash
thts agents uninit              # Interactive confirmation
thts agents uninit --force      # Skip confirmation
thts agents uninit --dry-run    # Preview what would be removed
thts agents uninit --global     # Remove global installation
```

This removes:

- All thts files from agent directories (instructions, skills, commands, agents)
- Instruction file modifications
- Gitignore patterns added by init

The agent directory itself is preserved if it contains other files.

**Note:** Running `thts uninit` (to remove thoughts/ integration) also removes
agent integration automatically, ensuring a clean teardown.

## Compatibility with HumanLayer

`thts` is a Go reimplementation of the `thoughts` subcommand from
[HumanLayer's CLI](https://github.com/humanlayer/humanlayer) (`humanlayer`).

### Why Two Tools?

| Tool                  | Best For                                                                    |
| --------------------- | --------------------------------------------------------------------------- |
| `thts`                | Standalone binary, no runtime dependencies, Go ecosystem                    |
| `humanlayer thoughts` | Already using HumanLayer, Node.js ecosystem, additional humanlayer features |

### What's Shared

Both tools use identical:

| Component               | Location                                                   |
| ----------------------- | ---------------------------------------------------------- |
| Thoughts repo structure | `~/thoughts/repos/<project>/`, `~/thoughts/global/`        |
| Symlink layout          | `thoughts/{user}/`, `thoughts/shared/`, `thoughts/global/` |
| Searchable directory    | `thoughts/searchable/` with hard links                     |
| Git hooks               | Pre-commit protection, post-commit sync                    |

### Config Handling

The tools use **different config files**:

| Tool       | Config Path                            | Format |
| ---------- | -------------------------------------- | ------ |
| thts       | `~/.config/thts/config.yaml`           | YAML   |
| humanlayer | `~/.config/humanlayer/humanlayer.json` | JSON   |

**thts can read HumanLayer's config as a fallback**, but never writes to it.
This means:

- Migrating from HumanLayer to thts is seamless—existing config is read
  automatically
- Changes made via thts are written to thts's own config
- HumanLayer won't see config changes made by thts

### Team Compatibility

Team members can use different tools on the same thoughts repo:

- Alice uses `thts` (prefers Go binaries)
- Bob uses `humanlayer thoughts` (already has HumanLayer installed)
- Both share the same thoughts repo
- Notes sync correctly regardless of which tool created them

Each team member maintains their own config file for their preferred tool.

### Command Mapping

| thts          | humanlayer thoughts          | Description                  |
| ------------- | ---------------------------- | ---------------------------- |
| `thts setup`  | `humanlayer thoughts setup`  | First-time configuration     |
| `thts init`   | `humanlayer thoughts init`   | Initialize in a project      |
| `thts sync`   | `humanlayer thoughts sync`   | Sync to thoughts repo        |
| `thts status` | `humanlayer thoughts status` | Show status                  |
| `thts add`    | -                            | Create thought with template |
| `thts edit`   | -                            | Open thoughts in editor      |
| `thts uninit` | `humanlayer thoughts uninit` | Remove from project          |
