package providers

import (
	"context"
	"fmt"
	"sync"

	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/upgrade"
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
}

// upgradeUseCases is the concrete implementation of UpgradeUseCasesProvider.
type upgradeUseCases struct {
	mu    sync.RWMutex
	infra InfrastructureProvider

	// Lazy-initialized use cases
	proposeUC        *upgrade.ProposeUseCase
	voteUC           *upgrade.VoteUseCase
	switchUC         *upgrade.SwitchBinaryUseCase
	executeUpgradeUC *upgrade.ExecuteUpgradeUseCase
	monitorUC        *upgrade.MonitorUseCase
}

// NewUpgradeUseCases creates a new UpgradeUseCasesProvider.
func NewUpgradeUseCases(infra InfrastructureProvider) UpgradeUseCasesProvider {
	return &upgradeUseCases{
		infra: infra,
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
