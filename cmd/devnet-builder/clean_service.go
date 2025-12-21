// Package main provides the CLI entry point for devnet-builder.
// clean_service.go contains the Clean Architecture service layer
// that uses DI Container and UseCases for all operations.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/di"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// CleanDevnetService provides unified access to devnet operations
// using Clean Architecture principles with DI Container and UseCases.
// This replaces the legacy DevnetService that directly used internal packages.
type CleanDevnetService struct {
	container *di.Container
	homeDir   string
	logger    *output.Logger
}

// NewCleanDevnetService creates a new CleanDevnetService with DI Container.
func NewCleanDevnetService(homeDir string, logger *output.Logger, opts ...di.Option) (*CleanDevnetService, error) {
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Create infrastructure factory and wire container
	factory := di.NewInfrastructureFactory(homeDir, logger)
	container, err := factory.WireContainer(opts...)
	if err != nil {
		return nil, err
	}

	return &CleanDevnetService{
		container: container,
		homeDir:   homeDir,
		logger:    logger,
	}, nil
}

// Container returns the DI container for advanced usage.
func (s *CleanDevnetService) Container() *di.Container {
	return s.container
}

// DevnetExists checks if a devnet exists.
func (s *CleanDevnetService) DevnetExists() bool {
	// Use repository from container
	repo := s.container.DevnetRepository()
	if repo == nil {
		return false
	}
	return repo.Exists(s.homeDir)
}

// Provision provisions a new devnet using ProvisionUseCase.
func (s *CleanDevnetService) Provision(ctx context.Context, input dto.ProvisionInput) (*dto.ProvisionOutput, error) {
	input.HomeDir = s.homeDir
	return s.container.ProvisionUseCase().Execute(ctx, input)
}

// Run starts devnet nodes using RunUseCase.
func (s *CleanDevnetService) Run(ctx context.Context, input dto.RunInput) (*dto.RunOutput, error) {
	input.HomeDir = s.homeDir
	return s.container.RunUseCase().Execute(ctx, input)
}

// Stop stops devnet nodes using StopUseCase.
func (s *CleanDevnetService) Stop(ctx context.Context, timeout time.Duration) (*dto.StopOutput, error) {
	input := dto.StopInput{
		HomeDir: s.homeDir,
		Timeout: timeout,
	}
	return s.container.StopUseCase().Execute(ctx, input)
}

// GetHealth returns health status using HealthUseCase.
func (s *CleanDevnetService) GetHealth(ctx context.Context) (*dto.HealthOutput, error) {
	input := dto.HealthInput{
		HomeDir: s.homeDir,
	}
	return s.container.HealthUseCase().Execute(ctx, input)
}

// Reset resets the devnet using ResetUseCase.
func (s *CleanDevnetService) Reset(ctx context.Context, hard bool) (*dto.ResetOutput, error) {
	input := dto.ResetInput{
		HomeDir:   s.homeDir,
		HardReset: hard,
	}
	return s.container.ResetUseCase().Execute(ctx, input)
}

// Destroy destroys the devnet using DestroyUseCase.
func (s *CleanDevnetService) Destroy(ctx context.Context, cleanCache bool) (*dto.DestroyOutput, error) {
	input := dto.DestroyInput{
		HomeDir:    s.homeDir,
		CleanCache: cleanCache,
	}
	return s.container.DestroyUseCase().Execute(ctx, input)
}

