package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	nodeLogFollow bool
	nodeLogLines  int
)

// NodeActionResult represents the JSON output for node commands.
type NodeActionResult struct {
	Node          int    `json:"node"`
	Action        string `json:"action"`
	Status        string `json:"status"`
	PreviousState string `json:"previous_state,omitempty"`
	CurrentState  string `json:"current_state,omitempty"`
	Error         string `json:"error,omitempty"`
}

func NewNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Control individual nodes",
		Long: `Control individual nodes in the devnet.

Subcommands allow you to start, stop, and view logs for specific nodes.

Examples:
  # Stop node 1
  devnet-builder node stop 1

  # Start node 1 again
  devnet-builder node start 1

  # View logs for node 0
  devnet-builder node logs 0

  # Follow logs for node 0
  devnet-builder node logs 0 -f`,
	}

	cmd.AddCommand(
		newNodeStartCmd(),
		newNodeStopCmd(),
		newNodeLogsCmd(),
	)

	return cmd
}

func newNodeStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <index>",
		Short: "Start a specific node",
		Long:  `Start a specific node by its index (0-3).`,
		Args:  cobra.ExactArgs(1),
		RunE:  runNodeStart,
	}
}

func newNodeStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <index>",
		Short: "Stop a specific node",
		Long:  `Stop a specific node by its index (0-3).`,
		Args:  cobra.ExactArgs(1),
		RunE:  runNodeStop,
	}
}

func newNodeLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <index>",
		Short: "View logs for a specific node",
		Long:  `View logs for a specific node by its index (0-3).`,
		Args:  cobra.ExactArgs(1),
		RunE:  runNodeLogs,
	}

	cmd.Flags().BoolVarP(&nodeLogFollow, "follow", "f", false,
		"Follow log output")
	cmd.Flags().IntVarP(&nodeLogLines, "lines", "n", 100,
		"Number of lines to show")

	return cmd
}

func parseNodeIndex(arg string, numValidators int) (int, error) {
	index, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("invalid node index: %s (must be a number)", arg)
	}
	if index < 0 || index >= numValidators {
		return 0, fmt.Errorf("node index out of range: %d (valid: 0-%d)", index, numValidators-1)
	}
	return index, nil
}

func runNodeStart(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Load devnet using consolidated helper
	loaded, err := loadDevnetOrFail(logger)
	if err != nil {
		return err
	}
	metadata := loaded.Metadata
	d := loaded.Devnet

	// Parse node index
	index, err := parseNodeIndex(args[0], metadata.NumValidators)
	if err != nil {
		return err
	}

	n := d.Nodes[index]

	// Check current state
	health := node.CheckNodeHealth(ctx, n)
	previousState := string(health.Status)

	if health.Status == node.NodeStatusRunning || health.Status == node.NodeStatusSyncing {
		return outputNodeResult(index, "start", "skipped", previousState, previousState,
			fmt.Errorf("node%d is already running", index))
	}

	// Start the node using factory
	factory := createNodeManagerFactory(metadata, logger)
	manager, err := factory.Create()
	if err != nil {
		return outputNodeResult(index, "start", "error", previousState, "stopped", err)
	}

	startErr := manager.Start(ctx, n, metadata.GenesisPath)
	if startErr != nil {
		return outputNodeResult(index, "start", "error", previousState, "stopped", startErr)
	}

	// Wait a bit and check if it started
	time.Sleep(2 * time.Second)
	newHealth := node.CheckNodeHealth(ctx, n)

	return outputNodeResult(index, "start", "success", previousState, string(newHealth.Status), nil)
}

func runNodeStop(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Load devnet using consolidated helper
	loaded, err := loadDevnetOrFail(logger)
	if err != nil {
		return err
	}
	metadata := loaded.Metadata
	d := loaded.Devnet

	// Parse node index
	index, err := parseNodeIndex(args[0], metadata.NumValidators)
	if err != nil {
		return err
	}

	n := d.Nodes[index]

	// Check current state
	health := node.CheckNodeHealth(ctx, n)
	previousState := string(health.Status)

	if health.Status == node.NodeStatusStopped || health.Status == node.NodeStatusError {
		return outputNodeResult(index, "stop", "skipped", previousState, previousState,
			fmt.Errorf("node%d is not running", index))
	}

	// Stop the node using factory
	timeout := 30 * time.Second
	factory := createNodeManagerFactory(metadata, logger)
	manager, err := factory.Create()
	if err != nil {
		return outputNodeResult(index, "stop", "error", previousState, previousState, err)
	}

	stopErr := manager.Stop(ctx, n, timeout)
	if stopErr != nil {
		return outputNodeResult(index, "stop", "error", previousState, previousState, stopErr)
	}

	return outputNodeResult(index, "stop", "success", previousState, "stopped", nil)
}

func runNodeLogs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Load devnet using consolidated helper
	loaded, err := loadDevnetOrFail(logger)
	if err != nil {
		return err
	}
	metadata := loaded.Metadata
	d := loaded.Devnet

	// Parse node index
	index, err := parseNodeIndex(args[0], metadata.NumValidators)
	if err != nil {
		return err
	}

	n := d.Nodes[index]

	// Handle logs based on execution mode
	switch metadata.ExecutionMode {
	case devnet.ModeDocker:
		if nodeLogFollow {
			return node.FollowDockerLogs(ctx, n.DockerContainerName())
		}
		// Get last N lines from docker logs
		lines, err := node.GetDockerLogs(ctx, n.DockerContainerName(), nodeLogLines)
		if err != nil {
			return fmt.Errorf("failed to get docker logs: %w", err)
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return nil

	case devnet.ModeLocal:
		logPath := n.LogFilePath()
		// Check if log file exists
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			return fmt.Errorf("log file not found: %s", logPath)
		}
		if nodeLogFollow {
			return node.FollowLocalLogs(ctx, logPath)
		}
		// Print last N lines
		lines, err := output.ReadLastLines(logPath, nodeLogLines)
		if err != nil {
			return fmt.Errorf("failed to read logs: %w", err)
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return nil

	default:
		return fmt.Errorf("unknown execution mode: %s", metadata.ExecutionMode)
	}
}

func followNodeLogs(ctx context.Context, metadata *devnet.DevnetMetadata, n *node.Node, logPath string) error {
	switch metadata.ExecutionMode {
	case devnet.ModeDocker:
		// For Docker, use docker logs -f
		return node.FollowDockerLogs(ctx, n.DockerContainerName())
	case devnet.ModeLocal:
		// For local mode, tail -f the log file
		return node.FollowLocalLogs(ctx, logPath)
	default:
		return fmt.Errorf("unknown execution mode: %s", metadata.ExecutionMode)
	}
}

func outputNodeResult(index int, action, status, prevState, currState string, err error) error {
	result := NodeActionResult{
		Node:          index,
		Action:        action,
		Status:        status,
		PreviousState: prevState,
		CurrentState:  currState,
	}

	if err != nil {
		result.Error = err.Error()
	}

	if jsonMode {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		if status == "success" {
			output.Success("node%d %s: %s -> %s", index, action, prevState, currState)
		} else if status == "skipped" {
			output.Info("node%d %s: skipped (%s)", index, action, result.Error)
		} else {
			output.Warn("node%d %s failed: %s", index, action, result.Error)
		}
	}

	if err != nil {
		return err
	}
	return nil
}
