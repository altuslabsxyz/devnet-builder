// Package application provides the application layer services that orchestrate
// domain operations and coordinate between different components.
//
// This layer follows Clean Architecture principles:
//   - Services depend on interfaces, not implementations
//   - Business logic is coordinated here, not in cmd or infrastructure
//   - All external dependencies are injected
package application

import (
	"context"
	"time"

	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// Logger defines the logging interface for services.
type Logger interface {
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Success(format string, args ...interface{})
}

// DevnetServiceConfig holds configuration for DevnetService.
type DevnetServiceConfig struct {
	HomeDir string
	Logger  *output.Logger
}

// DevnetService orchestrates devnet lifecycle operations.
// It coordinates between provision, run, health, and reset services.
type DevnetService struct {
	config       DevnetServiceConfig
	provisionSvc *devnet.ProvisionService
	runSvc       *devnet.RunService
	healthSvc    *devnet.HealthService
	resetSvc     *devnet.ResetService
	logger       *output.Logger
}

// NewDevnetService creates a new DevnetService with the given configuration.
func NewDevnetService(config DevnetServiceConfig) *DevnetService {
	logger := config.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	return &DevnetService{
		config:       config,
		provisionSvc: devnet.NewProvisionService(logger),
		runSvc:       devnet.NewRunService(logger),
		healthSvc:    devnet.NewHealthService(logger),
		resetSvc:     devnet.NewResetService(logger),
		logger:       logger,
	}
}

// Provision creates a new devnet without starting nodes.
// This allows users to modify configuration before running.
func (s *DevnetService) Provision(ctx context.Context, opts devnet.ProvisionOptions) (*devnet.ProvisionResult, error) {
	return s.provisionSvc.Provision(ctx, opts)
}

// Run starts nodes from a provisioned devnet.
func (s *DevnetService) Run(ctx context.Context, opts devnet.RunOptions) (*devnet.RunResult, error) {
	return s.runSvc.Run(ctx, opts)
}

// Start provisions and starts a devnet in one operation.
func (s *DevnetService) Start(ctx context.Context, opts devnet.StartOptions) (*devnet.Devnet, error) {
	return s.runSvc.Start(ctx, opts)
}

// Stop stops all nodes in a devnet.
func (s *DevnetService) Stop(ctx context.Context, d *devnet.Devnet, timeout time.Duration) error {
	return d.Stop(ctx, timeout)
}

// GetHealth returns the health status of all nodes.
func (s *DevnetService) GetHealth(ctx context.Context, d *devnet.Devnet) []*node.NodeHealth {
	return s.healthSvc.CheckHealth(ctx, d.Nodes)
}

// SoftReset clears chain data but preserves genesis and configuration.
func (s *DevnetService) SoftReset(ctx context.Context, d *devnet.Devnet) error {
	return s.resetSvc.SoftReset(ctx, d)
}

// HardReset clears all data including genesis (requires re-provisioning).
func (s *DevnetService) HardReset(ctx context.Context, d *devnet.Devnet) error {
	return s.resetSvc.HardReset(ctx, d)
}

// LoadDevnet loads an existing devnet from disk.
func (s *DevnetService) LoadDevnet(homeDir string) (*devnet.Devnet, error) {
	return devnet.LoadDevnetWithNodes(homeDir, s.logger)
}

// DevnetExists checks if a devnet exists at the given path.
func (s *DevnetService) DevnetExists(homeDir string) bool {
	return devnet.DevnetExists(homeDir)
}
