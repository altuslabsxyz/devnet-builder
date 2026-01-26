// cmd/dvb/get.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	k8syaml "sigs.k8s.io/yaml"
)

func newGetCmd() *cobra.Command {
	var (
		namespace string
		output    string
		labelSel  string
		showNodes bool
	)

	cmd := &cobra.Command{
		Use:   "get [resource] [name]",
		Short: "Display devnet resources",
		Long: `Display one or many devnet resources.

Resource types:
  devnets, devnet, dn    - Devnet definitions
  nodes, node            - Individual nodes within a devnet

Examples:
  # List all devnets
  dvb get devnets

  # List devnets in a specific namespace
  dvb get devnets -n production

  # Get a specific devnet
  dvb get devnet my-devnet

  # Get devnet with node details
  dvb get devnet my-devnet --show-nodes

  # List nodes in a devnet
  dvb get nodes --devnet my-devnet

  # Output in wide format
  dvb get devnets -o wide`,
		Args: cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// First argument: resource type
				return []string{
					"devnets\tList all devnets",
					"devnet\tGet a specific devnet",
					"dn\tShorthand for devnet",
					"nodes\tList nodes (use with --devnet)",
					"node\tShorthand for nodes",
				}, cobra.ShellCompDirectiveNoFileComp
			}
			// Second argument: resource name - requires daemon connection
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd, args, namespace, output, labelSel, showNodes)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default, empty = all namespaces for list)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: wide, yaml, json")
	cmd.Flags().StringVarP(&labelSel, "selector", "l", "", "Label selector (e.g., 'env=prod')")
	cmd.Flags().BoolVar(&showNodes, "show-nodes", false, "Show nodes when getting a devnet")
	cmd.Flags().BoolP("all-namespaces", "A", false, "List across all namespaces (no-op, for kubectl muscle memory)")

	return cmd
}

func runGet(cmd *cobra.Command, args []string, namespace, output, labelSel string, showNodes bool) error {
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd")
	}

	resource := args[0]
	var name string
	if len(args) > 1 {
		name = args[1]
	}

	switch resource {
	case "devnets", "devnet", "dn":
		if name != "" {
			return getDevnet(cmd, namespace, name, output, showNodes)
		}
		return listDevnets(cmd, namespace, output, labelSel)
	case "nodes", "node":
		return fmt.Errorf("use 'dvb node list <devnet>' to list nodes")
	default:
		return fmt.Errorf("unknown resource type: %s", resource)
	}
}

func listDevnets(cmd *cobra.Command, namespace, output, labelSel string) error {
	devnets, err := daemonClient.ListDevnets(cmd.Context(), namespace)
	if err != nil {
		return err
	}

	if len(devnets) == 0 {
		fmt.Println("No devnets found")
		return nil
	}

	// Handle yaml/json output
	switch output {
	case "yaml":
		for i, d := range devnets {
			if i > 0 {
				fmt.Println("---")
			}
			out, err := k8syaml.Marshal(protoDevnetToYAML(d))
			if err != nil {
				return fmt.Errorf("failed to marshal yaml: %w", err)
			}
			fmt.Print(string(out))
		}
		return nil
	case "json":
		out, err := json.MarshalIndent(devnets, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal json: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	switch output {
	case "wide":
		fmt.Fprintln(w, "NAMESPACE\tNAME\tPHASE\tNODES\tREADY\tHEIGHT\tMODE\tPLUGIN\tVERSION")
		for _, d := range devnets {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\t%s\n",
				d.Metadata.Namespace,
				d.Metadata.Name,
				colorPhase(d.Status.Phase),
				d.Status.Nodes,
				d.Status.ReadyNodes,
				d.Status.CurrentHeight,
				d.Spec.Mode,
				d.Spec.Plugin,
				d.Status.SdkVersion)
		}
	default:
		fmt.Fprintln(w, "NAMESPACE\tNAME\tPHASE\tNODES\tREADY\tHEIGHT")
		for _, d := range devnets {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\n",
				d.Metadata.Namespace,
				d.Metadata.Name,
				colorPhase(d.Status.Phase),
				d.Status.Nodes,
				d.Status.ReadyNodes,
				d.Status.CurrentHeight)
		}
	}
	w.Flush()

	return nil
}

