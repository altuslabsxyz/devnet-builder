// Package core provides core CLI commands.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

var (
	eventsSince  string
	eventsUntil  string
	eventsFilter []string
	eventsWatch  bool
)

// DockerEvent represents a Docker event from the events API.
type DockerEvent struct {
	Status         string            `json:"status"`
	ID             string            `json:"id"`
	From           string            `json:"from"`
	Type           string            `json:"Type"`
	Action         string            `json:"Action"`
	Actor          DockerEventActor  `json:"Actor"`
	Scope          string            `json:"scope"`
	Time           int64             `json:"time"`
	TimeNano       int64             `json:"timeNano"`
}

// DockerEventActor contains the actor information for a Docker event.
type DockerEventActor struct {
	ID         string            `json:"ID"`
	Attributes map[string]string `json:"Attributes"`
}

// DevnetEvent represents a processed devnet event for display.
type DevnetEvent struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Action    string `json:"action"`
	Node      string `json:"node,omitempty"`
	Container string `json:"container,omitempty"`
	Details   string `json:"details,omitempty"`
}

// NewEventsCmd creates the events command.
func NewEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Stream devnet container events",
		Long: `Stream real-time events from devnet containers.

Shows container lifecycle events like start, stop, restart, die, health_status,
and other Docker container events for the devnet nodes.

Events are filtered to show only containers belonging to the current devnet.

Examples:
  # Watch live events
  devnet-builder events

  # Show events from the last hour
  devnet-builder events --since 1h

  # Filter by event type (start, stop, die, restart, health_status)
  devnet-builder events --filter type=start --filter type=stop

  # Output as JSON
  devnet-builder events --json

  # Don't watch, just show historical events
  devnet-builder events --since 1h --no-watch`,
		RunE: runEvents,
	}

	cmd.Flags().StringVar(&eventsSince, "since", "",
		"Show events since timestamp (e.g., '2023-01-01T00:00:00', '1h', '30m')")
	cmd.Flags().StringVar(&eventsUntil, "until", "",
		"Show events until timestamp (only with --no-watch)")
	cmd.Flags().StringArrayVar(&eventsFilter, "filter", nil,
		"Filter events (e.g., 'type=start', 'type=die')")
	cmd.Flags().BoolVar(&eventsWatch, "no-watch", false,
		"Don't watch for new events, just show historical")

	return cmd
}

