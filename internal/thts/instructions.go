package thts

import (
	"fmt"
	"sort"

	thtsfiles "github.com/scottames/thts"
	"github.com/scottames/thts/internal/config"
)

// BuildInstructionsData creates template data from config for rendering instructions.
// It flattens categories and sub-categories into a sorted list of CategoryRows.
func BuildInstructionsData(cfg *config.Config) thtsfiles.InstructionsData {
	categories := cfg.GetCategories()
	rows := flattenCategories(categories)

	// Sort for deterministic output
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	return thtsfiles.InstructionsData{
		User:       cfg.User,
		Categories: rows,
	}
}

// BuildInstructionsDataForProfile creates template data for a specific profile.
func BuildInstructionsDataForProfile(cfg *config.Config, profileName string) thtsfiles.InstructionsData {
	categories := cfg.GetCategoriesForProfile(profileName)
	rows := flattenCategories(categories)

	// Sort for deterministic output
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	return thtsfiles.InstructionsData{
		User:       cfg.User,
		Categories: rows,
	}
}

// flattenCategories converts a category map into a flat list of CategoryRows,
// including sub-categories with their parent path.
func flattenCategories(categories map[string]*config.Category) []thtsfiles.CategoryRow {
	var rows []thtsfiles.CategoryRow

	for name, cat := range categories {
		// Add the category itself
		rows = append(rows, thtsfiles.CategoryRow{
			Name:        name,
			Description: cat.Description,
			Location:    buildLocation(name, cat.GetScope()),
			Trigger:     cat.Trigger,
			Template:    cat.Template,
		})

		// Add sub-categories
		for subName, subCat := range cat.SubCategories {
			fullPath := fmt.Sprintf("%s/%s", name, subName)
			rows = append(rows, thtsfiles.CategoryRow{
				Name:        fullPath,
				Description: subCat.Description,
				Location:    buildLocation(fullPath, subCat.GetScope(cat.GetScope())),
				Trigger:     subCat.Trigger,
				Template:    subCat.Template,
			})
		}
	}

	return rows
}

// buildLocation creates the location string based on scope.
func buildLocation(path string, scope config.CategoryScope) string {
	switch scope {
	case config.CategoryScopeUser:
		return fmt.Sprintf("`thoughts/{user}/%s/`", path)
	case config.CategoryScopeBoth:
		return fmt.Sprintf("`thoughts/shared/%s/` or `thoughts/{user}/%s/`", path, path)
	default: // CategoryScopeShared
		return fmt.Sprintf("`thoughts/shared/%s/`", path)
	}
}
