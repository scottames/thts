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
