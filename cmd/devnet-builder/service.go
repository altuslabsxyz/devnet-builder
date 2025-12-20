// Package main provides the CLI entry point for devnet-builder.
// service.go contains the unified service layer that bridges cmd and infrastructure.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/internal/snapshot"
)

// DevnetService provides unified access to devnet operations.
// It bridges the cmd layer with both legacy packages and the new DI container.
type DevnetService struct {
	homeDir string
	logger  *output.Logger
}

// NewDevnetService creates a new DevnetService.
func NewDevnetService(homeDir string, logger *output.Logger) *DevnetService {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &DevnetService{
		homeDir: homeDir,
		logger:  logger,
	}
}

// DevnetExists checks if a devnet exists.
func (s *DevnetService) DevnetExists() bool {
	return devnet.DevnetExists(s.homeDir)
}

// LoadDevnetInfo loads devnet information for display.
func (s *DevnetService) LoadDevnetInfo(ctx context.Context) (*dto.DevnetInfo, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	return s.convertToDevnetInfo(d), nil
}

// GetStatus returns the full status of the devnet.
func (s *DevnetService) GetStatus(ctx context.Context) (*dto.StatusOutput, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	// Get health status
	health := d.GetHealth(ctx)

	// Determine overall status
	overallStatus := string(d.Metadata.Status)
	runningCount := 0
	for _, h := range health {
		if h.Status == node.NodeStatusRunning || h.Status == node.NodeStatusSyncing {
			runningCount++
		}
	}

	if runningCount == len(d.Nodes) {
		overallStatus = "running"
	} else if runningCount > 0 {
		overallStatus = "partial"
	} else {
		overallStatus = "stopped"
	}

	// Convert to DTOs
	nodeStatuses := make([]dto.NodeHealthStatus, len(health))
	for i, h := range health {
		nodeStatuses[i] = dto.NodeHealthStatus{
			Index:       h.Index,
			Name:        fmt.Sprintf("node%d", h.Index),
			Status:      ports.NodeStatus(h.Status),
			IsRunning:   h.Status == node.NodeStatusRunning || h.Status == node.NodeStatusSyncing,
			BlockHeight: h.BlockHeight,
			PeerCount:   h.PeerCount,
			CatchingUp:  h.CatchingUp,
			Error:       h.Error,
		}
	}

	return &dto.StatusOutput{
		Devnet:        s.convertToDevnetInfo(d),
		OverallStatus: overallStatus,
		Nodes:         nodeStatuses,
		AllHealthy:    runningCount == len(d.Nodes),
	}, nil
}

// Stop stops all devnet nodes.
func (s *DevnetService) Stop(ctx context.Context, timeout time.Duration) (*dto.StopOutput, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	s.logger.Info("Stopping devnet nodes...")
	if err := d.Stop(ctx, timeout); err != nil {
		return nil, fmt.Errorf("failed to stop devnet: %w", err)
	}

	return &dto.StopOutput{
		StoppedNodes: len(d.Nodes),
	}, nil
}

// Start starts all devnet nodes.
func (s *DevnetService) Start(ctx context.Context, timeout time.Duration) (*dto.RunOutput, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	s.logger.Info("Starting devnet nodes...")

	// Use legacy Run
	runOpts := devnet.RunOptions{
		HomeDir:       s.homeDir,
		Mode:          d.Metadata.ExecutionMode,
		StableVersion: d.Metadata.StableVersion,
		HealthTimeout: timeout,
		Logger:        s.logger,
	}

	result, err := devnet.Run(ctx, runOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to start devnet: %w", err)
	}

	// Convert to DTO
	nodeStatuses := make([]dto.NodeStatus, len(result.Devnet.Nodes))
	allRunning := true
	for i, n := range result.Devnet.Nodes {
		isRunning := n.Status == node.NodeStatusRunning
		nodeStatuses[i] = dto.NodeStatus{
			Index:     n.Index,
			Name:      n.Name,
			IsRunning: isRunning,
		}
		if !isRunning {
			allRunning = false
		}
	}

	return &dto.RunOutput{
		Nodes:      nodeStatuses,
		AllRunning: allRunning,
	}, nil
}

