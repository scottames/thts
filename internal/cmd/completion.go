package cmd

import (
	"os"

	"github.com/scottames/thts/internal/config"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion <shell>",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for bash, zsh, or fish.

To load completions:

Bash:
  $ source <(thts completion bash)
  # To persist, add to your ~/.bashrc:
  $ thts completion bash > /etc/bash_completion.d/thts

Zsh:
  $ source <(thts completion zsh)
  # To persist, add to your ~/.zshrc or place in fpath:
  $ thts completion zsh > "${fpath[1]}/_thts"

Fish:
  $ thts completion fish | source
  # To persist:
  $ thts completion fish > ~/.config/fish/completions/thts.fish
`,
}

var bashCompletionCmd = &cobra.Command{
	Use:   "bash",
	Short: "Generate bash completion script",
	Long:  `Generate bash completion script for thts.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Root().GenBashCompletion(os.Stdout)
	},
}

var zshCompletionCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generate zsh completion script",
	Long:  `Generate zsh completion script for thts.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Root().GenZshCompletion(os.Stdout)
	},
}

var fishCompletionCmd = &cobra.Command{
	Use:   "fish",
	Short: "Generate fish completion script",
	Long:  `Generate fish completion script for thts.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Root().GenFishCompletion(os.Stdout, true)
	},
}

func init() {
	completionCmd.AddCommand(bashCompletionCmd)
	completionCmd.AddCommand(zshCompletionCmd)
	completionCmd.AddCommand(fishCompletionCmd)
	rootCmd.AddCommand(completionCmd)
}

// CompleteProfiles returns profile names for shell completion.
func CompleteProfiles(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// CompleteCategories returns category paths for shell completion.
// It handles both top-level categories and sub-categories:
// - Empty or partial input: returns matching top-level categories
// - Input ending with /: returns sub-categories of that category
// - Input with / and partial: returns matching sub-categories
func CompleteCategories(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		// Fall back to default categories when config isn't available
		return completeCategoriesFromConfig(config.DefaultCategories(), toComplete)
	}

	return completeCategoriesFromConfig(cfg.GetCategories(), toComplete)
}

// CompleteCategoriesForProfile returns category paths for a specific profile.
func CompleteCategoriesForProfile(_ *cobra.Command, _ []string, toComplete, profileName string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		// Fall back to default categories when config isn't available
		return completeCategoriesFromConfig(config.DefaultCategories(), toComplete)
	}

	return completeCategoriesFromConfig(cfg.GetCategoriesForProfile(profileName), toComplete)
}

// completeCategoriesFromConfig generates completions from a category map.
func completeCategoriesFromConfig(categories map[string]*config.Category, toComplete string) ([]string, cobra.ShellCompDirective) {
	completions := completeCategoryPaths(categories, toComplete)
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeCategoryPaths returns sorted category paths matching the input.
func completeCategoryPaths(categories map[string]*config.Category, toComplete string) []string {
	// Check if we're completing a sub-category (input contains /)
	if idx := indexByte(toComplete, '/'); idx >= 0 {
		categoryName := toComplete[:idx]
		subPrefix := toComplete[idx+1:]

		cat, exists := categories[categoryName]
		if !exists || cat.SubCategories == nil {
			return []string{}
		}

		return matchingSubCategories(categoryName, cat.SubCategories, subPrefix)
	}

	// Complete top-level categories
	return matchingCategories(categories, toComplete)
}

// matchingCategories returns sorted category names matching the prefix.
func matchingCategories(categories map[string]*config.Category, prefix string) []string {
	var matches []string
	for name := range categories {
		if hasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	sortStrings(matches)
	return matches
}

// matchingSubCategories returns sorted sub-category paths matching the prefix.
func matchingSubCategories(categoryName string, subCategories map[string]*config.SubCategory, prefix string) []string {
	var matches []string
	for name := range subCategories {
		if hasPrefix(name, prefix) {
			matches = append(matches, categoryName+"/"+name)
		}
	}
	sortStrings(matches)
	return matches
}

// hasPrefix checks if s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// indexByte returns the index of the first instance of c in s, or -1 if not present.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
