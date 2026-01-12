# thts

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
| `thts profile`       | Manage profiles (create/list/show/delete)           |
| `thts agents init`   | Install agent integration (claude, codex, opencode) |
| `thts agents uninit` | Remove agent integration from project               |

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
commands/          # Embedded: thts-handoff.md, thts-resume.md (claude only)
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
binary and copied to agent directories by `thts agents init`.

### Supported Agents

| Agent    | Directory    | Skill Format        | Settings File   |
| -------- | ------------ | ------------------- | --------------- |
| Claude   | `.claude/`   | `skills/*.md`       | `settings.json` |
| Codex    | `.codex/`    | `skills/*/SKILL.md` | `config.toml`   |
| OpenCode | `.opencode/` | `skill/*/SKILL.md`  | `opencode.json` |

### Embedded Files

| File                   | Purpose                       | Agent Support |
| ---------------------- | ----------------------------- | ------------- |
| `AGENTS.md`            | Shared thoughts/ instructions | All           |
| `thts-integrate`       | On-demand activation skill    | All           |
| `thts-handoff.md`      | Session handoff command       | Claude only   |
| `thts-resume.md`       | Resume from handoff command   | Claude only   |
| `thoughts-locator.md`  | Find documents agent          | All           |
| `thoughts-analyzer.md` | Analyze documents agent       | All           |

## Reference

Based on [HumanLayer CLI](https://github.com/humanlayer/humanlayer)'s `thoughts`
subcommand.

**IMPORTANT**: `thts` should always be feature-compatible with humanlayer's
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
│   └── decisions/       # Architecture/design decisions
├── global/              # Cross-repository thoughts
│   ├── {user}/          # Personal cross-repo notes
│   └── shared/          # Team cross-repo notes
└── searchable/          # Hard links for search tools (read-only)
```

#### Path Handling

The `searchable/` directory contains hard links to enable search tools to find
content. When referencing files found there, always report the canonical path:

- `thoughts/searchable/shared/research/api.md` →
  `thoughts/shared/research/api.md`
- `thoughts/searchable/{user}/notes/todo.md` → `thoughts/{user}/notes/todo.md`

Only remove `searchable/` from the path - preserve all other directory
structure.

---

### When to Use thoughts/

#### Before Starting Work

Use the `thoughts-locator` agent to discover relevant documents, then
`thoughts-analyzer` for deep analysis of the most relevant ones.

Check thoughts/ for existing context when:

- Beginning research on a topic
- Starting implementation of a feature
- Debugging an issue
- Resuming work from a previous session

#### While Working

Consider capturing to thoughts/ when you:

- Discover non-obvious behavior or gotchas
- Make architectural decisions with rationale
- Find important patterns or conventions
- Complete research that others might benefit from

#### After Completing Work

Write to thoughts/ when:

- Finishing research that should be preserved
- Completing a plan that will guide implementation
- Ending a session that someone will resume (use handoff)
- Making decisions that should be documented

**Always run `thts sync` after writing to thoughts/.**

---

### Where to Write

| Content Type            | Location                     | When to Use                 |
| ----------------------- | ---------------------------- | --------------------------- |
| Quick notes, scratchpad | `thoughts/{user}/notes/`     | Personal, informal notes    |
| Research findings       | `thoughts/shared/research/`  | Findings others should see  |
| Implementation plans    | `thoughts/shared/plans/`     | Plans guiding future work   |
| Session handoffs        | `thoughts/shared/handoffs/`  | Use `/thts-handoff` command |
| Decisions/ADRs          | `thoughts/shared/decisions/` | Architectural decisions     |
| Ticket context          | `thoughts/{user}/tickets/`   | Personal ticket notes       |

---

### Output Formats

#### Research Documents

Location: `thoughts/shared/research/YYYY-MM-DD-description.md`

```markdown
---
date: [ISO timestamp with timezone]
researcher: [your name]
topic: "[Research topic/question]"
tags: [relevant, component, names]
status: complete
---

# Research: [Topic]

## Research Question

[Original question or area of investigation]

## Summary

[High-level findings - what was discovered]

## Detailed Findings

### [Component/Area]

[Findings with file:line references where applicable]

## Code References

- `path/to/file.ext:123` - Description
- `another/file.ts:45-67` - Description

## Historical Context

[Relevant insights from other thoughts/ documents, if any]

## Open Questions

[Areas needing further investigation, if any]
```

#### Implementation Plans

Location: `thoughts/shared/plans/YYYY-MM-DD-description.md`

```markdown
---
date: [ISO timestamp]
author: [your name]
topic: "[Feature/Task] Implementation Plan"
tags: [implementation, relevant, components]
status: draft|in_progress|complete
---

# [Feature/Task] Implementation Plan

## Overview

[Brief description of what we're implementing and why]

## Current State

[What exists now, key constraints]

## Desired End State

[What success looks like, how to verify]

## What We're NOT Doing

[Explicit out-of-scope items]

## Implementation Phases

### Phase 1: [Name]

**Changes:**

- `path/to/file.ext` - [description of changes]

**Success Criteria:**

- [ ] [Verifiable criterion]
- [ ] [Another criterion]

### Phase 2: [Name]

[Continue pattern...]

## Testing Strategy

[How to verify the implementation works]
```

#### Quick Notes

Location: `thoughts/{user}/notes/YYYY-MM-DD-description.md` unless specified by
the user.

#### Decisions/ADRs

Location: `thoughts/shared/decisions/YYYY-MM-DD-description.md`

```markdown
---
date: [ISO timestamp]
author: [your name]
status: proposed|accepted|deprecated
---

# Decision: [Title]

## Context

[What situation led to this decision]

## Decision

[What was decided]

## Rationale

[Why this choice over alternatives]

## Consequences

[What this enables/prevents going forward]
```

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
