package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/b-harvest/devnet-builder/cmd/devnet-builder/shared"
	"github.com/b-harvest/devnet-builder/internal/application"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsTail   int
	logsSince  string
)

// NewLogsCmd creates the logs command.
func NewLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [node]",
		Short: "View node logs",
		Long: `View logs from devnet nodes.

By default, shows logs from all nodes. Optionally specify a node index
(0, 1, 2, 3) or name (node0, node1, node2, node3) to view logs from a specific node.

Examples:
  # View all node logs
  devnet-builder logs

  # View logs from node0 only (both formats work)
  devnet-builder logs 0
  devnet-builder logs node0

  # Follow logs (like tail -f)
  devnet-builder logs -f
  devnet-builder logs 0 -f

  # Show last 50 lines
  devnet-builder logs --tail 50
  devnet-builder logs -n 50

  # Combine options
  devnet-builder logs 0 --tail 10 -f

  # Show logs since 5 minutes ago
  devnet-builder logs --since 5m`,
		Args: cobra.MaximumNArgs(1),
		RunE: runLogs,
	}

	cmd.Flags().BoolVarP(&logsFollow, "follow", "f", false,
		"Follow log output")
	cmd.Flags().IntVarP(&logsTail, "tail", "n", 100,
		"Number of lines to show")
	cmd.Flags().StringVar(&logsSince, "since", "",
		"Show logs since duration (e.g., 5m)")

	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	homeDir := shared.GetHomeDir()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Get number of validators
	numValidators, err := svc.GetNumValidators(ctx)
	if err != nil {
		return err
	}

	// Determine which nodes to show logs for
	var targetIndices []int
	if len(args) > 0 {
		nodeName := args[0]
		var index int

		// Support both "node0" and "0" formats
		if strings.HasPrefix(nodeName, "node") {
			indexStr := strings.TrimPrefix(nodeName, "node")
			index, err = strconv.Atoi(indexStr)
		} else {
			index, err = strconv.Atoi(nodeName)
		}

		if err != nil || index < 0 || index >= numValidators {
			return fmt.Errorf("invalid node: %s (expected 0-%d or node0-node%d)", nodeName, numValidators-1, numValidators-1)
		}
		targetIndices = []int{index}
	} else {
		// All nodes
		for i := 0; i < numValidators; i++ {
			targetIndices = append(targetIndices, i)
		}
	}

	// Show network info header
	if !logsFollow {
		network, _ := svc.GetBlockchainNetwork(ctx)
		chainID, _ := svc.GetChainID(ctx)
		output.Info("Logs from %s devnet (%s)", network, chainID)
		fmt.Println()
	}

	// Get logs based on execution mode
	isDocker, err := svc.IsDockerMode(ctx)
	if err != nil {
		return err
	}

	if isDocker {
		return showDockerLogsWithService(ctx, svc, targetIndices)
	}
	return showLocalLogsWithService(ctx, svc, targetIndices)
}

func showDockerLogsWithService(ctx context.Context, svc *application.DevnetService, nodeIndices []int) error {
	if logsFollow {
		// For follow mode with multiple nodes, we need to interleave
		if len(nodeIndices) > 1 {
			output.Warn("Follow mode with multiple nodes - showing all nodes, press Ctrl+C to stop")
		}

		// Start follow processes for each node
		for _, idx := range nodeIndices {
			modeInfo, err := svc.GetExecutionModeInfo(ctx, idx)
			if err != nil {
				return err
			}

			// Use docker logs -f for following
			cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--tail", fmt.Sprintf("%d", logsTail), modeInfo.ContainerName)
			cmd.Stdout = &prefixWriter{prefix: fmt.Sprintf("[node%d] ", idx), writer: os.Stdout}
			cmd.Stderr = &prefixWriter{prefix: fmt.Sprintf("[node%d] ", idx), writer: os.Stderr}

			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to start log follow for node%d: %w", idx, err)
			}

			// Only follow first node in single-process mode
			if len(nodeIndices) == 1 {
				return cmd.Wait()
			}
		}

		// Wait forever (user will Ctrl+C)
		select {}
	}

	// Non-follow mode - get logs from each node
	for _, idx := range nodeIndices {
		result, err := svc.GetNodeLogs(ctx, idx, logsTail, logsSince)
		if err != nil {
			output.Warn("Failed to get logs for node%d: %v", idx, err)
			continue
		}

		// Print logs with node prefix
		for _, line := range result.Lines {
			if line != "" {
				fmt.Printf("[node%d] %s\n", idx, line)
			}
		}
	}

	return nil
}

func showLocalLogsWithService(ctx context.Context, svc *application.DevnetService, nodeIndices []int) error {
	if logsFollow {
		// Use tail -f for local logs
		for _, idx := range nodeIndices {
			modeInfo, err := svc.GetExecutionModeInfo(ctx, idx)
			if err != nil {
				return err
			}

			logPath := modeInfo.LogPath
			var args []string
			if logsTail > 0 {
				args = []string{"-n", fmt.Sprintf("%d", logsTail), "-f", logPath}
			} else {
				args = []string{"-f", logPath}
			}

			cmd := exec.CommandContext(ctx, "tail", args...)
			cmd.Stdout = &prefixWriter{prefix: fmt.Sprintf("[node%d] ", idx), writer: os.Stdout}
			cmd.Stderr = &prefixWriter{prefix: fmt.Sprintf("[node%d] ", idx), writer: os.Stderr}

			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to follow logs for node%d: %w", idx, err)
			}

			if len(nodeIndices) == 1 {
				return cmd.Wait()
			}
		}

		// Wait forever
		select {}
	}

	// Non-follow mode
	for _, idx := range nodeIndices {
		result, err := svc.GetNodeLogs(ctx, idx, logsTail, "")
		if err != nil {
			output.Warn("Failed to get logs for node%d: %v", idx, err)
			continue
		}

		for _, line := range result.Lines {
			if line != "" {
				fmt.Printf("[node%d] %s\n", idx, line)
			}
		}
	}

	return nil
}

// prefixWriter adds a prefix to each line written.
type prefixWriter struct {
	prefix string
	writer *os.File
	buffer []byte
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	pw.buffer = append(pw.buffer, p...)

	for {
		idx := -1
		for i, b := range pw.buffer {
			if b == '\n' {
				idx = i
				break
			}
		}

		if idx == -1 {
			break
		}

		line := pw.buffer[:idx]
		pw.buffer = pw.buffer[idx+1:]

		if len(line) > 0 {
			fmt.Fprintf(pw.writer, "%s%s\n", pw.prefix, string(line))
		}
	}

	return len(p), nil
}
