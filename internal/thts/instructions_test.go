package thts

import (
	"testing"

	"github.com/scottames/thts/internal/config"
)

func TestBuildInstructionsData_DefaultCategories(t *testing.T) {
	cfg := &config.Config{
		User: "testuser",
	}

	data := BuildInstructionsData(cfg)

	if data.User != "testuser" {
		t.Errorf("expected User 'testuser', got %q", data.User)
	}

	// Should have 5 default categories
	if len(data.Categories) != 5 {
		t.Errorf("expected 5 categories, got %d", len(data.Categories))
	}

	// Verify categories are sorted
	for i := 1; i < len(data.Categories); i++ {
		if data.Categories[i-1].Name >= data.Categories[i].Name {
			t.Errorf("categories not sorted: %q >= %q", data.Categories[i-1].Name, data.Categories[i].Name)
		}
	}
}

func TestBuildInstructionsData_CustomCategories(t *testing.T) {
	cfg := &config.Config{
		User: "testuser",
		Categories: map[string]*config.Category{
			"tickets": {
				Description: "Ticket documentation",
				Scope:       config.CategoryScopeUser,
			},
			"ideas": {
				Description: "Future ideas",
				Scope:       config.CategoryScopeShared,
			},
		},
	}

	data := BuildInstructionsData(cfg)

	if len(data.Categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(data.Categories))
	}

	// Find tickets category
	var tickets, ideas *struct {
		Name     string
		Location string
	}
	for _, cat := range data.Categories {
		if cat.Name == "tickets" {
			tickets = &struct {
				Name     string
				Location string
			}{cat.Name, cat.Location}
		}
		if cat.Name == "ideas" {
			ideas = &struct {
				Name     string
				Location string
			}{cat.Name, cat.Location}
		}
	}

	if tickets == nil {
		t.Fatal("tickets category not found")
	}
	if ideas == nil {
		t.Fatal("ideas category not found")
	}

	// Verify scope-based locations
	expectedTicketsLoc := "`thoughts/{user}/tickets/`"
	if tickets.Location != expectedTicketsLoc {
		t.Errorf("tickets location: expected %q, got %q", expectedTicketsLoc, tickets.Location)
	}

	expectedIdeasLoc := "`thoughts/shared/ideas/`"
	if ideas.Location != expectedIdeasLoc {
		t.Errorf("ideas location: expected %q, got %q", expectedIdeasLoc, ideas.Location)
	}
}

func TestBuildInstructionsData_SubCategories(t *testing.T) {
	cfg := &config.Config{
		User: "testuser",
		Categories: map[string]*config.Category{
			"plans": {
				Description: "Implementation plans",
				Scope:       config.CategoryScopeShared,
				SubCategories: map[string]*config.SubCategory{
					"active": {
						Description: "Active plans",
					},
					"complete": {
						Description: "Completed plans",
						Trigger:     "When implementation verified complete",
					},
				},
			},
		},
	}

	data := BuildInstructionsData(cfg)

	// Should have 3 rows: plans, plans/active, plans/complete
	if len(data.Categories) != 3 {
		t.Errorf("expected 3 categories (1 parent + 2 sub), got %d", len(data.Categories))
	}

	// Find sub-categories
	var active, complete bool
	for _, cat := range data.Categories {
		if cat.Name == "plans/active" {
			active = true
			expectedLoc := "`thoughts/shared/plans/active/`"
			if cat.Location != expectedLoc {
				t.Errorf("plans/active location: expected %q, got %q", expectedLoc, cat.Location)
			}
		}
		if cat.Name == "plans/complete" {
			complete = true
			if cat.Trigger != "When implementation verified complete" {
				t.Errorf("plans/complete trigger: expected trigger text, got %q", cat.Trigger)
			}
		}
	}

	if !active {
		t.Error("plans/active sub-category not found")
	}
	if !complete {
		t.Error("plans/complete sub-category not found")
	}
}

