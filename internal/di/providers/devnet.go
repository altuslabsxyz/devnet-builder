package providers

import (
	"context"
	"sync"
	"time"

	appdevnet "github.com/altuslabsxyz/devnet-builder/internal/application/devnet"
	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
)

// DevnetUseCasesProvider provides access to devnet-related use cases.
// This provider groups all use cases related to devnet lifecycle management:
// provisioning, running, stopping, health checking, resetting, destroying, and exporting.
type DevnetUseCasesProvider interface {
	ProvisionUseCase() *appdevnet.ProvisionUseCase
	RunUseCase() *appdevnet.RunUseCase
	StopUseCase() *appdevnet.StopUseCase
	HealthUseCase() *appdevnet.HealthUseCase
	ResetUseCase() *appdevnet.ResetUseCase
	DestroyUseCase() *appdevnet.DestroyUseCase
	ExportUseCase(ctx context.Context) *appdevnet.ExportUseCase

	// NodeLifecycleManager provides a focused interface for node lifecycle operations.
	// This follows the Interface Segregation Principle.
	NodeLifecycleManager() ports.NodeLifecycleManager
}

// devnetUseCases is the concrete implementation of DevnetUseCasesProvider.
type devnetUseCases struct {
	mu    sync.RWMutex
	infra InfrastructureProvider

	// Lazy-initialized use cases
	provisionUC *appdevnet.ProvisionUseCase
	runUC       *appdevnet.RunUseCase
	stopUC      *appdevnet.StopUseCase
	healthUC    *appdevnet.HealthUseCase
	resetUC     *appdevnet.ResetUseCase
	destroyUC   *appdevnet.DestroyUseCase
	exportUC    *appdevnet.ExportUseCase
}

// NewDevnetUseCases creates a new DevnetUseCasesProvider.
func NewDevnetUseCases(infra InfrastructureProvider) DevnetUseCasesProvider {
	return &devnetUseCases{
		infra: infra,
	}
}

// ProvisionUseCase returns the provision use case (lazy init).
func (d *devnetUseCases) ProvisionUseCase() *appdevnet.ProvisionUseCase {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.provisionUC == nil {
		d.provisionUC = appdevnet.NewProvisionUseCase(
			d.infra.DevnetRepository(),
			d.infra.NodeRepository(),
			d.infra.SnapshotFetcher(),
			d.infra.GenesisFetcher(),
			d.infra.StateExportService(),
			d.infra.NodeInitializer(),
			d.infra.NetworkModule(),
			d.infra.Logger(),
		)
	}
	return d.provisionUC
}

// RunUseCase returns the run use case (lazy init).
func (d *devnetUseCases) RunUseCase() *appdevnet.RunUseCase {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.runUC == nil {
		d.runUC = appdevnet.NewRunUseCase(
			d.infra.DevnetRepository(),
			d.infra.NodeRepository(),
			d.infra.ProcessExecutor(),
			d.infra.HealthChecker(),
			d.infra.NetworkModule(),
			d.infra.Logger(),
		)
	}
	return d.runUC
}

// StopUseCase returns the stop use case (lazy init).
func (d *devnetUseCases) StopUseCase() *appdevnet.StopUseCase {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopUC == nil {
		d.stopUC = appdevnet.NewStopUseCase(
			d.infra.DevnetRepository(),
			d.infra.NodeRepository(),
			d.infra.ProcessExecutor(),
			d.infra.Logger(),
		)
	}
	return d.stopUC
}

// HealthUseCase returns the health use case (lazy init).
func (d *devnetUseCases) HealthUseCase() *appdevnet.HealthUseCase {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.healthUC == nil {
		d.healthUC = appdevnet.NewHealthUseCase(
			d.infra.DevnetRepository(),
			d.infra.NodeRepository(),
			d.infra.HealthChecker(),
			d.infra.Logger(),
		)
	}
	return d.healthUC
}

// ResetUseCase returns the reset use case (lazy init).
func (d *devnetUseCases) ResetUseCase() *appdevnet.ResetUseCase {
	// Get StopUseCase first without holding the lock to avoid deadlock
	stopUC := d.StopUseCase()

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.resetUC == nil {
		d.resetUC = appdevnet.NewResetUseCase(
			d.infra.DevnetRepository(),
			d.infra.NodeRepository(),
			stopUC,
			d.infra.Logger(),
		)
	}
	return d.resetUC
}

// DestroyUseCase returns the destroy use case (lazy init).
func (d *devnetUseCases) DestroyUseCase() *appdevnet.DestroyUseCase {
	// Get StopUseCase first without holding the lock to avoid deadlock
	stopUC := d.StopUseCase()

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.destroyUC == nil {
		d.destroyUC = appdevnet.NewDestroyUseCase(
			d.infra.DevnetRepository(),
			stopUC,
			d.infra.Logger(),
		)
	}
	return d.destroyUC
}

// ExportUseCase returns the export use case (lazy init).
func (d *devnetUseCases) ExportUseCase(ctx context.Context) *appdevnet.ExportUseCase {
	// Get NodeLifecycleManager first without holding the lock to avoid deadlock
	nodeLifecycle := d.NodeLifecycleManager()

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.exportUC == nil {
		d.exportUC = appdevnet.NewExportUseCase(
			ctx,
			d.infra.DevnetRepository(),
			d.infra.NodeRepository(),
			d.infra.ExportRepository(),
			nodeLifecycle,
			d.infra.Logger(),
		)
	}
	return d.exportUC
}

// NodeLifecycleManager returns the node lifecycle manager adapter.
func (d *devnetUseCases) NodeLifecycleManager() ports.NodeLifecycleManager {
	// Get dependencies first without holding the lock to avoid deadlock
	stopUC := d.StopUseCase()
	runUC := d.RunUseCase()

	return &nodeLifecycleAdapter{
		stopUC: stopUC,
		runUC:  runUC,
	}
}

// nodeLifecycleAdapter implements ports.NodeLifecycleManager by delegating
// to StopUseCase and RunUseCase.
type nodeLifecycleAdapter struct {
	stopUC *appdevnet.StopUseCase
	runUC  *appdevnet.RunUseCase
}

// StopAll stops all running nodes with the given timeout.
func (a *nodeLifecycleAdapter) StopAll(ctx context.Context, homeDir string, timeout time.Duration) (int, error) {
	input := dto.StopInput{
		HomeDir: homeDir,
		Timeout: timeout,
	}
	result, err := a.stopUC.Execute(ctx, input)
	if err != nil {
		return 0, err
	}
	return result.StoppedNodes, nil
}

// StartAll starts all nodes with the given timeout.
func (a *nodeLifecycleAdapter) StartAll(ctx context.Context, homeDir string, timeout time.Duration) (int, bool, error) {
	input := dto.RunInput{
		HomeDir:     homeDir,
		WaitForSync: false,
		Timeout:     timeout,
	}
	result, err := a.runUC.Execute(ctx, input)
	if err != nil {
		return 0, false, err
	}
	return len(result.Nodes), result.AllRunning, nil
}
