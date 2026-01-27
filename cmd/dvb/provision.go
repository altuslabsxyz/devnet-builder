// cmd/dvb/provision.go
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/builder"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// provisionOptions holds options for the provision command
type provisionOptions struct {
	name        string
	network     string
	chainID     string
	validators  int
	fullNodes   int
	binaryPath  string
	dataDir     string
	useMocks    bool // Use mock implementations (for testing/demo)
	interactive bool // Use interactive wizard mode
}

func newProvisionCmd() *cobra.Command {
	opts := &provisionOptions{}

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision a new devnet",
		Long: `Provision a new devnet using the ProvisioningOrchestrator.

This command provisions a devnet in standalone mode without requiring the daemon.
It coordinates the full provisioning flow: building binary (if needed), forking
genesis, initializing node directories, and starting node processes.

Use -i/--interactive for a guided wizard experience.

Examples:
  # Interactive wizard mode (recommended for first-time users)
  dvb provision -i

  # Provision a devnet with default settings
  dvb provision --name my-devnet

  # Provision with custom chain ID and 4 validators
  dvb provision --name my-devnet --chain-id my-chain-1 --validators 4

  # Provision using a pre-built binary
  dvb provision --name my-devnet --binary-path /usr/local/bin/stabled

  # Provision with 3 validators and 2 full nodes
  dvb provision --name my-devnet --validators 3 --full-nodes 2

  # Provision with custom data directory
  dvb provision --name my-devnet --data-dir /path/to/devnets`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Interactive wizard mode
			if opts.interactive {
				wizardOpts, err := RunProvisionWizard()
				if err != nil {
					if err.Error() == "cancelled" {
						return nil
					}
					return err
				}
				if wizardOpts == nil {
					return nil // User cancelled
				}
				// Transfer wizard options to provision options
				opts.name = wizardOpts.Name
				opts.network = wizardOpts.Network
				opts.chainID = wizardOpts.ChainID
				opts.validators = wizardOpts.Validators
				opts.fullNodes = wizardOpts.FullNodes
				opts.binaryPath = wizardOpts.BinaryPath
				opts.dataDir = wizardOpts.DataDir
			}
			return runProvision(cmd.Context(), opts)
		},
	}

	// Interactive mode flag
	cmd.Flags().BoolVarP(&opts.interactive, "interactive", "i", false, "Use interactive wizard mode")

	// Name flag (required unless in interactive mode)
	cmd.Flags().StringVar(&opts.name, "name", "", "Devnet name (required unless using -i)")

	// Optional flags with defaults
	cmd.Flags().StringVar(&opts.network, "network", "stable", "Plugin/network name (e.g., stable, cosmos)")
	cmd.Flags().StringVar(&opts.chainID, "chain-id", "", "Chain ID (default: <name>-devnet)")
	cmd.Flags().IntVar(&opts.validators, "validators", 1, "Number of validators")
	cmd.Flags().IntVar(&opts.fullNodes, "full-nodes", 0, "Number of full nodes")
	cmd.Flags().StringVar(&opts.binaryPath, "binary-path", "", "Path to chain binary (skips build if provided)")
	cmd.Flags().StringVar(&opts.dataDir, "data-dir", "", "Base data directory (default: ~/.devnet-builder)")
	cmd.Flags().BoolVar(&opts.useMocks, "mocks", false, "Use mock implementations (for testing/demo without real binaries)")

	return cmd
}

