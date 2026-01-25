package manage

import (
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/spf13/cobra"
)

var (
	diffFilePath string
	diffOutput   string
)

// NewDiffCmd creates the diff command
func NewDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between YAML and current state",
		Long: `Compare a YAML devnet definition against the current running state.

Shows what would change if you ran 'apply' with this YAML file.

Examples:
  # Show diff for a devnet configuration
  lagos diff -f devnet.yaml

  # Output as JSON
  lagos diff -f devnet.yaml --output json`,
		RunE: runDiff,
	}

	cmd.Flags().StringVarP(&diffFilePath, "file", "f", "", "Path to YAML file or directory (required)")
	cmd.Flags().StringVarP(&diffOutput, "output", "o", "text", "Output format: text, json")

	cmd.MarkFlagRequired("file")

	return cmd
}

func runDiff(cmd *cobra.Command, args []string) error {
	// Load YAML
	loader := config.NewYAMLLoader()
	devnets, err := loader.Load(diffFilePath)
	if err != nil {
		return fmt.Errorf("failed to load YAML: %w", err)
	}

	for _, devnet := range devnets {
		printDiff(cmd, devnet)
	}

	return nil
}

func printDiff(cmd *cobra.Command, devnet config.YAMLDevnet) {
	fmt.Fprintf(cmd.OutOrStdout(), "Devnet: %s\n\n", devnet.Metadata.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "--- current (none)\n")
	fmt.Fprintf(cmd.OutOrStdout(), "+++ desired\n")
	fmt.Fprintf(cmd.OutOrStdout(), "@@ spec @@\n")
	fmt.Fprintf(cmd.OutOrStdout(), "+  network: %s\n", devnet.Spec.Network)
	fmt.Fprintf(cmd.OutOrStdout(), "+  validators: %d\n", devnet.Spec.Validators)
	fmt.Fprintf(cmd.OutOrStdout(), "+  mode: %s\n", devnet.Spec.Mode)
	if devnet.Spec.NetworkVersion != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "+  networkVersion: %s\n", devnet.Spec.NetworkVersion)
	}
	fmt.Fprintln(cmd.OutOrStdout())
}
