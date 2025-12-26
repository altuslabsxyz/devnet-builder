package docker

import "errors"

// Orchestrator errors
var (
	ErrValidationFailed           = errors.New("deployment configuration validation failed")
	ErrPortAllocationFailed       = errors.New("failed to allocate port range")
	ErrPortConflictDetected       = errors.New("allocated ports conflict with host services")
	ErrImagePullFailed            = errors.New("failed to pull Docker image")
	ErrCustomBuildFailed          = errors.New("failed to build custom Docker image")
	ErrContainerStartFailed       = errors.New("failed to start container")
	ErrHealthCheckTimeout         = errors.New("container health check timeout")
	ErrPartialDeployment          = errors.New("partial deployment (some containers failed)")
	ErrOrchestratorRollbackFailed = errors.New("rollback encountered errors")
)

// NetworkManager errors
var (
	ErrDockerDaemonUnavailable = errors.New("docker daemon not accessible")
	ErrInvalidDevnetName       = errors.New("devnet name contains invalid characters")
	ErrNoAvailableSubnets      = errors.New("no available subnets in 172.x.0.0/16 range")
	ErrNetworkCreationFailed   = errors.New("docker network create command failed")
	ErrNetworkNotFound         = errors.New("docker network not found")
	ErrNetworkHasContainers    = errors.New("network has attached containers, cannot delete")
	ErrNetworkDeletionFailed   = errors.New("docker network rm command failed")
)
