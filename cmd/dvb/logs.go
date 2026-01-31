// cmd/dvb/logs.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/dvbcontext"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// logsOptions holds options for the logs command
type logsOptions struct {
	devnet    string
	node      string
	follow    bool
	tail      int
	dataDir   string
	timestamp bool
}

func newNodeLogsCmd() *cobra.Command {
	opts := &logsOptions{}

	cmd := &cobra.Command{
		Use:   "logs [devnet] [index]",
		Short: "View logs from a node",
		Long: `View logs from a node in a devnet.

With context set (dvb use <devnet>):
  dvb node logs             - Interactive picker to select node
  dvb node logs <index>     - Show logs from a specific node

Without context or explicit devnet:
  dvb node logs <devnet> <index>  - Show logs from a specific node
  dvb node logs <devnet>          - Show merged logs from all nodes

In daemon mode, streams logs from the daemon.
In standalone mode, reads log files from the data directory.

Examples:
  # Set context and use interactive picker
  dvb use my-devnet
  dvb node logs

  # Set context and view logs from node 0
  dvb use my-devnet
  dvb node logs 0

  # Show logs from a specific node (explicit devnet)
  dvb node logs my-devnet 0

  # Follow logs in real-time
  dvb node logs -f

  # Show last 100 lines
  dvb node logs --tail 100

  # Show logs with timestamps
  dvb node logs --timestamps`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var explicitDevnet, nodeArg string

			if len(args) == 0 {
				// No args - requires context and will use picker
				if currentContext == nil {
					return fmt.Errorf("no devnet specified. Either:\n  - Provide devnet: dvb node logs <devnet> [index]\n  - Set context: dvb use <devnet>")
				}
				// nodeArg stays empty, will be picked later
			} else if len(args) == 1 {
				// Determine if single arg is a node identifier or devnet name.
				// Node identifiers are: numeric (0, 1, 2) or "validator-N" pattern
				if looksLikeNodeIdentifier(args[0]) {
					// Treat as node - requires context
					if currentContext == nil {
						return fmt.Errorf("no devnet specified. Either:\n  - Provide devnet: dvb node logs <devnet> %s\n  - Set context: dvb use <devnet>", args[0])
					}
					nodeArg = args[0]
				} else {
					// Treat as devnet name (show all nodes)
					explicitDevnet = args[0]
				}
			} else if len(args) == 2 {
				explicitDevnet = args[0]
				nodeArg = args[1]
			}

			ns, devnetName, err := resolveWithSuggestions(explicitDevnet, "")
			if err != nil {
				return err
			}

			// If context is set and no node specified, use picker
			if currentContext != nil && nodeArg == "" && daemonClient != nil {
				index, err := dvbcontext.PickNode(daemonClient, ns, devnetName)
				if err != nil {
					return fmt.Errorf("failed to pick node: %w", err)
				}
				nodeArg = fmt.Sprintf("%d", index)
			}

			printContextHeader(explicitDevnet, currentContext)

			opts.devnet = devnetName
			opts.node = nodeArg
			return runLogs(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.follow, "follow", "f", false, "Follow log output (like tail -f)")
	cmd.Flags().IntVarP(&opts.tail, "tail", "n", 0, "Number of lines to show from the end (0 = all)")
	cmd.Flags().StringVar(&opts.dataDir, "data-dir", "", "Base data directory (default: ~/.devnet-builder)")
	cmd.Flags().BoolVarP(&opts.timestamp, "timestamps", "t", false, "Show timestamps")

	return cmd
}

func runLogs(ctx context.Context, opts *logsOptions) error {
	// Determine data directory
	dataDir := opts.dataDir
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".devnet-builder")
	}

	// Try daemon streaming if available and a specific node is requested
	if daemonClient != nil && !standalone && opts.node != "" {
		index, err := parseNodeIndex(opts.node)
		if err == nil {
			return streamLogsFromDaemon(ctx, opts, index)
		}
		// If we can't parse the index, fall through to file-based
	}

	// Fall back to file-based logs (standalone mode or multi-node)
	devnetPath := filepath.Join(dataDir, "devnets", opts.devnet)
	if _, err := os.Stat(devnetPath); os.IsNotExist(err) {
		return fmt.Errorf("devnet '%s' not found", opts.devnet)
	}

	nodesDir := filepath.Join(devnetPath, "nodes")
	if _, err := os.Stat(nodesDir); os.IsNotExist(err) {
		return fmt.Errorf("no nodes directory found for devnet '%s'", opts.devnet)
	}

	// Get list of nodes to show logs for
	var nodes []string
	if opts.node != "" {
		// Specific node requested
		nodePath := filepath.Join(nodesDir, opts.node)
		if _, err := os.Stat(nodePath); os.IsNotExist(err) {
			return fmt.Errorf("node '%s' not found in devnet '%s'", opts.node, opts.devnet)
		}
		nodes = []string{opts.node}
	} else {
		// All nodes
		entries, err := os.ReadDir(nodesDir)
		if err != nil {
			return fmt.Errorf("failed to read nodes directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				nodes = append(nodes, entry.Name())
			}
		}
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in devnet '%s'", opts.devnet)
	}

	// Find log files for each node
	logFiles := make(map[string]string)
	for _, node := range nodes {
		logFile := findLogFile(nodesDir, node)
		if logFile != "" {
			logFiles[node] = logFile
		}
	}

	if len(logFiles) == 0 {
		fmt.Printf("No log files found for devnet '%s'\n", opts.devnet)
		fmt.Println()
		fmt.Println("Logs are typically created when nodes are running.")
		fmt.Println("Make sure nodes are started with output redirected to log files.")
		return nil
	}

	// Single node - simple output
	if len(logFiles) == 1 {
		for node, logFile := range logFiles {
			return streamLogFile(ctx, logFile, node, opts)
		}
	}

	// Multiple nodes - interleaved output
	return streamMultipleLogFiles(ctx, logFiles, opts)
}

