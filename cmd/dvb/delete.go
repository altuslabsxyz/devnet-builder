// cmd/dvb/delete.go
package main

import (
	"fmt"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var (
		filePath string
		force    bool
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "delete [resource] [name]",
		Short: "Delete devnet resources",
		Long: `Delete devnet resources by name or from a YAML file.

Examples:
  # Delete a devnet by name
  dvb delete devnet my-devnet

  # Delete devnets defined in a YAML file
  dvb delete -f devnet.yaml

  # Delete without confirmation
  dvb delete devnet my-devnet --force

  # Preview what would be deleted
  dvb delete -f devnet.yaml --dry-run`,
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			// If -f is provided, delete from file
			if filePath != "" {
				return runDeleteFromFile(cmd, filePath, force, dryRun)
			}

			// Otherwise expect resource type and name
			if len(args) < 2 {
				return fmt.Errorf("requires resource type and name, or use -f <file>")
			}

			resourceType := args[0]
			name := args[1]

			switch resourceType {
			case "devnet", "devnets", "dn":
				return runDeleteDevnet(cmd, name, force, dryRun)
			default:
				return fmt.Errorf("unknown resource type: %s", resourceType)
			}
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file containing resources to delete")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be deleted without actually deleting")

	return cmd
}

// runDeleteFromFile deletes devnets defined in a YAML file
func runDeleteFromFile(cmd *cobra.Command, filePath string, force, dryRun bool) error {
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
			fmt.Printf("  - devnet/%s\n", devnets[i].Metadata.Name)
		}
		fmt.Println("\nRun without --dry-run to delete.")
		return nil
	}

	// Confirm if not forced
	if !force {
		fmt.Printf("This will delete %d devnet(s):\n", len(devnets))
		for i := range devnets {
			fmt.Printf("  - devnet/%s\n", devnets[i].Metadata.Name)
		}
		fmt.Print("\nAre you sure? [y/N] ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Check daemon connection
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd")
	}

	// Delete each devnet
	var hasErrors bool
	for i := range devnets {
		name := devnets[i].Metadata.Name
		err := daemonClient.DeleteDevnet(cmd.Context(), name)
		if err != nil {
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

// runDeleteDevnet deletes a single devnet by name
func runDeleteDevnet(cmd *cobra.Command, name string, force, dryRun bool) error {
	// Preview mode
	if dryRun {
		fmt.Printf("Would delete devnet/%s\n", name)
		fmt.Println("\nRun without --dry-run to delete.")
		return nil
	}

	// Confirm if not forced
	if !force {
		fmt.Printf("Are you sure you want to delete devnet %q? [y/N] ", name)
		var response string
		if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Check daemon connection
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd")
	}

	err := daemonClient.DeleteDevnet(cmd.Context(), name)
	if err != nil {
		return err
	}

	color.Green("devnet/%s deleted", name)
	return nil
}