// Restart restarts all devnet nodes.
func (s *DevnetService) Restart(ctx context.Context, timeout time.Duration) (*dto.RestartOutput, error) {
	stopResult, err := s.Stop(ctx, timeout)
	if err != nil {
		return nil, err
	}

	time.Sleep(2 * time.Second) // Brief pause between stop and start

	startResult, err := s.Start(ctx, timeout)
	if err != nil {
		return &dto.RestartOutput{
			StoppedNodes: stopResult.StoppedNodes,
			StartedNodes: 0,
			AllRunning:   false,
		}, err
	}

	return &dto.RestartOutput{
		StoppedNodes: stopResult.StoppedNodes,
		StartedNodes: len(startResult.Nodes),
		AllRunning:   startResult.AllRunning,
	}, nil
}

// Destroy destroys the devnet.
func (s *DevnetService) Destroy(ctx context.Context, cleanCache bool) (*dto.DestroyOutput, error) {
	devnetDir := filepath.Join(s.homeDir, "devnet")
	stoppedNodes := 0

	// Stop running processes first
	if s.DevnetExists() {
		s.logger.Info("Stopping running processes...")
		result, err := s.Stop(ctx, 30*time.Second)
		if err != nil {
			s.logger.Warn("Failed to stop some processes: %v", err)
		} else {
			stoppedNodes = result.StoppedNodes
		}
	}

	// Remove devnet directory
	s.logger.Info("Removing devnet directory...")
	if err := os.RemoveAll(devnetDir); err != nil {
		return nil, fmt.Errorf("failed to remove devnet: %w", err)
	}

	// Clean cache if requested
	cacheCleared := false
	if cleanCache {
		s.logger.Info("Cleaning snapshot cache...")
		if err := snapshot.ClearAllCaches(s.homeDir); err != nil {
			s.logger.Warn("Failed to clear cache: %v", err)
		} else {
			cacheCleared = true
		}
	}

	return &dto.DestroyOutput{
		RemovedDir:   devnetDir,
		NodesStopped: stoppedNodes,
		CacheCleared: cacheCleared,
	}, nil
}

// Reset resets the devnet.
func (s *DevnetService) Reset(ctx context.Context, hard bool) (*dto.ResetOutput, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	// Stop first if running
	if d.Metadata.Status == devnet.StatusRunning {
		s.logger.Info("Stopping running nodes...")
		if err := d.Stop(ctx, 30*time.Second); err != nil {
			s.logger.Warn("Failed to stop nodes: %v", err)
		}
	}

	if hard {
		return s.hardReset(ctx, d)
	}
	return s.softReset(ctx, d)
}

func (s *DevnetService) softReset(ctx context.Context, d *devnet.Devnet) (*dto.ResetOutput, error) {
	s.logger.Info("Performing soft reset...")
	removed := make([]string, 0)

	for _, n := range d.Nodes {
		dataDir := filepath.Join(n.HomeDir, "data")
		if err := os.RemoveAll(dataDir); err != nil {
			s.logger.Warn("Failed to remove data for node %d: %v", n.Index, err)
		}
		removed = append(removed, dataDir)
	}

	d.Metadata.Status = devnet.StatusCreated
	if err := d.Metadata.Save(); err != nil {
		return nil, fmt.Errorf("failed to update metadata: %w", err)
	}

	return &dto.ResetOutput{
		Type:    "soft",
		Removed: removed,
	}, nil
}

func (s *DevnetService) hardReset(ctx context.Context, d *devnet.Devnet) (*dto.ResetOutput, error) {
	s.logger.Info("Performing hard reset...")
	devnetDir := filepath.Join(s.homeDir, "devnet")

	if err := os.RemoveAll(devnetDir); err != nil {
		return nil, fmt.Errorf("failed to remove devnet directory: %w", err)
	}

	return &dto.ResetOutput{
		Type:    "hard",
		Removed: []string{devnetDir},
	}, nil
}

