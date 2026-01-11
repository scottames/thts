# thts Integration Instructions

## thoughts/ Directory Integration

This project uses `thts` to manage a `thoughts/` directory for persistent notes,
research, plans, and context across sessions.

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
│   └── decisions/       # Architecture/design decisions
├── global/              # Cross-repository thoughts
│   ├── {user}/          # Personal cross-repo notes
│   └── shared/          # Team cross-repo notes
└── searchable/          # Hard links for search tools (read-only)
```

### Path Handling

The `searchable/` directory contains hard links to enable search tools to find
content. When referencing files found there, always report the canonical path:

- `thoughts/searchable/shared/research/api.md` →
  `thoughts/shared/research/api.md`
- `thoughts/searchable/{user}/notes/todo.md` → `thoughts/{user}/notes/todo.md`

Only remove `searchable/` from the path - preserve all other directory
structure.

---

## When to Use thoughts/

### Before Starting Work

Use the `thoughts-locator` agent to discover relevant documents, then
`thoughts-analyzer` for deep analysis of the most relevant ones.

Check thoughts/ for existing context when:

- Beginning research on a topic
- Starting implementation of a feature
- Debugging an issue
- Resuming work from a previous session

### While Working

Consider capturing to thoughts/ when you:

- Discover non-obvious behavior or gotchas
- Make architectural decisions with rationale
- Find important patterns or conventions
- Complete research that others might benefit from

### After Completing Work

Write to thoughts/ when:

- Finishing research that should be preserved
- Completing a plan that will guide implementation
- Ending a session that someone will resume (use handoff)
- Making decisions that should be documented

**Always run `thts sync` after writing to thoughts/.**

---

## Where to Write

| Content Type            | Location                     | When to Use                 |
| ----------------------- | ---------------------------- | --------------------------- |
| Quick notes, scratchpad | `thoughts/{user}/notes/`     | Personal, informal notes    |
| Research findings       | `thoughts/shared/research/`  | Findings others should see  |
| Implementation plans    | `thoughts/shared/plans/`     | Plans guiding future work   |
| Session handoffs        | `thoughts/shared/handoffs/`  | Use `/thts-handoff` command |
| Decisions/ADRs          | `thoughts/shared/decisions/` | Architectural decisions     |
| Ticket context          | `thoughts/{user}/tickets/`   | Personal ticket notes       |

---

## Output Formats

### Research Documents

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

### Implementation Plans

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

### Quick Notes

Location: `thoughts/{user}/notes/YYYY-MM-DD-description.md` unless specified by
the user.

### Decisions/ADRs

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
