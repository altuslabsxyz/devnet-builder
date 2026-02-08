// cmd/dvb/completion.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for dvb.

To load completions:

Bash:
  $ source <(dvb completion bash)
  # To load for each session:
  $ echo 'source <(dvb completion bash)' >> ~/.bashrc

Zsh:
  $ source <(dvb completion zsh)
  # To load for each session:
  $ echo 'source <(dvb completion zsh)' >> ~/.zshrc

Fish:
  $ dvb completion fish | source
  # To load for each session:
  $ dvb completion fish > ~/.config/fish/completions/dvb.fish

PowerShell:
  PS> dvb completion powershell | Out-String | Invoke-Expression
  # To load for each session, add to your profile:
  PS> dvb completion powershell >> $PROFILE`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletion(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", args[0])
			}
		},
	}

	return cmd
}