func runProvision(ctx context.Context, opts *provisionOptions) error {
	// Validate options
	if opts.name == "" {
		return fmt.Errorf("--name is required (or use -i for interactive mode)")
	}
	if opts.validators < 1 {
		return fmt.Errorf("--validators must be at least 1")
	}
	if opts.fullNodes < 0 {
		return fmt.Errorf("--full-nodes cannot be negative")
	}

	// Determine chain ID
	chainID := opts.chainID
	if chainID == "" {
		chainID = fmt.Sprintf("%s-devnet", opts.name)
	}

	// Determine data directory
	dataDir := opts.dataDir
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".devnet-builder")
	}

	// Create devnet-specific data directory
	devnetDataDir := filepath.Join(dataDir, "devnets", opts.name)
	if err := os.MkdirAll(devnetDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Print provisioning info
	fmt.Fprintf(os.Stderr, "Provisioning devnet...\n")
	fmt.Fprintf(os.Stderr, "  Name:       %s\n", opts.name)
	fmt.Fprintf(os.Stderr, "  Network:    %s\n", opts.network)
	fmt.Fprintf(os.Stderr, "  Chain ID:   %s\n", chainID)
	fmt.Fprintf(os.Stderr, "  Validators: %d\n", opts.validators)
	if opts.fullNodes > 0 {
		fmt.Fprintf(os.Stderr, "  Full Nodes: %d\n", opts.fullNodes)
	}
	if opts.binaryPath != "" {
		fmt.Fprintf(os.Stderr, "  Binary:     %s\n", opts.binaryPath)
	}
	fmt.Fprintf(os.Stderr, "  Data Dir:   %s\n", devnetDataDir)
	fmt.Fprintf(os.Stderr, "\n")

	// Create orchestrator - use real implementations by default, mocks only when --mocks flag is set
	var orch *provisioner.ProvisioningOrchestrator
	if opts.useMocks {
		// Use mock implementations (for testing/demo)
		fmt.Fprintf(os.Stderr, "  Mode:       mock (no real binaries)\n\n")
		mockBuilder := newStandaloneBinaryBuilder(opts.network, opts.binaryPath)
		mockForker := newStandaloneGenesisForker()
		mockInitializer := newStandaloneNodeInitializer()
		mockRuntime := newStandaloneNodeRuntime()

		config := provisioner.OrchestratorConfig{
			BinaryBuilder:   mockBuilder,
			GenesisForker:   mockForker,
			NodeInitializer: mockInitializer,
			NodeRuntime:     mockRuntime,
			DataDir:         devnetDataDir,
			Logger:          logger,
		}
		orch = provisioner.NewProvisioningOrchestrator(config)
	} else {
		// Use real implementations via wiring layer
		var err error
		orch, err = CreateOrchestrator(OrchestratorOptions{
			Network:    opts.network,
			BinaryPath: opts.binaryPath,
			DataDir:    devnetDataDir,
			Logger:     logger,
		})
		if err != nil {
			return fmt.Errorf("failed to create orchestrator: %w", err)
		}
	}

	// Set up progress callback
	orch.OnProgress(func(phase provisioner.ProvisioningPhase, message string) {
		printPhaseProgress(phase, message)
	})

	// Build provision options
	provisionOpts := ports.ProvisionOptions{
		DevnetName:    opts.name,
		ChainID:       chainID,
		Network:       opts.network,
		NumValidators: opts.validators,
		NumFullNodes:  opts.fullNodes,
		BinaryPath:    opts.binaryPath,
		DataDir:       devnetDataDir,
		GenesisSource: plugintypes.GenesisSource{
			Mode:        plugintypes.GenesisModeRPC,
			NetworkType: "mainnet",
		},
		GenesisPatchOpts: plugintypes.DefaultDevnetPatchOptions(chainID),
	}

	// Execute provisioning
	result, err := orch.Execute(ctx, provisionOpts)
	if err != nil {
		color.Red("Provisioning failed: %v", err)
		return err
	}

	// Print success
	fmt.Fprintf(os.Stderr, "\n")
	color.Green("Devnet provisioned successfully!")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Devnet Name:   %s\n", result.DevnetName)
	fmt.Fprintf(os.Stderr, "  Chain ID:      %s\n", result.ChainID)
	fmt.Fprintf(os.Stderr, "  Binary Path:   %s\n", result.BinaryPath)
	fmt.Fprintf(os.Stderr, "  Genesis Path:  %s\n", result.GenesisPath)
	fmt.Fprintf(os.Stderr, "  Nodes:         %d (%d validators, %d full nodes)\n",
		result.NodeCount, result.ValidatorCount, result.FullNodeCount)
	fmt.Fprintf(os.Stderr, "  Data Dir:      %s\n", result.DataDir)

	return nil
}

// printPhaseProgress prints the current phase with colored output
func printPhaseProgress(phase provisioner.ProvisioningPhase, message string) {
	var prefix string
	switch phase {
	case provisioner.PhaseBuilding:
		prefix = color.YellowString("[Building]")
	case provisioner.PhaseForking:
		prefix = color.YellowString("[Forking]")
	case provisioner.PhaseInitializing:
		prefix = color.YellowString("[Initializing]")
	case provisioner.PhaseStarting:
		prefix = color.YellowString("[Starting]")
	case provisioner.PhaseHealthChecking:
		prefix = color.YellowString("[HealthCheck]")
	case provisioner.PhaseRunning:
		prefix = color.GreenString("[Running]")
	case provisioner.PhaseDegraded:
		prefix = color.YellowString("[Degraded]")
	case provisioner.PhaseFailed:
		prefix = color.RedString("[Failed]")
	default:
		prefix = fmt.Sprintf("[%s]", phase)
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, message)
}

// =============================================================================
// Standalone Mock Implementations
// =============================================================================

// standaloneBinaryBuilder is a placeholder binary builder for standalone mode
type standaloneBinaryBuilder struct {
	network    string
	binaryPath string
}

func newStandaloneBinaryBuilder(network, binaryPath string) *standaloneBinaryBuilder {
	return &standaloneBinaryBuilder{
		network:    network,
		binaryPath: binaryPath,
	}
}

