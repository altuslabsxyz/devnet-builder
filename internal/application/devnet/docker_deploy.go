package devnet

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	domainports "github.com/altuslabsxyz/devnet-builder/internal/domain/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/node"
	"github.com/altuslabsxyz/devnet-builder/types"
)

// DockerDeployUseCase handles Docker-based devnet deployment with network isolation
type DockerDeployUseCase struct {
	orchestrator domainports.DeploymentOrchestrator
	devnetRepo   ports.DevnetRepository
	nodeRepo     ports.NodeRepository
	provisionUC  *ProvisionUseCase
	logger       ports.Logger
}

// NewDockerDeployUseCase creates a new Docker deployment use case
func NewDockerDeployUseCase(
	orchestrator domainports.DeploymentOrchestrator,
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	provisionUC *ProvisionUseCase,
	logger ports.Logger,
) *DockerDeployUseCase {
	return &DockerDeployUseCase{
		orchestrator: orchestrator,
		devnetRepo:   devnetRepo,
		nodeRepo:     nodeRepo,
		provisionUC:  provisionUC,
		logger:       logger,
	}
}

// Execute performs a Docker-based deployment
func (uc *DockerDeployUseCase) Execute(ctx context.Context, input dto.DeploymentInput) (*dto.DeploymentOutput, error) {
	uc.logger.Info("Starting Docker deployment for %s", input.DevnetName)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Step 1: Provision the devnet (initialize nodes, create genesis, etc.)
	uc.logger.Info("Provisioning devnet...")
	provisionInput := dto.ProvisionInput{
		HomeDir:       input.HomeDir,
		Network:       string(types.NetworkSourceTestnet), // Default to testnet for now
		NumValidators: input.ValidatorCount,
		NumAccounts:   0, // No separate accounts for Docker deployment
		Mode:          string(types.ExecutionModeDocker),
		DockerImage:   input.Image,
		StableVersion: "",
		NoCache:       false,
		UseSnapshot:   false,
	}

	provisionOutput, err := uc.provisionUC.Execute(ctx, provisionInput)
	if err != nil {
		return nil, fmt.Errorf("provisioning failed: %w", err)
	}

	// Step 2: Use orchestrator to deploy containers with network isolation
	uc.logger.Info("Deploying containers with network isolation...")
	deployConfig := &domainports.DeploymentConfig{
		DevnetName:     input.DevnetName,
		ValidatorCount: input.ValidatorCount,
		Image:          input.Image,
		ChainID:        provisionOutput.ChainID,
		HomeDir:        input.HomeDir,
		ResourceLimits: convertResourceLimits(input.ResourceLimits),
		CustomBuild:    convertCustomBuild(input.CustomBuild),
	}

	result, err := uc.orchestrator.Deploy(ctx, deployConfig)
	if err != nil {
		return nil, fmt.Errorf("container deployment failed: %w", err)
	}

	// Step 3: Update devnet metadata with Docker config
	metadata, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	metadata.DockerConfig = &ports.DockerConfigMetadata{
		NetworkID:      result.NetworkID,
		NetworkName:    fmt.Sprintf("devnet-%s-network", input.DevnetName),
		Subnet:         result.Subnet,
		PortRangeStart: result.PortAllocation.PortRangeStart,
		PortRangeEnd:   result.PortAllocation.PortRangeEnd,
		Image:          input.Image,
	}
	metadata.Status = ports.StateRunning
	now := time.Now()
	metadata.LastStarted = &now

	if err := uc.devnetRepo.Save(ctx, metadata); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	// Convert result to DTO
	return &dto.DeploymentOutput{
		DevnetName:     result.DevnetName,
		NetworkID:      result.NetworkID,
		Subnet:         result.Subnet,
		Containers:     convertContainerInfo(result.Containers),
		PortRangeStart: result.PortAllocation.PortRangeStart,
		PortRangeEnd:   result.PortAllocation.PortRangeEnd,
		Success:        result.Success,
	}, nil
}

// convertResourceLimits converts DTO resource limits to domain type
func convertResourceLimits(limits *dto.ResourceLimits) *domainports.ResourceLimits {
	if limits == nil {
		return nil
	}
	return &domainports.ResourceLimits{
		Memory: limits.Memory,
		CPUs:   limits.CPUs,
	}
}

// convertCustomBuild converts DTO custom build config to domain type
func convertCustomBuild(build *dto.CustomBuildConfig) *domainports.CustomBuildConfig {
	if build == nil {
		return nil
	}
	return &domainports.CustomBuildConfig{
		PluginPath:  build.PluginPath,
		ChainBinary: build.ChainBinary,
		BuildArgs:   build.BuildArgs,
	}
}