func runEvents(cmd *cobra.Command, args []string) error {
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

	// Check if running in docker mode
	isDocker, err := svc.IsDockerMode(ctx)
	if err != nil {
		return err
	}
	if !isDocker {
		return fmt.Errorf("events command only works in docker mode")
	}

	// Get container prefix for filtering
	containerPrefix, err := getDevnetContainerPrefix(ctx, svc)
	if err != nil {
		return fmt.Errorf("failed to determine container prefix: %w", err)
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Build docker events command
	dockerArgs := []string{"events", "--format", "{{json .}}"}

	// Add container filter
	dockerArgs = append(dockerArgs, "--filter", "type=container")
	dockerArgs = append(dockerArgs, "--filter", fmt.Sprintf("container=%s", containerPrefix))

	// Add time filters
	if eventsSince != "" {
		dockerArgs = append(dockerArgs, "--since", eventsSince)
	}
	if eventsUntil != "" {
		dockerArgs = append(dockerArgs, "--until", eventsUntil)
	} else if eventsWatch {
		// If no-watch and no until, set until to now
		dockerArgs = append(dockerArgs, "--until", time.Now().Format(time.RFC3339))
	}

	// Add event type filters
	for _, f := range eventsFilter {
		if strings.HasPrefix(f, "type=") {
			action := strings.TrimPrefix(f, "type=")
			dockerArgs = append(dockerArgs, "--filter", fmt.Sprintf("event=%s", action))
		}
	}

	dockerCmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	stdout, err := dockerCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := dockerCmd.Start(); err != nil {
		return fmt.Errorf("failed to start docker events: %w", err)
	}

	if !jsonMode && !eventsWatch {
		output.Info("Streaming events for devnet containers (prefix: %s)...", containerPrefix)
		output.Info("Press Ctrl+C to stop\n")
	}

	// Read events
	decoder := json.NewDecoder(stdout)
	for {
		var event DockerEvent
		if err := decoder.Decode(&event); err != nil {
			// Check if context was cancelled
			select {
			case <-ctx.Done():
				return nil
			default:
				// EOF or other error
				if err.Error() == "EOF" {
					return nil
				}
				// Continue on parse errors
				continue
			}
		}

		// Process and display the event
		devnetEvent := processDockerEvent(event, containerPrefix)
		if devnetEvent == nil {
			continue
		}

		if jsonMode {
			enc := json.NewEncoder(os.Stdout)
			_ = enc.Encode(devnetEvent)
		} else {
			displayEvent(devnetEvent)
		}
	}
}

// getDevnetContainerPrefix returns the container name prefix for the devnet.
func getDevnetContainerPrefix(ctx context.Context, svc *application.DevnetService) (string, error) {
	// Get the first node's container name and extract the prefix
	numValidators, err := svc.GetNumValidators(ctx)
	if err != nil {
		return "", err
	}

	if numValidators == 0 {
		return "", fmt.Errorf("no validators found")
	}

	modeInfo, err := svc.GetExecutionModeInfo(ctx, 0)
	if err != nil {
		return "", err
	}

	// Container name is typically "devnet-node0", extract "devnet-"
	containerName := modeInfo.ContainerName
	if containerName == "" {
		return "", fmt.Errorf("container name not found")
	}

	// Extract prefix (everything before "node")
	if idx := strings.LastIndex(containerName, "node"); idx > 0 {
		return containerName[:idx], nil
	}

	// Fallback: use container name as-is (will match exactly)
	return containerName, nil
}

// processDockerEvent converts a Docker event to a DevnetEvent.
func processDockerEvent(event DockerEvent, prefix string) *DevnetEvent {
	// Filter by container name prefix
	containerName := event.Actor.Attributes["name"]
	if containerName == "" || !strings.HasPrefix(containerName, prefix) {
		return nil
	}

	// Extract node name from container name
	nodeName := strings.TrimPrefix(containerName, prefix)

	// Format timestamp
	timestamp := time.Unix(event.Time, 0).Format("2006-01-02 15:04:05")

	// Build event
	devnetEvent := &DevnetEvent{
		Timestamp: timestamp,
		Type:      event.Type,
		Action:    event.Action,
		Node:      nodeName,
		Container: containerName,
	}

	// Add details based on action
	switch event.Action {
	case "health_status":
		if health, ok := event.Actor.Attributes["health_status"]; ok {
			devnetEvent.Details = fmt.Sprintf("health: %s", health)
		}
	case "die":
		if exitCode, ok := event.Actor.Attributes["exitCode"]; ok {
			devnetEvent.Details = fmt.Sprintf("exit code: %s", exitCode)
		}
	case "exec_create", "exec_start":
		devnetEvent.Details = "exec command"
	}

	return devnetEvent
}

// displayEvent prints a formatted event.
func displayEvent(event *DevnetEvent) {
	// Color-code by action
	var actionColor string
	switch event.Action {
	case "start":
		actionColor = "\033[32m" // Green
	case "stop", "kill":
		actionColor = "\033[33m" // Yellow
	case "die":
		actionColor = "\033[31m" // Red
	case "restart":
		actionColor = "\033[36m" // Cyan
	case "health_status":
		actionColor = "\033[35m" // Magenta
	default:
		actionColor = "\033[0m" // Default
	}
	resetColor := "\033[0m"

	// Format output
	if event.Details != "" {
		fmt.Printf("%s  %s%-12s%s  %-8s  %s\n",
			event.Timestamp,
			actionColor, event.Action, resetColor,
			event.Node,
			event.Details)
	} else {
		fmt.Printf("%s  %s%-12s%s  %s\n",
			event.Timestamp,
			actionColor, event.Action, resetColor,
			event.Node)
	}
}
