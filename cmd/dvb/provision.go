// cmd/dvb/provision.go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	k8syaml "sigs.k8s.io/yaml"
)

// provisionOptions holds options for the provision command
type provisionOptions struct {
	name        string
	namespace   string
	network     string
	networkType string
	validators  int
	fullNodes   int
	mode        string
	chainID     string
	sdkVersion  string
	interactive bool   // Use interactive wizard mode
	listPlugins bool   // List available network plugins
	output      string // Output format (yaml)
}

func newProvisionCmd() *cobra.Command {
	opts := &provisionOptions{}

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision a new devnet",
		Long: `Provision a new devnet via the devnetd daemon.

This command creates a new devnet by delegating to the daemon, which handles
the full provisioning flow: building binary, forking genesis, initializing
node directories, and starting node processes.

The daemon discovers available network plugins from ~/.devnet-builder/plugins/.
Use --list-plugins to see available networks.

Use -i/--interactive for a guided wizard experience.

Examples:
  # List available network plugins
  dvb provision --list-plugins

  # Interactive wizard mode (recommended for first-time users)
  dvb provision -i

  # Provision a devnet with default settings
  dvb provision --name my-devnet

  # Provision with custom settings
  dvb provision --name my-devnet --network cosmos --validators 4

  # Provision with network type (mainnet/testnet fork)
  dvb provision --name my-devnet --network-type mainnet

  # Output spec as YAML (dry-run, does not create devnet)
  dvb provision --name my-devnet -o yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// List plugins mode
			if opts.listPlugins {
				return runListPlugins(cmd.Context())
			}

			// Interactive wizard mode
			if opts.interactive {
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
				// Transfer wizard options to provision options
				opts.name = wizardOpts.Name
				opts.network = wizardOpts.Network
				opts.validators = wizardOpts.Validators
				opts.fullNodes = wizardOpts.FullNodes
				opts.networkType = wizardOpts.ForkNetwork // Map fork network to network type
				opts.mode = wizardOpts.Mode
				opts.chainID = wizardOpts.ChainID
				opts.sdkVersion = wizardOpts.BinaryVersion
			}

			// Output mode - outputs spec without creating devnet
			if opts.output != "" {
				if opts.output != "yaml" {
					return fmt.Errorf("unsupported output format: %q (supported: yaml)", opts.output)
				}
				return formatProvisionYAML(os.Stdout, opts)
			}

			return runProvision(cmd.Context(), opts)
		},
	}

	// Interactive and list modes
	cmd.Flags().BoolVarP(&opts.interactive, "interactive", "i", false, "Use interactive wizard mode")
	cmd.Flags().BoolVar(&opts.listPlugins, "list-plugins", false, "List available network plugins")

	// Name flag (required unless in interactive or list mode)
	cmd.Flags().StringVar(&opts.name, "name", "", "Devnet name (required unless using -i or --list-plugins)")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Namespace (default: 'default')")

	// Network configuration
	cmd.Flags().StringVar(&opts.network, "network", "stable", "Network plugin name (e.g., stable, cosmos)")
	cmd.Flags().StringVar(&opts.networkType, "network-type", "", "Network type for genesis fork (e.g., mainnet, testnet)")
	cmd.Flags().StringVar(&opts.sdkVersion, "sdk-version", "", "SDK/binary version to use")

	// Node configuration
	cmd.Flags().IntVar(&opts.validators, "validators", 4, "Number of validators")
	cmd.Flags().IntVar(&opts.fullNodes, "full-nodes", 0, "Number of full nodes")
	cmd.Flags().StringVar(&opts.mode, "mode", "docker", "Execution mode (docker or local)")
	cmd.Flags().StringVar(&opts.chainID, "chain-id", "", "Chain ID for the devnet (e.g., mainnet-1, mydevnet-1)")

	// Output format
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output format: yaml (outputs spec without creating devnet)")

	return cmd
}

// runListPlugins lists available network plugins from the daemon
func runListPlugins(ctx context.Context) error {
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd")
	}

	networks, err := daemonClient.ListNetworks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	if len(networks) == 0 {
		fmt.Println("No network plugins found.")
		fmt.Println()
		fmt.Println("Install plugins to ~/.devnet-builder/plugins/")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDISPLAY NAME\tBINARY\tVERSION\tNETWORKS")
	for _, n := range networks {
		networks := "-"
		if len(n.AvailableNetworks) > 0 {
			networks = fmt.Sprintf("%v", n.AvailableNetworks)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			n.Name,
			n.DisplayName,
			n.BinaryName,
			n.DefaultBinaryVersion,
			networks,
		)
	}
	w.Flush()

	return nil
}

func runProvision(ctx context.Context, opts *provisionOptions) error {
	// Require daemon to be running
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd\n\nThe provision command requires the devnetd daemon to be running.\nNetwork plugins are loaded by the daemon from ~/.devnet-builder/plugins/")
	}

	// Validate options
	if opts.name == "" {
		return fmt.Errorf("--name is required (or use -i for interactive mode)")
	}
	if opts.validators < 1 {
		return fmt.Errorf("--validators must be at least 1")
	}
	if opts.fullNodes < 0 {
		return fmt.Errorf("--full-nodes cannot be negative")
	}

	// Print provisioning info
	fmt.Fprintf(os.Stderr, "Provisioning devnet via daemon...\n")
	fmt.Fprintf(os.Stderr, "  Name:       %s\n", opts.name)
	fmt.Fprintf(os.Stderr, "  Namespace:  %s\n", opts.namespace)
	fmt.Fprintf(os.Stderr, "  Network:    %s\n", opts.network)
	fmt.Fprintf(os.Stderr, "  Validators: %d\n", opts.validators)
	if opts.fullNodes > 0 {
		fmt.Fprintf(os.Stderr, "  Full Nodes: %d\n", opts.fullNodes)
	}
	if opts.networkType != "" {
		fmt.Fprintf(os.Stderr, "  Fork from:  %s\n", opts.networkType)
	} else {
		fmt.Fprintf(os.Stderr, "  Genesis:    fresh (new chain)\n")
	}
	fmt.Fprintf(os.Stderr, "  Mode:       %s\n", opts.mode)
	if opts.chainID != "" {
		fmt.Fprintf(os.Stderr, "  Chain ID:   %s\n", opts.chainID)
	}
	fmt.Fprintf(os.Stderr, "\n")

	// Build devnet spec
	spec := &v1.DevnetSpec{
		Plugin:      opts.network,
		NetworkType: opts.networkType,
		Validators:  int32(opts.validators),
		FullNodes:   int32(opts.fullNodes),
		Mode:        opts.mode,
		SdkVersion:  opts.sdkVersion,
		ForkNetwork: opts.networkType, // ForkNetwork triggers genesis forking (uses same value as NetworkType)
		ChainId:     opts.chainID,
	}

	// Create devnet via daemon
	devnet, err := daemonClient.CreateDevnet(ctx, opts.namespace, opts.name, spec, nil)
	if err != nil {
		color.Red("Provisioning failed: %v", err)
		return err
	}

	// Print success
	fmt.Fprintf(os.Stderr, "\n")
	color.Green("✓ Devnet %q created", devnet.Metadata.Name)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Namespace:    %s\n", devnet.Metadata.Namespace)
	fmt.Fprintf(os.Stderr, "  Phase:        %s\n", devnet.Status.Phase)
	fmt.Fprintf(os.Stderr, "  Plugin:       %s\n", devnet.Spec.Plugin)
	fmt.Fprintf(os.Stderr, "  Validators:   %d\n", devnet.Spec.Validators)
	if devnet.Spec.FullNodes > 0 {
		fmt.Fprintf(os.Stderr, "  Full Nodes:   %d\n", devnet.Spec.FullNodes)
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "View status with: dvb describe %s\n", opts.name)

	return nil
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

// provisionOptionsToYAML converts provision options to YAML output format
func provisionOptionsToYAML(opts *provisionOptions) *YAMLProvisionOutput {
	return &YAMLProvisionOutput{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLProvisionMetadataOutput{
			Name:      opts.name,
			Namespace: opts.namespace,
		},
		Spec: YAMLProvisionSpecOutput{
			Network:        opts.network,
			NetworkType:    opts.networkType,
			NetworkVersion: opts.sdkVersion,
			Validators:     opts.validators,
			FullNodes:      opts.fullNodes,
			Mode:           opts.mode,
			ChainID:        opts.chainID,
			ForkNetwork:    opts.networkType, // ForkNetwork uses same value as NetworkType
		},
	}
}

// formatProvisionYAML outputs provision options as YAML
func formatProvisionYAML(w io.Writer, opts *provisionOptions) error {
	yamlOutput := provisionOptionsToYAML(opts)
	out, err := k8syaml.Marshal(yamlOutput)
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %w", err)
	}
	fmt.Fprint(w, string(out))
	return nil
}
