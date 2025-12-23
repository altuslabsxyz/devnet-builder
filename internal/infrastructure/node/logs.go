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

// GetDockerLogs retrieves the last N lines from a Docker container's logs.
func GetDockerLogs(ctx context.Context, containerName string, lines int) ([]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", lines), containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get docker logs: %w", err)
	}

	// Split output into lines
	var result []string
	scanner := bufio.NewScanner(bufio.NewReader(
		&bytesReader{data: output},
	))
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	return result, nil
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
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
