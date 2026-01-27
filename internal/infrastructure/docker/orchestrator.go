package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/domain/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/node"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

// OrchestratorImpl implements the DeploymentOrchestrator interface
type OrchestratorImpl struct {
	networkManager ports.NetworkManager
	portAllocator  ports.PortAllocator
	dockerManager  *node.DockerManager
	stateDir       string // Directory for storing deployment state files
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

	// Default state directory: ~/.devnet-builder/state
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".devnet-builder", "state")

	return &OrchestratorImpl{
		networkManager: networkManager,
		portAllocator:  portAllocator,
		dockerManager:  dockerManager,
		stateDir:       stateDir,
		logger:         logger,
	}
}

// NewOrchestratorWithStateDir creates a new deployment orchestrator with a custom state directory
func NewOrchestratorWithStateDir(
	networkManager ports.NetworkManager,
	portAllocator ports.PortAllocator,
	dockerManager *node.DockerManager,
	stateDir string,
	logger *output.Logger,
) *OrchestratorImpl {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &OrchestratorImpl{
		networkManager: networkManager,
		portAllocator:  portAllocator,
		dockerManager:  dockerManager,
		stateDir:       stateDir,
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

	// Execute each phase sequentially, persisting state after each
	if err := o.phaseValidate(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}
	_ = o.saveState(state) // Best-effort state persistence

	if err := o.phaseCreateNetwork(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}
	_ = o.saveState(state)

	if err := o.phaseAllocatePorts(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}
	_ = o.saveState(state)

	if err := o.phasePullImage(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}
	_ = o.saveState(state)

	if err := o.phaseStartContainers(ctx, config, state); err != nil {
		return nil, o.handleFailure(ctx, state, err)
	}
	_ = o.saveState(state)

	// Health checking phase would go here (not implemented yet)

	// Finalize deployment and persist final state
	result := o.finalizeDeployment(ctx, config, state)
	_ = o.saveState(state)
	return result, nil
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

	// Clean up state file after successful rollback
	_ = o.deleteState(state.DevnetName)

	o.logger.Info("Rollback completed successfully for %s", state.DevnetName)
	return nil
}

// GetState retrieves current deployment state for a devnet
func (o *OrchestratorImpl) GetState(ctx context.Context, devnetName string) (*ports.DeploymentState, error) {
	state, err := o.loadState(devnetName)
	if err != nil {
		// If file doesn't exist, return nil (no active deployment)
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	return state, nil
}

// stateFilePath returns the path to the state file for a devnet
func (o *OrchestratorImpl) stateFilePath(devnetName string) string {
	return filepath.Join(o.stateDir, fmt.Sprintf("%s.json", devnetName))
}

// persistedState is the JSON-serializable representation of DeploymentState
type persistedState struct {
	DevnetName        string                `json:"devnet_name"`
	Phase             ports.DeploymentPhase `json:"phase"`
	NetworkID         *string               `json:"network_id,omitempty"`
	PortRange         *ports.PortAllocation `json:"port_range,omitempty"`
	StartedContainers []string              `json:"started_containers"`
	HealthyContainers []string              `json:"healthy_containers"`
	StartedAt         time.Time             `json:"started_at"`
}

// saveState persists the deployment state to a JSON file
func (o *OrchestratorImpl) saveState(state *ports.DeploymentState) error {
	if state == nil {
		return nil
	}

	// Ensure state directory exists
	if err := os.MkdirAll(o.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Convert to persisted format (errors are not easily serializable)
	ps := persistedState{
		DevnetName:        state.DevnetName,
		Phase:             state.Phase,
		NetworkID:         state.NetworkID,
		PortRange:         state.PortRange,
		StartedContainers: state.StartedContainers,
		HealthyContainers: state.HealthyContainers,
		StartedAt:         state.StartedAt,
	}

	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	statePath := o.stateFilePath(state.DevnetName)
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	o.logger.Debug("State saved to %s", statePath)
	return nil
}

// loadState loads deployment state from a JSON file
func (o *OrchestratorImpl) loadState(devnetName string) (*ports.DeploymentState, error) {
	statePath := o.stateFilePath(devnetName)

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err // Returns os.IsNotExist if file doesn't exist
	}

	var ps persistedState
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Convert back to DeploymentState
	state := &ports.DeploymentState{
		DevnetName:        ps.DevnetName,
		Phase:             ps.Phase,
		NetworkID:         ps.NetworkID,
		PortRange:         ps.PortRange,
		StartedContainers: ps.StartedContainers,
		HealthyContainers: ps.HealthyContainers,
		Errors:            []ports.DeploymentError{}, // Errors are not persisted
		StartedAt:         ps.StartedAt,
	}

	o.logger.Debug("State loaded from %s", statePath)
	return state, nil
}

// deleteState removes the state file for a devnet
func (o *OrchestratorImpl) deleteState(devnetName string) error {
	statePath := o.stateFilePath(devnetName)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}
	o.logger.Debug("State deleted: %s", statePath)
	return nil
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

	// Handle custom build if specified
	if config.CustomBuild != nil {
		return o.buildCustomImage(ctx, config)
	}

	// Standard image pull
	o.logger.Info("Pulling Docker image: %s", config.Image)

	// Validate/pull image
	if err := o.dockerManager.ValidateImage(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrImagePullFailed, err)
	}

	o.logger.Info("Image ready: %s", config.Image)
	return nil
}

// buildCustomImage builds a Docker image from source
func (o *OrchestratorImpl) buildCustomImage(ctx context.Context, config *ports.DeploymentConfig) error {
	cb := config.CustomBuild

	// Generate image tag based on devnet name
	imageTag := fmt.Sprintf("devnet-%s:custom", config.DevnetName)
	o.logger.Info("Building custom Docker image: %s", imageTag)

	// Validate plugin path exists
	if cb.PluginPath == "" {
		return fmt.Errorf("%w: plugin path is required", ErrCustomBuildFailed)
	}

	if _, err := os.Stat(cb.PluginPath); os.IsNotExist(err) {
		return fmt.Errorf("%w: plugin path does not exist: %s", ErrCustomBuildFailed, cb.PluginPath)
	}

	// Build docker build command args
	args := []string{"build", "-t", imageTag}

	// Add build args if specified
	for key, value := range cb.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	// Add chain binary as build arg if specified
	if cb.ChainBinary != "" {
		args = append(args, "--build-arg", fmt.Sprintf("CHAIN_BINARY=%s", cb.ChainBinary))
	}

	// Add context path
	args = append(args, cb.PluginPath)

	o.logger.Debug("Running: docker %s", strings.Join(args, " "))

	// Run docker build
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = cb.PluginPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		o.logger.Error("Docker build failed: %s", string(output))
		return fmt.Errorf("%w: docker build failed: %v", ErrCustomBuildFailed, err)
	}

	o.logger.Debug("Build output: %s", string(output))
	o.logger.Info("Custom image built successfully: %s", imageTag)

	// Update the docker manager to use the built image
	o.dockerManager.Image = imageTag

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
