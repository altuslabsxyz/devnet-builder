// cmd/dvb/delete.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var (
		filePath  string
		namespace string
		force     bool
		dryRun    bool
		dataDir   string
	)

	cmd := &cobra.Command{
		Use:   "delete [resource] [name]",
		Short: "Delete devnet resources",
		Long: `Delete devnet resources by name or from a YAML file.

In daemon mode, the daemon handles resource cleanup.
In standalone mode, removes devnet data from the filesystem.

Examples:
  # Delete a devnet by name
  dvb delete devnet my-devnet

  # Delete a devnet in a specific namespace
  dvb delete devnet my-devnet -n production

  # Delete devnets defined in a YAML file
  dvb delete -f devnet.yaml

  # Delete without confirmation
  dvb delete devnet my-devnet --force

  # Preview what would be deleted
  dvb delete -f devnet.yaml --dry-run

  # Delete in standalone mode with custom data directory
  dvb delete devnet my-devnet --data-dir /path/to/data`,
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			// If -f is provided, delete from file
			if filePath != "" {
				return runDeleteFromFile(cmd, namespace, filePath, force, dryRun, dataDir)
			}

			// Otherwise expect resource type and name
			if len(args) < 2 {
				return fmt.Errorf("requires resource type and name, or use -f <file>")
			}

			resourceType := args[0]
			name := args[1]

			switch resourceType {
			case "devnet", "devnets", "dn":
				return runDeleteDevnet(cmd, namespace, name, force, dryRun, dataDir)
			default:
				return fmt.Errorf("unknown resource type: %s", resourceType)
			}
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file containing resources to delete")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be deleted without actually deleting")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Base data directory for standalone mode (default: ~/.devnet-builder)")

	return cmd
}

// runDeleteFromFile deletes devnets defined in a YAML file
func runDeleteFromFile(cmd *cobra.Command, namespace, filePath string, force, dryRun bool, dataDir string) error {
	// Check file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("cannot access %s: %w", filePath, err)
	}

	// Load YAML
	loader := config.NewYAMLLoader()
	devnets, err := loader.Load(filePath)
	if err != nil {
		return fmt.Errorf("failed to load YAML: %w", err)
	}

	if len(devnets) == 0 {
		return fmt.Errorf("no devnet configurations found in %s", filePath)
	}

	// Preview mode
	if dryRun {
		fmt.Printf("Would delete %d devnet(s):\n", len(devnets))
		for i := range devnets {
			ns := devnets[i].Metadata.Namespace
			if ns == "" {
				ns = namespace
			}
			fmt.Printf("  - devnet/%s (namespace: %s)\n", devnets[i].Metadata.Name, ns)
		}
		fmt.Println("\nRun without --dry-run to delete.")
		return nil
	}

	// Confirm if not forced
	if !force {
		fmt.Printf("This will delete %d devnet(s):\n", len(devnets))
		for i := range devnets {
			ns := devnets[i].Metadata.Namespace
			if ns == "" {
				ns = namespace
			}
			fmt.Printf("  - devnet/%s (namespace: %s)\n", devnets[i].Metadata.Name, ns)
		}
		fmt.Print("\nAre you sure? [y/N] ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Try daemon first if available and not in standalone mode
	if daemonClient != nil && !standalone {
		var hasErrors bool
		for i := range devnets {
			name := devnets[i].Metadata.Name
			ns := devnets[i].Metadata.Namespace
			if ns == "" {
				ns = namespace
			}
			err := daemonClient.DeleteDevnet(cmd.Context(), ns, name)
			if err != nil {
				color.Red("devnet/%s (namespace: %s) deletion failed: %v", name, ns, err)
				hasErrors = true
				continue
			}
			color.Green("devnet/%s deleted (namespace: %s)", name, ns)
		}

		if hasErrors {
			return fmt.Errorf("some deletions failed")
		}
		return nil
	}

	// Standalone mode: delete from filesystem
	return deleteDevnetsStandalone(devnets, dataDir)
}

// runDeleteDevnet deletes a single devnet by name
func runDeleteDevnet(cmd *cobra.Command, namespace, name string, force, dryRun bool, dataDir string) error {
	// Preview mode
	if dryRun {
		fmt.Printf("Would delete devnet/%s (namespace: %s)\n", name, namespace)
		fmt.Println("\nRun without --dry-run to delete.")
		return nil
	}

	// Confirm if not forced
	if !force {
		fmt.Printf("Are you sure you want to delete devnet %q (namespace: %s)? [y/N] ", name, namespace)
		var response string
		if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Try daemon first if available and not in standalone mode
	if daemonClient != nil && !standalone {
		err := daemonClient.DeleteDevnet(cmd.Context(), namespace, name)
		if err != nil {
			return err
		}
		color.Green("devnet/%s deleted (namespace: %s)", name, namespace)
		return nil
	}

	// Standalone mode: delete from filesystem
	return deleteDevnetStandalone(name, dataDir)
}

// deleteDevnetsStandalone deletes multiple devnets from the filesystem
func deleteDevnetsStandalone(devnets []config.YAMLDevnet, dataDir string) error {
	// Determine data directory
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".devnet-builder")
	}

	var hasErrors bool
	for i := range devnets {
		name := devnets[i].Metadata.Name
		devnetPath := filepath.Join(dataDir, "devnets", name)

		// Check if devnet exists
		if _, err := os.Stat(devnetPath); os.IsNotExist(err) {
			color.Yellow("devnet/%s not found (skipping)", name)
			continue
		}

		// Remove the devnet directory
		if err := os.RemoveAll(devnetPath); err != nil {
			color.Red("devnet/%s deletion failed: %v", name, err)
			hasErrors = true
			continue
		}

		color.Green("devnet/%s deleted", name)
	}

	if hasErrors {
		return fmt.Errorf("some deletions failed")
	}

	return nil
}

// deleteDevnetStandalone deletes a single devnet from the filesystem
func deleteDevnetStandalone(name string, dataDir string) error {
	// Determine data directory
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".devnet-builder")
	}

	devnetPath := filepath.Join(dataDir, "devnets", name)

	// Check if devnet exists
	if _, err := os.Stat(devnetPath); os.IsNotExist(err) {
		return fmt.Errorf("devnet %q not found", name)
	}

	// Remove the devnet directory
	if err := os.RemoveAll(devnetPath); err != nil {
		return fmt.Errorf("failed to delete devnet: %w", err)
	}

	color.Green("devnet/%s deleted", name)
	return nil
}
