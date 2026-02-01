// cmd/dvb/diff.go
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

func newDiffCmd() *cobra.Command {
	var (
		filePath string
		output   string
	)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between YAML config and current state",
		Long: `Show what changes would be made by applying a YAML configuration.

The diff command compares the desired state (from YAML) with the current state
and displays the differences without making any changes. This is useful for
previewing what 'dvb provision -f' would do.

Examples:
  # Show diff for a single file
  dvb diff -f devnet.yaml

  # Show diff for all YAML files in a directory
  dvb diff -f ./devnets/

  # Output in JSON format
  dvb diff -f devnet.yaml -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			if filePath == "" {
				return fmt.Errorf("--file/-f is required")
			}

			// Load YAML files
			devnets, err := loadDiffYAMLFiles(filePath)
			if err != nil {
				return fmt.Errorf("failed to load YAML: %w", err)
			}

			if len(devnets) == 0 {
				return fmt.Errorf("no valid devnet configurations found in %s", filePath)
			}

			// Process each devnet
			hasChanges := false
			for _, yamlDevnet := range devnets {
				changed, err := showDiff(cmd, yamlDevnet, output)
				if err != nil {
					return err
				}
				if changed {
					hasChanges = true
				}
			}

			if !hasChanges {
				color.Green("\n✓ No changes detected")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file or directory (required)")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text, json")

	return cmd
}

// loadDiffYAMLFiles loads devnet configurations from a file or directory
func loadDiffYAMLFiles(path string) ([]*config.YAMLDevnet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", path, err)
	}

	var files []string
	if info.IsDir() {
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
		parsed, err := parseDiffYAMLFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", file, err)
		}
		devnets = append(devnets, parsed...)
	}

	return devnets, nil
}

// parseDiffYAMLFile parses a YAML file that may contain multiple documents
func parseDiffYAMLFile(path string) ([]*config.YAMLDevnet, error) {
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

		if err := doc.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed for document %d in %s: %w", docIndex, path, err)
		}

		devnets = append(devnets, &doc)
	}

	return devnets, nil
}

// showDiff displays the differences for a single devnet
func showDiff(cmd *cobra.Command, yamlDevnet *config.YAMLDevnet, output string) (bool, error) {
	ctx := cmd.Context()
	name := yamlDevnet.Metadata.Name
	namespace := yamlDevnet.Metadata.Namespace

	// Check if devnet exists
	existing, err := daemonClient.GetDevnet(ctx, namespace, name)
	exists := err == nil && existing != nil

	if !exists {
		// Devnet doesn't exist - would be created
		return printCreateDiff(yamlDevnet, output)
	}

	// Compare existing with desired
	return printUpdateDiff(yamlDevnet, existing, output)
}

// printCreateDiff shows what would be created
func printCreateDiff(yamlDevnet *config.YAMLDevnet, output string) (bool, error) {
	name := yamlDevnet.Metadata.Name

	fmt.Printf("\n")
	color.Green("+ devnet/%s (create)", name)
	fmt.Printf("\n")

	// Show spec details
	printFieldDiff("network", "", yamlDevnet.Spec.Network, true)
	if yamlDevnet.Spec.NetworkType != "" {
		printFieldDiff("networkType", "", yamlDevnet.Spec.NetworkType, true)
	}
	if yamlDevnet.Spec.NetworkVersion != "" {
		printFieldDiff("networkVersion", "", yamlDevnet.Spec.NetworkVersion, true)
	}

	validators := yamlDevnet.Spec.Validators
	if validators == 0 {
		validators = 4
	}
	printFieldDiff("validators", "", fmt.Sprintf("%d", validators), true)

	if yamlDevnet.Spec.FullNodes > 0 {
		printFieldDiff("fullNodes", "", fmt.Sprintf("%d", yamlDevnet.Spec.FullNodes), true)
	}

	mode := yamlDevnet.Spec.Mode
	if mode == "" {
		mode = "docker"
	}
	printFieldDiff("mode", "", mode, true)

	// Show nodes that would be created
	fmt.Printf("\n  Nodes:\n")
	for i := 0; i < validators; i++ {
		color.Green("    + %s-%d (validator)", name, i)
	}
	for i := 0; i < yamlDevnet.Spec.FullNodes; i++ {
		color.Green("    + %s-full-%d (full)", name, i)
	}

	// Show labels if present
	if len(yamlDevnet.Metadata.Labels) > 0 {
		fmt.Printf("\n  Labels:\n")
		for k, v := range yamlDevnet.Metadata.Labels {
			color.Green("    + %s: %s", k, v)
		}
	}

	return true, nil
}

// printUpdateDiff shows what would change for an existing devnet
func printUpdateDiff(yamlDevnet *config.YAMLDevnet, existing *v1.Devnet, output string) (bool, error) {
	name := yamlDevnet.Metadata.Name
	hasChanges := false

	// Compare specs
	desiredSpec := diffYamlToProtoSpec(&yamlDevnet.Spec)
	existingSpec := existing.Spec

	changes := []fieldChange{}

	// Compare plugin/network
	if desiredSpec.Plugin != existingSpec.Plugin {
		changes = append(changes, fieldChange{"network", existingSpec.Plugin, desiredSpec.Plugin})
	}

	// Compare network type
	if desiredSpec.NetworkType != existingSpec.NetworkType {
		changes = append(changes, fieldChange{"networkType", existingSpec.NetworkType, desiredSpec.NetworkType})
	}

	// Compare validators
	if desiredSpec.Validators != existingSpec.Validators {
		changes = append(changes, fieldChange{"validators", fmt.Sprintf("%d", existingSpec.Validators), fmt.Sprintf("%d", desiredSpec.Validators)})
	}

	// Compare full nodes
	if desiredSpec.FullNodes != existingSpec.FullNodes {
		changes = append(changes, fieldChange{"fullNodes", fmt.Sprintf("%d", existingSpec.FullNodes), fmt.Sprintf("%d", desiredSpec.FullNodes)})
	}

	// Compare mode
	if desiredSpec.Mode != existingSpec.Mode {
		changes = append(changes, fieldChange{"mode", existingSpec.Mode, desiredSpec.Mode})
	}

	// Compare SDK version
	if desiredSpec.SdkVersion != existingSpec.SdkVersion && desiredSpec.SdkVersion != "" {
		changes = append(changes, fieldChange{"sdkVersion", existingSpec.SdkVersion, desiredSpec.SdkVersion})
	}

	if len(changes) == 0 {
		// No spec changes
		color.White("  devnet/%s (no changes)", name)
		return false, nil
	}

	hasChanges = true
	fmt.Printf("\n")
	color.Yellow("~ devnet/%s (update)", name)
	fmt.Printf("\n")

	for _, c := range changes {
		printFieldDiff(c.field, c.oldVal, c.newVal, false)
	}

	// Show node changes if validator/fullnode count changed
	if desiredSpec.Validators != existingSpec.Validators || desiredSpec.FullNodes != existingSpec.FullNodes {
		fmt.Printf("\n  Nodes:\n")

		// Validator changes
		if desiredSpec.Validators > existingSpec.Validators {
			for i := existingSpec.Validators; i < desiredSpec.Validators; i++ {
				color.Green("    + %s-%d (validator)", name, i)
			}
		} else if desiredSpec.Validators < existingSpec.Validators {
			for i := desiredSpec.Validators; i < existingSpec.Validators; i++ {
				color.Red("    - %s-%d (validator)", name, i)
			}
		}

		// Full node changes
		if desiredSpec.FullNodes > existingSpec.FullNodes {
			for i := existingSpec.FullNodes; i < desiredSpec.FullNodes; i++ {
				color.Green("    + %s-full-%d (full)", name, i)
			}
		} else if desiredSpec.FullNodes < existingSpec.FullNodes {
			for i := desiredSpec.FullNodes; i < existingSpec.FullNodes; i++ {
				color.Red("    - %s-full-%d (full)", name, i)
			}
		}
	}

	return hasChanges, nil
}

type fieldChange struct {
	field  string
	oldVal string
	newVal string
}

// printFieldDiff prints a single field diff
func printFieldDiff(field, oldVal, newVal string, isNew bool) {
	if isNew {
		color.Green("    + %s: %s", field, newVal)
	} else {
		color.Red("    - %s: %s", field, oldVal)
		color.Green("    + %s: %s", field, newVal)
	}
}

// diffYamlToProtoSpec converts YAML spec to proto DevnetSpec (for diff comparison)
func diffYamlToProtoSpec(yamlSpec *config.YAMLDevnetSpec) *v1.DevnetSpec {
	spec := &v1.DevnetSpec{
		Plugin:      yamlSpec.Network,
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
