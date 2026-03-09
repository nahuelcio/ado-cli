package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var autocompleteCmd = &cobra.Command{
	Use:   "autocomplete",
	Short: "Generate shell completion scripts",
	Long: `Generate completion scripts for your shell.
Supported shells: bash, zsh, fish

To load completions:

Bash:
  $ source <(ado autocomplete bash)

  # To load completions for each session, execute once:
  # Linux:
  $ ado autocomplete bash > /etc/bash_completion.d/ado
  # macOS:
  $ ado autocomplete bash > /usr/local/etc/bash_completion.d/ado

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ ado autocomplete zsh > "${fpath[1]}/_ado"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ ado autocomplete fish | source

  # To load completions for each session, execute once:
  $ ado autocomplete fish > ~/.config/fish/completions/ado.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := args[0]
		switch shell {
		case "bash":
			if err := rootCmd.GenBashCompletion(os.Stdout); err != nil {
				return err
			}
		case "zsh":
			if err := rootCmd.GenZshCompletion(os.Stdout); err != nil {
				return err
			}
		case "fish":
			if err := rootCmd.GenFishCompletion(os.Stdout, true); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
		}
		return nil
	},
}
