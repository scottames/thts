# thts

<!-- NOTE: CLAUDE.md symlinks to this file. Only edit AGENTS.md. -->

Go CLI for managing developer thoughts/notes across repositories. Replicates
HumanLayer's `thoughts` subcommand with full feature compatibility.

## Commands

| Command              | Description                                         |
| -------------------- | --------------------------------------------------- |
| `thts setup`         | Initial setup - configure thoughts repo location    |
| `thts init`          | Initialize thoughts in current git repo             |
| `thts sync`          | Sync thoughts to central repo                       |
| `thts status`        | Show thoughts status                                |
| `thts uninit`        | Remove thoughts from current repo                   |
| `thts config`        | View/edit configuration                             |
| `thts edit`          | Open thoughts directory in editor                   |
| `thts profile`       | Manage profiles (create/list/show/delete)           |
| `thts init agents`   | Install agent integration (claude, codex, opencode) |
| `thts uninit agents` | Remove agent integration from project               |
| `thts completion`    | Generate shell completion scripts (bash/zsh/fish)   |

## Project Structure

```plaintext
cmd/thts/          # Entry point
internal/
  agents/          # Agent type registry and detection
  cmd/             # Cobra commands
  config/          # Config loading/saving, paths, types
  fs/              # Filesystem utilities (symlinks, gitignore)
  git/             # Git operations, hooks
  thts/            # Searchable directory (hard links)
instructions/      # Embedded: AGENTS.md (shared instructions)
skills/            # Embedded: skills per agent (claude/codex/opencode)
commands/          # Embedded: thts-handoff.md, thts-resume.md per agent
agents/            # Embedded: thoughts-locator.md, thoughts-analyzer.md
embed.go           # Go embed declarations for above
```

## UI Standards

Terminal output uses the `internal/ui` package for consistency.

### Message Types

| Function       | Symbol | Color  | Usage               |
| -------------- | ------ | ------ | ------------------- |
| `ui.Success()` | `✓`    | Green  | Operation completed |
| `ui.Info()`    | `ℹ`    | Blue   | Informational       |
| `ui.Warning()` | `⚠`    | Yellow | Non-fatal issues    |
| `ui.Error()`   | `✗`    | Red    | Fatal errors        |

### Headers and Sections

- `ui.Header("Title")` - Bordered box for major sections
- `ui.SubHeader("Section:")` - Bold text for subsections

### Tables

- `ui.NewTable("Col1", "Col2")` - For structured data display
- `ui.KeyValueTable(rows)` - For key-value pairs

### Text Styling

- `ui.Accent(text)` - Cyan, for paths and commands
- `ui.Muted(text)` - Gray, for secondary info
- `ui.Bullet(text)` - Indented bullet point

## Agent Integration Files

Files in `instructions/`, `skills/`, `commands/`, `agents/` are embedded in the
binary and copied to agent directories by `thts init agents`.

### Supported Agents

| Agent    | Directory    | Skill Format        | Commands Dir        | Settings File   |
| -------- | ------------ | ------------------- | ------------------- | --------------- |
| Claude   | `.claude/`   | `skills/*.md`       | `commands/`         | `settings.json` |
| Codex    | `.codex/`    | `skills/*/SKILL.md` | `prompts/` (global) | `config.toml`   |
| OpenCode | `.opencode/` | `skills/*/SKILL.md` | `commands/`         | `opencode.json` |
| Gemini   | `.gemini/`   | `skills/*/SKILL.md` | `commands/*.toml`   | `settings.json` |

**Note:** Codex calls commands "prompts" and they are global-only (`~/.codex/prompts/`).
OpenCode uses XDG for global config (`~/.config/opencode/`).
Gemini uses TOML format for commands and doesn't support the agents feature.

### Hook Support

| Agent    | Hook Support | Hook Events                             | Settings File         |
| -------- | ------------ | --------------------------------------- | --------------------- |
| Claude   | Yes          | `SessionStart`, `UserPromptSubmit`      | `settings.local.json` |
| Gemini   | Yes          | `SessionStart`, `BeforeAgent`           | `settings.local.json` |
| OpenCode | Yes (plugin) | `session.created`, `session.compacting` | N/A (plugin)          |
| Codex    | No           | N/A                                     | N/A                   |

### Embedded Files

