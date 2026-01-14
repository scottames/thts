package thtsfiles

import (
	"strings"
	"testing"
)

func TestGetInstructions_DefaultCategories(t *testing.T) {
	data := InstructionsData{
		User: "testuser",
		Categories: []CategoryRow{
			{
				Name:        "decisions",
				Description: "Architecture decisions",
				Location:    "`thoughts/shared/decisions/`",
				Template:    "decision.md",
			},
			{
				Name:        "notes",
				Description: "Quick notes",
				Location:    "`thoughts/shared/notes/` or `thoughts/{user}/notes/`",
				Trigger:     "Gotchas discovered",
				Template:    "note.md",
			},
			{
				Name:        "plans",
				Description: "Implementation plans",
				Location:    "`thoughts/shared/plans/`",
				Trigger:     "After plan approval",
				Template:    "plan.md",
			},
			{
				Name:        "research",
				Description: "Research findings",
				Location:    "`thoughts/shared/research/`",
				Trigger:     "Research completes",
				Template:    "research.md",
			},
		},
	}

	result, err := GetInstructions(data)
	if err != nil {
		t.Fatalf("GetInstructions failed: %v", err)
	}

	// Verify static content preserved
	if !strings.Contains(result, "# thts Integration Instructions") {
		t.Error("missing header")
	}
	if !strings.Contains(result, "## Before Starting Work") {
		t.Error("missing static section")
	}

	// Verify Auto-Save Triggers table populated
	if !strings.Contains(result, "**Gotchas discovered**") {
		t.Error("missing trigger in Auto-Save table")
	}
	if !strings.Contains(result, "**After plan approval**") {
		t.Error("missing plans trigger")
	}

	// Verify Where to Write table populated
	if !strings.Contains(result, "| Architecture decisions |") {
		t.Error("missing category in Where to Write table")
	}
	if !strings.Contains(result, "`thoughts/shared/decisions/`") {
		t.Error("missing location in Where to Write table")
	}

	// Verify Templates table populated
	if !strings.Contains(result, "`thoughts/.templates/decision.md`") {
		t.Error("missing template path")
	}

	// Verify categories without triggers don't appear in trigger table
	// decisions has no trigger, so shouldn't be in Auto-Save section
	// But we need to check context - it should be in Where to Write but not Auto-Save triggers
	autoSaveSection := extractSection(result, "## Auto-Save Triggers", "## Before Starting Work")
	if strings.Contains(autoSaveSection, "Architecture decisions") {
		t.Error("category without trigger appeared in Auto-Save table")
	}
}

func TestGetInstructions_CustomCategories(t *testing.T) {
	data := InstructionsData{
		User: "testuser",
		Categories: []CategoryRow{
			{
				Name:        "tickets",
				Description: "Ticket documentation",
				Location:    "`thoughts/{user}/tickets/`",
			},
		},
	}

	result, err := GetInstructions(data)
	if err != nil {
		t.Fatalf("GetInstructions failed: %v", err)
	}

	// Verify custom category appears
	if !strings.Contains(result, "Ticket documentation") {
		t.Error("missing custom category description")
	}
	if !strings.Contains(result, "`thoughts/{user}/tickets/`") {
		t.Error("missing custom category location")
	}
}

func TestGetInstructions_SubCategories(t *testing.T) {
	data := InstructionsData{
		User: "testuser",
		Categories: []CategoryRow{
			{
				Name:        "plans",
				Description: "Implementation plans",
				Location:    "`thoughts/shared/plans/`",
				Template:    "plan.md",
			},
			{
				Name:        "plans/active",
				Description: "Active plans",
				Location:    "`thoughts/shared/plans/active/`",
			},
			{
				Name:        "plans/complete",
				Description: "Completed plans",
				Location:    "`thoughts/shared/plans/complete/`",
				Trigger:     "Implementation verified complete",
			},
		},
	}

	result, err := GetInstructions(data)
	if err != nil {
		t.Fatalf("GetInstructions failed: %v", err)
	}

	// Verify sub-categories appear
	if !strings.Contains(result, "Active plans") {
		t.Error("missing sub-category: plans/active")
	}
	if !strings.Contains(result, "Completed plans") {
		t.Error("missing sub-category: plans/complete")
	}
	if !strings.Contains(result, "`thoughts/shared/plans/complete/`") {
		t.Error("missing sub-category location")
	}
}

func TestGetInstructions_EmptyCategories(t *testing.T) {
	data := InstructionsData{
		User:       "testuser",
		Categories: []CategoryRow{},
	}

	result, err := GetInstructions(data)
	if err != nil {
		t.Fatalf("GetInstructions failed: %v", err)
	}

	// Should still render static content
	if !strings.Contains(result, "# thts Integration Instructions") {
		t.Error("missing header with empty categories")
	}

	// Tables should be empty but headers present
	if !strings.Contains(result, "## Where to Write") {
		t.Error("missing Where to Write section")
	}
}

func TestGetInstructions_BothScope(t *testing.T) {
	data := InstructionsData{
		User: "testuser",
		Categories: []CategoryRow{
			{
				Name:        "notes",
				Description: "Quick notes",
				Location:    "`thoughts/shared/notes/` or `thoughts/{user}/notes/`",
			},
		},
	}

	result, err := GetInstructions(data)
	if err != nil {
		t.Fatalf("GetInstructions failed: %v", err)
	}

	if !strings.Contains(result, "`thoughts/shared/notes/` or `thoughts/{user}/notes/`") {
		t.Error("missing 'both' scope location format")
	}
}

// extractSection extracts content between two headers.
func extractSection(content, startHeader, endHeader string) string {
	startIdx := strings.Index(content, startHeader)
	if startIdx == -1 {
		return ""
	}
	endIdx := strings.Index(content[startIdx+len(startHeader):], endHeader)
	if endIdx == -1 {
		return content[startIdx:]
	}
	return content[startIdx : startIdx+len(startHeader)+endIdx]
}