// convertContainerInfo converts domain container info to DTO
func convertContainerInfo(containers []*domainports.ContainerInfo) []*dto.ContainerInfo {
	result := make([]*dto.ContainerInfo, len(containers))
	for i, c := range containers {
		result[i] = &dto.ContainerInfo{
			ID:        c.ID,
			Name:      c.Name,
			NodeIndex: c.NodeIndex,
		}
		if c.Ports != nil {
			result[i].RPCPort = c.Ports.RPC
			result[i].P2PPort = c.Ports.P2P
			result[i].GRPCPort = c.Ports.GRPC
			result[i].EVMRPCPort = c.Ports.EVMRPC
			result[i].EVMWSPort = c.Ports.EVMWS
		}
	}
	return result
}

// DockerDestroyUseCase handles cleanup of Docker-based devnets
type DockerDestroyUseCase struct {
	orchestrator   domainports.DeploymentOrchestrator
	networkManager domainports.NetworkManager
	portAllocator  domainports.PortAllocator
	devnetRepo     ports.DevnetRepository
	logger         ports.Logger
}

// NewDockerDestroyUseCase creates a new Docker destroy use case
func NewDockerDestroyUseCase(
	orchestrator domainports.DeploymentOrchestrator,
	networkManager domainports.NetworkManager,
	portAllocator domainports.PortAllocator,
	devnetRepo ports.DevnetRepository,
	logger ports.Logger,
) *DockerDestroyUseCase {
	return &DockerDestroyUseCase{
		orchestrator:   orchestrator,
		networkManager: networkManager,
		portAllocator:  portAllocator,
		devnetRepo:     devnetRepo,
		logger:         logger,
	}
}

// Execute destroys a Docker-based devnet
func (uc *DockerDestroyUseCase) Execute(ctx context.Context, homeDir string) error {
	uc.logger.Info("Destroying Docker devnet at %s", homeDir)

	// Load metadata to get Docker config
	metadata, err := uc.devnetRepo.Load(ctx, homeDir)
	if err != nil {
		uc.logger.Warn("Could not load metadata: %v", err)
		// Continue with cleanup anyway
	}

	// Build deployment state for rollback
	state := &domainports.DeploymentState{
		DevnetName:        filepath.Base(homeDir),
		StartedContainers: []string{},
	}

	if metadata != nil && metadata.DockerConfig != nil {
		// Set network ID for cleanup
		state.NetworkID = &metadata.DockerConfig.NetworkID

		// Set port range for cleanup
		state.PortRange = &domainports.PortAllocation{
			DevnetName:     filepath.Base(homeDir),
			PortRangeStart: metadata.DockerConfig.PortRangeStart,
			PortRangeEnd:   metadata.DockerConfig.PortRangeEnd,
		}

		// Find running containers for this devnet
		containers, err := uc.findDevnetContainers(ctx, metadata.DockerConfig.NetworkID)
		if err != nil {
			uc.logger.Warn("Could not find containers: %v", err)
		} else {
			state.StartedContainers = containers
		}
	}

	// Use orchestrator to rollback (cleanup all resources)
	if err := uc.orchestrator.Rollback(ctx, state); err != nil {
		uc.logger.Error("Rollback failed: %v", err)
		return fmt.Errorf("cleanup failed: %w", err)
	}

	uc.logger.Info("Docker devnet destroyed successfully")
	return nil
}

// findDevnetContainers finds all containers attached to a Docker network.
// Uses docker network inspect to get the list of container IDs.
func (uc *DockerDestroyUseCase) findDevnetContainers(ctx context.Context, networkID string) ([]string, error) {
	if networkID == "" {
		return []string{}, nil
	}

	// Run docker network inspect to get network details
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", networkID)
	output, err := cmd.Output()
	if err != nil {
		// Network might not exist anymore, that's OK
		uc.logger.Debug("Network inspect failed (may not exist): %v", err)
		return []string{}, nil
	}

	// Parse the JSON output from docker network inspect
	var inspectResults []struct {
		Containers map[string]struct {
			Name        string `json:"Name"`
			EndpointID  string `json:"EndpointID"`
			MacAddress  string `json:"MacAddress"`
			IPv4Address string `json:"IPv4Address"`
		} `json:"Containers"`
	}

	if err := json.Unmarshal(output, &inspectResults); err != nil {
		return nil, fmt.Errorf("failed to parse network inspect output: %w", err)
	}

	if len(inspectResults) == 0 {
		return []string{}, nil
	}

	// Extract container IDs from the Containers map
	containers := make([]string, 0, len(inspectResults[0].Containers))
	for containerID := range inspectResults[0].Containers {
		containers = append(containers, containerID)
	}

	uc.logger.Debug("Found %d containers in network %s", len(containers), networkID)
	return containers, nil
}

// GetDefaultNodePorts returns default ports for a node at given index in Docker mode
func GetDefaultNodePorts(basePort, nodeIndex int) *node.NodePorts {
	offset := nodeIndex * 100
	return &node.NodePorts{
		RPC:    basePort + offset,
		P2P:    basePort + offset + 1,
		GRPC:   basePort + offset + 2,
		EVMRPC: basePort + offset + 3,
		EVMWS:  basePort + offset + 4,
		PProf:  basePort + offset + 5,
	}
}