// GetStatus returns the full status of the devnet.
func (s *CleanDevnetService) GetStatus(ctx context.Context) (*dto.StatusOutput, error) {
	// Load devnet info
	repo := s.container.DevnetRepository()
	metadata, err := repo.Load(ctx, s.homeDir)
	if err != nil {
		return nil, err
	}

	// Get health status
	health, err := s.GetHealth(ctx)
	if err != nil {
		return nil, err
	}

	// Determine overall status
	overallStatus := string(metadata.Status)
	runningCount := 0
	for _, h := range health.Nodes {
		if h.IsRunning {
			runningCount++
		}
	}

	if runningCount == metadata.NumValidators {
		overallStatus = "running"
	} else if runningCount > 0 {
		overallStatus = "partial"
	} else {
		overallStatus = "stopped"
	}

	// Build DevnetInfo from metadata
	devnetInfo := &dto.DevnetInfo{
		HomeDir:           metadata.HomeDir,
		ChainID:           metadata.ChainID,
		NetworkSource:     metadata.NetworkName,
		BlockchainNetwork: metadata.BlockchainNetwork,
		ExecutionMode:     string(metadata.ExecutionMode),
		DockerImage:       metadata.DockerImage,
		NumValidators:     metadata.NumValidators,
		NumAccounts:       metadata.NumAccounts,
		InitialVersion:    metadata.InitialVersion,
		CurrentVersion:    metadata.CurrentVersion,
		Status:            string(metadata.Status),
		CreatedAt:         metadata.CreatedAt,
	}

	// Load nodes info
	nodeRepo := s.container.NodeRepository()
	nodes, _ := nodeRepo.LoadAll(ctx, s.homeDir)
	devnetInfo.Nodes = make([]dto.NodeInfo, len(nodes))
	for i, n := range nodes {
		devnetInfo.Nodes[i] = dto.NodeInfo{
			Index:   n.Index,
			Name:    n.Name,
			HomeDir: n.HomeDir,
			NodeID:  n.NodeID,
			Ports:   n.Ports,
		}
	}

	return &dto.StatusOutput{
		Devnet:        devnetInfo,
		OverallStatus: overallStatus,
		Nodes:         health.Nodes,
		AllHealthy:    health.AllHealthy,
	}, nil
}

// Restart restarts all devnet nodes.
func (s *CleanDevnetService) Restart(ctx context.Context, timeout time.Duration) (*dto.RestartOutput, error) {
	// Stop first
	stopResult, err := s.Stop(ctx, timeout)
	if err != nil {
		return nil, err
	}

	// Brief pause between stop and start
	time.Sleep(2 * time.Second)

	// Start again
	runInput := dto.RunInput{
		HomeDir:     s.homeDir,
		WaitForSync: false,
		Timeout:     timeout,
	}
	runResult, err := s.Run(ctx, runInput)
	if err != nil {
		return &dto.RestartOutput{
			StoppedNodes: stopResult.StoppedNodes,
			StartedNodes: 0,
			AllRunning:   false,
		}, err
	}

	return &dto.RestartOutput{
		StoppedNodes: stopResult.StoppedNodes,
		StartedNodes: len(runResult.Nodes),
		AllRunning:   runResult.AllRunning,
	}, nil
}

// LoadDevnetInfo loads devnet information for display.
func (s *CleanDevnetService) LoadDevnetInfo(ctx context.Context) (*dto.DevnetInfo, error) {
	repo := s.container.DevnetRepository()
	metadata, err := repo.Load(ctx, s.homeDir)
	if err != nil {
		return nil, err
	}

	// Load nodes
	nodeRepo := s.container.NodeRepository()
	nodes, _ := nodeRepo.LoadAll(ctx, s.homeDir)

	info := &dto.DevnetInfo{
		HomeDir:           metadata.HomeDir,
		ChainID:           metadata.ChainID,
		NetworkSource:     metadata.NetworkName,
		BlockchainNetwork: metadata.BlockchainNetwork,
		ExecutionMode:     string(metadata.ExecutionMode),
		DockerImage:       metadata.DockerImage,
		NumValidators:     metadata.NumValidators,
		NumAccounts:       metadata.NumAccounts,
		InitialVersion:    metadata.InitialVersion,
		CurrentVersion:    metadata.CurrentVersion,
		Status:            string(metadata.Status),
		CreatedAt:         metadata.CreatedAt,
		Nodes:             make([]dto.NodeInfo, len(nodes)),
	}

	for i, n := range nodes {
		info.Nodes[i] = dto.NodeInfo{
			Index:   n.Index,
			Name:    n.Name,
			HomeDir: n.HomeDir,
			NodeID:  n.NodeID,
			Ports:   n.Ports,
		}
	}

	return info, nil
}

