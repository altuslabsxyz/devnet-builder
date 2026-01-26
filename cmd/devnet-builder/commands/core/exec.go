package core

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/application"
	"github.com/altuslabsxyz/devnet-builder/types"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

var (
	execTTY       bool
	execInteract  bool
	execContainer string
)

// NewExecCmd creates the exec command.
func NewExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [node] -- [command...]",
		Short: "Execute command in a node container",
		Long: `Execute a command inside a devnet node container.

Specify the node by index (0, 1, 2, 3) or name (node0, node1, node2, node3).
The command to execute must follow "--".

Examples:
  # Run a shell command in node0
  devnet-builder exec 0 -- ls -la

  # Get an interactive shell in node1
  devnet-builder exec node1 -it -- /bin/sh

  # Query the node status
  devnet-builder exec 0 -- simd status

  # Check node configuration
  devnet-builder exec node0 -- cat /root/.simapp/config/config.toml`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: false,
		RunE:               runExec,
	}

	cmd.Flags().BoolVarP(&execTTY, "tty", "t", false,
		"Allocate a pseudo-TTY")
	cmd.Flags().BoolVarP(&execInteract, "interactive", "i", false,
		"Keep STDIN open")

	return cmd
}

func runExec(cmd *cobra.Command, args []string) error {
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

	// Check if running in docker mode
	isDocker, err := svc.IsDockerMode(ctx)
	if err != nil {
		return err
	}
	if !isDocker {
		return fmt.Errorf("exec command only works in docker mode")
	}

	// Get number of validators
	numValidators, err := svc.GetNumValidators(ctx)
	if err != nil {
		return err
	}

	// Parse node argument
	if len(args) == 0 {
		return fmt.Errorf("node index or name required")
	}

	nodeIndex, err := parseNodeArg(args[0], numValidators)
	if err != nil {
		return err
	}

	// Get command arguments (after --)
	execArgs := args[1:]
	if len(execArgs) == 0 {
		return fmt.Errorf("command required after node argument (use -- to separate)")
	}

	// Get execution mode info for container name
	modeInfo, err := svc.GetExecutionModeInfo(ctx, nodeIndex)
	if err != nil {
		return err
	}

	if modeInfo.Mode != types.ExecutionModeDocker {
		return fmt.Errorf("node%d is not running in docker mode", nodeIndex)
	}

	containerName := modeInfo.ContainerName
	if containerName == "" {
		return fmt.Errorf("container name not found for node%d", nodeIndex)
	}

	// Build docker exec command
	dockerArgs := []string{"exec"}
	if execInteract {
		dockerArgs = append(dockerArgs, "-i")
	}
	if execTTY {
		dockerArgs = append(dockerArgs, "-t")
	}
	dockerArgs = append(dockerArgs, containerName)
	dockerArgs = append(dockerArgs, execArgs...)

	// Execute docker command
	dockerCmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	dockerCmd.Stdin = os.Stdin
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr

	return dockerCmd.Run()
}

// parseNodeArg parses a node argument (e.g., "0", "node0") into an index.
func parseNodeArg(arg string, numNodes int) (int, error) {
	if arg == "" {
		return 0, fmt.Errorf("node argument cannot be empty")
	}

	var index int
	var err error

	// Support both "node0" and "0" formats
	if strings.HasPrefix(arg, "node") {
		indexStr := strings.TrimPrefix(arg, "node")
		index, err = strconv.Atoi(indexStr)
	} else {
		index, err = strconv.Atoi(arg)
	}

	if err != nil {
		return 0, fmt.Errorf("invalid node: %s (expected 0-%d or node0-node%d)", arg, numNodes-1, numNodes-1)
	}

	if index < 0 || index >= numNodes {
		return 0, fmt.Errorf("invalid node: %s (expected 0-%d or node0-node%d)", arg, numNodes-1, numNodes-1)
	}

	return index, nil
}
