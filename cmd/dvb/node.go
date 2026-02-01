// cmd/dvb/node.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/dvbcontext"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/cosmos"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage individual nodes",
		Long: `Manage individual nodes within a devnet.

Subcommands allow you to list, start, stop, restart nodes and view logs.
When context is set (dvb use <devnet>), most commands support interactive
node selection if no index is provided.

Examples:
  # Set context first
  dvb use my-devnet

  # List nodes
  dvb node list

  # View logs (interactive picker if multiple nodes)
  dvb node logs

  # Start/stop/restart with picker
  dvb node start
  dvb node stop
  dvb node restart

  # Or specify index explicitly
  dvb node logs 0
  dvb node start 1`,
	}

	cmd.AddCommand(
		newNodeListCmd(),
		newNodeGetCmd(),
		newNodeHealthCmd(),
		newNodePortsCmd(),
		newNodeLogsCmd(),
		newNodeStartCmd(),
		newNodeStopCmd(),
		newNodeRestartCmd(),
		newNodeExecCmd(),
		newNodeInitCmd(),
	)

	return cmd
}

// nodeListOptions holds options for the node list command
type nodeListOptions struct {
	namespace string
	watch     bool
	interval  int
	wide      bool
}

func newNodeListCmd() *cobra.Command {
	opts := &nodeListOptions{}

	cmd := &cobra.Command{
		Use:   "list [devnet-name]",
		Short: "List nodes in a devnet",
		Long: `List nodes in a devnet with their status.

With context set (dvb use <devnet>), the devnet argument is optional.

Use -w/--watch to continuously monitor node status in real-time.
Press Ctrl+C to stop watching.

Examples:
  # List nodes using context
  dvb use my-devnet
  dvb node list

  # List all nodes in a devnet (explicit)
  dvb node list my-devnet

  # Watch node status in real-time (updates every 2 seconds)
  dvb node list -w

  # Watch with custom interval (5 seconds)
  dvb node list -w --interval 5

  # Wide output with additional details
  dvb node list --wide`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			if len(args) > 0 {
				explicitDevnet = args[0]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, opts.namespace)
			if err != nil {
				return err
			}
			opts.namespace = ns

			// Print context header (skip for watch mode as it clears screen)
			if !opts.watch {
				printContextHeader(explicitDevnet, currentContext)
			}

			if opts.watch {
				return runNodeListWatch(cmd.Context(), devnetName, opts)
			}

			return runNodeListOnce(cmd.Context(), devnetName, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Namespace (defaults to server default)")
	cmd.Flags().BoolVarP(&opts.watch, "watch", "w", false, "Watch for changes (like kubectl -w)")
	cmd.Flags().IntVar(&opts.interval, "interval", 2, "Watch interval in seconds (default: 2)")
	cmd.Flags().BoolVar(&opts.wide, "wide", false, "Wide output with additional details")

	return cmd
}

// runNodeListOnce runs a single list operation
func runNodeListOnce(ctx context.Context, devnetName string, opts *nodeListOptions) error {
	nodes, err := daemonClient.ListNodes(ctx, opts.namespace, devnetName)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		fmt.Printf("No nodes found in devnet %q\n", devnetName)
		return nil
	}

	printNodeTable(nodes, opts.wide)
	return nil
}

// runNodeListWatch continuously watches node status
func runNodeListWatch(ctx context.Context, devnetName string, opts *nodeListOptions) error {
	interval := time.Duration(opts.interval) * time.Second
	if interval < time.Second {
		interval = time.Second
	}

	// Track previous state for change detection
	previousPhases := make(map[int32]string)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Print initial state
	nodes, err := daemonClient.ListNodes(ctx, opts.namespace, devnetName)
	if err != nil {
		return err
	}

	// Store initial phases
	for _, n := range nodes {
		previousPhases[n.Metadata.Index] = n.Status.Phase
	}

	// Clear screen and print header
	clearScreen()
	printWatchHeader(devnetName, interval)
	printNodeTable(nodes, opts.wide)

	// Watch loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			nodes, err := daemonClient.ListNodes(ctx, opts.namespace, devnetName)
			if err != nil {
				// Print error but continue watching
				fmt.Fprintf(os.Stderr, "\rError: %v", err)
				continue
			}

			// Check for changes
			hasChanges := false
			for _, n := range nodes {
				if prev, ok := previousPhases[n.Metadata.Index]; !ok || prev != n.Status.Phase {
					hasChanges = true
					previousPhases[n.Metadata.Index] = n.Status.Phase
				}
			}

			// Always refresh in watch mode
			clearScreen()
			printWatchHeader(devnetName, interval)
			printNodeTable(nodes, opts.wide)

			// Show change indicator
			if hasChanges {
				color.Yellow("  (changed)")
			}
		}
	}
}