func (b *standaloneBinaryBuilder) Build(ctx context.Context, spec builder.BuildSpec) (*builder.BuildResult, error) {
	// In standalone mode, we require a binary path to be provided
	// or we simulate finding the binary in PATH
	binaryName := b.getBinaryName()
	binaryPath := b.binaryPath
	if binaryPath == "" {
		// Try to find in PATH
		binaryPath = binaryName
	}

	return &builder.BuildResult{
		BinaryPath: binaryPath,
		GitCommit:  "standalone",
		GitRef:     "standalone",
		BuiltAt:    time.Now(),
	}, nil
}

func (b *standaloneBinaryBuilder) GetCached(ctx context.Context, spec builder.BuildSpec) (*builder.BuildResult, bool) {
	return nil, false
}

func (b *standaloneBinaryBuilder) Clean(ctx context.Context, maxAge time.Duration) error {
	return nil
}

func (b *standaloneBinaryBuilder) getBinaryName() string {
	switch b.network {
	case "stable":
		return "stabled"
	case "cosmos", "gaia":
		return "gaiad"
	default:
		return b.network + "d"
	}
}

// standaloneGenesisForker is a placeholder genesis forker for standalone mode
type standaloneGenesisForker struct{}

func newStandaloneGenesisForker() *standaloneGenesisForker {
	return &standaloneGenesisForker{}
}

func (f *standaloneGenesisForker) Fork(ctx context.Context, opts ports.ForkOptions) (*ports.ForkResult, error) {
	// Create a minimal genesis placeholder
	chainID := opts.PatchOpts.ChainID
	if chainID == "" {
		chainID = "devnet"
	}

	genesis := fmt.Sprintf(`{
  "chain_id": "%s",
  "genesis_time": "%s",
  "app_state": {}
}`, chainID, time.Now().UTC().Format(time.RFC3339))

	return &ports.ForkResult{
		Genesis:       []byte(genesis),
		SourceChainID: "placeholder",
		NewChainID:    chainID,
		SourceMode:    opts.Source.Mode,
		FetchedAt:     time.Now(),
	}, nil
}

// standaloneNodeInitializer is a placeholder node initializer for standalone mode
type standaloneNodeInitializer struct {
	initCalls []string
}

func newStandaloneNodeInitializer() *standaloneNodeInitializer {
	return &standaloneNodeInitializer{
		initCalls: make([]string, 0),
	}
}

func (i *standaloneNodeInitializer) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	i.initCalls = append(i.initCalls, moniker)

	// Create node directory structure
	dirs := []string{
		filepath.Join(nodeDir, "config"),
		filepath.Join(nodeDir, "data"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write placeholder config files
	configContent := fmt.Sprintf("# Config for %s\nmoniker = %q\nchain-id = %q\n", moniker, moniker, chainID)
	configPath := filepath.Join(nodeDir, "config", "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (i *standaloneNodeInitializer) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	// Generate a placeholder node ID
	return fmt.Sprintf("node_%s", strings.ReplaceAll(filepath.Base(nodeDir), "-", "_")), nil
}

func (i *standaloneNodeInitializer) CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	return nil, nil
}

func (i *standaloneNodeInitializer) CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*ports.AccountKeyInfo, error) {
	return nil, nil
}

func (i *standaloneNodeInitializer) GetAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	return nil, nil
}

func (i *standaloneNodeInitializer) GetTestMnemonic(validatorIndex int) string {
	return ""
}

// standaloneNodeRuntime is a placeholder node runtime for standalone mode
type standaloneNodeRuntime struct {
	startedNodes []string
}

func newStandaloneNodeRuntime() *standaloneNodeRuntime {
	return &standaloneNodeRuntime{
		startedNodes: make([]string, 0),
	}
}

func (r *standaloneNodeRuntime) StartNode(ctx context.Context, node *types.Node, opts runtime.StartOptions) error {
	r.startedNodes = append(r.startedNodes, node.Metadata.Name)
	// In standalone mode, we don't actually start processes
	// This is a placeholder that simulates successful start
	return nil
}

func (r *standaloneNodeRuntime) StopNode(ctx context.Context, nodeID string, graceful bool) error {
	return nil
}

func (r *standaloneNodeRuntime) RestartNode(ctx context.Context, nodeID string) error {
	return nil
}

func (r *standaloneNodeRuntime) GetNodeStatus(ctx context.Context, nodeID string) (*runtime.NodeStatus, error) {
	return &runtime.NodeStatus{
		Running:   true,
		StartedAt: time.Now(),
	}, nil
}

func (r *standaloneNodeRuntime) GetLogs(ctx context.Context, nodeID string, opts runtime.LogOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (r *standaloneNodeRuntime) Cleanup(ctx context.Context) error {
	return nil
}
