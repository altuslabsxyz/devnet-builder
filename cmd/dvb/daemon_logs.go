// cmd/dvb/daemon_logs.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// newDaemonLogsCmd creates the 'daemon logs' subcommand.
func newDaemonLogsCmd() *cobra.Command {
	var (
		follow  bool
		tail    int
		dataDir string
		level   string
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View daemon logs",
		Long: `View logs from the devnetd daemon.

The daemon writes structured logs to ~/.devnet-builder/daemon.log when running.
This command streams those logs for debugging provisioning issues.

Examples:
  # View recent daemon logs
  dvb daemon logs

  # Follow logs in real-time (like tail -f)
  dvb daemon logs -f

  # Show last 50 lines
  dvb daemon logs --tail 50

  # Filter by log level
  dvb daemon logs --level error`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine log file path
			if dataDir == "" {
				home, _ := os.UserHomeDir()
				dataDir = filepath.Join(home, ".devnet-builder")
			}
			logPath := filepath.Join(dataDir, "daemon.log")

			// Check if log file exists
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Daemon log file not found: %s\n\n", logPath)
				fmt.Fprintln(os.Stderr, "The daemon may not be running or hasn't written logs yet.")
				fmt.Fprintln(os.Stderr, "To start the daemon, run: devnetd")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Tip: You can also check daemon status with: dvb daemon status")
				return nil
			}

			if follow {
				return followDaemonLogs(cmd.Context(), logPath, level)
			}

			return readDaemonLogs(logPath, tail, level)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (like tail -f)")
	cmd.Flags().IntVarP(&tail, "tail", "n", 100, "Number of lines to show from the end (0 = all)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: ~/.devnet-builder)")
	cmd.Flags().StringVar(&level, "level", "", "Filter by log level (debug, info, warn, error)")

	return cmd
}

// readDaemonLogs reads the daemon log file and outputs the last N lines.
func readDaemonLogs(logPath string, tail int, level string) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if shouldShowLine(line, level) {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file: %w", err)
	}

	// Get last N lines
	start := 0
	if tail > 0 && len(lines) > tail {
		start = len(lines) - tail
	}

	// Output lines
	for i := start; i < len(lines); i++ {
		printDaemonLogLine(os.Stdout, lines[i])
	}

	return nil
}

// followDaemonLogs follows the daemon log file like tail -f.
// It respects context cancellation for graceful shutdown.
func followDaemonLogs(ctx context.Context, logPath string, level string) error {
	// First, output existing content from the end
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Seek to end and read last few lines for context
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat log file: %w", err)
	}
	var startPos int64
	if stat.Size() > 8192 {
		startPos = stat.Size() - 8192
	}
	if _, err := file.Seek(startPos, 0); err != nil {
		file.Close()
		return fmt.Errorf("failed to seek in log file: %w", err)
	}

	// Skip partial first line if we seeked
	if startPos > 0 {
		reader := bufio.NewReader(file)
		_, _, _ = reader.ReadLine() // Discard partial line
	}

	// Read remaining content
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if shouldShowLine(line, level) {
			printDaemonLogLine(os.Stdout, line)
		}
	}

	// Remember current position
	currentPos, _ := file.Seek(0, io.SeekCurrent)
	file.Close()

	// Set up file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(logPath); err != nil {
		return fmt.Errorf("failed to watch log file: %w", err)
	}

	fmt.Fprintln(os.Stderr, color.CyanString("Following daemon logs (Ctrl+C to stop)..."))
	fmt.Fprintln(os.Stderr, "")

	// Watch for changes with context cancellation support
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				// File was modified, read new content
				file, err := os.Open(logPath)
				if err != nil {
					continue
				}

				_, _ = file.Seek(currentPos, 0)
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := scanner.Text()
					if shouldShowLine(line, level) {
						printDaemonLogLine(os.Stdout, line)
					}
				}
				currentPos, _ = file.Seek(0, io.SeekCurrent)
				file.Close()
			}
		case err := <-watcher.Errors:
			fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
		case <-time.After(100 * time.Millisecond):
			// Check for log file rotation
			newStat, statErr := os.Stat(logPath)
			if statErr != nil || newStat.Size() < currentPos {
				// File was rotated or truncated, reset position
				currentPos = 0
				_ = watcher.Remove(logPath)
				_ = watcher.Add(logPath)
			}
		}
	}
}

// shouldShowLine checks if a log line matches the level filter.
func shouldShowLine(line string, level string) bool {
	if level == "" {
		return true
	}

	levelLower := strings.ToLower(level)
	lineLower := strings.ToLower(line)

	// Check for slog-style level markers
	switch levelLower {
	case "error":
		return strings.Contains(lineLower, "level=error") ||
			strings.Contains(lineLower, "\"level\":\"error\"")
	case "warn", "warning":
		return strings.Contains(lineLower, "level=error") ||
			strings.Contains(lineLower, "level=warn") ||
			strings.Contains(lineLower, "\"level\":\"error\"") ||
			strings.Contains(lineLower, "\"level\":\"warn\"")
	case "info":
		return strings.Contains(lineLower, "level=error") ||
			strings.Contains(lineLower, "level=warn") ||
			strings.Contains(lineLower, "level=info") ||
			strings.Contains(lineLower, "\"level\":\"error\"") ||
			strings.Contains(lineLower, "\"level\":\"warn\"") ||
			strings.Contains(lineLower, "\"level\":\"info\"")
	case "debug":
		return true // Show all
	}

	return true
}

// printDaemonLogLine formats and prints a daemon log line with colorization.
func printDaemonLogLine(w io.Writer, line string) {
	// Colorize based on level
	if strings.Contains(line, "level=ERROR") || strings.Contains(line, "level=error") {
		color.New(color.FgRed).Fprintln(w, line)
	} else if strings.Contains(line, "level=WARN") || strings.Contains(line, "level=warn") {
		color.New(color.FgYellow).Fprintln(w, line)
	} else if strings.Contains(line, "level=DEBUG") || strings.Contains(line, "level=debug") {
		color.New(color.FgHiBlack).Fprintln(w, line)
	} else {
		fmt.Fprintln(w, line)
	}
}
