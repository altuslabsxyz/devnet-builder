package docker

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain/ports"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/node"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// Orchestrator errors
var (
	ErrValidationFailed     = errors.New("deployment configuration validation failed")
	ErrPortAllocationFailed = errors.New("failed to allocate port range")
	ErrPortConflictDetected = errors.New("allocated ports conflict with host services")
	ErrImagePullFailed      = errors.New("failed to pull Docker image")
	ErrCustomBuildFailed    = errors.New("failed to build custom Docker image")
	ErrContainerStartFailed = errors.New("failed to start container")
	ErrHealthCheckTimeout   = errors.New("container health check timeout")
	ErrPartialDeployment    = errors.New("partial deployment (some containers failed)")
	ErrOrchestratorRollbackFailed = errors.New("rollback encountered errors")
)

// OrchestratorImpl implements the DeploymentOrchestrator interface
type OrchestratorImpl struct {
	networkManager ports.NetworkManager
	portAllocator  ports.PortAllocator
	dockerManager  *node.DockerManager
	logger         *output.Logger
}

// NewOrchestrator creates a new deployment orchestrator
func NewOrchestrator(
	networkManager ports.NetworkManager,
	portAllocator ports.PortAllocator,
	dockerManager *node.DockerManager,
	logger *output.Logger,
) *OrchestratorImpl {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &OrchestratorImpl{
		networkManager: networkManager,
		portAllocator:  portAllocator,
		dockerManager:  dockerManager,
		logger:         logger,
	}
}

// Deploy orchestrates complete devnet deployment
func (o *OrchestratorImpl) Deploy(ctx context.Context, config *ports.DeploymentConfig) (*ports.DeploymentResult, error) {
	state := &ports.DeploymentState{
		DevnetName:        config.DevnetName,
		StartedAt:         time.Now(),
		StartedContainers: []string{},
		HealthyContainers: []string{},
		Errors:            []ports.DeploymentError{},
	}

	// Execute each phase sequentially
	if err := o.phaseValidate(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}

	if err := o.phaseCreateNetwork(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}

	if err := o.phaseAllocatePorts(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}

	if err := o.phasePullImage(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}

	if err := o.phaseStartContainers(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}

	// Health checking phase would go here (not implemented yet)

	// Finalize deployment
	return o.finalizeDeployment(ctx, config, state), nil
}

// Rollback manually triggers rollback of a deployment
func (o *OrchestratorImpl) Rollback(ctx context.Context, state *ports.DeploymentState) error {
	if state == nil {
		return nil
	}

	state.Phase = ports.PhaseRollingBack
	o.logger.Info("Rolling back deployment for %s", state.DevnetName)

	var errs []error

	// Step 1: Stop all started containers
	for _, containerID := range state.StartedContainers {
		o.logger.Debug("Stopping container %s", containerID)
		cmd := exec.CommandContext(ctx, "docker", "stop", "-t", "5", containerID)
		if err := cmd.Run(); err != nil {
			o.logger.Warn("Failed to stop container %s: %v", containerID, err)
			errs = append(errs, fmt.Errorf("stop container %s: %w", containerID, err))
		}
	}

	// Step 2: Remove all stopped containers
	for _, containerID := range state.StartedContainers {
		o.logger.Debug("Removing container %s", containerID)
		cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
		if err := cmd.Run(); err != nil {
			o.logger.Warn("Failed to remove container %s: %v", containerID, err)
			errs = append(errs, fmt.Errorf("remove container %s: %w", containerID, err))
		}
	}

	// Step 3: Delete Docker network
	if state.NetworkID != nil && *state.NetworkID != "" {
		o.logger.Debug("Deleting network %s", *state.NetworkID)
		if err := o.networkManager.DeleteNetwork(ctx, *state.NetworkID); err != nil {
			o.logger.Warn("Failed to delete network %s: %v", *state.NetworkID, err)
			errs = append(errs, fmt.Errorf("delete network %s: %w", *state.NetworkID, err))
		}
	}

	// Step 4: Release port allocation
	if state.PortRange != nil {
		o.logger.Debug("Releasing port allocation for %s", state.DevnetName)
		if err := o.portAllocator.ReleaseRange(ctx, state.DevnetName); err != nil {
			o.logger.Warn("Failed to release ports for %s: %v", state.DevnetName, err)
			errs = append(errs, fmt.Errorf("release ports: %w", err))
		}
	}

	if len(errs) > 0 {
		return &MultiError{Errors: errs}
	}

	o.logger.Info("Rollback completed successfully for %s", state.DevnetName)
	return nil
}

