package devnet

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	domainExport "github.com/altuslabsxyz/devnet-builder/internal/domain/export"
	infraExport "github.com/altuslabsxyz/devnet-builder/internal/infrastructure/export"
	"github.com/altuslabsxyz/devnet-builder/types"
)

// Mock implementations

type mockDevnetRepository struct {
	loadFunc func(ctx context.Context, homeDir string) (*ports.DevnetMetadata, error)
}

func (m *mockDevnetRepository) Load(ctx context.Context, homeDir string) (*ports.DevnetMetadata, error) {
	if m.loadFunc != nil {
		return m.loadFunc(ctx, homeDir)
	}
	return &ports.DevnetMetadata{
		ChainID:        "test-chain-1",
		NetworkName:    "testnet",
		NumValidators:  4,
		NumAccounts:    10,
		ExecutionMode:  types.ExecutionModeLocal,
		HomeDir:        homeDir,
		BinaryName:     "testd",
		CurrentVersion: "v1.0.0",
	}, nil
}

func (m *mockDevnetRepository) Save(ctx context.Context, metadata *ports.DevnetMetadata) error {
	return nil
}

func (m *mockDevnetRepository) Delete(ctx context.Context, homeDir string) error {
	return nil
}

func (m *mockDevnetRepository) Exists(homeDir string) bool {
	return true
}

type mockNodeRepository struct {
	loadAllFunc func(ctx context.Context, homeDir string) ([]*ports.NodeMetadata, error)
}

func (m *mockNodeRepository) LoadAll(ctx context.Context, homeDir string) ([]*ports.NodeMetadata, error) {
	if m.loadAllFunc != nil {
		return m.loadAllFunc(ctx, homeDir)
	}
	// Return nodes with PID (running)
	pid := 12345
	return []*ports.NodeMetadata{
		{
			Name: "node0",
			PID:  &pid,
			Ports: ports.PortConfig{
				RPC: 26657,
			},
		},
	}, nil
}

func (m *mockNodeRepository) Load(ctx context.Context, homeDir string, index int) (*ports.NodeMetadata, error) {
	return nil, nil
}

func (m *mockNodeRepository) Save(ctx context.Context, node *ports.NodeMetadata) error {
	return nil
}

func (m *mockNodeRepository) Delete(ctx context.Context, homeDir string, index int) error {
	return nil
}

type mockExportRepository struct {
	saveFunc          func(ctx context.Context, export interface{}) error
	listForDevnetFunc func(ctx context.Context, homeDir string) (interface{}, error)
	validateFunc      func(ctx context.Context, exportPath string) (interface{}, error)
}

func (m *mockExportRepository) Save(ctx context.Context, export interface{}) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, export)
	}
	return nil
}

func (m *mockExportRepository) Load(ctx context.Context, exportPath string) (interface{}, error) {
	return nil, nil
}

func (m *mockExportRepository) ListForDevnet(ctx context.Context, homeDir string) (interface{}, error) {
	if m.listForDevnetFunc != nil {
		return m.listForDevnetFunc(ctx, homeDir)
	}
	return []*domainExport.Export{}, nil
}

func (m *mockExportRepository) Delete(ctx context.Context, exportPath string) error {
	return nil
}

func (m *mockExportRepository) Validate(ctx context.Context, exportPath string) (interface{}, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, exportPath)
	}
	return &infraExport.ValidationResult{
		IsComplete: true,
	}, nil
}

type mockLogger struct{}

func (m *mockLogger) Debug(format string, args ...interface{})   {}
func (m *mockLogger) Info(format string, args ...interface{})    {}
func (m *mockLogger) Warn(format string, args ...interface{})    {}
func (m *mockLogger) Error(format string, args ...interface{})   {}
func (m *mockLogger) Fatal(format string, args ...interface{})   {}
func (m *mockLogger) Success(format string, args ...interface{}) {}
func (m *mockLogger) Print(format string, args ...interface{})   {}
func (m *mockLogger) Println(format string, args ...interface{}) {}
func (m *mockLogger) Writer() io.Writer                          { return os.Stdout }
func (m *mockLogger) ErrWriter() io.Writer                       { return os.Stderr }
func (m *mockLogger) IsVerbose() bool                            { return false }
func (m *mockLogger) SetVerbose(verbose bool)                    {}

// mockNodeLifecycleManager implements ports.NodeLifecycleManager for testing.
type mockNodeLifecycleManager struct {
	stopAllFunc  func(ctx context.Context, homeDir string, timeout time.Duration) (int, error)
	startAllFunc func(ctx context.Context, homeDir string, timeout time.Duration) (int, bool, error)
}

func (m *mockNodeLifecycleManager) StopAll(ctx context.Context, homeDir string, timeout time.Duration) (int, error) {
	if m.stopAllFunc != nil {
		return m.stopAllFunc(ctx, homeDir, timeout)
	}
	// Default: successfully stop 1 node
	return 1, nil
}