// GetNodeHealth returns the health status of a specific node.
func (s *CleanDevnetService) GetNodeHealth(ctx context.Context, nodeIndex int) (*dto.NodeHealthStatus, error) {
	health, err := s.GetHealth(ctx)
	if err != nil {
		return nil, err
	}

	for _, h := range health.Nodes {
		if h.Index == nodeIndex {
			return &h, nil
		}
	}

	return nil, &NodeNotFoundError{Index: nodeIndex}
}

// GetNumValidators returns the number of validators in the devnet.
func (s *CleanDevnetService) GetNumValidators(ctx context.Context) (int, error) {
	repo := s.container.DevnetRepository()
	metadata, err := repo.Load(ctx, s.homeDir)
	if err != nil {
		return 0, err
	}
	return metadata.NumValidators, nil
}

// IsDockerMode returns true if the devnet uses Docker execution mode.
func (s *CleanDevnetService) IsDockerMode(ctx context.Context) (bool, error) {
	repo := s.container.DevnetRepository()
	metadata, err := repo.Load(ctx, s.homeDir)
	if err != nil {
		return false, err
	}
	return metadata.ExecutionMode == ports.ModeDocker, nil
}

// GetBlockchainNetwork returns the blockchain network name.
func (s *CleanDevnetService) GetBlockchainNetwork(ctx context.Context) (string, error) {
	repo := s.container.DevnetRepository()
	metadata, err := repo.Load(ctx, s.homeDir)
	if err != nil {
		return "", err
	}
	return metadata.BlockchainNetwork, nil
}

// Start starts all devnet nodes (convenience wrapper for Run).
func (s *CleanDevnetService) Start(ctx context.Context, timeout time.Duration) (*dto.RunOutput, error) {
	input := dto.RunInput{
		HomeDir:     s.homeDir,
		WaitForSync: false,
		Timeout:     timeout,
	}
	return s.container.RunUseCase().Execute(ctx, input)
}

// GetChainID returns the chain ID.
func (s *CleanDevnetService) GetChainID(ctx context.Context) (string, error) {
	repo := s.container.DevnetRepository()
	metadata, err := repo.Load(ctx, s.homeDir)
	if err != nil {
		return "", err
	}
	return metadata.ChainID, nil
}

// GetNodeLogPath returns the log file path for a node.
func (s *CleanDevnetService) GetNodeLogPath(ctx context.Context, nodeIndex int) (string, error) {
	nodeRepo := s.container.NodeRepository()
	node, err := nodeRepo.Load(ctx, s.homeDir, nodeIndex)
	if err != nil {
		return "", err
	}
	// Construct log path from node home directory
	return node.HomeDir + "/stable.log", nil
}

// GetNodeLogs returns log lines for a specific node.
func (s *CleanDevnetService) GetNodeLogs(ctx context.Context, nodeIndex, lines int, since string) (*dto.LogsOutput, error) {
	nodeRepo := s.container.NodeRepository()
	node, err := nodeRepo.Load(ctx, s.homeDir, nodeIndex)
	if err != nil {
		return nil, err
	}

	logPath := node.HomeDir + "/stable.log"
	logLines, err := readLogLines(logPath, lines)
	if err != nil {
		return nil, err
	}

	return &dto.LogsOutput{
		NodeIndex: nodeIndex,
		Lines:     logLines,
	}, nil
}

// GetExecutionModeInfo returns execution mode information for a node.
func (s *CleanDevnetService) GetExecutionModeInfo(ctx context.Context, nodeIndex int) (*dto.ExecutionModeInfo, error) {
	repo := s.container.DevnetRepository()
	metadata, err := repo.Load(ctx, s.homeDir)
	if err != nil {
		return nil, err
	}

	nodeRepo := s.container.NodeRepository()
	node, err := nodeRepo.Load(ctx, s.homeDir, nodeIndex)
	if err != nil {
		return nil, err
	}

	containerName := ""
	if metadata.ExecutionMode == ports.ModeDocker {
		containerName = "stable-devnet-node" + string(rune('0'+nodeIndex))
	}

	return &dto.ExecutionModeInfo{
		Mode:          string(metadata.ExecutionMode),
		DockerImage:   metadata.DockerImage,
		ContainerName: containerName,
		LogPath:       node.HomeDir + "/stable.log",
	}, nil
}

