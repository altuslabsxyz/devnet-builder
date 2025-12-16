package upgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stablelabs/stable-devnet/internal/cache"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
)

// SwitchOptions contains options for switching to a new binary.
type SwitchOptions struct {
	Mode          devnet.ExecutionMode // "docker" or "local"
	TargetImage   string               // Docker image for upgrade
	TargetBinary  string               // Local binary path
	TargetVersion string               // Version string for the upgrade (e.g., "v1.2.0")
	CachePath     string               // Path to cached binary (for symlink switch)
	CommitHash    string               // Commit hash of the cached binary
	HomeDir       string               // Base directory for devnet
	Metadata      *devnet.DevnetMetadata
	Logger        *output.Logger
}

// SwitchBinary stops nodes and switches to the new binary.
func SwitchBinary(ctx context.Context, opts *SwitchOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Stop nodes first
	logger.Debug("Stopping nodes...")
	if err := stopNodes(ctx, opts); err != nil {
		return WrapError(StageSwitching, "stop nodes", err, "Check node processes")
	}

	// Give nodes time to fully stop
	time.Sleep(2 * time.Second)

	// Switch binary based on mode
	if opts.Mode == devnet.ModeDocker {
		if err := switchDockerImage(ctx, opts, logger); err != nil {
			return err
		}
	} else {
		if err := switchLocalBinary(ctx, opts, logger); err != nil {
			return err
		}
	}

	// Start nodes with new binary
	logger.Debug("Starting nodes with new binary...")
	if err := startNodes(ctx, opts); err != nil {
		return WrapError(StageSwitching, "start nodes", err, "Check new binary compatibility")
	}

	// Update metadata with new version
	if opts.Metadata != nil {
		if opts.TargetImage != "" {
			// Extract version from image tag
			parts := strings.Split(opts.TargetImage, ":")
			if len(parts) > 1 {
				opts.Metadata.StableVersion = parts[len(parts)-1]
			}
		}
		// Update current version if provided
		if opts.TargetVersion != "" {
			opts.Metadata.SetCurrentVersion(opts.TargetVersion)
		}
		opts.Metadata.Save()
	}

	return nil
}

func stopNodes(ctx context.Context, opts *SwitchOptions) error {
	if opts.Mode == devnet.ModeDocker {
		return stopDockerNodes(ctx, opts)
	}
	return stopLocalNodes(ctx, opts)
}

func stopDockerNodes(ctx context.Context, opts *SwitchOptions) error {
	devnetDir := filepath.Join(opts.HomeDir, "devnet")

	// Check if docker-compose.yml exists
	composePath := filepath.Join(devnetDir, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		// Try docker compose v2
		cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "down")
		cmd.Dir = devnetDir
		return cmd.Run()
	}

	// Use docker-compose
	cmd := exec.CommandContext(ctx, "docker", "compose", "down")
	cmd.Dir = devnetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose down failed: %s: %w", string(output), err)
	}

	return nil
}

func stopLocalNodes(ctx context.Context, opts *SwitchOptions) error {
	// Send SIGTERM first for graceful shutdown
	cmd := exec.CommandContext(ctx, "pkill", "-TERM", "-f", "stabled.*start")
	cmd.Run() // Ignore errors - process might not exist

	// Give processes time to gracefully stop
	time.Sleep(3 * time.Second)

	// Force kill if still running
	cmd = exec.CommandContext(ctx, "pkill", "-9", "-f", "stabled.*start")
	cmd.Run()

	// Wait a bit more for ports to be released
	time.Sleep(2 * time.Second)

	// Verify all node ports are free
	for i := 0; i < opts.Metadata.NumValidators; i++ {
		rpcPort := 26657 + (i * 10000)
		if err := waitForPortFree(ctx, rpcPort, 10*time.Second); err != nil {
			// Try one more aggressive kill
			cmd = exec.CommandContext(ctx, "fuser", "-k", fmt.Sprintf("%d/tcp", rpcPort))
			cmd.Run()
			time.Sleep(1 * time.Second)
		}
	}

	return nil
}

