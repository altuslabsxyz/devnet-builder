package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/node"
	"github.com/stablelabs/stable-devnet/internal/output"
)

var (
	logsFollow bool
	logsTail   int
	logsSince  string
)

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
	logger := output.DefaultLogger

	// Load devnet using consolidated helper
	loaded, err := loadDevnetOrFail(logger)
	if err != nil {
		return err
	}
	d := loaded.Devnet

	// Determine which nodes to show logs for
	var targetNodes []*node.Node
	if len(args) > 0 {
		nodeName := args[0]
		var index int
		var err error

		// Support both "node0" and "0" formats
		if strings.HasPrefix(nodeName, "node") {
			indexStr := strings.TrimPrefix(nodeName, "node")
			index, err = strconv.Atoi(indexStr)
		} else {
			// Try parsing as just a number
			index, err = strconv.Atoi(nodeName)
		}

		if err != nil || index < 0 || index >= len(d.Nodes) {
			return fmt.Errorf("invalid node: %s (expected 0-%d or node0-node%d)", nodeName, len(d.Nodes)-1, len(d.Nodes)-1)
		}
		targetNodes = []*node.Node{d.Nodes[index]}
	} else {
		targetNodes = d.Nodes
	}

	// Show network info header
	if !logsFollow {
		output.Info("Logs from %s devnet (%s)", d.Metadata.BlockchainNetwork, d.Metadata.ChainID)
		fmt.Println()
	}

	// Get logs based on execution mode
	switch d.Metadata.ExecutionMode {
	case devnet.ModeDocker:
		return showDockerLogs(ctx, targetNodes, d.Logger)
	case devnet.ModeLocal:
		return showLocalLogs(ctx, targetNodes, d.Logger)
	default:
		return fmt.Errorf("unknown execution mode: %s", d.Metadata.ExecutionMode)
	}
}

func showDockerLogs(ctx context.Context, nodes []*node.Node, logger *output.Logger) error {
	manager := node.NewDockerManager("", logger)

	if logsFollow {
		// For follow mode with multiple nodes, we need to use docker-compose or interleave
		// For simplicity, if following multiple nodes, just follow the first one
		if len(nodes) > 1 {
			output.Warn("Follow mode with multiple nodes - showing all nodes, press Ctrl+C to stop")
		}

		// Start follow processes for each node
		for _, n := range nodes {
			cmd, err := manager.FollowLogs(ctx, n, logsTail)
			if err != nil {
				return fmt.Errorf("failed to follow logs for %s: %w", n.Name, err)
			}

			// Prefix each line with node name
			cmd.Stdout = &prefixWriter{prefix: fmt.Sprintf("[%s] ", n.Name), writer: os.Stdout}
			cmd.Stderr = &prefixWriter{prefix: fmt.Sprintf("[%s] ", n.Name), writer: os.Stderr}

			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to start log follow for %s: %w", n.Name, err)
			}

			// Only follow first node in single-process mode
			if len(nodes) == 1 {
				return cmd.Wait()
			}
		}

		// Wait forever (user will Ctrl+C)
		select {}
	}

	// Non-follow mode - get logs from each node
	for _, n := range nodes {
		logs, err := manager.GetLogs(ctx, n, logsTail, logsSince)
		if err != nil {
			logger.Warn("Failed to get logs for %s: %v", n.Name, err)
			continue
		}

		// Print logs with node prefix
		lines := strings.Split(logs, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Printf("[%s] %s\n", n.Name, line)
			}
		}
	}

	return nil
}

func showLocalLogs(ctx context.Context, nodes []*node.Node, logger *output.Logger) error {
	manager := node.NewLocalManager("", logger)

	if logsFollow {
		// Use tail -f for local logs
		for _, n := range nodes {
			logPath := n.LogFilePath()

			var args []string
			if logsTail > 0 {
				args = []string{"-n", fmt.Sprintf("%d", logsTail), "-f", logPath}
			} else {
				args = []string{"-f", logPath}
			}

			cmd := exec.CommandContext(ctx, "tail", args...)
			cmd.Stdout = &prefixWriter{prefix: fmt.Sprintf("[%s] ", n.Name), writer: os.Stdout}
			cmd.Stderr = &prefixWriter{prefix: fmt.Sprintf("[%s] ", n.Name), writer: os.Stderr}

			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to follow logs for %s: %w", n.Name, err)
			}

			if len(nodes) == 1 {
				return cmd.Wait()
			}
		}

		// Wait forever
		select {}
	}

	// Non-follow mode
	for _, n := range nodes {
		logs, err := manager.GetLogs(ctx, n, logsTail)
		if err != nil {
			logger.Warn("Failed to get logs for %s: %v", n.Name, err)
			continue
		}

		lines := strings.Split(logs, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Printf("[%s] %s\n", n.Name, line)
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
