// cmd/dvb/apply.go
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newApplyCmd() *cobra.Command {
	var (
		filePath string
		dryRun   bool
		force    bool
		output   string
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a devnet configuration from YAML",
		Long: `Apply a devnet configuration from a YAML file.

The apply command compares the desired state (from YAML) with the current state
and makes minimal changes to reconcile them. It's idempotent - running it
multiple times produces the same result.

Examples:
  # Apply a devnet configuration
  dvb apply -f devnet.yaml

  # Preview changes without applying
  dvb apply -f devnet.yaml --dry-run

  # Apply all YAML files in a directory
  dvb apply -f ./devnets/

  # Force recreation (destroy + create)
  dvb apply -f devnet.yaml --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			if filePath == "" {
				return fmt.Errorf("--file/-f is required")
			}

			// Load YAML files
			devnets, err := loadYAMLFiles(filePath)
			if err != nil {
				return fmt.Errorf("failed to load YAML: %w", err)
			}

			if len(devnets) == 0 {
				return fmt.Errorf("no valid devnet configurations found in %s", filePath)
			}

			// Process each devnet
			for _, yamlDevnet := range devnets {
				if err := applyDevnet(cmd, yamlDevnet, dryRun, force, output); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file or directory (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().BoolVar(&force, "force", false, "Force recreation (destroy existing + create new)")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text, json")

	return cmd
}

// loadYAMLFiles loads devnet configurations from a file or directory
func loadYAMLFiles(path string) ([]*config.YAMLDevnet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", path, err)
	}

	var files []string
	if info.IsDir() {
		// Find all YAML files in directory
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				files = append(files, filepath.Join(path, name))
			}
		}
	} else {
		files = []string{path}
	}

	var devnets []*config.YAMLDevnet
	for _, file := range files {
		parsed, err := parseYAMLFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", file, err)
		}
		devnets = append(devnets, parsed...)
	}

	return devnets, nil
}

// parseYAMLFile parses a YAML file that may contain multiple documents
func parseYAMLFile(path string) ([]*config.YAMLDevnet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var devnets []*config.YAMLDevnet
	decoder := yaml.NewDecoder(f)

	for docIndex := 0; ; docIndex++ {
		var doc config.YAMLDevnet
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("document %d: %w", docIndex, err)
		}

		// Validate the document
		if err := doc.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed for document %d in %s: %w", docIndex, path, err)
		}

		devnets = append(devnets, &doc)
	}

	return devnets, nil
}

// applyDevnet applies a single devnet configuration
func applyDevnet(cmd *cobra.Command, yamlDevnet *config.YAMLDevnet, dryRun, force bool, output string) error {
	ctx := cmd.Context()
	name := yamlDevnet.Metadata.Name

	// Check if devnet exists
	existing, err := daemonClient.GetDevnet(ctx, name)
	exists := err == nil && existing != nil

	// Convert YAML to proto spec
	spec := yamlToProtoSpec(&yamlDevnet.Spec)

	if dryRun {
		return printDryRun(yamlDevnet, exists, force, output)
	}

	// Handle force recreation
	if exists && force {
		color.Yellow("⚠ Destroying existing devnet %q for recreation", name)
		if err := daemonClient.DeleteDevnet(ctx, name); err != nil {
			return fmt.Errorf("failed to destroy existing devnet: %w", err)
		}
		exists = false
	}

	// Create or update
	if exists {
		// For now, just report that devnet already exists
		// TODO: Implement update logic when daemon supports it
		color.Yellow("! Devnet %q already exists (update not yet supported)", name)
		fmt.Printf("  Current phase: %s\n", existing.Status.Phase)
		fmt.Printf("  Use --force to recreate\n")
		return nil
	}

	// Create new devnet
	devnet, err := daemonClient.CreateDevnet(ctx, name, spec, yamlDevnet.Metadata.Labels)
	if err != nil {
		return fmt.Errorf("failed to create devnet: %w", err)
	}

	color.Green("✓ Devnet %q created", devnet.Metadata.Name)
	fmt.Printf("  Phase: %s\n", devnet.Status.Phase)
	fmt.Printf("  Plugin: %s\n", devnet.Spec.Plugin)
	fmt.Printf("  Validators: %d\n", devnet.Spec.Validators)
	if devnet.Spec.FullNodes > 0 {
		fmt.Printf("  Full Nodes: %d\n", devnet.Spec.FullNodes)
	}

	return nil
}

// yamlToProtoSpec converts YAML spec to proto DevnetSpec
func yamlToProtoSpec(yamlSpec *config.YAMLDevnetSpec) *v1.DevnetSpec {
	spec := &v1.DevnetSpec{
		Plugin:      yamlSpec.Network, // YAML uses "network", proto uses "plugin"
		NetworkType: yamlSpec.NetworkType,
		Validators:  int32(yamlSpec.Validators),
		FullNodes:   int32(yamlSpec.FullNodes),
		Mode:        yamlSpec.Mode,
		SdkVersion:  yamlSpec.NetworkVersion,
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

// printDryRun prints what would happen without actually applying
func printDryRun(yamlDevnet *config.YAMLDevnet, exists, force bool, output string) error {
	name := yamlDevnet.Metadata.Name

	fmt.Printf("Devnet: %s (dry-run)\n\n", name)

	if exists && !force {
		color.Yellow("! Devnet already exists (no changes)")
		fmt.Printf("  Use --force to recreate\n")
		return nil
	}

	action := "create"
	if exists && force {
		action = "recreate"
	}

	fmt.Printf("Plan: 1 to %s\n\n", action)

	// Show what would be created
	if action == "recreate" {
		color.Yellow("~ devnet/%s (recreate)", name)
	} else {
		color.Green("+ devnet/%s", name)
	}

	fmt.Printf("    network:    %s\n", yamlDevnet.Spec.Network)
	if yamlDevnet.Spec.NetworkVersion != "" {
		fmt.Printf("    version:    %s\n", yamlDevnet.Spec.NetworkVersion)
	}
	if yamlDevnet.Spec.Mode != "" {
		fmt.Printf("    mode:       %s\n", yamlDevnet.Spec.Mode)
	}
	validators := yamlDevnet.Spec.Validators
	if validators == 0 {
		validators = 4
	}
	fmt.Printf("    validators: %d\n", validators)
	if yamlDevnet.Spec.FullNodes > 0 {
		fmt.Printf("    fullNodes:  %d\n", yamlDevnet.Spec.FullNodes)
	}

	// Show nodes that would be created
	for i := 0; i < validators; i++ {
		color.Green("  + node/%s-%d (validator)", name, i)
	}
	for i := 0; i < yamlDevnet.Spec.FullNodes; i++ {
		color.Green("  + node/%s-full-%d (full)", name, i)
	}

	fmt.Printf("\nRun without --dry-run to apply.\n")
	return nil
}
