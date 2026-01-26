// cmd/dvb/get.go
package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	var (
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

  # Get a specific devnet
  dvb get devnet my-devnet

  # Get devnet with node details
  dvb get devnet my-devnet --show-nodes

  # List nodes in a devnet
  dvb get nodes --devnet my-devnet

  # Output in wide format
  dvb get devnets -o wide`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd, args, output, labelSel, showNodes)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: wide, yaml, json")
	cmd.Flags().StringVarP(&labelSel, "selector", "l", "", "Label selector (e.g., 'env=prod')")
	cmd.Flags().BoolVar(&showNodes, "show-nodes", false, "Show nodes when getting a devnet")
	cmd.Flags().BoolP("all-namespaces", "A", false, "List across all namespaces (no-op, for kubectl muscle memory)")

	return cmd
}

func runGet(cmd *cobra.Command, args []string, output, labelSel string, showNodes bool) error {
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
			return getDevnet(cmd, name, output, showNodes)
		}
		return listDevnets(cmd, output, labelSel)
	case "nodes", "node":
		return fmt.Errorf("use 'dvb node list <devnet>' to list nodes")
	default:
		return fmt.Errorf("unknown resource type: %s", resource)
	}
}

func listDevnets(cmd *cobra.Command, output, labelSel string) error {
	devnets, err := daemonClient.ListDevnets(cmd.Context())
	if err != nil {
		return err
	}

	if len(devnets) == 0 {
		fmt.Println("No devnets found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	switch output {
	case "wide":
		fmt.Fprintln(w, "NAME\tPHASE\tNODES\tREADY\tHEIGHT\tMODE\tPLUGIN\tVERSION")
		for _, d := range devnets {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%s\t%s\t%s\n",
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
		fmt.Fprintln(w, "NAME\tPHASE\tNODES\tREADY\tHEIGHT")
		for _, d := range devnets {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n",
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

func getDevnet(cmd *cobra.Command, name, output string, showNodes bool) error {
	devnet, err := daemonClient.GetDevnet(cmd.Context(), name)
	if err != nil {
		return err
	}

	// Print devnet info
	printDevnetDetail(devnet)

	// Optionally show nodes
	if showNodes {
		fmt.Println()
		nodes, err := daemonClient.ListNodes(cmd.Context(), name)
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}
		printNodes(nodes)
	}

	return nil
}

func printDevnetDetail(d *v1.Devnet) {
	fmt.Printf("Name:         %s\n", d.Metadata.Name)
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
