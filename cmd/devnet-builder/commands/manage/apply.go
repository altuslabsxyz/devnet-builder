package manage

import (
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/spf13/cobra"
)

var (
	applyFilePath string
	applyDryRun   bool
	applyForce    bool
	applyOutput   string
)

// NewApplyCmd creates the apply command
func NewApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a devnet configuration from YAML",
		Long: `Apply a devnet configuration from a YAML file.

The apply command compares the desired state (from YAML) with the current state
and makes minimal changes to reconcile them. It's idempotent - running it
multiple times produces the same result.

Examples:
  # Apply a devnet configuration
  lagos apply -f devnet.yaml

  # Preview changes without applying
  lagos apply -f devnet.yaml --dry-run

  # Apply all YAML files in a directory
  lagos apply -f ./devnets/

  # Force recreation (destroy + create)
  lagos apply -f devnet.yaml --force`,
		RunE: runApply,
	}

	cmd.Flags().StringVarP(&applyFilePath, "file", "f", "", "Path to YAML file or directory (required)")
	cmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().BoolVar(&applyForce, "force", false, "Force recreation (destroy existing + create new)")
	cmd.Flags().StringVarP(&applyOutput, "output", "o", "text", "Output format: text, json")

	cmd.MarkFlagRequired("file")

	return cmd
}

func runApply(cmd *cobra.Command, args []string) error {
	// Load YAML
	loader := config.NewYAMLLoader()
	devnets, err := loader.Load(applyFilePath)
	if err != nil {
		return fmt.Errorf("failed to load YAML: %w", err)
	}

	// For now, just print what would be done
	for _, devnet := range devnets {
		if applyDryRun {
			printDryRun(cmd, devnet)
		} else {
			return fmt.Errorf("apply without --dry-run not yet implemented")
		}
	}

	return nil
}

func printDryRun(cmd *cobra.Command, devnet config.YAMLDevnet) {
	fmt.Fprintf(cmd.OutOrStdout(), "Devnet: %s (dry-run)\n\n", devnet.Metadata.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Plan: 1 to create, 0 to update, 0 to destroy\n\n")
	fmt.Fprintf(cmd.OutOrStdout(), "+ devnet/%s\n", devnet.Metadata.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "    network:        %s\n", devnet.Spec.Network)
	fmt.Fprintf(cmd.OutOrStdout(), "    networkVersion: %s\n", devnet.Spec.NetworkVersion)
	fmt.Fprintf(cmd.OutOrStdout(), "    mode:           %s\n", devnet.Spec.Mode)
	fmt.Fprintf(cmd.OutOrStdout(), "    validators:     %d\n", devnet.Spec.Validators)
	fmt.Fprintln(cmd.OutOrStdout())

	for i := 0; i < devnet.Spec.Validators; i++ {
		fmt.Fprintf(cmd.OutOrStdout(), "  + node/%s-%d (validator)\n", devnet.Metadata.Name, i)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Run without --dry-run to apply.")
}
