package agents

import (
	"strings"
	"testing"
)

func TestAdjustHeaderLevels(t *testing.T) {
	t.Run("increments headers by offset", func(t *testing.T) {
		input := "# Title\n## Section\n### Subsection"
		expected := "## Title\n### Section\n#### Subsection"

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("preserves non-header lines", func(t *testing.T) {
		input := "# Title\nSome text\n## Section\nMore text"
		expected := "## Title\nSome text\n### Section\nMore text"

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("skips headers inside fenced code blocks", func(t *testing.T) {
		input := "# Title\n```markdown\n# This is inside code\n## Also inside\n```\n## Real Section"
		expected := "## Title\n```markdown\n# This is inside code\n## Also inside\n```\n### Real Section"

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("handles tilde fenced code blocks", func(t *testing.T) {
		input := "# Title\n~~~\n# Inside tilde block\n~~~\n## Section"
		expected := "## Title\n~~~\n# Inside tilde block\n~~~\n### Section"

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("zero offset returns unchanged", func(t *testing.T) {
		input := "# Title\n## Section"

		result := adjustHeaderLevels(input, 0)
		if result != input {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, input)
		}
	})

	t.Run("negative offset returns unchanged", func(t *testing.T) {
		input := "# Title\n## Section"

		result := adjustHeaderLevels(input, -1)
		if result != input {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, input)
		}
	})

	t.Run("handles offset of 2", func(t *testing.T) {
		input := "# Title\n## Section"
		expected := "### Title\n#### Section"

		result := adjustHeaderLevels(input, 2)
		if result != expected {
			t.Errorf("adjustHeaderLevels() = %q, want %q", result, expected)
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		result := adjustHeaderLevels("", 1)
		if result != "" {
			t.Errorf("adjustHeaderLevels() = %q, want empty string", result)
		}
	})

	t.Run("handles multiple code blocks", func(t *testing.T) {
		input := strings.Join([]string{
			"# Title",
			"```",
			"# code header",
			"```",
			"## Section",
			"```go",
			"// # comment that looks like header",
			"```",
			"### Subsection",
		}, "\n")

		expected := strings.Join([]string{
			"## Title",
			"```",
			"# code header",
			"```",
			"### Section",
			"```go",
			"// # comment that looks like header",
			"```",
			"#### Subsection",
		}, "\n")

		result := adjustHeaderLevels(input, 1)
		if result != expected {
			t.Errorf("adjustHeaderLevels() =\n%s\nwant\n%s", result, expected)
		}
	})
}