// GetState retrieves current deployment state for a devnet
func (o *OrchestratorImpl) GetState(ctx context.Context, devnetName string) (*ports.DeploymentState, error) {
	// TODO: Implement state persistence and retrieval
	// For now, return nil (no active deployment tracking)
	return nil, nil
}

// phaseValidate validates deployment configuration
func (o *OrchestratorImpl) phaseValidate(ctx context.Context, config *ports.DeploymentConfig, state *ports.DeploymentState) error {
	state.Phase = ports.PhaseValidating
	o.logger.Info("Validating deployment configuration for %s", config.DevnetName)

	// Validate devnet name
	if config.DevnetName == "" {
		return fmt.Errorf("%w: devnet name cannot be empty", ErrValidationFailed)
	}

	// Validate validator count
	if config.ValidatorCount < 1 || config.ValidatorCount > 100 {
		return fmt.Errorf("%w: validator count must be 1-100, got %d", ErrValidationFailed, config.ValidatorCount)
	}

	// Validate image
	if config.Image == "" {
		return fmt.Errorf("%w: docker image cannot be empty", ErrValidationFailed)
	}

	// Check Docker daemon availability
	if !node.IsDockerAvailable(ctx) {
		return fmt.Errorf("%w: docker daemon not accessible", ErrValidationFailed)
	}

	return nil
}

// phaseCreateNetwork creates isolated Docker network
func (o *OrchestratorImpl) phaseCreateNetwork(ctx context.Context, config *ports.DeploymentConfig, state *ports.DeploymentState) error {
	state.Phase = ports.PhaseNetworkCreating
	o.logger.Info("Creating Docker network for %s", config.DevnetName)

	networkID, subnet, err := o.networkManager.CreateNetwork(ctx, config.DevnetName)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNetworkCreationFailed, err)
	}

	state.NetworkID = &networkID
	o.logger.Info("Network created: %s (subnet: %s)", networkID, subnet)

	return nil
}

// phaseAllocatePorts allocates port range for devnet
func (o *OrchestratorImpl) phaseAllocatePorts(ctx context.Context, config *ports.DeploymentConfig, state *ports.DeploymentState) error {
	state.Phase = ports.PhasePortAllocating
	o.logger.Info("Allocating port range for %s", config.DevnetName)

	allocation, err := o.portAllocator.AllocateRange(ctx, config.DevnetName, config.ValidatorCount)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPortAllocationFailed, err)
	}

	// Validate port availability
	conflicts, err := o.portAllocator.ValidatePortAvailability(ctx, allocation)
	if err != nil {
		return fmt.Errorf("port validation failed: %w", err)
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("%w: ports in use: %v", ErrPortConflictDetected, conflicts)
	}

	state.PortRange = allocation
	o.logger.Info("Ports allocated: %d-%d", allocation.PortRangeStart, allocation.PortRangeEnd)

	return nil
}

// phasePullImage pulls or builds Docker image
func (o *OrchestratorImpl) phasePullImage(ctx context.Context, config *ports.DeploymentConfig, state *ports.DeploymentState) error {
	state.Phase = ports.PhaseImagePulling
	o.logger.Info("Pulling Docker image: %s", config.Image)

	// TODO: Handle custom build if config.CustomBuild is set
	if config.CustomBuild != nil {
		return fmt.Errorf("%w: custom builds not yet implemented", ErrCustomBuildFailed)
	}

	// Validate/pull image
	if err := o.dockerManager.ValidateImage(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrImagePullFailed, err)
	}

	o.logger.Info("Image ready: %s", config.Image)
	return nil
}

