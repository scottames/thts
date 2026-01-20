#!/usr/bin/env bash
# thts session start hook - injects bootstrap instructions
# Requires: jq

set -euo pipefail

# Read session info from stdin (JSON)
INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty')

# Mark session as not expanded yet
EXPANDED_FILE="/tmp/thts-expanded-${SESSION_ID}"
rm -f "$EXPANDED_FILE" 2>/dev/null || true

# Output bootstrap context
cat << 'BOOTSTRAP'
## thts Integration (Bootstrap)

This project uses thts for managing thoughts/notes. The thoughts/ directory
is available for research, plans, decisions, and handoffs.

**Quick reference:**
- `/thts-handoff` - Create session handoff document
- `/thts-resume <path>` - Resume from handoff
- `thoughts/searchable/` - Search-friendly hard links

Full instructions load automatically when relevant keywords are detected.
BOOTSTRAP
