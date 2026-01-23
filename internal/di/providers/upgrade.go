package providers

import (
	"context"
	"fmt"
	"sync"

	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/application/upgrade"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/persistence"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

// UpgradeUseCasesProvider provides access to upgrade-related use cases.
// This provider groups all use cases related to chain upgrades:
// proposing, voting, switching binaries, executing upgrades, and monitoring.
type UpgradeUseCasesProvider interface {
	ProposeUseCase() *upgrade.ProposeUseCase
	VoteUseCase() *upgrade.VoteUseCase
	SwitchBinaryUseCase() *upgrade.SwitchBinaryUseCase
	ExecuteUpgradeUseCase(devnetProvider DevnetUseCasesProvider) *upgrade.ExecuteUpgradeUseCase
	MonitorUseCase() *upgrade.MonitorUseCase

	// State management for resumable upgrades
	StateManager() ports.UpgradeStateManager
	StateTransitioner() ports.UpgradeStateTransitioner
	StateDetector() ports.UpgradeStateDetector
	ResumableExecuteUseCase(devnetProvider DevnetUseCasesProvider) *upgrade.ResumableExecuteUpgradeUseCase
	ResumeUseCase(devnetProvider DevnetUseCasesProvider, logger *output.Logger) *upgrade.ResumeUseCase
}

// upgradeUseCases is the concrete implementation of UpgradeUseCasesProvider.
type upgradeUseCases struct {
	mu      sync.RWMutex
	infra   InfrastructureProvider
	homeDir string

	// Lazy-initialized use cases
	proposeUC        *upgrade.ProposeUseCase
	voteUC           *upgrade.VoteUseCase
	switchUC         *upgrade.SwitchBinaryUseCase
	executeUpgradeUC *upgrade.ExecuteUpgradeUseCase
	monitorUC        *upgrade.MonitorUseCase

	// State management (lazy-initialized)
	stateManager    ports.UpgradeStateManager
	transitioner    ports.UpgradeStateTransitioner
	stateDetector   ports.UpgradeStateDetector
	resumableExecUC *upgrade.ResumableExecuteUpgradeUseCase
	resumeUC        *upgrade.ResumeUseCase
}

// NewUpgradeUseCases creates a new UpgradeUseCasesProvider.
func NewUpgradeUseCases(infra InfrastructureProvider, homeDir string) UpgradeUseCasesProvider {
	return &upgradeUseCases{
		infra:   infra,
		homeDir: homeDir,
	}
}

// ProposeUseCase returns the propose use case (lazy init).
func (u *upgradeUseCases) ProposeUseCase() *upgrade.ProposeUseCase {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.proposeUC == nil {
		u.proposeUC = upgrade.NewProposeUseCase(
			u.infra.DevnetRepository(),
			u.infra.RPCClient(),
			u.infra.ValidatorKeyLoader(),
			u.infra.Logger(),
		)
	}
	return u.proposeUC
}

// VoteUseCase returns the vote use case (lazy init).
func (u *upgradeUseCases) VoteUseCase() *upgrade.VoteUseCase {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.voteUC == nil {
		u.voteUC = upgrade.NewVoteUseCase(
			u.infra.DevnetRepository(),
			u.infra.RPCClient(),
			u.infra.ValidatorKeyLoader(),
			u.infra.Logger(),
		)
	}
	return u.voteUC
}

// SwitchBinaryUseCase returns the switch binary use case (lazy init).
func (u *upgradeUseCases) SwitchBinaryUseCase() *upgrade.SwitchBinaryUseCase {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.switchUC == nil {
		u.switchUC = upgrade.NewSwitchBinaryUseCase(
			u.infra.DevnetRepository(),
			u.infra.NodeRepository(),
			u.infra.ProcessExecutor(),
			u.infra.BinaryCache(),
			u.infra.Logger(),
		)
	}
	return u.switchUC
}

// ExecuteUpgradeUseCase returns the execute upgrade use case (lazy init).
// It requires the DevnetUseCasesProvider to access the ExportUseCase.
func (u *upgradeUseCases) ExecuteUpgradeUseCase(devnetProvider DevnetUseCasesProvider) *upgrade.ExecuteUpgradeUseCase {
	// Get dependencies first without holding the lock to avoid deadlock
	ctx := context.Background()
	proposeUC := u.ProposeUseCase()
	voteUC := u.VoteUseCase()
	switchUC := u.SwitchBinaryUseCase()
	exportUC := devnetProvider.ExportUseCase(ctx)

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.executeUpgradeUC == nil {
		// Create export adapter to match ports interface
		exportAdapter := &exportUseCaseAdapter{concrete: exportUC}

		u.executeUpgradeUC = upgrade.NewExecuteUpgradeUseCase(
			proposeUC,
			voteUC,
			switchUC,
			exportAdapter,
			u.infra.RPCClient(),
			u.infra.DevnetRepository(),
			u.infra.HealthChecker(),
			u.infra.Logger(),
		)
	}
	return u.executeUpgradeUC
}

// MonitorUseCase returns the monitor use case (lazy init).
func (u *upgradeUseCases) MonitorUseCase() *upgrade.MonitorUseCase {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.monitorUC == nil {
		u.monitorUC = upgrade.NewMonitorUseCase(
			u.infra.RPCClient(),
			u.infra.Logger(),
		)
	}
	return u.monitorUC
}

// exportUseCaseAdapter adapts the concrete ExportUseCase to the ports interface.
type exportUseCaseAdapter struct {
	concrete interface {
		Execute(ctx context.Context, input dto.ExportInput) (*dto.ExportOutput, error)
	}
}

func (a *exportUseCaseAdapter) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	exportInput, ok := input.(dto.ExportInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type for export: expected dto.ExportInput, got %T", input)
	}
	return a.concrete.Execute(ctx, exportInput)
}

