package config

// DefaultCategories returns the default category configuration.
// These match HumanLayer's default structure for compatibility.
func DefaultCategories() map[string]*Category {
	return map[string]*Category{
		"research": {
			Description: "Research findings and analysis",
			Trigger:     "Any research phase produces findings",
			Template:    "research.md",
			Scope:       CategoryScopeShared,
		},
		"plans": {
			Description: "Implementation plans and design docs",
			Trigger:     "After plan mode approval",
			Template:    "plan.md",
			Scope:       CategoryScopeShared,
		},
		"handoffs": {
			Description: "Session handoff documents",
			Scope:       CategoryScopeShared,
		},
		"decisions": {
			Description: "Architecture and design decisions",
			Template:    "decision.md",
			Scope:       CategoryScopeShared,
		},
		"notes": {
			Description: "Quick notes, gotchas, learnings",
			Trigger:     "Non-obvious behavior, bugs, workarounds discovered",
			Template:    "note.md",
			Scope:       CategoryScopeBoth,
		},
	}
}

// DefaultScope is the default scope for new thoughts.
const DefaultScopeValue = ScopeUser

// DefaultCommitMessage returns the default commit message template for manual sync.
func DefaultCommitMessage() string {
	return `sync: {{.Date.Format "2006-01-02T15:04:05Z07:00"}}`
}

// DefaultCommitMessageHook returns the default commit message template for hook auto-sync.
func DefaultCommitMessageHook() string {
	return "sync(auto): {{.CommitMessage}}"
}