// findLogFile locates the log file for a node
func findLogFile(nodesDir, node string) string {
	nodePath := filepath.Join(nodesDir, node)

	// Common log file locations
	possiblePaths := []string{
		filepath.Join(nodePath, "node.log"),
		filepath.Join(nodePath, "stdout.log"),
		filepath.Join(nodePath, "output.log"),
		filepath.Join(nodePath, "data", "node.log"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try to find any .log file
	entries, err := os.ReadDir(nodePath)
	if err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".log") {
				return filepath.Join(nodePath, entry.Name())
			}
		}
	}

	return ""
}

// streamLogFile streams a single log file to stdout
func streamLogFile(ctx context.Context, logFile, node string, opts *logsOptions) error {
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// If tail is requested, seek to the end and read backwards
	if opts.tail > 0 {
		lines, err := tailLines(file, opts.tail)
		if err != nil {
			return err
		}
		for _, line := range lines {
			printLogLine(node, line, opts.timestamp)
		}
		if !opts.follow {
			return nil
		}
	}

	// Read and print lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		printLogLine(node, scanner.Text(), opts.timestamp)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file: %w", err)
	}

	// Follow mode
	if opts.follow {
		return followLogFile(ctx, file, node, opts)
	}

	return nil
}

// followLogFile continuously reads new lines from a log file
func followLogFile(ctx context.Context, file *os.File, node string, opts *logsOptions) error {
	reader := bufio.NewReader(file)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				// Wait a bit before checking for more data
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if err != nil {
				return fmt.Errorf("error reading log file: %w", err)
			}
			printLogLine(node, strings.TrimRight(line, "\n"), opts.timestamp)
		}
	}
}

// streamMultipleLogFiles streams multiple log files with node prefixes
func streamMultipleLogFiles(ctx context.Context, logFiles map[string]string, opts *logsOptions) error {
	if opts.follow {
		// For follow mode with multiple files, we'd need goroutines
		// For now, just read all files sequentially
		fmt.Println("Note: Follow mode with multiple nodes reads files sequentially.")
		fmt.Println()
	}

	// Collect all lines with timestamps for sorting
	type logEntry struct {
		node    string
		line    string
		lineNum int
	}

	var entries []logEntry
	for node, logFile := range logFiles {
		file, err := os.Open(logFile)
		if err != nil {
			color.Yellow("Warning: Could not open log file for %s: %v", node, err)
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			entries = append(entries, logEntry{
				node:    node,
				line:    scanner.Text(),
				lineNum: lineNum,
			})
			lineNum++
		}
		file.Close()
	}

	// Sort by line number (approximation of time order for files written similarly)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].lineNum < entries[j].lineNum
	})

	// Apply tail if requested
	if opts.tail > 0 && len(entries) > opts.tail {
		entries = entries[len(entries)-opts.tail:]
	}

	// Print entries
	for _, entry := range entries {
		printLogLine(entry.node, entry.line, opts.timestamp)
	}

	return nil
}

// printLogLine prints a log line with optional node prefix
func printLogLine(node, line string, showTimestamp bool) {
	if node != "" {
		// Color-code by node name
		nodeColor := getNodeColor(node)
		fmt.Printf("%s %s\n", nodeColor("["+node+"]"), line)
	} else {
		fmt.Println(line)
	}
}

// getNodeColor returns a color function based on node name
func getNodeColor(node string) func(format string, a ...interface{}) string {
	// Assign colors based on node name hash
	colors := []func(format string, a ...interface{}) string{
		color.CyanString,
		color.GreenString,
		color.YellowString,
		color.MagentaString,
		color.BlueString,
	}

	hash := 0
	for _, c := range node {
		hash += int(c)
	}
	return colors[hash%len(colors)]
}

// tailLines reads the last n lines from a file
func tailLines(file *os.File, n int) ([]string, error) {
	// Simple implementation: read all lines and return the last n
	// For large files, a more efficient implementation would seek from the end
	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	// Reset file position for potential follow mode
	_, _ = file.Seek(0, io.SeekEnd)

	return lines, nil
}

// looksLikeNodeIdentifier returns true if the string looks like a node identifier.
// Node identifiers are: pure numeric (0, 1, 2) or "validator-N" / "node-N" patterns.
func looksLikeNodeIdentifier(s string) bool {
	// Pure numeric
	if _, err := parseNodeIndex(s); err == nil {
		return true
	}
	// Common node name patterns
	if strings.HasPrefix(s, "validator-") || strings.HasPrefix(s, "node-") || strings.HasPrefix(s, "full-") {
		return true
	}
	return false
}

// streamLogsFromDaemon streams logs from the daemon for a specific node.
func streamLogsFromDaemon(ctx context.Context, opts *logsOptions, index int) error {
	nodeColor := getNodeColor(opts.node)

	return daemonClient.StreamNodeLogs(ctx, opts.devnet, index, opts.follow, "", opts.tail,
		func(entry *client.LogEntry) error {
			if opts.timestamp && !entry.Timestamp.IsZero() {
				fmt.Printf("%s %s %s\n",
					color.WhiteString(entry.Timestamp.Format(time.RFC3339)),
					nodeColor("["+opts.node+"]"),
					entry.Message)
			} else {
				fmt.Printf("%s %s\n", nodeColor("["+opts.node+"]"), entry.Message)
			}
			return nil
		})
}