// waitForPortFree waits for a port to become available
func waitForPortFree(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := exec.CommandContext(ctx, "lsof", "-i", fmt.Sprintf(":%d", port))
		if err := cmd.Run(); err != nil {
			// lsof returns error when port is not in use - this is what we want
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("port %d still in use after %v", port, timeout)
}

func switchDockerImage(ctx context.Context, opts *SwitchOptions, logger *output.Logger) error {
	logger.Debug("Switching to Docker image: %s", opts.TargetImage)

	// Pull the new image first
	cmd := exec.CommandContext(ctx, "docker", "pull", opts.TargetImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return WrapError(StageSwitching, "pull Docker image", err,
			fmt.Sprintf("Failed to pull %s. Verify image exists and you have access.", opts.TargetImage))
	}
	logger.Debug("Image pulled: %s", string(output))

	// Update docker-compose.yml with new image
	devnetDir := filepath.Join(opts.HomeDir, "devnet")
	composePath := filepath.Join(devnetDir, "docker-compose.yml")

	if _, err := os.Stat(composePath); err == nil {
		// Read compose file
		content, err := os.ReadFile(composePath)
		if err != nil {
			return fmt.Errorf("failed to read docker-compose.yml: %w", err)
		}

		// Replace image tag
		// This is a simple replacement - in production, use proper YAML parsing
		oldImage := fmt.Sprintf("ghcr.io/stablelabs/stable:%s", opts.Metadata.StableVersion)
		newContent := strings.ReplaceAll(string(content), oldImage, opts.TargetImage)

		// Also handle the case where it's using STABLED_TAG env var
		// Set environment variable for compose
		os.Setenv("STABLED_TAG", extractTag(opts.TargetImage))

		if err := os.WriteFile(composePath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to update docker-compose.yml: %w", err)
		}
	}

	return nil
}

func switchLocalBinary(ctx context.Context, opts *SwitchOptions, logger *output.Logger) error {
	// If we have a cache path, use atomic symlink switch
	if opts.CachePath != "" && opts.CommitHash != "" {
		logger.Debug("Switching to cached binary via symlink: %s", opts.CachePath)

		// Initialize symlink manager
		symlinkMgr := cache.NewSymlinkManager(opts.HomeDir)
		binaryCache := cache.NewBinaryCache(opts.HomeDir, logger)
		if err := binaryCache.Initialize(); err != nil {
			return WrapError(StageSwitching, "initialize cache", err, "Check cache directory permissions")
		}

		// Verify cached binary exists and is valid
		if err := binaryCache.Validate(opts.CommitHash); err != nil {
			return WrapError(StageSwitching, "validate cached binary", err,
				fmt.Sprintf("Cached binary invalid: %s", opts.CommitHash))
		}

		// Atomic symlink switch
		if err := symlinkMgr.SwitchToCache(binaryCache, opts.CommitHash); err != nil {
			return WrapError(StageSwitching, "switch symlink", err, "Check filesystem permissions")
		}

		logger.Debug("Symlink switched to: %s", binaryCache.GetBinaryPath(opts.CommitHash))
		return nil
	}

	// Fallback: direct binary path
	logger.Debug("Switching to local binary: %s", opts.TargetBinary)

	// Verify binary exists
	if _, err := os.Stat(opts.TargetBinary); os.IsNotExist(err) {
		return WrapError(StageSwitching, "verify binary", ErrBinaryNotFound,
			fmt.Sprintf("Binary not found at %s", opts.TargetBinary))
	}

	// Verify binary is executable
	info, err := os.Stat(opts.TargetBinary)
	if err != nil {
		return err
	}
	if info.Mode()&0111 == 0 {
		return WrapError(StageSwitching, "verify binary", ErrBinaryNotFound,
			fmt.Sprintf("Binary at %s is not executable", opts.TargetBinary))
	}

	return nil
}

func startNodes(ctx context.Context, opts *SwitchOptions) error {
	if opts.Mode == devnet.ModeDocker {
		return startDockerNodes(ctx, opts)
	}
	return startLocalNodes(ctx, opts)
}

func startDockerNodes(ctx context.Context, opts *SwitchOptions) error {
	devnetDir := filepath.Join(opts.HomeDir, "devnet")

	// Start with docker compose
	cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d")
	cmd.Dir = devnetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up failed: %s: %w", string(output), err)
	}

	return nil
}