// StartNode starts a specific node.
func (s *CleanDevnetService) StartNode(ctx context.Context, nodeIndex int) (*dto.NodeActionOutput, error) {
	nodeRepo := s.container.NodeRepository()
	node, err := nodeRepo.Load(ctx, s.homeDir, nodeIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to load node %d: %w", nodeIndex, err)
	}

	// Check if already running
	health, err := s.GetNodeHealth(ctx, nodeIndex)
	if err == nil && health.IsRunning {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "start",
			Status:        "skipped",
			PreviousState: "running",
			CurrentState:  "running",
			Error:         fmt.Sprintf("node%d is already running", nodeIndex),
		}, nil
	}

	// Get network module for start command
	networkModule := s.container.NetworkModule()
	if networkModule == nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "start",
			Status:        "error",
			PreviousState: "stopped",
			CurrentState:  "stopped",
			Error:         "network module not configured",
		}, fmt.Errorf("network module not configured")
	}

	executor := s.container.Executor()
	if executor == nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "start",
			Status:        "error",
			PreviousState: "stopped",
			CurrentState:  "stopped",
			Error:         "process executor not configured",
		}, fmt.Errorf("process executor not configured")
	}

	// Build start command
	args := networkModule.StartCommand(node.HomeDir)
	cmd := ports.Command{
		Binary:  networkModule.BinaryName(),
		Args:    args,
		WorkDir: node.HomeDir,
		LogPath: filepath.Join(node.HomeDir, networkModule.LogFileName()),
		PIDPath: filepath.Join(node.HomeDir, networkModule.PIDFileName()),
	}

	// Start the node
	handle, err := executor.Start(ctx, cmd)
	if err != nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "start",
			Status:        "error",
			PreviousState: "stopped",
			CurrentState:  "stopped",
			Error:         err.Error(),
		}, err
	}

	// Update node with PID
	pid := handle.PID()
	node.PID = &pid
	if err := nodeRepo.Save(ctx, node); err != nil {
		s.logger.Warn("Failed to save node %d state: %v", nodeIndex, err)
	}

	// Wait briefly and check status
	time.Sleep(2 * time.Second)

	return &dto.NodeActionOutput{
		NodeIndex:     nodeIndex,
		Action:        "start",
		Status:        "success",
		PreviousState: "stopped",
		CurrentState:  "running",
	}, nil
}

// StopNode stops a specific node.
func (s *CleanDevnetService) StopNode(ctx context.Context, nodeIndex int, timeout time.Duration) (*dto.NodeActionOutput, error) {
	nodeRepo := s.container.NodeRepository()
	node, err := nodeRepo.Load(ctx, s.homeDir, nodeIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to load node %d: %w", nodeIndex, err)
	}

	// Check if already stopped
	if node.PID == nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "stop",
			Status:        "skipped",
			PreviousState: "stopped",
			CurrentState:  "stopped",
			Error:         fmt.Sprintf("node%d is not running", nodeIndex),
		}, nil
	}

	executor := s.container.Executor()
	if executor == nil {
		return &dto.NodeActionOutput{
			NodeIndex:     nodeIndex,
			Action:        "stop",
			Status:        "error",
			PreviousState: "running",
			CurrentState:  "running",
			Error:         "process executor not configured",
		}, fmt.Errorf("process executor not configured")
	}

	// Create handle for the process
	handle := &cleanPIDHandle{pid: *node.PID}

	// Stop the node
	if err := executor.Stop(ctx, handle, timeout); err != nil {
		// Try force kill
		if killErr := executor.Kill(handle); killErr != nil {
			return &dto.NodeActionOutput{
				NodeIndex:     nodeIndex,
				Action:        "stop",
				Status:        "error",
				PreviousState: "running",
				CurrentState:  "unknown",
				Error:         fmt.Sprintf("failed to stop: %v, force kill failed: %v", err, killErr),
			}, err
		}
	}

	// Update node state
	node.PID = nil
	if err := nodeRepo.Save(ctx, node); err != nil {
		s.logger.Warn("Failed to save node %d state: %v", nodeIndex, err)
	}

	return &dto.NodeActionOutput{
		NodeIndex:     nodeIndex,
		Action:        "stop",
		Status:        "success",
		PreviousState: "running",
		CurrentState:  "stopped",
	}, nil
}

