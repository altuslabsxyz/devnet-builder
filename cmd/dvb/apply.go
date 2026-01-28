// cmd/dvb/apply.go
package main

import (
	"fmt"
	"os"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	var (
		filePath string
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a devnet configuration from YAML",
		Long: `Apply a devnet configuration from a YAML file or directory.

The apply command compares the desired state (from YAML) with the current state
and makes minimal changes to reconcile them. It's idempotent - running it
multiple times produces the same result.

The devnetd daemon must be running and handles the devnet lifecycle.
Network plugins are discovered from ~/.devnet-builder/plugins/.

Examples:
  # Apply a devnet configuration
  dvb apply -f devnet.yaml

  # Preview changes without applying
  dvb apply -f devnet.yaml --dry-run

  # Apply all YAML files in a directory
  dvb apply -f ./devnets/`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file/-f is required")
			}

			return runApply(cmd, filePath, dryRun)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file or directory (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")

	return cmd
}

// runApply is the main execution function for the apply command
func runApply(cmd *cobra.Command, filePath string, dryRun bool) error {
	// Check file/directory exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("cannot access %s: %w", filePath, err)
	}

	// Load YAML using the YAMLLoader
	loader := config.NewYAMLLoader()
	devnets, err := loader.Load(filePath)
	if err != nil {
		return fmt.Errorf("failed to load YAML: %w", err)
	}

	if len(devnets) == 0 {
		return fmt.Errorf("no valid devnet configurations found in %s", filePath)
	}

	// Validate all configs with detailed errors
	validator := config.NewYAMLValidator()
	var hasErrors bool
	for i := range devnets {
		result := validator.ValidateWithSource(&devnets[i], filePath)
		if !result.Valid {
			fmt.Fprint(os.Stderr, config.FormatValidationErrors(result, filePath))
			hasErrors = true
		}
	}
	if hasErrors {
		return fmt.Errorf("validation failed")
	}

	// If dry-run, print preview and return
	if dryRun {
		for i := range devnets {
			printApplyDryRun(&devnets[i])
		}
		fmt.Printf("\nRun without --dry-run to apply.\n")
		return nil
	}

	// Require daemon to be running
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd\n\nThe apply command requires the devnetd daemon to be running.\nNetwork plugins are loaded by the daemon from ~/.devnet-builder/plugins/")
	}

	// Apply each devnet via daemon
	for i := range devnets {
		resp, err := applyDevnet(cmd, &devnets[i])
		if err != nil {
			return err
		}
		printApplyResult(&devnets[i], resp)
	}
	return nil
}

// applyDevnet applies a single devnet configuration
func applyDevnet(cmd *cobra.Command, devnet *config.YAMLDevnet) (*v1.ApplyDevnetResponse, error) {
	ctx := cmd.Context()
	name := devnet.Metadata.Name
	namespace := devnet.Metadata.Namespace

	// Convert YAML to proto spec
	spec := yamlToProtoSpec(&devnet.Spec)

	// Call ApplyDevnet (idempotent create or update) with namespace
	resp, err := daemonClient.ApplyDevnet(ctx, namespace, name, spec, devnet.Metadata.Labels, devnet.Metadata.Annotations)
	if err != nil {
		return nil, fmt.Errorf("failed to apply devnet %q: %w", name, err)
	}

	return resp, nil
}

// yamlToProtoSpec converts YAML spec to proto DevnetSpec
func yamlToProtoSpec(yamlSpec *config.YAMLDevnetSpec) *v1.DevnetSpec {
	spec := &v1.DevnetSpec{
		Plugin:      yamlSpec.Network, // YAML uses "network", proto uses "plugin"
		NetworkType: yamlSpec.NetworkType,
		Validators:  int32(yamlSpec.Validators),
		FullNodes:   int32(yamlSpec.FullNodes),
		Mode:        yamlSpec.Mode,
		SdkVersion:  yamlSpec.NetworkVersion, // YAML uses "networkVersion", proto uses "sdkVersion"

		// Genesis forking options
		ForkNetwork: yamlSpec.ForkNetwork,
		GenesisPath: yamlSpec.GenesisPath,
		SnapshotUrl: yamlSpec.SnapshotURL,
		RpcUrl:      yamlSpec.RPCURL,
	}

	// Set defaults
	if spec.Validators == 0 {
		spec.Validators = 4
	}
	if spec.Mode == "" {
		spec.Mode = "docker"
	}

	return spec
}

// printApplyDryRun prints dry-run information for a devnet
func printApplyDryRun(devnet *config.YAMLDevnet) {
	name := devnet.Metadata.Name
	namespace := devnet.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	fmt.Printf("\ndevnet/%s (namespace: %s, dry-run)\n", name, namespace)
	fmt.Printf("  network:    %s\n", devnet.Spec.Network)
	if devnet.Spec.NetworkVersion != "" {
		fmt.Printf("  version:    %s\n", devnet.Spec.NetworkVersion)
	}
	if devnet.Spec.NetworkType != "" {
		fmt.Printf("  type:       %s\n", devnet.Spec.NetworkType)
	}

	mode := devnet.Spec.Mode
	if mode == "" {
		mode = "docker"
	}
	fmt.Printf("  mode:       %s\n", mode)

	validators := devnet.Spec.Validators
	if validators == 0 {
		validators = 4
	}
	fmt.Printf("  validators: %d\n", validators)

	if devnet.Spec.FullNodes > 0 {
		fmt.Printf("  fullNodes:  %d\n", devnet.Spec.FullNodes)
	}

	// Show fork configuration if specified
	if devnet.Spec.ForkNetwork != "" {
		fmt.Printf("  fork:       %s\n", devnet.Spec.ForkNetwork)
	}
	if devnet.Spec.GenesisPath != "" {
		fmt.Printf("  genesis:    %s\n", devnet.Spec.GenesisPath)
	}
	if devnet.Spec.SnapshotURL != "" {
		fmt.Printf("  snapshot:   %s\n", devnet.Spec.SnapshotURL)
	}
	if devnet.Spec.RPCURL != "" {
		fmt.Printf("  rpc:        %s\n", devnet.Spec.RPCURL)
	}

	// Show labels if present
	if len(devnet.Metadata.Labels) > 0 {
		fmt.Printf("  labels:\n")
		for k, v := range devnet.Metadata.Labels {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}
}

// printApplyResult prints the result of applying a devnet with kubectl-style output
func printApplyResult(devnet *config.YAMLDevnet, resp *v1.ApplyDevnetResponse) {
	name := devnet.Metadata.Name
	namespace := devnet.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Color based on action
	switch resp.Action {
	case "created":
		color.Green("devnet/%s created (namespace: %s)", name, namespace)
	case "configured":
		color.Yellow("devnet/%s configured (namespace: %s)", name, namespace)
	case "unchanged":
		fmt.Printf("devnet/%s unchanged (namespace: %s)\n", name, namespace)
	default:
		// Fallback for unknown action
		fmt.Printf("devnet/%s %s (namespace: %s)\n", name, resp.Action, namespace)
	}
}
