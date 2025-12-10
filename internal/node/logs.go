package node

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// FollowDockerLogs follows the logs of a Docker container.
func FollowDockerLogs(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// FollowLocalLogs follows a local log file using tail -f.
func FollowLocalLogs(ctx context.Context, logPath string) error {
	cmd := exec.CommandContext(ctx, "tail", "-f", logPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StreamLogs streams logs from a reader to stdout.
func StreamLogs(ctx context.Context, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("error reading logs: %w", err)
				}
				return nil
			}
			fmt.Println(scanner.Text())
		}
	}
}