func getDevnet(cmd *cobra.Command, namespace, name, output string, showNodes bool) error {
	devnet, err := daemonClient.GetDevnet(cmd.Context(), namespace, name)
	if err != nil {
		return err
	}

	// Handle yaml/json output
	switch output {
	case "yaml":
		out, err := k8syaml.Marshal(protoDevnetToYAML(devnet))
		if err != nil {
			return fmt.Errorf("failed to marshal yaml: %w", err)
		}
		fmt.Print(string(out))
		return nil
	case "json":
		out, err := json.MarshalIndent(devnet, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal json: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	// Print devnet info (default table format)
	printDevnetDetail(devnet)

	// Optionally show nodes
	if showNodes {
		fmt.Println()
		nodes, err := daemonClient.ListNodes(cmd.Context(), namespace, name)
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}
		printNodes(nodes)
	}

	return nil
}

func printDevnetDetail(d *v1.Devnet) {
	fmt.Printf("Name:         %s\n", d.Metadata.Name)
	fmt.Printf("Namespace:    %s\n", d.Metadata.Namespace)
	fmt.Printf("Phase:        %s\n", colorPhase(d.Status.Phase))
	fmt.Printf("Plugin:       %s\n", d.Spec.Plugin)
	fmt.Printf("Mode:         %s\n", d.Spec.Mode)
	fmt.Printf("Validators:   %d\n", d.Spec.Validators)
	if d.Spec.FullNodes > 0 {
		fmt.Printf("Full Nodes:   %d\n", d.Spec.FullNodes)
	}
	fmt.Printf("Nodes Ready:  %d/%d\n", d.Status.ReadyNodes, d.Status.Nodes)
	if d.Status.CurrentHeight > 0 {
		fmt.Printf("Block Height: %d\n", d.Status.CurrentHeight)
	}
	if d.Status.SdkVersion != "" {
		fmt.Printf("SDK Version:  %s\n", d.Status.SdkVersion)
	}
	if d.Status.Message != "" {
		fmt.Printf("Message:      %s\n", d.Status.Message)
	}
	if len(d.Metadata.Labels) > 0 {
		fmt.Printf("Labels:       ")
		first := true
		for k, v := range d.Metadata.Labels {
			if !first {
				fmt.Printf(", ")
			}
			fmt.Printf("%s=%s", k, v)
			first = false
		}
		fmt.Println()
	}
}

func printNodes(nodes []*v1.Node) {
	if len(nodes) == 0 {
		fmt.Println("No nodes found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "INDEX\tROLE\tPHASE\tHEALTH\tHEIGHT\tPEERS")
	for _, n := range nodes {
		health := "Unknown"
		if n.Status.Health != nil {
			health = n.Status.Health.Status
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\n",
			n.Metadata.Index,
			n.Spec.Role,
			colorPhase(n.Status.Phase),
			health,
			n.Status.BlockHeight,
			n.Status.PeerCount)
	}
	w.Flush()
}

func colorPhase(phase string) string {
	switch phase {
	case "Running":
		return color.GreenString(phase)
	case "Pending", "Provisioning", "Starting":
		return color.YellowString(phase)
	case "Stopped":
		return color.WhiteString(phase)
	case "Degraded", "Unhealthy", "Failed":
		return color.RedString(phase)
	default:
		return phase
	}
}

// YAMLDevnetOutput represents a devnet in kubectl-style YAML format
type YAMLDevnetOutput struct {
	APIVersion string                   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                   `json:"kind" yaml:"kind"`
	Metadata   YAMLDevnetMetadataOutput `json:"metadata" yaml:"metadata"`
	Spec       YAMLDevnetSpecOutput     `json:"spec" yaml:"spec"`
	Status     YAMLDevnetStatusOutput   `json:"status" yaml:"status"`
}

// YAMLDevnetMetadataOutput is the metadata section
type YAMLDevnetMetadataOutput struct {
	Name        string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

// YAMLDevnetSpecOutput is the spec section
type YAMLDevnetSpecOutput struct {
	Network        string `json:"network" yaml:"network"`
	NetworkType    string `json:"networkType,omitempty" yaml:"networkType,omitempty"`
	NetworkVersion string `json:"networkVersion,omitempty" yaml:"networkVersion,omitempty"`
	Validators     int32  `json:"validators" yaml:"validators"`
	FullNodes      int32  `json:"fullNodes,omitempty" yaml:"fullNodes,omitempty"`
	Mode           string `json:"mode" yaml:"mode"`
}

// YAMLDevnetStatusOutput is the status section
type YAMLDevnetStatusOutput struct {
	Phase         string `json:"phase" yaml:"phase"`
	Nodes         int32  `json:"nodes" yaml:"nodes"`
	ReadyNodes    int32  `json:"readyNodes" yaml:"readyNodes"`
	CurrentHeight int64  `json:"currentHeight,omitempty" yaml:"currentHeight,omitempty"`
	SdkVersion    string `json:"sdkVersion,omitempty" yaml:"sdkVersion,omitempty"`
	Message       string `json:"message,omitempty" yaml:"message,omitempty"`
}

// protoDevnetToYAML converts a proto Devnet to YAML output format
func protoDevnetToYAML(d *v1.Devnet) *YAMLDevnetOutput {
	return &YAMLDevnetOutput{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLDevnetMetadataOutput{
			Name:        d.Metadata.Name,
			Namespace:   d.Metadata.Namespace,
			Labels:      d.Metadata.Labels,
			Annotations: d.Metadata.Annotations,
		},
		Spec: YAMLDevnetSpecOutput{
			Network:        d.Spec.Plugin, // proto uses Plugin, YAML uses network
			NetworkType:    d.Spec.NetworkType,
			NetworkVersion: d.Spec.SdkVersion, // proto uses SdkVersion, YAML uses networkVersion
			Validators:     d.Spec.Validators,
			FullNodes:      d.Spec.FullNodes,
			Mode:           d.Spec.Mode,
		},
		Status: YAMLDevnetStatusOutput{
			Phase:         d.Status.Phase,
			Nodes:         d.Status.Nodes,
			ReadyNodes:    d.Status.ReadyNodes,
			CurrentHeight: d.Status.CurrentHeight,
			SdkVersion:    d.Status.SdkVersion,
			Message:       d.Status.Message,
		},
	}
}
