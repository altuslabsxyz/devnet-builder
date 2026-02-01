// cmd/dvb/provision.go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	k8syaml "sigs.k8s.io/yaml"
)

// ProvisionMode represents the mode of operation for the provision command
type ProvisionMode int

const (
	// InteractiveMode runs the interactive wizard
	InteractiveMode ProvisionMode = iota
	// FlagMode uses command-line flags
	FlagMode
	// FileMode loads configuration from a YAML file
	FileMode
)

// provisionOptions holds options for the provision command
type provisionOptions struct {
	name          string
	namespace     string
	network       string
	networkType   string
	validators    int
	fullNodes     int
	mode          string
	binaryVersion string
	file          string // YAML config file path
	dryRun        bool   // Preview changes without applying
	listPlugins   bool   // List available network plugins
}

func newProvisionCmd() *cobra.Command {
	opts := &provisionOptions{}

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision a new devnet",
		Long: `Provision a new devnet via the devnetd daemon.

This command creates or updates a devnet by delegating to the daemon, which handles
the full provisioning flow: building binary, forking genesis, initializing
node directories, and starting node processes.

The daemon discovers available network plugins from ~/.devnet-builder/plugins/.
Use --list-plugins to see available networks.

Run without arguments for an interactive wizard experience.

Examples:
  # List available network plugins
  dvb provision --list-plugins

  # Interactive wizard mode (no args required)
  dvb provision

  # Provision with flags (--name and --network required)
  dvb provision --name my-devnet --network stable

  # Provision with custom settings
  dvb provision --name my-devnet --network cosmos --validators 4

  # Provision from a YAML file
  dvb provision -f devnet.yaml

  # Preview changes without applying (dry-run)
  dvb provision --name my-devnet --network stable --dry-run
  dvb provision -f devnet.yaml --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// List plugins mode
			if opts.listPlugins {
				return runListPlugins(cmd.Context())
			}

			// Detect provision mode
			mode := detectProvisionMode(opts)

			switch mode {
			case FileMode:
				return runFileMode(cmd.Context(), opts)
			case FlagMode:
				return runFlagMode(cmd.Context(), opts)
			case InteractiveMode:
				return runInteractiveMode(cmd.Context(), opts)
			default:
				return fmt.Errorf("unknown provision mode")
			}
		},
	}

	// File mode
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "YAML config file")

	// List plugins
	cmd.Flags().BoolVar(&opts.listPlugins, "list-plugins", false, "List available network plugins")

	// Dry-run mode
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Preview changes without applying")

	// Name and namespace
	cmd.Flags().StringVar(&opts.name, "name", "", "Devnet name (required in flag mode)")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Namespace")

	// Network configuration
	cmd.Flags().StringVar(&opts.network, "network", "stable", "Network plugin name (e.g., stable, cosmos)")
	cmd.Flags().StringVar(&opts.networkType, "network-type", "", "Network type for genesis fork (e.g., mainnet, testnet)")
	cmd.Flags().StringVar(&opts.binaryVersion, "binary-version", "", "Binary version to use")

	// Node configuration
	cmd.Flags().IntVar(&opts.validators, "validators", 4, "Number of validators")
	cmd.Flags().IntVar(&opts.fullNodes, "full-nodes", 0, "Number of full nodes")
	cmd.Flags().StringVar(&opts.mode, "mode", "docker", "Execution mode (docker or local)")

	// Mark flags as mutually exclusive
	cmd.MarkFlagsMutuallyExclusive("file", "name")
	cmd.MarkFlagsMutuallyExclusive("dry-run", "list-plugins")

	return cmd
}

// detectProvisionMode determines which mode to use based on flags
func detectProvisionMode(opts *provisionOptions) ProvisionMode {
	// Order: file > flags > interactive
	if opts.file != "" {
		return FileMode
	}
	if opts.name != "" {
		return FlagMode
	}
	return InteractiveMode
}

// runInteractiveMode handles interactive wizard mode
func runInteractiveMode(ctx context.Context, opts *provisionOptions) error {
	// Require daemon to be running
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd\n\nThe provision command requires the devnetd daemon to be running.\nNetwork plugins are loaded by the daemon from ~/.devnet-builder/plugins/")
	}

	// Run the wizard to collect options
	wizardOpts, err := RunProvisionWizard(daemonClient)
	if err != nil {
		if err.Error() == "cancelled" {
			return nil
		}
		return err
	}
	if wizardOpts == nil {
		return nil // User cancelled
	}

	// Build spec from wizard options
	spec := &v1.DevnetSpec{
		Plugin:      wizardOpts.Network,
		NetworkType: wizardOpts.ForkNetwork,
		Validators:  int32(wizardOpts.Validators),
		FullNodes:   int32(wizardOpts.FullNodes),
		Mode:        wizardOpts.Mode,
		SdkVersion:  wizardOpts.BinaryVersion,
		ForkNetwork: wizardOpts.ForkNetwork,
		ChainId:     wizardOpts.ChainID,
	}

	namespace := "default"
	if opts.namespace != "" {
		namespace = opts.namespace
	}

	// Handle upsert logic with wizard confirmation
	return executeUpsert(ctx, namespace, wizardOpts.Name, spec, nil, nil, opts.dryRun, false)
}

// runFlagMode handles flag-based provisioning
func runFlagMode(ctx context.Context, opts *provisionOptions) error {
	// Require daemon to be running
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd\n\nThe provision command requires the devnetd daemon to be running.\nNetwork plugins are loaded by the daemon from ~/.devnet-builder/plugins/")
	}

	// Validate required flags
	if opts.name == "" {
		return fmt.Errorf("--name is required in flag mode")
	}
	if opts.network == "" {
		return fmt.Errorf("--network is required in flag mode")
	}

	// Validate options
	if opts.validators < 1 {
		return fmt.Errorf("--validators must be at least 1")
	}
	if opts.fullNodes < 0 {
		return fmt.Errorf("--full-nodes cannot be negative")
	}
	if opts.mode != "docker" && opts.mode != "local" {
		return fmt.Errorf("--mode must be 'docker' or 'local'")
	}

	// Build devnet spec
	spec := &v1.DevnetSpec{
		Plugin:      opts.network,
		NetworkType: opts.networkType,
		Validators:  int32(opts.validators),
		FullNodes:   int32(opts.fullNodes),
		Mode:        opts.mode,
		SdkVersion:  opts.binaryVersion,
		ForkNetwork: opts.networkType,
	}

	namespace := opts.namespace
	if namespace == "" {
		namespace = "default"
	}

	// Handle upsert logic with confirmation prompt
	return executeUpsert(ctx, namespace, opts.name, spec, nil, nil, opts.dryRun, false)
}

// runFileMode handles file-based provisioning
func runFileMode(ctx context.Context, opts *provisionOptions) error {
	// Require daemon to be running
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd\n\nThe provision command requires the devnetd daemon to be running.\nNetwork plugins are loaded by the daemon from ~/.devnet-builder/plugins/")
	}

	// Load and validate the YAML file
	loader := config.NewYAMLLoader()
	devnets, err := loader.LoadFile(opts.file)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// File mode only supports single devnet
	if len(devnets) != 1 {
		return fmt.Errorf("file mode requires exactly one devnet definition, found %d", len(devnets))
	}

	yamlDevnet := devnets[0]
	proto := yamlDevnet.ToProto()

	namespace := proto.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// File mode updates silently (declarative intent)
	return executeUpsert(ctx, namespace, proto.Metadata.Name, proto.Spec, proto.Metadata.Labels, proto.Metadata.Annotations, opts.dryRun, true)
}

// CheckDevnetExists checks if a devnet exists via the daemon
func CheckDevnetExists(ctx context.Context, namespace, name string) (bool, *v1.Devnet, error) {
	if daemonClient == nil {
		return false, nil, fmt.Errorf("daemon not running")
	}

	devnet, err := daemonClient.GetDevnet(ctx, namespace, name)
	if err != nil {
		// Check if the error indicates the devnet doesn't exist
		if strings.Contains(err.Error(), "not found") {
			return false, nil, nil
		}
		return false, nil, err
	}

	return true, devnet, nil
}

// executeUpsert handles the create/update logic with appropriate confirmation
func executeUpsert(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels, annotations map[string]string, dryRun, silentUpdate bool) error {
	// Check if devnet exists
	exists, currentDevnet, err := CheckDevnetExists(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to check devnet existence: %w", err)
	}

	// Handle dry-run mode
	if dryRun {
		return PrintDryRun(namespace, name, spec, exists, currentDevnet)
	}

	// If exists and not silent update, prompt for confirmation
	if exists && !silentUpdate {
		confirmed, err := ConfirmUpdate(name, currentDevnet, spec)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Execute create or update
	if exists {
		return executeUpdate(ctx, namespace, name, spec, labels, annotations)
	}
	return executeCreate(ctx, namespace, name, spec, labels)
}

// ConfirmUpdate prompts the user to confirm an update operation.
// This function is exported for use by wizard.go.
func ConfirmUpdate(name string, current *v1.Devnet, proposed *v1.DevnetSpec) (bool, error) {
	fmt.Printf("\nDevnet %q already exists.\n\n", name)

	for {
		fmt.Print("[U]pdate  [V]iew changes  [C]ancel: ")
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "u", "update":
			return true, nil
		case "v", "view":
			PrintDiff(current, proposed)
			fmt.Println()
		case "c", "cancel", "":
			return false, nil
		default:
			fmt.Println("Invalid option. Please enter U, V, or C.")
		}
	}
}

// PrintDiff prints a side-by-side diff of current vs proposed spec.
// This function is exported for use by wizard.go.
func PrintDiff(current *v1.Devnet, proposed *v1.DevnetSpec) {
	if current == nil || current.Spec == nil || proposed == nil {
		return
	}

	currentSpec := current.Spec

	fmt.Println()
	fmt.Println("+-----------------+-----------------+")
	fmt.Println("| Current         | Proposed        |")
	fmt.Println("+-----------------+-----------------+")

	// Compare fields
	printDiffRow("validators", fmt.Sprintf("%d", currentSpec.Validators), fmt.Sprintf("%d", proposed.Validators))
	printDiffRow("fullNodes", fmt.Sprintf("%d", currentSpec.FullNodes), fmt.Sprintf("%d", proposed.FullNodes))
	printDiffRow("mode", currentSpec.Mode, proposed.Mode)
	printDiffRow("network", currentSpec.Plugin, proposed.Plugin)
	printDiffRow("networkType", currentSpec.NetworkType, proposed.NetworkType)
	if currentSpec.SdkVersion != "" || proposed.SdkVersion != "" {
		printDiffRow("binaryVersion", currentSpec.SdkVersion, proposed.SdkVersion)
	}

	fmt.Println("+-----------------+-----------------+")
}

// printDiffRow prints a single row in the diff table
func printDiffRow(field, current, proposed string) {
	if current == proposed {
		fmt.Printf("| %-15s | %-15s |\n", fmt.Sprintf("%s: %s", field, current), fmt.Sprintf("%s: %s", field, proposed))
	} else {
		// Highlight changed values
		currentStr := fmt.Sprintf("%s: %s", field, current)
		proposedStr := fmt.Sprintf("%s: %s", field, proposed)
		fmt.Printf("| %-15s | %s |\n", currentStr, color.YellowString("%-15s", proposedStr))
	}
}

// PrintDryRun outputs the YAML spec that would be applied.
// This function is exported for use by wizard.go.
func PrintDryRun(namespace, name string, spec *v1.DevnetSpec, exists bool, currentDevnet *v1.Devnet) error {
	// Print action header
	action := "CREATED"
	if exists {
		action = "UPDATED"
	}
	fmt.Printf("# Dry-run: devnet/%s would be %s\n\n", name, action)

	// Build YAML output
	yamlOutput := &YAMLProvisionOutput{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLProvisionMetadataOutput{
			Name:      name,
			Namespace: namespace,
		},
		Spec: YAMLProvisionSpecOutput{
			Network:        spec.Plugin,
			NetworkType:    spec.NetworkType,
			NetworkVersion: spec.SdkVersion,
			Validators:     int(spec.Validators),
			FullNodes:      int(spec.FullNodes),
			Mode:           spec.Mode,
		},
	}

	out, err := k8syaml.Marshal(yamlOutput)
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %w", err)
	}

	// If updating, add inline comments for changed fields
	if exists && currentDevnet != nil && currentDevnet.Spec != nil {
		currentSpec := currentDevnet.Spec
		lines := strings.Split(string(out), "\n")
		var annotatedLines []string

		for _, line := range lines {
			annotatedLine := line

			// Add "# was: X" comments for changed fields
			if strings.Contains(line, "validators:") && spec.Validators != currentSpec.Validators {
				annotatedLine = fmt.Sprintf("%s      # was: %d", line, currentSpec.Validators)
			} else if strings.Contains(line, "fullNodes:") && spec.FullNodes != currentSpec.FullNodes {
				annotatedLine = fmt.Sprintf("%s       # was: %d", line, currentSpec.FullNodes)
			} else if strings.Contains(line, "mode:") && spec.Mode != currentSpec.Mode {
				annotatedLine = fmt.Sprintf("%s            # was: %s", line, currentSpec.Mode)
			} else if strings.Contains(line, "networkType:") && spec.NetworkType != currentSpec.NetworkType {
				annotatedLine = fmt.Sprintf("%s     # was: %s", line, currentSpec.NetworkType)
			}

			annotatedLines = append(annotatedLines, annotatedLine)
		}

		fmt.Print(strings.Join(annotatedLines, "\n"))
	} else {
		fmt.Print(string(out))
	}

	return nil
}

// executeCreate creates a new devnet
func executeCreate(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels map[string]string) error {
	// Print provisioning info
	fmt.Fprintf(os.Stderr, "Provisioning devnet via daemon...\n")
	fmt.Fprintf(os.Stderr, "  Name:       %s\n", name)
	fmt.Fprintf(os.Stderr, "  Namespace:  %s\n", namespace)
	fmt.Fprintf(os.Stderr, "  Network:    %s\n", spec.Plugin)
	fmt.Fprintf(os.Stderr, "  Validators: %d\n", spec.Validators)
	if spec.FullNodes > 0 {
		fmt.Fprintf(os.Stderr, "  Full Nodes: %d\n", spec.FullNodes)
	}
	if spec.NetworkType != "" {
		fmt.Fprintf(os.Stderr, "  Fork from:  %s\n", spec.NetworkType)
	} else {
		fmt.Fprintf(os.Stderr, "  Genesis:    fresh (new chain)\n")
	}
	fmt.Fprintf(os.Stderr, "  Mode:       %s\n", spec.Mode)
	fmt.Fprintf(os.Stderr, "\n")

	// Create devnet via daemon
	devnet, err := daemonClient.CreateDevnet(ctx, namespace, name, spec, labels)
	if err != nil {
		color.Red("Provisioning failed: %v", err)
		return err
	}

	// Print success
	fmt.Fprintf(os.Stderr, "\n")
	color.Green("Devnet %q created", devnet.Metadata.Name)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Namespace:    %s\n", devnet.Metadata.Namespace)
	fmt.Fprintf(os.Stderr, "  Phase:        %s\n", devnet.Status.Phase)
	fmt.Fprintf(os.Stderr, "  Plugin:       %s\n", devnet.Spec.Plugin)
	fmt.Fprintf(os.Stderr, "  Validators:   %d\n", devnet.Spec.Validators)
	if devnet.Spec.FullNodes > 0 {
		fmt.Fprintf(os.Stderr, "  Full Nodes:   %d\n", devnet.Spec.FullNodes)
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "View status with: dvb describe %s\n", name)

	return nil
}

// executeUpdate updates an existing devnet
func executeUpdate(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels, annotations map[string]string) error {
	fmt.Fprintf(os.Stderr, "Updating devnet %q...\n", name)

	// Use ApplyDevnet for updates (idempotent)
	resp, err := daemonClient.ApplyDevnet(ctx, namespace, name, spec, labels, annotations)
	if err != nil {
		color.Red("Update failed: %v", err)
		return err
	}

	// Print success based on action
	fmt.Fprintf(os.Stderr, "\n")
	switch resp.Action {
	case "unchanged":
		color.Yellow("Devnet %q unchanged (already at desired state)", name)
	case "configured":
		color.Green("Devnet %q updated", name)
	default:
		color.Green("Devnet %q %s", name, resp.Action)
	}

	if resp.Devnet != nil {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "  Namespace:    %s\n", resp.Devnet.Metadata.Namespace)
		fmt.Fprintf(os.Stderr, "  Phase:        %s\n", resp.Devnet.Status.Phase)
		fmt.Fprintf(os.Stderr, "  Plugin:       %s\n", resp.Devnet.Spec.Plugin)
		fmt.Fprintf(os.Stderr, "  Validators:   %d\n", resp.Devnet.Spec.Validators)
		if resp.Devnet.Spec.FullNodes > 0 {
			fmt.Fprintf(os.Stderr, "  Full Nodes:   %d\n", resp.Devnet.Spec.FullNodes)
		}
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "View status with: dvb describe %s\n", name)

	return nil
}

// runListPlugins lists available network plugins from the daemon.
// Delegates to runPluginsList to avoid code duplication.
func runListPlugins(ctx context.Context) error {
	return runPluginsList(ctx)
}

// YAMLProvisionOutput represents a provision spec in kubectl-style YAML format
type YAMLProvisionOutput struct {
	APIVersion string                      `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                      `json:"kind" yaml:"kind"`
	Metadata   YAMLProvisionMetadataOutput `json:"metadata" yaml:"metadata"`
	Spec       YAMLProvisionSpecOutput     `json:"spec" yaml:"spec"`
}