// cleanPIDHandle implements ports.ProcessHandle for stopping by PID.
type cleanPIDHandle struct {
	pid int
}

func (h *cleanPIDHandle) PID() int        { return h.pid }
func (h *cleanPIDHandle) IsRunning() bool { return false }
func (h *cleanPIDHandle) Wait() error     { return nil }
func (h *cleanPIDHandle) Kill() error     { return nil }

// ExportKeys exports validator and account keys.
func (s *CleanDevnetService) ExportKeys(ctx context.Context, keyType string) (*dto.ExportKeysOutput, error) {
	repo := s.container.DevnetRepository()
	metadata, err := repo.Load(ctx, s.homeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	devnetDir := filepath.Join(s.homeDir, "devnet")
	output := &dto.ExportKeysOutput{
		ValidatorKeys: []dto.ValidatorKeyInfo{},
		AccountKeys:   []dto.AccountKeyInfo{},
	}

	// Export validator keys
	if keyType == "all" || keyType == "validators" {
		for i := 0; i < metadata.NumValidators; i++ {
			key, err := s.readValidatorKeyFromFile(devnetDir, i)
			if err != nil {
				key = &dto.ValidatorKeyInfo{
					Index:   i,
					Name:    fmt.Sprintf("validator%d", i),
					Address: fmt.Sprintf("(error: %v)", err),
				}
			}
			output.ValidatorKeys = append(output.ValidatorKeys, *key)
		}
	}

	// Export account keys
	if keyType == "all" || keyType == "accounts" {
		for i := 0; i < metadata.NumAccounts; i++ {
			key, err := s.readAccountKeyFromFile(devnetDir, i)
			if err != nil {
				key = &dto.AccountKeyInfo{
					Index:   i,
					Name:    fmt.Sprintf("account%d", i),
					Address: fmt.Sprintf("(error: %v)", err),
				}
			}
			output.AccountKeys = append(output.AccountKeys, *key)
		}
	}

	return output, nil
}

// validatorKeyFile represents the JSON structure of validator key file.
type validatorKeyFile struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	ValAddress string `json:"val_address"`
	PubKey     string `json:"pub_key"`
	Mnemonic   string `json:"mnemonic"`
}

// accountKeyFile represents the JSON structure of account key file.
type accountKeyFile struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	PubKey   string `json:"pub_key"`
	Mnemonic string `json:"mnemonic"`
}

// readValidatorKeyFromFile reads validator key info from JSON file.
func (s *CleanDevnetService) readValidatorKeyFromFile(devnetDir string, index int) (*dto.ValidatorKeyInfo, error) {
	nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", index))
	validatorFile := filepath.Join(nodeDir, fmt.Sprintf("validator%d.json", index))

	data, err := os.ReadFile(validatorFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read validator key file: %w", err)
	}

	var keyFile validatorKeyFile
	if err := json.Unmarshal(data, &keyFile); err != nil {
		return nil, fmt.Errorf("failed to parse validator key file: %w", err)
	}

	return &dto.ValidatorKeyInfo{
		Index:      index,
		Name:       keyFile.Name,
		Address:    keyFile.Address,
		ValAddress: keyFile.ValAddress,
		Mnemonic:   keyFile.Mnemonic,
	}, nil
}

