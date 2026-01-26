// cmd/dvb/node.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
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
		newNodeInitCmd(),
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