// StateManager returns the upgrade state manager (lazy init).
func (u *upgradeUseCases) StateManager() ports.UpgradeStateManager {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.stateManager == nil {
		u.stateManager = persistence.NewFileUpgradeStateManager(u.homeDir)
	}
	return u.stateManager
}

// StateTransitioner returns the state transitioner (lazy init).
func (u *upgradeUseCases) StateTransitioner() ports.UpgradeStateTransitioner {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.transitioner == nil {
		u.transitioner = upgrade.NewStateTransitioner()
	}
	return u.transitioner
}

// StateDetector returns the state detector (lazy init).
func (u *upgradeUseCases) StateDetector() ports.UpgradeStateDetector {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.stateDetector == nil {
		u.stateDetector = upgrade.NewStateDetector(u.infra.RPCClient())
	}
	return u.stateDetector
}

// ResumableExecuteUseCase returns the resumable execute use case (lazy init).
func (u *upgradeUseCases) ResumableExecuteUseCase(devnetProvider DevnetUseCasesProvider) *upgrade.ResumableExecuteUpgradeUseCase {
	// Get dependencies first without holding the lock
	ctx := context.Background()
	executeUC := u.ExecuteUpgradeUseCase(devnetProvider)
	proposeUC := u.ProposeUseCase()
	voteUC := u.VoteUseCase()
	switchUC := u.SwitchBinaryUseCase()
	stateManager := u.StateManager()
	transitioner := u.StateTransitioner()
	stateDetector := u.StateDetector()
	exportUC := devnetProvider.ExportUseCase(ctx)

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.resumableExecUC == nil {
		// Create export adapter to match ports interface
		exportAdapter := &exportUseCaseAdapter{concrete: exportUC}

		u.resumableExecUC = upgrade.NewResumableExecuteUpgradeUseCase(
			executeUC,
			proposeUC,
			voteUC,
			switchUC,
			stateManager,
			transitioner,
			stateDetector,
			u.infra.RPCClient(),
			exportAdapter,
			u.infra.DevnetRepository(),
			u.infra.Logger(),
		)
	}
	return u.resumableExecUC
}

// ResumeUseCase returns the resume use case (lazy init).
func (u *upgradeUseCases) ResumeUseCase(devnetProvider DevnetUseCasesProvider, logger *output.Logger) *upgrade.ResumeUseCase {
	// Get dependencies first without holding the lock
	stateManager := u.StateManager()
	stateDetector := u.StateDetector()
	transitioner := u.StateTransitioner()
	resumableExecUC := u.ResumableExecuteUseCase(devnetProvider)

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.resumeUC == nil {
		u.resumeUC = upgrade.NewResumeUseCase(
			stateManager,
			stateDetector,
			transitioner,
			resumableExecUC,
			logger,
		)
	}
	return u.resumeUC
}