func (m *mockNodeLifecycleManager) StartAll(ctx context.Context, homeDir string, timeout time.Duration) (int, bool, error) {
	if m.startAllFunc != nil {
		return m.startAllFunc(ctx, homeDir, timeout)
	}
	// Default: successfully start 1 node
	return 1, true, nil
}

// Tests

func TestNewExportUseCase(t *testing.T) {
	devnetRepo := &mockDevnetRepository{}
	nodeRepo := &mockNodeRepository{}
	exportRepo := &mockExportRepository{}
	nodeLifecycle := &mockNodeLifecycleManager{}
	logger := &mockLogger{}

	uc := NewExportUseCase(context.Background(), devnetRepo, nodeRepo, exportRepo, nodeLifecycle, logger)

	if uc == nil {
		t.Fatal("expected non-nil ExportUseCase")
	}

	if uc.devnetRepo == nil {
		t.Error("expected non-nil devnetRepo")
	}

	if uc.nodeRepo == nil {
		t.Error("expected non-nil nodeRepo")
	}

	if uc.exportRepo == nil {
		t.Error("expected non-nil exportRepo")
	}

	if uc.nodeLifecycle == nil {
		t.Error("expected non-nil nodeLifecycle")
	}

	if uc.hashCalc == nil {
		t.Error("expected non-nil hashCalc")
	}

	if uc.heightResolver == nil {
		t.Error("expected non-nil heightResolver")
	}

	if uc.exportExec == nil {
		t.Error("expected non-nil exportExec")
	}

	if uc.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestExportUseCase_Execute_DevnetLoadFailure(t *testing.T) {
	devnetRepo := &mockDevnetRepository{
		loadFunc: func(ctx context.Context, homeDir string) (*ports.DevnetMetadata, error) {
			return nil, errors.New("failed to load devnet")
		},
	}

	uc := NewExportUseCase(context.Background(), devnetRepo, &mockNodeRepository{}, &mockExportRepository{}, &mockNodeLifecycleManager{}, &mockLogger{})

	input := dto.ExportInput{
		HomeDir: "/tmp/test-devnet",
	}

	_, err := uc.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("expected error for devnet load failure")
	}

	if err.Error() != "failed to load devnet: failed to load devnet" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExportUseCase_Execute_NodeLoadFailure(t *testing.T) {
	nodeRepo := &mockNodeRepository{
		loadAllFunc: func(ctx context.Context, homeDir string) ([]*ports.NodeMetadata, error) {
			return nil, errors.New("failed to load nodes")
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, nodeRepo, &mockExportRepository{}, &mockNodeLifecycleManager{}, &mockLogger{})

	input := dto.ExportInput{
		HomeDir: "/tmp/test-devnet",
	}

	_, err := uc.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("expected error for node load failure")
	}
}

func TestExportUseCase_Execute_DevnetNotRunning(t *testing.T) {
	nodeRepo := &mockNodeRepository{
		loadAllFunc: func(ctx context.Context, homeDir string) ([]*ports.NodeMetadata, error) {
			// Return nodes with no PID (not running)
			return []*ports.NodeMetadata{
				{
					Name: "node0",
					PID:  nil,
				},
			}, nil
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, nodeRepo, &mockExportRepository{}, &mockNodeLifecycleManager{}, &mockLogger{})

	input := dto.ExportInput{
		HomeDir: "/tmp/test-devnet",
	}

	_, err := uc.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("expected error when devnet is not running")
	}

	if err.Error() != "devnet is not running; cannot determine block height" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExportUseCase_List_Success(t *testing.T) {
	exportRepo := &mockExportRepository{
		listForDevnetFunc: func(ctx context.Context, homeDir string) (interface{}, error) {
			return []*domainExport.Export{}, nil
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, &mockNodeRepository{}, exportRepo, &mockNodeLifecycleManager{}, &mockLogger{})

	output, err := uc.List(context.Background(), "/tmp/test-devnet")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == nil {
		t.Fatal("expected non-nil output")
	}

	if output.TotalCount != 0 {
		t.Errorf("expected count 0, got %d", output.TotalCount)
	}
}

func TestExportUseCase_List_RepositoryFailure(t *testing.T) {
	exportRepo := &mockExportRepository{
		listForDevnetFunc: func(ctx context.Context, homeDir string) (interface{}, error) {
			return nil, errors.New("repository error")
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, &mockNodeRepository{}, exportRepo, &mockNodeLifecycleManager{}, &mockLogger{})

	_, err := uc.List(context.Background(), "/tmp/test-devnet")

	if err == nil {
		t.Fatal("expected error for repository failure")
	}
}

func TestExportUseCase_List_InvalidType(t *testing.T) {
	exportRepo := &mockExportRepository{
		listForDevnetFunc: func(ctx context.Context, homeDir string) (interface{}, error) {
			// Return wrong type
			return "invalid", nil
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, &mockNodeRepository{}, exportRepo, &mockNodeLifecycleManager{}, &mockLogger{})

	_, err := uc.List(context.Background(), "/tmp/test-devnet")

	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestExportUseCase_Inspect_Success(t *testing.T) {
	// Create valid test export
	hash := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
	binaryInfo, _ := domainExport.NewBinaryInfo("/usr/bin/testd", "", hash, "v1.0.0", types.ExecutionModeLocal)
	metadata, _ := domainExport.NewExportMetadata(
		testTimestamp(),
		1000000,
		"testnet",
		0,
		"/usr/bin/testd",
		hash,
		"v1.0.0",
		"",
		"test-chain-1",
		4,
		10,
		"local",
		"/tmp/test-devnet",
	)
	export, _ := domainExport.NewExport(
		"/tmp/exports/testnet-a1b2c3d4-1000000-20240115120000",
		testTimestamp(),
		1000000,
		"testnet",
		binaryInfo,
		metadata,
		"/tmp/exports/testnet-a1b2c3d4-1000000-20240115120000/genesis.json",
	)

	exportRepo := &mockExportRepository{
		validateFunc: func(ctx context.Context, exportPath string) (interface{}, error) {
			return &infraExport.ValidationResult{
				Export:       export,
				IsComplete:   true,
				MissingFiles: []string{},
			}, nil
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, &mockNodeRepository{}, exportRepo, &mockNodeLifecycleManager{}, &mockLogger{})

	output, err := uc.Inspect(context.Background(), "/tmp/exports/testnet-a1b2c3d4-1000000-20240115120000")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == nil {
		t.Fatal("expected non-nil output")
	}

	if !output.IsComplete {
		t.Error("expected export to be complete")
	}
}

func TestExportUseCase_Inspect_ValidationFailure(t *testing.T) {
	exportRepo := &mockExportRepository{
		validateFunc: func(ctx context.Context, exportPath string) (interface{}, error) {
			return nil, errors.New("validation error")
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, &mockNodeRepository{}, exportRepo, &mockNodeLifecycleManager{}, &mockLogger{})

	_, err := uc.Inspect(context.Background(), "/tmp/exports/invalid")

	if err == nil {
		t.Fatal("expected error for validation failure")
	}
}

func TestExportUseCase_Inspect_IncompleteExport(t *testing.T) {
	hash := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
	binaryInfo, _ := domainExport.NewBinaryInfo("/usr/bin/testd", "", hash, "v1.0.0", types.ExecutionModeLocal)
	metadata, _ := domainExport.NewExportMetadata(
		testTimestamp(),
		1000000,
		"testnet",
		0,
		"/usr/bin/testd",
		hash,
		"v1.0.0",
		"",
		"test-chain-1",
		4,
		10,
		"local",
		"/tmp/test-devnet",
	)
	export, _ := domainExport.NewExport(
		"/tmp/exports/testnet-a1b2c3d4-1000000-20240115120000",
		testTimestamp(),
		1000000,
		"testnet",
		binaryInfo,
		metadata,
		"/tmp/exports/testnet-a1b2c3d4-1000000-20240115120000/genesis.json",
	)

	exportRepo := &mockExportRepository{
		validateFunc: func(ctx context.Context, exportPath string) (interface{}, error) {
			return &infraExport.ValidationResult{
				Export:       export,
				IsComplete:   false,
				MissingFiles: []string{"genesis.json"},
			}, domainExport.ErrExportIncomplete
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, &mockNodeRepository{}, exportRepo, &mockNodeLifecycleManager{}, &mockLogger{})

	output, err := uc.Inspect(context.Background(), "/tmp/exports/testnet-a1b2c3d4-1000000-20240115120000")

	// Should not return error for incomplete export (handled gracefully)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.IsComplete {
		t.Error("expected export to be incomplete")
	}

	if len(output.MissingFiles) != 1 {
		t.Errorf("expected 1 missing file, got %d", len(output.MissingFiles))
	}
}

func TestExportUseCase_Inspect_InvalidType(t *testing.T) {
	exportRepo := &mockExportRepository{
		validateFunc: func(ctx context.Context, exportPath string) (interface{}, error) {
			// Return wrong type
			return "invalid", nil
		},
	}

	uc := NewExportUseCase(context.Background(), &mockDevnetRepository{}, &mockNodeRepository{}, exportRepo, &mockNodeLifecycleManager{}, &mockLogger{})

	_, err := uc.Inspect(context.Background(), "/tmp/exports/invalid")

	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

// Helper functions

func testTimestamp() time.Time {
	t, _ := time.Parse("20060102150405", "20240115120000")
	return t
}
