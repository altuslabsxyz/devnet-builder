package nodeconfig

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/stablelabs/stable-devnet/internal/output"
)

// isGHCRImage returns true if the image is from GitHub Container Registry.
// GHCR images have stabled as entrypoint, so we don't need to prefix commands.
func isGHCRImage(image string) bool {
	return strings.HasPrefix(image, "ghcr.io/")
}

// ExecutionMode defines how nodes are executed.
type ExecutionMode string

const (
	ModeDocker ExecutionMode = "docker"
	ModeLocal  ExecutionMode = "local"
)

// NodeInitializer handles node initialization with stabled.
type NodeInitializer struct {
	mode        ExecutionMode
	dockerImage string
	logger      *output.Logger
}

// NewNodeInitializer creates a new NodeInitializer.
func NewNodeInitializer(mode ExecutionMode, dockerImage string, logger *output.Logger) *NodeInitializer {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &NodeInitializer{
		mode:        mode,
		dockerImage: dockerImage,
		logger:      logger,
	}
}

// Initialize runs `stabled init` for a node.
func (i *NodeInitializer) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	i.logger.Debug("Initializing node %s at %s", moniker, nodeDir)

	if i.mode == ModeDocker {
		return i.initDocker(ctx, nodeDir, moniker, chainID)
	}
	return i.initLocal(ctx, nodeDir, moniker, chainID)
}

func (i *NodeInitializer) initDocker(ctx context.Context, nodeDir, moniker, chainID string) error {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/root/.stabled", nodeDir),
		i.dockerImage,
	}
	// GHCR images have stabled as entrypoint, others need explicit command
	if !isGHCRImage(i.dockerImage) {
		args = append(args, "stabled")
	}
	args = append(args, "init", moniker,
		"--chain-id", chainID,
		"--home", "/root/.stabled",
	)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Print detailed error for diagnosis
		i.logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  "docker",
			Args:     args,
			WorkDir:  nodeDir,
			Stderr:   string(cmdOutput),
			ExitCode: getExitCode(err),
			Error:    err,
		})
		return fmt.Errorf("docker init failed: %w", err)
	}
	return nil
}

func (i *NodeInitializer) initLocal(ctx context.Context, nodeDir, moniker, chainID string) error {
	args := []string{"init", moniker, "--chain-id", chainID, "--home", nodeDir}
	cmd := exec.CommandContext(ctx, "stabled", args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Print detailed error for diagnosis
		i.logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  "stabled",
			Args:     args,
			WorkDir:  nodeDir,
			Stderr:   string(cmdOutput),
			ExitCode: getExitCode(err),
			Error:    err,
		})
		return fmt.Errorf("stabled init failed: %w", err)
	}
	return nil
}

// GetNodeID retrieves the node ID using `stabled comet show-node-id`.
func (i *NodeInitializer) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	i.logger.Debug("Getting node ID for %s", nodeDir)

	if i.mode == ModeDocker {
		return i.getNodeIDDocker(ctx, nodeDir)
	}
	return i.getNodeIDLocal(ctx, nodeDir)
}

func (i *NodeInitializer) getNodeIDDocker(ctx context.Context, nodeDir string) (string, error) {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/root/.stabled", nodeDir),
		i.dockerImage,
	}
	// GHCR images have stabled as entrypoint, others need explicit command
	if !isGHCRImage(i.dockerImage) {
		args = append(args, "stabled")
	}
	args = append(args, "comet", "show-node-id",
		"--home", "/root/.stabled",
	)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmdOutput, err := cmd.Output()
	if err != nil {
		// Print detailed error for diagnosis
		if exitErr, ok := err.(*exec.ExitError); ok {
			i.logger.PrintCommandError(&output.CommandErrorInfo{
				Command:  "docker",
				Args:     args,
				WorkDir:  nodeDir,
				Stderr:   string(exitErr.Stderr),
				ExitCode: exitErr.ExitCode(),
				Error:    err,
			})
		}
		return "", fmt.Errorf("docker show-node-id failed: %w", err)
	}
	return strings.TrimSpace(string(cmdOutput)), nil
}

func (i *NodeInitializer) getNodeIDLocal(ctx context.Context, nodeDir string) (string, error) {
	args := []string{"comet", "show-node-id", "--home", nodeDir}
	cmd := exec.CommandContext(ctx, "stabled", args...)
	cmdOutput, err := cmd.Output()
	if err != nil {
		// Print detailed error for diagnosis
		if exitErr, ok := err.(*exec.ExitError); ok {
			i.logger.PrintCommandError(&output.CommandErrorInfo{
				Command:  "stabled",
				Args:     args,
				WorkDir:  nodeDir,
				Stderr:   string(exitErr.Stderr),
				ExitCode: exitErr.ExitCode(),
				Error:    err,
			})
		}
		return "", fmt.Errorf("stabled show-node-id failed: %w", err)
	}
	return strings.TrimSpace(string(cmdOutput)), nil
}

// Export runs `stabled export` to export the current state as genesis.
func (i *NodeInitializer) Export(ctx context.Context, nodeDir, destPath string) error {
	i.logger.Debug("Exporting genesis from %s", nodeDir)

	if i.mode == ModeDocker {
		return i.exportDocker(ctx, nodeDir, destPath)
	}
	return i.exportLocal(ctx, nodeDir, destPath)
}

func (i *NodeInitializer) exportDocker(ctx context.Context, nodeDir, destPath string) error {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/root/.stabled", nodeDir),
		"-v", fmt.Sprintf("%s:/output", destPath),
	}
	// GHCR images have stabled as entrypoint, need to override it for bash
	if isGHCRImage(i.dockerImage) {
		args = append(args, "--entrypoint", "bash", i.dockerImage, "-c",
			"stabled export --home /root/.stabled > /output/genesis.json")
	} else {
		args = append(args, i.dockerImage, "bash", "-c",
			"stabled export --home /root/.stabled > /output/genesis.json")
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker export failed: %s: %w", string(output), err)
	}
	return nil
}

func (i *NodeInitializer) exportLocal(ctx context.Context, nodeDir, destPath string) error {
	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("stabled export --home %s > %s", nodeDir, destPath),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stabled export failed: %s: %w", string(output), err)
	}
	return nil
}

// BuildPersistentPeers builds the persistent_peers string from node IDs and ports.
func BuildPersistentPeers(nodeIDs []string, baseP2PPort int) string {
	var peers []string
	for i, nodeID := range nodeIDs {
		port := baseP2PPort + (i * 10000)
		peer := fmt.Sprintf("%s@127.0.0.1:%d", nodeID, port)
		peers = append(peers, peer)
	}
	return strings.Join(peers, ",")
}

// BuildPersistentPeersWithExclusion builds persistent_peers excluding the specified node index.
func BuildPersistentPeersWithExclusion(nodeIDs []string, baseP2PPort int, excludeIndex int) string {
	var peers []string
	for i, nodeID := range nodeIDs {
		if i == excludeIndex {
			continue
		}
		port := baseP2PPort + (i * 10000)
		peer := fmt.Sprintf("%s@127.0.0.1:%d", nodeID, port)
		peers = append(peers, peer)
	}
	return strings.Join(peers, ",")
}

// getExitCode extracts the exit code from an error.
func getExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