func TestBuildInstructionsData_ScopeInheritance(t *testing.T) {
	cfg := &config.Config{
		User: "testuser",
		Categories: map[string]*config.Category{
			"notes": {
				Description: "Notes",
				Scope:       config.CategoryScopeUser,
				SubCategories: map[string]*config.SubCategory{
					"private": {
						Description: "Private notes",
						// Inherits user scope from parent
					},
					"team": {
						Description: "Team notes",
						Scope:       config.CategoryScopeShared, // Overrides parent
					},
				},
			},
		},
	}

	data := BuildInstructionsData(cfg)

	for _, cat := range data.Categories {
		switch cat.Name {
		case "notes":
			expected := "`thoughts/{user}/notes/`"
			if cat.Location != expected {
				t.Errorf("notes location: expected %q, got %q", expected, cat.Location)
			}
		case "notes/private":
			expected := "`thoughts/{user}/notes/private/`"
			if cat.Location != expected {
				t.Errorf("notes/private location: expected %q, got %q", expected, cat.Location)
			}
		case "notes/team":
			expected := "`thoughts/shared/notes/team/`"
			if cat.Location != expected {
				t.Errorf("notes/team location: expected %q, got %q", expected, cat.Location)
			}
		}
	}
}

func TestBuildInstructionsData_BothScope(t *testing.T) {
	cfg := &config.Config{
		User: "testuser",
		Categories: map[string]*config.Category{
			"notes": {
				Description: "Quick notes",
				Scope:       config.CategoryScopeBoth,
			},
		},
	}

	data := BuildInstructionsData(cfg)

	if len(data.Categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(data.Categories))
	}

	expected := "`thoughts/shared/notes/` or `thoughts/{user}/notes/`"
	if data.Categories[0].Location != expected {
		t.Errorf("notes location: expected %q, got %q", expected, data.Categories[0].Location)
	}
}

func TestBuildInstructionsDataForProfile(t *testing.T) {
	cfg := &config.Config{
		User: "testuser",
		Categories: map[string]*config.Category{
			"global-cat": {
				Description: "Global category",
			},
		},
		Profiles: map[string]*config.ProfileConfig{
			"work": {
				ThoughtsRepo: "~/work-thoughts",
				Categories: map[string]*config.Category{
					"tickets": {
						Description: "Work tickets",
						Scope:       config.CategoryScopeUser,
					},
				},
			},
		},
	}

	// Work profile should use its own categories
	workData := BuildInstructionsDataForProfile(cfg, "work")
	if len(workData.Categories) != 1 {
		t.Errorf("work profile: expected 1 category, got %d", len(workData.Categories))
	}
	if workData.Categories[0].Name != "tickets" {
		t.Errorf("work profile: expected 'tickets', got %q", workData.Categories[0].Name)
	}

	// Non-existent profile should fall back to global
	defaultData := BuildInstructionsDataForProfile(cfg, "nonexistent")
	if len(defaultData.Categories) != 1 {
		t.Errorf("nonexistent profile: expected 1 category, got %d", len(defaultData.Categories))
	}
	if defaultData.Categories[0].Name != "global-cat" {
		t.Errorf("nonexistent profile: expected 'global-cat', got %q", defaultData.Categories[0].Name)
	}
}

func TestBuildLocation(t *testing.T) {
	tests := []struct {
		path     string
		scope    config.CategoryScope
		expected string
	}{
		{"research", config.CategoryScopeShared, "`thoughts/shared/research/`"},
		{"tickets", config.CategoryScopeUser, "`thoughts/{user}/tickets/`"},
		{"notes", config.CategoryScopeBoth, "`thoughts/shared/notes/` or `thoughts/{user}/notes/`"},
		{"plans/active", config.CategoryScopeShared, "`thoughts/shared/plans/active/`"},
		{"", config.CategoryScopeShared, "`thoughts/shared//`"}, // Edge case
	}

	for _, tc := range tests {
		result := buildLocation(tc.path, tc.scope)
		if result != tc.expected {
			t.Errorf("buildLocation(%q, %q): expected %q, got %q",
				tc.path, tc.scope, tc.expected, result)
		}
	}
}