| File                    | Purpose                       | Agent Support  |
| ----------------------- | ----------------------------- | -------------- |
| `AGENTS.md`             | Shared thoughts/ instructions | All            |
| `thts-integrate`        | On-demand activation skill    | All            |
| `thts-handoff.md`       | Session handoff command       | All            |
| `thts-resume.md`        | Resume from handoff command   | All            |
| `thoughts-locator.md`   | Find documents agent          | All            |
| `thoughts-analyzer.md`  | Analyze documents agent       | All            |
| `thts-session-start.sh` | Hook: bootstrap instructions  | Claude, Gemini |
| `thts-prompt-check.sh`  | Hook: keyword detection       | Claude, Gemini |
| `thts-integration.ts`   | Plugin: instruction injection | OpenCode       |

## Reference

Based on [HumanLayer CLI](https://github.com/humanlayer/humanlayer)'s `thoughts`
subcommand.

**IMPORTANT**: `thts` should always be feature-compatible with humanlayer's
`thoughts` subcommand so users can switch between them.

## Environment Variables

Override config without modifying files. Priority: flag > env var > config > default.

| Variable           | Description                        | Example                          |
| ------------------ | ---------------------------------- | -------------------------------- |
| `THTS_CONFIG_PATH` | Custom config file path            | `THTS_CONFIG_PATH=~/alt.yaml`    |
| `THTS_USER`        | Override username                  | `THTS_USER=testuser thts status` |
| `THTS_PROFILE`     | Default profile (like `--profile`) | `THTS_PROFILE=work thts init`    |
| `THTS_SYNC_MODE`   | Sync mode: full, pull, local       | `THTS_SYNC_MODE=local thts sync` |

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
go build -o thts ./cmd/thts
```

### Testing

Tests should not be an afterthought or skipped.

```bash
go test ./...                                    # Unit tests
go test -tags=integration ./internal/cmd/...     # Integration tests
go test -tags=integration ./...                  # All tests
```

**Coverage targets:** config/git >70%, fs/thts >60% (all met)

#### Verification Scripts

For CLI behavior that unit tests can't easily cover (flag interactions,
stdout/stderr separation, output formats), create `scripts/verify-<feature>.sh`.

See `thoughts/shared/notes/2026-01-21-verification-script-pattern.md` for the
pattern and gotchas (especially around `set -e` and capturing expected errors).

## Documentation

**Any relevant docs should be updated or created to consider any work
"complete".**

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
| Created by | `thts setup`                    | `thts init`                        |

Editing a file in the thoughts directory actually edits the file in the thoughts
repo through the symlink. This should be called out when relevant.

## Dogfooding

We're dogfooding `thts` in this repo - capture any bugs/issues/learnings to
thoughts/shared/notes/2026-01-11-thts-meta-testing.md

<!-- thts-start -->

## thts Integration Instructions

### thoughts/ Directory Integration

This project uses `thts` to manage a `thoughts/` directory for persistent notes,
research, plans, and context across sessions.

The `thoughts/` directory is git-ignored but always available for your use.

#### Directory Structure

```plaintext
thoughts/
├── {user}/              # Your personal notes for this repo
│   ├── notes/           # Quick notes, scratchpad
│   ├── tickets/         # Ticket documentation
│   └── ...
├── shared/              # Team-shared documents
│   ├── research/        # Research findings
│   ├── plans/           # Implementation plans
│   ├── handoffs/        # Session handoff documents
│   ├── notes/           # Shared notes, gotchas, learnings
│   └── decisions/       # Architecture/design decisions
├── global/              # Cross-repository thoughts
│   ├── {user}/          # Personal cross-repo notes
│   └── shared/          # Team cross-repo notes
├── .templates/          # Document templates (read these when writing)
└── searchable/          # Hard links for search tools (read-only)
```

**Note:** `{user}` is your username from thts config (`~/.config/thts/config.yaml`).

**searchable/ paths:** When referencing files from `thoughts/searchable/`, report
the canonical path (remove `searchable/` from the path).

---

### Auto-Save Triggers

Save to thoughts/ automatically (without asking) when:

| Trigger                                                          | Location                     | Template                 |
| ---------------------------------------------------------------- | ---------------------------- | ------------------------ |
| **Research completes** - Any research phase produces findings    | `thoughts/shared/research/`  | `.templates/research.md` |
| **Gotchas discovered** - Non-obvious behavior, bugs, workarounds | `thoughts/shared/notes/`     | `.templates/note.md`     |
| **Plans finalized** - After plan mode approval                   | `thoughts/shared/plans/`     | `.templates/plan.md`     |
| **Decisions made** - Architecture/design choices with rationale  | `thoughts/shared/decisions/` | `.templates/decision.md` |
| **Session ending** - Incomplete work that needs handoff          | Suggest `/thts-handoff`      | -                        |

**File naming:** `YYYY-MM-DD-descriptive-name.md`

#### Preserving Plans

When a plan is finalized, copy it to thoughts/ to preserve beyond the session:

| Agent    | Plan Location       | Action                           |
| -------- | ------------------- | -------------------------------- |
| Claude   | `~/.claude/plans/`  | Copy to `thoughts/shared/plans/` |
| Codex    | Agent's plan output | Save to `thoughts/shared/plans/` |
| OpenCode | Agent's plan output | Save to `thoughts/shared/plans/` |

---

### Before Starting Work

Before starting, do a quick triage:

1. If the task is research, debugging, resume/handoff, or history-heavy, use
   `thoughts-locator` to discover relevant documents, then `thoughts-analyzer`
   for deep analysis of the most relevant ones.
2. If the task is straightforward and localized, proceed without thoughts
   agents.
3. If it's unclear, ask the user whether to include thoughts context. Use the
   Ask question tool if available; otherwise ask one concise question directly.

Context lookup is usually useful when:

- Beginning research on a topic
- Debugging an issue
- Resuming work from a previous session
- Working on tasks that depend on prior decisions or repository history

---

### While Working

ALWAYS save to thoughts/ when you:

- Discover non-obvious behavior or gotchas
- Make architectural decisions with rationale
- Find important patterns or conventions
- Complete research that others might benefit from

---

### After Completing Work

MUST write to thoughts/ when:

- Finishing research that should be preserved
- Completing a plan that will guide implementation
- Making decisions that should be documented

**Always run `thts sync` after writing to thoughts/.**

---

### Where to Write

**Path shorthand:** When the user says `thts/<path>` or `thoughts/<path>`, resolve using:

- Default: project-specific, shared (e.g., `thts/notes` → `thoughts/shared/notes/`)
- `my/` = personal scope (e.g., `thts/my/notes` → `thoughts/{user}/notes/`)
- `global/` = cross-project (e.g., `thts/global/notes` → `thoughts/global/shared/notes/`)
- Combine: `thts/global/my/notes` → `thoughts/global/{user}/notes/`

| Content Type            | Location                     |
| ----------------------- | ---------------------------- |
| Implementation plans    | `thoughts/shared/plans/`     |
| Research findings       | `thoughts/shared/research/`  |
| Session handoffs        | `thoughts/shared/handoffs/`  |
| Decisions/ADRs          | `thoughts/shared/decisions/` |
| Gotchas/learnings       | `thoughts/shared/notes/`     |
| Quick notes, scratchpad | `thoughts/{user}/notes/`     |
| Ticket context          | `thoughts/{user}/tickets/`   |

---

### Templates

Read the template from `thoughts/.templates/` before writing:

| Document Type | Template Path                     |
| ------------- | --------------------------------- |
| Research      | `thoughts/.templates/research.md` |
| Plan          | `thoughts/.templates/plan.md`     |
| Decision/ADR  | `thoughts/.templates/decision.md` |
| Note/Gotcha   | `thoughts/.templates/note.md`     |

---

### What NOT to Save

Do not write to thoughts/:

- Secrets, credentials, API keys, tokens
- Temporary debugging output you won't need again
- Large generated files or logs
- Content that belongs in repo documentation

---

### Syncing

After writing to thoughts/, always sync:

```bash
thts sync
```

This commits and pushes your changes to the central thoughts repository.

---

### Sub-Agents

When researching, use these specialized agents:

#### thoughts-locator

Use to **find** relevant documents in thoughts/. Returns organized list of paths
by category. Does not analyze content deeply.

```text
Prompt: "Find any existing research or notes about [topic]"
```

#### thoughts-analyzer

Use to **extract insights** from specific documents. Filters for high-value,
actionable information. Use after locator identifies relevant files.

```text
Prompt: "Analyze thoughts/shared/research/2024-01-15-api-design.md for key decisions"
```

Pattern: Locate first, then analyze the most relevant results.

---

### Available Commands

These commands are available for the user to invoke:

- `/thts-handoff` - Create a handoff document when ending a session
- `/thts-resume` - Resume from a handoff document
- `/thts-integrate` - Explicitly activate thoughts/ integration for current task

Suggest `/thts-handoff` when the user is ending a session with work to continue.

<!-- thts-end -->