func startLocalNodes(ctx context.Context, opts *SwitchOptions) error {
	devnetDir := filepath.Join(opts.HomeDir, "devnet")
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Extract EVM chain ID from cosmos chain ID (e.g., "stable_988-1" -> "988")
	evmChainID := ""
	if opts.Metadata != nil && opts.Metadata.ChainID != "" {
		parts := strings.Split(opts.Metadata.ChainID, "_")
		if len(parts) >= 2 {
			evmPart := parts[len(parts)-1]
			if idx := strings.Index(evmPart, "-"); idx > 0 {
				evmChainID = evmPart[:idx]
			} else {
				evmChainID = evmPart
			}
		}
	}

	// Start each node
	for i := 0; i < opts.Metadata.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))

		// Determine binary to use (prefer symlink path if available)
		binary := opts.TargetBinary
		if opts.CachePath != "" {
			// Use symlink path
			binary = filepath.Join(opts.HomeDir, cache.BinSubdir, cache.SymlinkName)
		} else if binary == "" {
			binary = "stabled"
		}

		// Calculate ports (matching nodeconfig.GetPortConfigForNode)
		offset := i * 10000
		rpcPort := 26657 + offset
		p2pPort := 26656 + offset
		grpcPort := 9090 + offset
		evmRPCPort := 8545 + offset
		evmWSPort := 8546 + offset

		// Wait for RPC port to be free before starting
		logger.Debug("Waiting for node%d port %d to be free...", i, rpcPort)
		if err := waitForPortFree(ctx, rpcPort, 15*time.Second); err != nil {
			return fmt.Errorf("port %d still in use for node%d: %w", rpcPort, i, err)
		}

		// Open log file for this node
		logPath := filepath.Join(nodeDir, "stabled.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file for node%d: %w", i, err)
		}

		// Build start command with all necessary flags
		args := []string{
			"start",
			"--home", nodeDir,
			fmt.Sprintf("--rpc.laddr=tcp://0.0.0.0:%d", rpcPort),
			fmt.Sprintf("--p2p.laddr=tcp://0.0.0.0:%d", p2pPort),
			fmt.Sprintf("--grpc.address=0.0.0.0:%d", grpcPort),
			"--api.enabled-unsafe-cors=true",
			"--api.enable=true",
			fmt.Sprintf("--json-rpc.address=0.0.0.0:%d", evmRPCPort),
			fmt.Sprintf("--json-rpc.ws-address=0.0.0.0:%d", evmWSPort),
		}

		// Add EVM chain ID if available
		if evmChainID != "" {
			args = append(args, fmt.Sprintf("--evm.evm-chain-id=%s", evmChainID))
		}

		cmd := exec.Command(binary, args...)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		cmd.Dir = nodeDir

		// Run in background
		if err := cmd.Start(); err != nil {
			logFile.Close()
			return fmt.Errorf("failed to start node%d: %w", i, err)
		}

		// Write PID file
		pidPath := filepath.Join(nodeDir, "stabled.pid")
		if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
			logger.Debug("Warning: failed to write PID file for node%d: %v", i, err)
		}

		logger.Debug("Started node%d (pid=%d)", i, cmd.Process.Pid)

		// Don't wait for the process - let it run in background
		go func(lf *os.File) {
			cmd.Wait()
			lf.Close()
		}(logFile)

		// Small delay between node starts to avoid race conditions
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func extractTag(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return "latest"
}

// UpdateMetadataVersion updates the devnet metadata with the new version.
func UpdateMetadataVersion(metadata *devnet.DevnetMetadata, newVersion string) error {
	metadata.StableVersion = newVersion
	return metadata.Save()
}
