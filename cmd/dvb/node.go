// cmd/dvb/node.go
package main

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage individual nodes",
		Long: `Manage individual nodes within a devnet.

Subcommands allow you to list, start, stop, and restart specific nodes.

Examples:
  # List nodes in a devnet
  dvb node list my-devnet

  # Stop node 1
  dvb node stop my-devnet 1

  # Start node 1
  dvb node start my-devnet 1

  # Restart node 0
  dvb node restart my-devnet 0`,
	}

	cmd.AddCommand(
		newNodeListCmd(),
		newNodeGetCmd(),
		newNodeStartCmd(),
		newNodeStopCmd(),
		newNodeRestartCmd(),
	)

	return cmd
}

func newNodeListCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "list <devnet-name>",
		Short: "List nodes in a devnet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnetName := args[0]
			nodes, err := daemonClient.ListNodes(cmd.Context(), namespace, devnetName)
			if err != nil {
				return err
			}

			if len(nodes) == 0 {
				fmt.Printf("No nodes found in devnet %q\n", devnetName)
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "INDEX\tROLE\tPHASE\tCONTAINER\tRESTARTS")
			for _, n := range nodes {
				containerID := n.Status.ContainerId
				if len(containerID) > 12 {
					containerID = containerID[:12]
				}
				if containerID == "" {
					containerID = "-"
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n",
					n.Metadata.Index,
					n.Spec.Role,
					n.Status.Phase,
					containerID,
					n.Status.RestartCount,
				)
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func newNodeGetCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "get <devnet-name> <index>",
		Short: "Get details of a specific node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnetName := args[0]
			index, err := parseNodeIndex(args[1])
			if err != nil {
				return err
			}

			node, err := daemonClient.GetNode(cmd.Context(), namespace, devnetName, index)
			if err != nil {
				return err
			}

			printNodeStatus(node)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func newNodeStartCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "start <devnet-name> <index>",
		Short: "Start a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnetName := args[0]
			index, err := parseNodeIndex(args[1])
			if err != nil {
				return err
			}

			node, err := daemonClient.StartNode(cmd.Context(), namespace, devnetName, index)
			if err != nil {
				return err
			}

			color.Green("✓ Node %s/%d starting", devnetName, index)
			fmt.Printf("  Phase: %s\n", node.Status.Phase)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func newNodeStopCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "stop <devnet-name> <index>",
		Short: "Stop a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnetName := args[0]
			index, err := parseNodeIndex(args[1])
			if err != nil {
				return err
			}

			node, err := daemonClient.StopNode(cmd.Context(), namespace, devnetName, index)
			if err != nil {
				return err
			}

			color.Green("✓ Node %s/%d stopping", devnetName, index)
			fmt.Printf("  Phase: %s\n", node.Status.Phase)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func newNodeRestartCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "restart <devnet-name> <index>",
		Short: "Restart a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnetName := args[0]
			index, err := parseNodeIndex(args[1])
			if err != nil {
				return err
			}

			node, err := daemonClient.RestartNode(cmd.Context(), namespace, devnetName, index)
			if err != nil {
				return err
			}

			color.Green("✓ Node %s/%d restarting", devnetName, index)
			fmt.Printf("  Phase: %s\n", node.Status.Phase)
			fmt.Printf("  Restarts: %d\n", node.Status.RestartCount)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func parseNodeIndex(s string) (int, error) {
	index, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid node index %q: must be a number", s)
	}
	if index < 0 {
		return 0, fmt.Errorf("invalid node index %d: must be non-negative", index)
	}
	return index, nil
}

func printNodeStatus(n *v1.Node) {
	// Phase with color
	phase := n.Status.Phase
	switch phase {
	case "Running":
		color.Green("● %s", phase)
	case "Pending", "Starting":
		color.Yellow("◐ %s", phase)
	case "Stopped":
		color.White("○ %s", phase)
	case "Stopping":
		color.Yellow("◑ %s", phase)
	case "Crashed":
		color.Red("✗ %s", phase)
	default:
		fmt.Printf("? %s", phase)
	}

	fmt.Printf("\nDevnet:     %s\n", n.Metadata.DevnetName)
	fmt.Printf("Index:      %d\n", n.Metadata.Index)
	fmt.Printf("Role:       %s\n", n.Spec.Role)

	if n.Spec.DesiredPhase != "" {
		fmt.Printf("Desired:    %s\n", n.Spec.DesiredPhase)
	}

	if n.Status.ContainerId != "" {
		containerID := n.Status.ContainerId
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}
		fmt.Printf("Container:  %s\n", containerID)
	}

	if n.Status.Pid > 0 {
		fmt.Printf("PID:        %d\n", n.Status.Pid)
	}

	if n.Status.BlockHeight > 0 {
		fmt.Printf("Height:     %d\n", n.Status.BlockHeight)
	}

	fmt.Printf("Restarts:   %d\n", n.Status.RestartCount)

	if n.Status.Message != "" {
		fmt.Printf("Message:    %s\n", n.Status.Message)
	}
}