// GetNodeLogs returns the log file path for a node.
func (s *DevnetService) GetNodeLogPath(nodeIndex int) (string, error) {
	if !s.DevnetExists() {
		return "", fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	metadata, err := devnet.LoadDevnetMetadata(s.homeDir)
	if err != nil {
		return "", fmt.Errorf("failed to load metadata: %w", err)
	}

	if nodeIndex < 0 || nodeIndex >= metadata.NumValidators {
		return "", fmt.Errorf("invalid node index: %d (valid: 0-%d)", nodeIndex, metadata.NumValidators-1)
	}

	module, err := network.Get(metadata.BlockchainNetwork)
	if err != nil {
		return "", fmt.Errorf("failed to get network module: %w", err)
	}

	logPath := filepath.Join(s.homeDir, "devnet", fmt.Sprintf("node%d", nodeIndex), module.LogFileName())
	return logPath, nil
}

// convertToDevnetInfo converts legacy Devnet to DTO.
func (s *DevnetService) convertToDevnetInfo(d *devnet.Devnet) *dto.DevnetInfo {
	nodes := make([]dto.NodeInfo, len(d.Nodes))
	for i, n := range d.Nodes {
		nodes[i] = dto.NodeInfo{
			Index:   n.Index,
			Name:    n.Name,
			HomeDir: n.HomeDir,
			Ports: ports.PortConfig{
				RPC:   n.Ports.RPC,
				P2P:   n.Ports.P2P,
				GRPC:  n.Ports.GRPC,
				EVM:   n.Ports.EVMRPC,
				EVMWS: n.Ports.EVMWS,
			},
			RPCURL: n.RPCURL(),
			EVMURL: n.EVMRPCURL(),
		}
	}

	return &dto.DevnetInfo{
		HomeDir:           d.Metadata.HomeDir,
		ChainID:           d.Metadata.ChainID,
		NetworkSource:     d.Metadata.NetworkSource,
		BlockchainNetwork: d.Metadata.BlockchainNetwork,
		ExecutionMode:     string(d.Metadata.ExecutionMode),
		DockerImage:       d.Metadata.DockerImage,
		NumValidators:     d.Metadata.NumValidators,
		NumAccounts:       d.Metadata.NumAccounts,
		InitialVersion:    d.Metadata.InitialVersion,
		CurrentVersion:    d.Metadata.CurrentVersion,
		Status:            string(d.Metadata.Status),
		CreatedAt:         d.Metadata.CreatedAt,
		Nodes:             nodes,
	}
}

// GetNodeHealth returns the health status of a specific node.
func (s *DevnetService) GetNodeHealth(ctx context.Context, nodeIndex int) (*dto.NodeHealthStatus, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	if nodeIndex < 0 || nodeIndex >= len(d.Nodes) {
		return nil, fmt.Errorf("invalid node index: %d (valid: 0-%d)", nodeIndex, len(d.Nodes)-1)
	}

	n := d.Nodes[nodeIndex]
	health := node.CheckNodeHealth(ctx, n)

	return &dto.NodeHealthStatus{
		Index:       health.Index,
		Name:        fmt.Sprintf("node%d", health.Index),
		Status:      ports.NodeStatus(health.Status),
		IsRunning:   health.Status == node.NodeStatusRunning || health.Status == node.NodeStatusSyncing,
		BlockHeight: health.BlockHeight,
		PeerCount:   health.PeerCount,
		CatchingUp:  health.CatchingUp,
		Error:       health.Error,
	}, nil
}

// StartNode starts a specific node.
func (s *DevnetService) StartNode(ctx context.Context, nodeIndex int) (*dto.NodeActionOutput, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	if nodeIndex < 0 || nodeIndex >= len(d.Nodes) {
		return nil, fmt.Errorf("invalid node index: %d (valid: 0-%d)", nodeIndex, len(d.Nodes)-1)
	}

	n := d.Nodes[nodeIndex]
	metadata := d.Metadata

	// Check current state
	health := node.CheckNodeHealth(ctx, n)
	previousState := string(health.Status)

	if health.Status == node.NodeStatusRunning || health.Status == node.NodeStatusSyncing {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "start",
			Status:        "skipped",
			PreviousState: previousState,
			CurrentState:  previousState,
			Error:         fmt.Sprintf("node%d is already running", nodeIndex),
		}, nil
	}

	// Create node manager factory
	factory := s.createNodeManagerFactory(metadata)
	manager, err := factory.Create()
	if err != nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "start",
			Status:        "error",
			PreviousState: previousState,
			CurrentState:  "stopped",
			Error:         err.Error(),
		}, err
	}

	// Start the node
	if err := manager.Start(ctx, n, metadata.GenesisPath); err != nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "start",
			Status:        "error",
			PreviousState: previousState,
			CurrentState:  "stopped",
			Error:         err.Error(),
		}, err
	}

	// Wait and check new status
	time.Sleep(2 * time.Second)
	newHealth := node.CheckNodeHealth(ctx, n)

	return &dto.NodeActionOutput{
		NodeIndex:     nodeIndex,
		Action:        "start",
		Status:        "success",
		PreviousState: previousState,
		CurrentState:  string(newHealth.Status),
	}, nil
}

