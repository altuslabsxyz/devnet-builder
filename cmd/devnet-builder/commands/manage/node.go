package manage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application"
	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
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

// NewNodeCmd creates the node command group.
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
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()
	jsonMode := cfg.JSONMode()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Get number of validators for validation
	numValidators, err := svc.GetNumValidators(ctx)
	if err != nil {
		return err
	}

	// Parse node index
	index, err := parseNodeIndex(args[0], numValidators)
	if err != nil {
		return err
	}

	// Start the node
	result, err := svc.StartNode(ctx, index)
	if err != nil && result == nil {
		return err
	}

	return outputNodeActionResult(result, err, jsonMode)
}

func runNodeStop(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()
	jsonMode := cfg.JSONMode()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Get number of validators for validation
	numValidators, err := svc.GetNumValidators(ctx)
	if err != nil {
		return err
	}

	// Parse node index
	index, err := parseNodeIndex(args[0], numValidators)
	if err != nil {
		return err
	}

	// Stop the node
	timeout := 30 * time.Second
	result, err := svc.StopNode(ctx, index, timeout)
	if err != nil && result == nil {
		return err
	}

	return outputNodeActionResult(result, err, jsonMode)
}

func runNodeLogs(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Get number of validators for validation
	numValidators, err := svc.GetNumValidators(ctx)
	if err != nil {
		return err
	}

	// Parse node index
	index, err := parseNodeIndex(args[0], numValidators)
	if err != nil {
		return err
	}

	// Get execution mode info
	modeInfo, err := svc.GetExecutionModeInfo(ctx, index)
	if err != nil {
		return err
	}

	// Handle logs based on execution mode
	if modeInfo.Mode == types.ExecutionModeDocker {
		if nodeLogFollow {
			return followDockerLogsForNode(ctx, modeInfo.ContainerName)
		}
		// Get last N lines from docker logs
		result, err := svc.GetNodeLogs(ctx, index, nodeLogLines, "")
		if err != nil {
			return fmt.Errorf("failed to get docker logs: %w", err)
		}
		for _, line := range result.Lines {
			fmt.Println(line)
		}
		return nil
	}

	// Local mode
	logPath := modeInfo.LogPath
	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s", logPath)
	}
	if nodeLogFollow {
		return followLocalLogsForNode(ctx, logPath)
	}
	// Print last N lines
	result, err := svc.GetNodeLogs(ctx, index, nodeLogLines, "")
	if err != nil {
		return fmt.Errorf("failed to read logs: %w", err)
	}
	for _, line := range result.Lines {
		fmt.Println(line)
	}
	return nil
}

func followDockerLogsForNode(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func followLocalLogsForNode(ctx context.Context, logPath string) error {
	cmd := exec.CommandContext(ctx, "tail", "-f", logPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func outputNodeActionResult(result *dto.NodeActionOutput, originalErr error, jsonMode bool) error {
	actionResult := NodeActionResult{
		Node:          result.NodeIndex,
		Action:        result.Action,
		Status:        result.Status,
		PreviousState: result.PreviousState,
		CurrentState:  result.CurrentState,
		Error:         result.Error,
	}

	if jsonMode {
		data, _ := json.MarshalIndent(actionResult, "", "  ")
		fmt.Println(string(data))
	} else {
		if result.Status == "success" {
			output.Success("node%d %s: %s -> %s", result.NodeIndex, result.Action, result.PreviousState, result.CurrentState)
		} else if result.Status == "skipped" {
			output.Info("node%d %s: skipped (%s)", result.NodeIndex, result.Action, result.Error)
		} else {
			output.Warn("node%d %s failed: %s", result.NodeIndex, result.Action, result.Error)
		}
	}

	// Return original error only if there was an actual error (not skipped)
	if result.Status == "error" {
		return originalErr
	}
	return nil
}
