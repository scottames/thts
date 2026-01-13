# thts Integration Instructions

## thoughts/ Directory Integration

This project uses `thts` to manage a `thoughts/` directory for persistent notes,
research, plans, and context across sessions.

The `thoughts/` directory is git-ignored but always available for your use.

### Directory Structure

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

## Auto-Save Triggers

Save to thoughts/ automatically (without asking) when:

| Trigger                                                          | Location                     | Template                 |
| ---------------------------------------------------------------- | ---------------------------- | ------------------------ |
| **Research completes** - Any research phase produces findings    | `thoughts/shared/research/`  | `.templates/research.md` |
| **Gotchas discovered** - Non-obvious behavior, bugs, workarounds | `thoughts/shared/notes/`     | `.templates/note.md`     |
| **Plans finalized** - After plan mode approval                   | `thoughts/shared/plans/`     | `.templates/plan.md`     |
| **Decisions made** - Architecture/design choices with rationale  | `thoughts/shared/decisions/` | `.templates/decision.md` |
| **Session ending** - Incomplete work that needs handoff          | Suggest `/thts-handoff`      | -                        |

**File naming:** `YYYY-MM-DD-descriptive-name.md`

### Preserving Plans

**Immediately after plan approval** (before starting implementation), copy to thoughts/:

| Agent    | Plan Location       | Action                           |
| -------- | ------------------- | -------------------------------- |
| Claude   | `~/.claude/plans/`  | Copy to `thoughts/shared/plans/` |
| Codex    | Agent's plan output | Save to `thoughts/shared/plans/` |
| OpenCode | Agent's plan output | Save to `thoughts/shared/plans/` |

---

## Before Starting Work

Use the `thoughts-locator` agent to discover relevant documents, then
`thoughts-analyzer` for deep analysis of the most relevant ones.

ALWAYS check thoughts/ for existing context when:

- Beginning research on a topic
- Starting implementation of a feature
- Debugging an issue
- Resuming work from a previous session

---

## While Working

ALWAYS save to thoughts/ when you:

- Discover non-obvious behavior or gotchas
- Make architectural decisions with rationale
- Find important patterns or conventions
- Complete research that others might benefit from

---

## After Completing Work

MUST write to thoughts/ when:

- Finishing research that should be preserved
- Completing a plan that will guide implementation
- Making decisions that should be documented

**Always run `thts sync` after writing to thoughts/.**

---

## Where to Write

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

## Templates

Read the template from `thoughts/.templates/` before writing:

| Document Type | Template Path                     |
| ------------- | --------------------------------- |
| Research      | `thoughts/.templates/research.md` |
| Plan          | `thoughts/.templates/plan.md`     |
| Decision/ADR  | `thoughts/.templates/decision.md` |
| Note/Gotcha   | `thoughts/.templates/note.md`     |

---

## What NOT to Save

Do not write to thoughts/:

- Secrets, credentials, API keys, tokens
- Temporary debugging output you won't need again
- Large generated files or logs
- Content that belongs in repo documentation

---

## Syncing

After writing to thoughts/, always sync:

```bash
thts sync
```

This commits and pushes your changes to the central thoughts repository.

---

## Sub-Agents

When researching, use these specialized agents:

### thoughts-locator

Use to **find** relevant documents in thoughts/. Returns organized list of paths
by category. Does not analyze content deeply.

```text
Prompt: "Find any existing research or notes about [topic]"
```

### thoughts-analyzer

Use to **extract insights** from specific documents. Filters for high-value,
actionable information. Use after locator identifies relevant files.

```text
Prompt: "Analyze thoughts/shared/research/2024-01-15-api-design.md for key decisions"
```

Pattern: Locate first, then analyze the most relevant results.

---

## Available Commands

These commands are available for the user to invoke:

- `/thts-handoff` - Create a handoff document when ending a session
- `/thts-resume` - Resume from a handoff document
- `/thts-integrate` - Explicitly activate thoughts/ integration for current task

Suggest `/thts-handoff` when the user is ending a session with work to continue.