// StopNode stops a specific node.
func (s *DevnetService) StopNode(ctx context.Context, nodeIndex int, timeout time.Duration) (*dto.NodeActionOutput, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	if nodeIndex < 0 || nodeIndex >= len(d.Nodes) {
		return nil, fmt.Errorf("invalid node index: %d (valid: 0-%d)", nodeIndex, len(d.Nodes)-1)
	}

	n := d.Nodes[nodeIndex]
	metadata := d.Metadata

	// Check current state
	health := node.CheckNodeHealth(ctx, n)
	previousState := string(health.Status)

	if health.Status == node.NodeStatusStopped || health.Status == node.NodeStatusError {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "stop",
			Status:        "skipped",
			PreviousState: previousState,
			CurrentState:  previousState,
			Error:         fmt.Sprintf("node%d is not running", nodeIndex),
		}, nil
	}

	// Create node manager factory
	factory := s.createNodeManagerFactory(metadata)
	manager, err := factory.Create()
	if err != nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "stop",
			Status:        "error",
			PreviousState: previousState,
			CurrentState:  previousState,
			Error:         err.Error(),
		}, err
	}

	// Stop the node
	if err := manager.Stop(ctx, n, timeout); err != nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "stop",
			Status:        "error",
			PreviousState: previousState,
			CurrentState:  previousState,
			Error:         err.Error(),
		}, err
	}

	return &dto.NodeActionOutput{
		NodeIndex:     nodeIndex,
		Action:        "stop",
		Status:        "success",
		PreviousState: previousState,
		CurrentState:  "stopped",
	}, nil
}

// GetNodeLogs returns log lines for a specific node.
func (s *DevnetService) GetNodeLogs(ctx context.Context, nodeIndex, lines int, since string) (*dto.LogsOutput, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	if nodeIndex < 0 || nodeIndex >= len(d.Nodes) {
		return nil, fmt.Errorf("invalid node index: %d (valid: 0-%d)", nodeIndex, len(d.Nodes)-1)
	}

	n := d.Nodes[nodeIndex]
	var logLines []string

	switch d.Metadata.ExecutionMode {
	case devnet.ModeDocker:
		manager := node.NewDockerManager("", s.logger)
		logs, err := manager.GetLogs(ctx, n, lines, since)
		if err != nil {
			return nil, fmt.Errorf("failed to get docker logs: %w", err)
		}
		logLines = splitLines(logs)

	case devnet.ModeLocal:
		manager := node.NewLocalManager("", s.logger)
		logs, err := manager.GetLogs(ctx, n, lines)
		if err != nil {
			return nil, fmt.Errorf("failed to get local logs: %w", err)
		}
		logLines = splitLines(logs)

	default:
		return nil, fmt.Errorf("unknown execution mode: %s", d.Metadata.ExecutionMode)
	}

	return &dto.LogsOutput{
		NodeIndex: nodeIndex,
		Lines:     logLines,
	}, nil
}

// GetExecutionModeInfo returns information about execution mode for a node.
func (s *DevnetService) GetExecutionModeInfo(ctx context.Context, nodeIndex int) (*dto.ExecutionModeInfo, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	if nodeIndex < 0 || nodeIndex >= len(d.Nodes) {
		return nil, fmt.Errorf("invalid node index: %d (valid: 0-%d)", nodeIndex, len(d.Nodes)-1)
	}

	n := d.Nodes[nodeIndex]

	info := &dto.ExecutionModeInfo{
		Mode:          string(d.Metadata.ExecutionMode),
		DockerImage:   d.Metadata.DockerImage,
		ContainerName: n.DockerContainerName(),
		LogPath:       n.LogFilePath(),
	}

	return info, nil
}