// phaseStartContainers starts all validator containers
func (o *OrchestratorImpl) phaseStartContainers(ctx context.Context, config *ports.DeploymentConfig, state *ports.DeploymentState) error {
	state.Phase = ports.PhaseContainerStarting
	o.logger.Info("Starting %d containers for %s", config.ValidatorCount, config.DevnetName)

	// Assumption: Nodes have already been provisioned (directories, genesis, config files exist)
	// This phase just starts Docker containers for the provisioned nodes

	if state.PortRange == nil {
		return fmt.Errorf("port range not allocated")
	}
	if state.NetworkID == nil || *state.NetworkID == "" {
		return fmt.Errorf("network not created")
	}

	basePort := state.PortRange.PortRangeStart
	networkID := *state.NetworkID
	genesisPath := fmt.Sprintf("%s/devnet/genesis.json", config.HomeDir)

	// Start containers sequentially for better error tracking
	for i := 0; i < config.ValidatorCount; i++ {
		o.logger.Info("Starting container %d/%d", i+1, config.ValidatorCount)

		// Calculate ports for this node
		portOffset := i * 100
		nodePorts := &node.NodePorts{
			RPC:    basePort + portOffset,
			P2P:    basePort + portOffset + 1,
			GRPC:   basePort + portOffset + 2,
			EVMRPC: basePort + portOffset + 3,
			EVMWS:  basePort + portOffset + 4,
			PProf:  basePort + portOffset + 5,
		}

		// Create node structure
		nodeHomeDir := fmt.Sprintf("%s/devnet/node%d", config.HomeDir, i)
		n := &node.Node{
			Index:   i,
			Name:    fmt.Sprintf("node%d", i),
			HomeDir: nodeHomeDir,
			Ports:   *nodePorts,
			Status:  node.NodeStatusStopped,
		}

		// Configure container
		containerConfig := &node.ContainerConfig{
			Node:        n,
			GenesisPath: genesisPath,
			NetworkID:   networkID,
		}

		// Add resource limits if specified
		if config.ResourceLimits != nil {
			containerConfig.ResourceLimits = &node.ResourceLimits{
				Memory: config.ResourceLimits.Memory,
				CPUs:   config.ResourceLimits.CPUs,
			}
		}

		// Start container
		if err := o.dockerManager.StartWithConfig(ctx, containerConfig); err != nil {
			return fmt.Errorf("failed to start container for node%d: %w", i, err)
		}

		// Track started container
		state.StartedContainers = append(state.StartedContainers, n.ContainerID)
		o.logger.Info("Container started: %s (node%d)", n.ContainerID, i)
	}

	o.logger.Info("All %d containers started successfully", config.ValidatorCount)
	return nil
}

// finalizeDeployment creates the deployment result
func (o *OrchestratorImpl) finalizeDeployment(ctx context.Context, config *ports.DeploymentConfig, state *ports.DeploymentState) *ports.DeploymentResult {
	state.Phase = ports.PhaseRunning
	duration := time.Since(state.StartedAt)

	o.logger.Info("Deployment completed successfully in %v", duration)

	// Get subnet from network
	subnet := ""
	if state.NetworkID != nil && *state.NetworkID != "" {
		subnet, _ = o.networkManager.GetNetworkSubnet(ctx, *state.NetworkID)
	}

	// Build container info from started containers
	containers := make([]*ports.ContainerInfo, len(state.StartedContainers))
	basePort := state.PortRange.PortRangeStart
	for i, containerID := range state.StartedContainers {
		portOffset := i * 100
		containers[i] = &ports.ContainerInfo{
			ID:        containerID,
			Name:      fmt.Sprintf("devnet-%s-node%d", config.DevnetName, i),
			NodeIndex: i,
			Ports: &ports.PortAssignment{
				RPC:    basePort + portOffset,
				P2P:    basePort + portOffset + 1,
				GRPC:   basePort + portOffset + 2,
				EVMRPC: basePort + portOffset + 3,
				EVMWS:  basePort + portOffset + 4,
			},
			HealthStatus: "starting",
		}
	}

	return &ports.DeploymentResult{
		DevnetName:     config.DevnetName,
		NetworkID:      *state.NetworkID,
		Subnet:         subnet,
		Containers:     containers,
		PortAllocation: state.PortRange,
		Duration:       duration,
		Success:        true,
	}
}

// handleFailure handles deployment failure and triggers rollback
func (o *OrchestratorImpl) handleFailure(ctx context.Context, state *ports.DeploymentState, err error) error {
	state.Errors = append(state.Errors, ports.DeploymentError{
		Phase:     state.Phase,
		Error:     err,
		Timestamp: time.Now(),
	})

	o.logger.Error("Deployment failed at phase %s: %v", state.Phase, err)

	if rollbackErr := o.Rollback(ctx, state); rollbackErr != nil {
		o.logger.Error("Rollback also failed: %v", rollbackErr)
		return fmt.Errorf("deployment failed: %w (rollback also failed: %v)", err, rollbackErr)
	}

	state.Phase = ports.PhaseFailed
	return fmt.Errorf("deployment failed: %w", err)
}

// MultiError represents multiple errors
type MultiError struct {
	Errors []error
}

func (m *MultiError) Error() string {
	var msgs []string
	for _, err := range m.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("multiple errors: %s", strings.Join(msgs, "; "))
}