// YAMLProvisionMetadataOutput is the metadata section for provision output
type YAMLProvisionMetadataOutput struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// YAMLProvisionSpecOutput is the spec section for provision output
type YAMLProvisionSpecOutput struct {
	Network        string `json:"network" yaml:"network"`
	NetworkType    string `json:"networkType,omitempty" yaml:"networkType,omitempty"`
	NetworkVersion string `json:"networkVersion,omitempty" yaml:"networkVersion,omitempty"`
	Validators     int    `json:"validators" yaml:"validators"`
	FullNodes      int    `json:"fullNodes,omitempty" yaml:"fullNodes,omitempty"`
	Mode           string `json:"mode" yaml:"mode"`
	ChainID        string `json:"chainId,omitempty" yaml:"chainId,omitempty"`
	ForkNetwork    string `json:"forkNetwork,omitempty" yaml:"forkNetwork,omitempty"`
}

// formatProvisionYAML outputs provision options as YAML (kept for compatibility)
func formatProvisionYAML(w io.Writer, namespace, name string, spec *v1.DevnetSpec) error {
	yamlOutput := &YAMLProvisionOutput{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLProvisionMetadataOutput{
			Name:      name,
			Namespace: namespace,
		},
		Spec: YAMLProvisionSpecOutput{
			Network:        spec.Plugin,
			NetworkType:    spec.NetworkType,
			NetworkVersion: spec.SdkVersion,
			Validators:     int(spec.Validators),
			FullNodes:      int(spec.FullNodes),
			Mode:           spec.Mode,
		},
	}
	out, err := k8syaml.Marshal(yamlOutput)
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %w", err)
	}
	fmt.Fprint(w, string(out))
	return nil
}
