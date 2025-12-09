package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for bash, zsh, or fish.

To load completions:

Bash:
  $ source <(devnet-builder completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ devnet-builder completion bash > /etc/bash_completion.d/devnet-builder
  # macOS:
  $ devnet-builder completion bash > /usr/local/etc/bash_completion.d/devnet-builder

Zsh:
  $ source <(devnet-builder completion zsh)

  # To load completions for each session, execute once:
  $ devnet-builder completion zsh > "${fpath[1]}/_devnet-builder"

Fish:
  $ devnet-builder completion fish | source

  # To load completions for each session, execute once:
  $ devnet-builder completion fish > ~/.config/fish/completions/devnet-builder.fish
`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE:      runCompletion,
	}

	return cmd
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]

	switch shell {
	case "bash":
		return cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		return cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		return cmd.Root().GenFishCompletion(os.Stdout, true)
	default:
		return fmt.Errorf("unsupported shell: %s (use bash, zsh, or fish)", shell)
	}
}