// GetNumValidators returns the number of validators in the devnet.
func (s *DevnetService) GetNumValidators() (int, error) {
	metadata, err := devnet.LoadDevnetMetadata(s.homeDir)
	if err != nil {
		return 0, fmt.Errorf("failed to load metadata: %w", err)
	}
	return metadata.NumValidators, nil
}

// GetDockerManager returns a DockerManager for log operations.
func (s *DevnetService) GetDockerManager() *node.DockerManager {
	return node.NewDockerManager("", s.logger)
}

// GetLocalManager returns a LocalManager for log operations.
func (s *DevnetService) GetLocalManager() *node.LocalManager {
	return node.NewLocalManager("", s.logger)
}

// GetNode returns a specific node by index.
func (s *DevnetService) GetNode(nodeIndex int) (*node.Node, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	if nodeIndex < 0 || nodeIndex >= len(d.Nodes) {
		return nil, fmt.Errorf("invalid node index: %d (valid: 0-%d)", nodeIndex, len(d.Nodes)-1)
	}

	return d.Nodes[nodeIndex], nil
}

// GetAllNodes returns all nodes in the devnet.
func (s *DevnetService) GetAllNodes() ([]*node.Node, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}

	d, err := devnet.LoadDevnetWithNodes(s.homeDir, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	return d.Nodes, nil
}

// GetBlockchainNetwork returns the blockchain network name.
func (s *DevnetService) GetBlockchainNetwork() (string, error) {
	metadata, err := devnet.LoadDevnetMetadata(s.homeDir)
	if err != nil {
		return "", fmt.Errorf("failed to load metadata: %w", err)
	}
	return metadata.BlockchainNetwork, nil
}

// GetChainID returns the chain ID.
func (s *DevnetService) GetChainID() (string, error) {
	metadata, err := devnet.LoadDevnetMetadata(s.homeDir)
	if err != nil {
		return "", fmt.Errorf("failed to load metadata: %w", err)
	}
	return metadata.ChainID, nil
}

// IsDockerMode returns true if the devnet uses Docker execution mode.
func (s *DevnetService) IsDockerMode() (bool, error) {
	metadata, err := devnet.LoadDevnetMetadata(s.homeDir)
	if err != nil {
		return false, fmt.Errorf("failed to load metadata: %w", err)
	}
	return metadata.ExecutionMode == devnet.ModeDocker, nil
}

// ExportKeys exports validator and account keys from the devnet.
func (s *DevnetService) ExportKeys(keyType string) (*devnet.KeyExport, error) {
	if !s.DevnetExists() {
		return nil, fmt.Errorf("no devnet found at %s", s.homeDir)
	}
	return devnet.ExportKeys(s.homeDir, keyType)
}

// createNodeManagerFactory creates a NodeManagerFactory from devnet metadata.
func (s *DevnetService) createNodeManagerFactory(metadata *devnet.DevnetMetadata) *node.NodeManagerFactory {
	var mode node.ExecutionMode
	switch metadata.ExecutionMode {
	case devnet.ModeDocker:
		mode = node.ModeDocker
	case devnet.ModeLocal:
		mode = node.ModeLocal
	}

	config := node.FactoryConfig{
		Mode:        mode,
		BinaryPath:  s.resolveBinaryPath(metadata),
		DockerImage: metadata.DockerImage,
		EVMChainID:  node.ExtractEVMChainID(metadata.ChainID),
		Logger:      s.logger,
	}

	return node.NewNodeManagerFactory(config)
}

// resolveBinaryPath returns the binary path for local execution mode.
func (s *DevnetService) resolveBinaryPath(metadata *devnet.DevnetMetadata) string {
	if metadata.CustomBinaryPath != "" {
		return metadata.CustomBinaryPath
	}
	return filepath.Join(metadata.HomeDir, "bin", metadata.GetBinaryName())
}

// splitLines splits a string into lines, filtering out empty lines.
func splitLines(s string) []string {
	var result []string
	for _, line := range filepath.SplitList(s) {
		if line != "" {
			result = append(result, line)
		}
	}
	// Use strings.Split for line splitting
	lines := make([]string, 0)
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// getDefaultService returns a service instance using global homeDir.
func getDefaultService() *DevnetService {
	return NewDevnetService(homeDir, output.DefaultLogger)
}