// readAccountKeyFromFile reads account key info from JSON file.
func (s *CleanDevnetService) readAccountKeyFromFile(devnetDir string, index int) (*dto.AccountKeyInfo, error) {
	accountFile := filepath.Join(devnetDir, fmt.Sprintf("account%d.json", index))

	data, err := os.ReadFile(accountFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read account key file: %w", err)
	}

	var keyFile accountKeyFile
	if err := json.Unmarshal(data, &keyFile); err != nil {
		return nil, fmt.Errorf("failed to parse account key file: %w", err)
	}

	return &dto.AccountKeyInfo{
		Index:    index,
		Name:     keyFile.Name,
		Address:  keyFile.Address,
		Mnemonic: keyFile.Mnemonic,
	}, nil
}

// GetNode returns node information by index.
func (s *CleanDevnetService) GetNode(ctx context.Context, nodeIndex int) (*dto.NodeInfo, error) {
	nodeRepo := s.container.NodeRepository()
	node, err := nodeRepo.Load(ctx, s.homeDir, nodeIndex)
	if err != nil {
		return nil, err
	}

	return &dto.NodeInfo{
		Index:   node.Index,
		Name:    node.Name,
		HomeDir: node.HomeDir,
		NodeID:  node.NodeID,
		Ports:   node.Ports,
	}, nil
}

// readLogLines reads the last N lines from a log file.
func readLogLines(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	// Return last N lines
	if len(lines) <= n {
		return lines, nil
	}
	return lines[len(lines)-n:], nil
}

// readFileLines reads all lines from a file.
func readFileLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return lines, nil
}

// NodeNotFoundError is returned when a node is not found.
type NodeNotFoundError struct {
	Index int
}

func (e *NodeNotFoundError) Error() string {
	return "node not found: " + strconv.Itoa(e.Index)
}

// getCleanService returns a CleanDevnetService instance using global homeDir.
// This is the new Clean Architecture service factory that replaces getDefaultService.
func getCleanService() (*CleanDevnetService, error) {
	return NewCleanDevnetService(homeDir, output.DefaultLogger)
}

// LoadMetadata returns the raw metadata for advanced operations.
func (s *CleanDevnetService) LoadMetadata(ctx context.Context) (*ports.DevnetMetadata, error) {
	repo := s.container.DevnetRepository()
	return repo.Load(ctx, s.homeDir)
}

// SaveMetadata saves updated metadata.
func (s *CleanDevnetService) SaveMetadata(ctx context.Context, metadata *ports.DevnetMetadata) error {
	repo := s.container.DevnetRepository()
	return repo.Save(ctx, metadata)
}

// IsRunning returns true if the devnet is in running state.
func (s *CleanDevnetService) IsRunning(ctx context.Context) (bool, error) {
	metadata, err := s.LoadMetadata(ctx)
	if err != nil {
		return false, err
	}
	return metadata.Status == ports.StateRunning, nil
}

// GetCurrentVersion returns the current binary version.
func (s *CleanDevnetService) GetCurrentVersion(ctx context.Context) (string, error) {
	metadata, err := s.LoadMetadata(ctx)
	if err != nil {
		return "", err
	}
	return metadata.CurrentVersion, nil
}

// SetCurrentVersion updates the current binary version.
func (s *CleanDevnetService) SetCurrentVersion(ctx context.Context, version string) error {
	metadata, err := s.LoadMetadata(ctx)
	if err != nil {
		return err
	}
	metadata.CurrentVersion = version
	return s.SaveMetadata(ctx, metadata)
}

// GetExecutionMode returns the execution mode (docker/local).
func (s *CleanDevnetService) GetExecutionMode(ctx context.Context) (ports.ExecutionMode, error) {
	metadata, err := s.LoadMetadata(ctx)
	if err != nil {
		return "", err
	}
	return metadata.ExecutionMode, nil
}

// GetNetworkSource returns the network source (mainnet/testnet).
func (s *CleanDevnetService) GetNetworkSource(ctx context.Context) (string, error) {
	metadata, err := s.LoadMetadata(ctx)
	if err != nil {
		return "", err
	}
	return metadata.NetworkName, nil
}

// StopAll stops all nodes with given timeout.
func (s *CleanDevnetService) StopAll(ctx context.Context, timeout time.Duration) error {
	_, err := s.Stop(ctx, timeout)
	return err
}