// clearScreen clears the terminal screen
func clearScreen() {
	// ANSI escape sequence to clear screen and move cursor to top-left
	fmt.Print("\033[2J\033[H")
}

// printWatchHeader prints the watch mode header
func printWatchHeader(devnetName string, interval time.Duration) {
	now := time.Now().Format("15:04:05")
	fmt.Printf("Every %s: dvb node list %s    %s\n\n",
		formatDuration(interval), devnetName, now)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d >= time.Minute {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

// printNodeTable prints nodes in a table format
func printNodeTable(nodes []*v1.Node, wide bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if wide {
		fmt.Fprintln(w, "INDEX\tHEALTH\tROLE\tPHASE\tCONTAINER\tRPC ENDPOINT\tRESTARTS\tMESSAGE")
	} else {
		fmt.Fprintln(w, "INDEX\tHEALTH\tROLE\tPHASE\tCONTAINER\tRESTARTS")
	}

	for _, n := range nodes {
		containerID := n.Status.ContainerId
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}
		if containerID == "" {
			containerID = "-"
		}

		healthIcon := getHealthIcon(n.Status.Phase)

		if wide {
			// Determine RPC endpoint: use node's Address if set (loopback subnet mode)
			// or fall back to legacy port offset calculation
			var rpcEndpoint string
			if n.Spec.Address != "" {
				rpcEndpoint = fmt.Sprintf("%s:26657", n.Spec.Address)
			} else {
				rpcPort := 26657 + int(n.Metadata.Index)*100
				rpcEndpoint = fmt.Sprintf("localhost:%d", rpcPort)
			}
			message := n.Status.Message
			if len(message) > 30 {
				message = message[:27] + "..."
			}
			if message == "" {
				message = "-"
			}

			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
				n.Metadata.Index,
				healthIcon,
				n.Spec.Role,
				n.Status.Phase,
				containerID,
				rpcEndpoint,
				n.Status.RestartCount,
				message,
			)
		} else {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%d\n",
				n.Metadata.Index,
				healthIcon,
				n.Spec.Role,
				n.Status.Phase,
				containerID,
				n.Status.RestartCount,
			)
		}
	}
	w.Flush()
}

// getNodeListSummary returns a summary string of node states
func getNodeListSummary(nodes []*v1.Node) string {
	running, stopped, other := 0, 0, 0
	for _, n := range nodes {
		switch n.Status.Phase {
		case "Running":
			running++
		case "Stopped":
			stopped++
		default:
			other++
		}
	}

	parts := []string{}
	if running > 0 {
		parts = append(parts, color.GreenString("%d running", running))
	}
	if stopped > 0 {
		parts = append(parts, color.WhiteString("%d stopped", stopped))
	}
	if other > 0 {
		parts = append(parts, color.YellowString("%d other", other))
	}

	return strings.Join(parts, ", ")
}

// getHealthIcon returns a colored health indicator based on node phase.
func getHealthIcon(phase string) string {
	switch phase {
	case "Running":
		return color.GreenString("●")
	case "Crashed":
		return color.RedString("✗")
	case "Stopped":
		return color.WhiteString("○")
	case "Pending", "Starting", "Stopping":
		return color.YellowString("◐")
	default:
		return color.YellowString("?")
	}
}

func newNodeGetCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "get [devnet-name] [index]",
		Short: "Get details of a specific node",
		Long: `Get details of a specific node.

With context set (dvb use <devnet>), the index is optional.
If not provided, an interactive picker will appear.

Examples:
  # Get node details using context with picker
  dvb use my-devnet
  dvb node get

  # Get node details using context
  dvb use my-devnet
  dvb node get 0

  # Get node details (explicit devnet)
  dvb node get my-devnet 0`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			var indexArg string

			if len(args) == 0 {
				// No args - use context and picker
			} else if len(args) == 1 {
				// Could be index (with context) or devnet name (without index)
				if _, err := parseNodeIndex(args[0]); err == nil {
					// It's a valid index
					indexArg = args[0]
				} else {
					// Treat as devnet name
					explicitDevnet = args[0]
				}
			} else {
				explicitDevnet = args[0]
				indexArg = args[1]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			var index int
			if indexArg == "" {
				// Use picker
				index, err = dvbcontext.PickNode(daemonClient, ns, devnetName)
				if err != nil {
					return fmt.Errorf("failed to pick node: %w", err)
				}
			} else {
				index, err = parseNodeIndex(indexArg)
				if err != nil {
					return err
				}
			}

			node, err := daemonClient.GetNode(cmd.Context(), ns, devnetName, index)
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

func newNodeHealthCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "health [devnet-name] [index]",
		Short: "Get health status of a node",
		Long: `Display the health status of a specific node.

With context set (dvb use <devnet>), the index is optional.
If not provided, an interactive picker will appear.

Shows whether the node is healthy, unhealthy, stopped, or in a transitional state.
This is useful for monitoring node status and diagnosing issues.

Health status values:
  - Healthy:       Node is running normally
  - Unhealthy:     Node has health check failures
  - Stopped:       Node is intentionally stopped
  - Transitioning: Node is changing state (starting, stopping)
  - Unknown:       Health cannot be determined

Examples:
  # Check health using context with picker
  dvb use my-devnet
  dvb node health

  # Check health using context
  dvb use my-devnet
  dvb node health 0

  # Check health of node 0 (explicit devnet)
  dvb node health my-devnet 0

  # Check health of node 1
  dvb node health my-devnet 1`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			var indexArg string

			if len(args) == 0 {
				// No args - use context and picker
			} else if len(args) == 1 {
				// Could be index (with context) or devnet name (without index)
				if _, err := parseNodeIndex(args[0]); err == nil {
					indexArg = args[0]
				} else {
					explicitDevnet = args[0]
				}
			} else {
				explicitDevnet = args[0]
				indexArg = args[1]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			var index int
			if indexArg == "" {
				index, err = dvbcontext.PickNode(daemonClient, ns, devnetName)
				if err != nil {
					return fmt.Errorf("failed to pick node: %w", err)
				}
			} else {
				index, err = parseNodeIndex(indexArg)
				if err != nil {
					return err
				}
			}

			health, err := daemonClient.GetNodeHealth(cmd.Context(), devnetName, index)
			if err != nil {
				return err
			}

			// Print health status with color
			printHealthStatus(devnetName, index, health)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func newNodePortsCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "ports [devnet-name] [index]",
		Short: "Show port mappings for a node",
		Long: `Display the port mappings for a specific node.

With context set (dvb use <devnet>), the index is optional.
If not provided, an interactive picker will appear.

Each node in a devnet has its ports offset by index * 100 to avoid conflicts.
This command shows both container ports and their mapped host ports.

Examples:
  # Show ports using context with picker
  dvb use my-devnet
  dvb node ports

  # Show ports using context
  dvb use my-devnet
  dvb node ports 0

  # Show ports for node 0 (host ports: 26656, 26657, 1317, 9090)
  dvb node ports my-devnet 0

  # Show ports for node 1 (host ports: 26756, 26757, 1417, 9190)
  dvb node ports my-devnet 1`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			var indexArg string

			if len(args) == 0 {
				// No args - use context and picker
			} else if len(args) == 1 {
				// Could be index (with context) or devnet name (without index)
				if _, err := parseNodeIndex(args[0]); err == nil {
					indexArg = args[0]
				} else {
					explicitDevnet = args[0]
				}
			} else {
				explicitDevnet = args[0]
				indexArg = args[1]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			var index int
			if indexArg == "" {
				index, err = dvbcontext.PickNode(daemonClient, ns, devnetName)
				if err != nil {
					return fmt.Errorf("failed to pick node: %w", err)
				}
			} else {
				index, err = parseNodeIndex(indexArg)
				if err != nil {
					return err
				}
			}

			ports, err := daemonClient.GetNodePorts(cmd.Context(), devnetName, index)
			if err != nil {
				return err
			}

			fmt.Printf("Ports for %s/%d:\n\n", ports.DevnetName, ports.Index)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SERVICE\tCONTAINER\tHOST\tPROTOCOL")
			for _, p := range ports.Ports {
				fmt.Fprintf(w, "%s\t%d\t%d\t%s\n",
					p.Name,
					p.ContainerPort,
					p.HostPort,
					p.Protocol,
				)
			}
			w.Flush()

			// Print helpful URLs
			fmt.Println()
			for _, p := range ports.Ports {
				switch p.Name {
				case "rpc":
					fmt.Printf("RPC endpoint:  http://localhost:%d\n", p.HostPort)
				case "rest":
					fmt.Printf("REST endpoint: http://localhost:%d\n", p.HostPort)
				case "grpc":
					fmt.Printf("gRPC endpoint: localhost:%d\n", p.HostPort)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func newNodeStartCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "start [devnet-name] [index]",
		Short: "Start a node",
		Long: `Start a stopped node.

With context set (dvb use <devnet>), the index is optional.
If not provided, an interactive picker will appear.

Examples:
  # Start node using context with picker
  dvb use my-devnet
  dvb node start

  # Start node using context
  dvb use my-devnet
  dvb node start 0

  # Start node (explicit devnet)
  dvb node start my-devnet 0`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			var indexArg string

			if len(args) == 0 {
				// No args - use context and picker
			} else if len(args) == 1 {
				// Could be index (with context) or devnet name (without index)
				if _, err := parseNodeIndex(args[0]); err == nil {
					indexArg = args[0]
				} else {
					explicitDevnet = args[0]
				}
			} else {
				explicitDevnet = args[0]
				indexArg = args[1]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			var index int
			if indexArg == "" {
				index, err = dvbcontext.PickNode(daemonClient, ns, devnetName)
				if err != nil {
					return fmt.Errorf("failed to pick node: %w", err)
				}
			} else {
				index, err = parseNodeIndex(indexArg)
				if err != nil {
					return err
				}
			}

			node, err := daemonClient.StartNode(cmd.Context(), ns, devnetName, index)
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
		Use:   "stop [devnet-name] [index]",
		Short: "Stop a node",
		Long: `Stop a running node.

With context set (dvb use <devnet>), the index is optional.
If not provided, an interactive picker will appear.

Examples:
  # Stop node using context with picker
  dvb use my-devnet
  dvb node stop

  # Stop node using context
  dvb use my-devnet
  dvb node stop 0

  # Stop node (explicit devnet)
  dvb node stop my-devnet 0`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			var indexArg string

			if len(args) == 0 {
				// No args - use context and picker
			} else if len(args) == 1 {
				// Could be index (with context) or devnet name (without index)
				if _, err := parseNodeIndex(args[0]); err == nil {
					indexArg = args[0]
				} else {
					explicitDevnet = args[0]
				}
			} else {
				explicitDevnet = args[0]
				indexArg = args[1]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			var index int
			if indexArg == "" {
				index, err = dvbcontext.PickNode(daemonClient, ns, devnetName)
				if err != nil {
					return fmt.Errorf("failed to pick node: %w", err)
				}
			} else {
				index, err = parseNodeIndex(indexArg)
				if err != nil {
					return err
				}
			}

			node, err := daemonClient.StopNode(cmd.Context(), ns, devnetName, index)
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
		Use:   "restart [devnet-name] [index]",
		Short: "Restart a node",
		Long: `Restart a node (stop then start).

With context set (dvb use <devnet>), the index is optional.
If not provided, an interactive picker will appear.

Examples:
  # Restart node using context with picker
  dvb use my-devnet
  dvb node restart

  # Restart node using context
  dvb use my-devnet
  dvb node restart 0

  # Restart node (explicit devnet)
  dvb node restart my-devnet 0`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			var indexArg string

			if len(args) == 0 {
				// No args - use context and picker
			} else if len(args) == 1 {
				// Could be index (with context) or devnet name (without index)
				if _, err := parseNodeIndex(args[0]); err == nil {
					indexArg = args[0]
				} else {
					explicitDevnet = args[0]
				}
			} else {
				explicitDevnet = args[0]
				indexArg = args[1]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			var index int
			if indexArg == "" {
				index, err = dvbcontext.PickNode(daemonClient, ns, devnetName)
				if err != nil {
					return fmt.Errorf("failed to pick node: %w", err)
				}
			} else {
				index, err = parseNodeIndex(indexArg)
				if err != nil {
					return err
				}
			}

			node, err := daemonClient.RestartNode(cmd.Context(), ns, devnetName, index)
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

func newNodeExecCmd() *cobra.Command {
	var timeout int
	var namespace string

	cmd := &cobra.Command{
		Use:   "exec [devnet-name] [index] -- <command> [args...]",
		Short: "Execute a command in a running node container",
		Long: `Execute a command inside a running node container.

With context set (dvb use <devnet>), the index is optional.
If not provided, an interactive picker will appear.

This command allows you to run arbitrary commands inside a node's container,
useful for debugging, inspecting state, or running ad-hoc operations.

The node must be in Running phase for exec to work.

Examples:
  # Execute using context with picker
  dvb use my-devnet
  dvb node exec -- stabled version

  # Execute using context
  dvb use my-devnet
  dvb node exec 0 -- stabled version

  # Check the chain binary version (explicit devnet)
  dvb node exec my-devnet 0 -- stabled version

  # List files in the home directory
  dvb node exec 0 -- ls -la /home/.stable

  # Query the node status via RPC
  dvb node exec 0 -- curl -s localhost:26657/status

  # Run a command with a longer timeout
  dvb node exec 0 --timeout 60 -- stabled query bank balances cosmos1...`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			// Find the -- separator position
			dashDashPos := -1
			for i, arg := range args {
				if arg == "--" {
					dashDashPos = i
					break
				}
			}

			var explicitDevnet string
			var indexArg string
			var command []string

			if dashDashPos == -1 {
				// No -- found, treat all args after index as command
				// This shouldn't happen with proper usage but handle it
				return fmt.Errorf("no command specified after --")
			}

			// Args before -- are: [devnet] [index]
			// Args after -- are: <command> [args...]
			beforeDash := args[:dashDashPos]
			command = args[dashDashPos+1:]

			if len(command) == 0 {
				return fmt.Errorf("no command specified after --")
			}

			if len(beforeDash) == 0 {
				// No args before --, use context and picker
			} else if len(beforeDash) == 1 {
				// Could be index (with context) or devnet name (without index)
				if _, err := parseNodeIndex(beforeDash[0]); err == nil {
					indexArg = beforeDash[0]
				} else {
					explicitDevnet = beforeDash[0]
				}
			} else if len(beforeDash) >= 2 {
				explicitDevnet = beforeDash[0]
				indexArg = beforeDash[1]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			var index int
			if indexArg == "" {
				index, err = dvbcontext.PickNode(daemonClient, ns, devnetName)
				if err != nil {
					return fmt.Errorf("failed to pick node: %w", err)
				}
			} else {
				index, err = parseNodeIndex(indexArg)
				if err != nil {
					return err
				}
			}

			result, err := daemonClient.ExecInNode(cmd.Context(), devnetName, index, command, timeout)
			if err != nil {
				return err
			}

			// Print stdout if any
			if result.Stdout != "" {
				fmt.Print(result.Stdout)
			}

			// Print stderr to stderr if any
			if result.Stderr != "" {
				fmt.Fprint(os.Stderr, result.Stderr)
			}

			// Exit with the command's exit code
			if result.ExitCode != 0 {
				os.Exit(result.ExitCode)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&timeout, "timeout", 30, "Command timeout in seconds")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

// nodeInitOptions holds options for the node init command
type nodeInitOptions struct {
	network       string
	chainID       string
	dataDir       string
	binaryPath    string
	numNodes      int
	monikerPrefix string
}

func newNodeInitCmd() *cobra.Command {
	opts := &nodeInitOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize node directories",
		Long: `Initialize one or more node directories for a devnet.

This command creates and initializes node home directories with the necessary
configuration files for running blockchain nodes. It uses the chain binary
to generate default configurations including keys, genesis, and config files.

The initialized nodes can then be used for local testing or devnet deployment.

Examples:
  # Initialize a single node with default settings
  dvb node init --chain-id my-devnet-1

  # Initialize 4 validator nodes
  dvb node init --chain-id my-devnet-1 --num-nodes 4

  # Initialize with custom data directory
  dvb node init --chain-id my-devnet-1 --data-dir /path/to/nodes

  # Initialize using a specific binary
  dvb node init --chain-id my-devnet-1 --binary-path /usr/local/bin/gaiad

  # Initialize with custom moniker prefix
  dvb node init --chain-id my-devnet-1 --num-nodes 3 --moniker-prefix node`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodeInit(cmd.Context(), opts)
		},
	}

	// Required flags
	cmd.Flags().StringVar(&opts.chainID, "chain-id", "", "Chain ID for the devnet (required)")
	_ = cmd.MarkFlagRequired("chain-id")

	// Optional flags with defaults
	cmd.Flags().StringVar(&opts.network, "network", "stable", "Network type (e.g., stable, cosmos)")
	cmd.Flags().StringVar(&opts.dataDir, "data-dir", "", "Base directory for node data (default ~/.devnet-builder/nodes)")
	cmd.Flags().StringVar(&opts.binaryPath, "binary-path", "", "Path to chain binary (uses network default if not specified)")
	cmd.Flags().IntVar(&opts.numNodes, "num-nodes", 1, "Number of nodes to initialize")
	cmd.Flags().StringVar(&opts.monikerPrefix, "moniker-prefix", "validator", "Prefix for node monikers")

	return cmd
}

func runNodeInit(ctx context.Context, opts *nodeInitOptions) error {
	// Validate number of nodes
	if opts.numNodes < 1 {
		return fmt.Errorf("--num-nodes must be at least 1")
	}

	// Determine data directory
	dataDir := opts.dataDir
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".devnet-builder", "nodes")
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Get plugin initializer based on network
	pluginInit, err := getPluginInitializer(opts.network)
	if err != nil {
		return err
	}

	// Determine binary path
	binaryPath := opts.binaryPath
	if binaryPath == "" {
		binaryPath = pluginInit.BinaryName()
	}

	// Create node initializer
	initializer := provisioner.NewNodeInitializer(provisioner.NodeInitializerConfig{
		DataDir:    dataDir,
		BinaryPath: binaryPath,
		PluginInit: pluginInit,
		Logger:     logger,
	})

	// Build node configs
	configs := make([]types.NodeInitConfig, opts.numNodes)
	for i := 0; i < opts.numNodes; i++ {
		moniker := fmt.Sprintf("%s-%d", opts.monikerPrefix, i)
		homeDir := filepath.Join(dataDir, opts.chainID, fmt.Sprintf("node%d", i))

		configs[i] = types.NodeInitConfig{
			HomeDir:        homeDir,
			Moniker:        moniker,
			ChainID:        opts.chainID,
			ValidatorIndex: i,
		}
	}

	// Print info
	fmt.Fprintf(os.Stderr, "Initializing nodes...\n")
	fmt.Fprintf(os.Stderr, "  Network:    %s\n", opts.network)
	fmt.Fprintf(os.Stderr, "  Chain ID:   %s\n", opts.chainID)
	fmt.Fprintf(os.Stderr, "  Binary:     %s\n", binaryPath)
	fmt.Fprintf(os.Stderr, "  Num Nodes:  %d\n", opts.numNodes)
	fmt.Fprintf(os.Stderr, "  Data Dir:   %s\n", dataDir)
	fmt.Fprintf(os.Stderr, "\n")

	// Initialize all nodes
	if err := initializer.InitializeMultipleNodes(ctx, configs); err != nil {
		return fmt.Errorf("failed to initialize nodes: %w", err)
	}

	// Get node IDs and print success
	color.Green("Nodes initialized successfully!")
	fmt.Fprintf(os.Stderr, "\n")

	for i, config := range configs {
		nodeID, err := initializer.GetNodeID(ctx, config.HomeDir)
		if err != nil {
			// Don't fail if we can't get the node ID, just show a warning
			fmt.Fprintf(os.Stderr, "  Node %d:\n", i)
			fmt.Fprintf(os.Stderr, "    Moniker:  %s\n", config.Moniker)
			fmt.Fprintf(os.Stderr, "    Path:     %s\n", config.HomeDir)
			fmt.Fprintf(os.Stderr, "    Node ID:  (error: %v)\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "  Node %d:\n", i)
			fmt.Fprintf(os.Stderr, "    Moniker:  %s\n", config.Moniker)
			fmt.Fprintf(os.Stderr, "    Path:     %s\n", config.HomeDir)
			fmt.Fprintf(os.Stderr, "    Node ID:  %s\n", nodeID)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	return nil
}

// getPluginInitializer returns the appropriate PluginInitializer for the given network
func getPluginInitializer(network string) (types.PluginInitializer, error) {
	switch network {
	case "stable":
		return cosmos.NewCosmosInitializer("stabled"), nil
	case "cosmos", "gaia":
		return cosmos.NewCosmosInitializer("gaiad"), nil
	default:
		return nil, fmt.Errorf("unknown network: %s (supported: stable, cosmos, gaia)", network)
	}
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

	// Show IP address if available
	if n.Spec.Address != "" {
		fmt.Printf("Address:    %s\n", n.Spec.Address)
	}

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

	// Show endpoints based on whether we have an IP address
	fmt.Printf("\nEndpoints:\n")
	if n.Spec.Address != "" {
		addr := n.Spec.Address
		fmt.Printf("  RPC:      http://%s:26657\n", addr)
		fmt.Printf("  REST:     http://%s:1317\n", addr)
		fmt.Printf("  gRPC:     %s:9090\n", addr)
		fmt.Printf("  P2P:      %s:26656\n", addr)
	} else {
		// Fallback to port-offset based display for legacy/docker mode
		offset := int(n.Metadata.Index) * 100
		fmt.Printf("  RPC:      http://localhost:%d\n", 26657+offset)
		fmt.Printf("  REST:     http://localhost:%d\n", 1317+offset)
		fmt.Printf("  gRPC:     localhost:%d\n", 9090+offset)
		fmt.Printf("  P2P:      localhost:%d\n", 26656+offset)
	}
}

func printHealthStatus(devnetName string, index int, health *client.NodeHealth) {
	// Print health icon and status with color
	switch health.Status {
	case "Healthy":
		color.Green("● Healthy")
	case "Unhealthy":
		color.Red("✗ Unhealthy")
	case "Stopped":
		color.White("○ Stopped")
	case "Transitioning":
		color.Yellow("◐ Transitioning")
	default:
		color.Yellow("? %s", health.Status)
	}

	fmt.Printf("\nDevnet:    %s\n", devnetName)
	fmt.Printf("Node:      %d\n", index)
	fmt.Printf("Status:    %s\n", health.Status)

	if health.Message != "" {
		fmt.Printf("Message:   %s\n", health.Message)
	}

	if !health.LastCheck.IsZero() {
		fmt.Printf("Last Check: %s\n", health.LastCheck.Format("2006-01-02 15:04:05"))
	}

	if health.ConsecutiveFailures > 0 {
		color.Yellow("Consecutive Failures: %d\n", health.ConsecutiveFailures)
	}
}
