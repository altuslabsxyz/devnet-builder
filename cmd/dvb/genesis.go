// cmd/dvb/genesis.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/cosmos"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// genesisForkOptions holds options for the genesis fork command
type genesisForkOptions struct {
	network       string
	networkType   string
	rpcURL        string
	snapshotURL   string
	localPath     string
	chainID       string
	binaryPath    string
	output        string
	votingPeriod  time.Duration
	unbondingTime time.Duration
	noCache       bool
}

func newGenesisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genesis",
		Short: "Genesis file operations",
		Long: `Manage genesis file operations for devnets.

Subcommands allow you to fork genesis from various sources including
mainnet RPC endpoints, snapshots, or local files.

Examples:
  # Fork genesis from mainnet RPC
  dvb genesis fork --network stable --chain-id my-devnet-1

  # Fork genesis from a local file
  dvb genesis fork --network stable --local-path ./genesis.json --chain-id my-devnet-1

  # Fork genesis with custom parameters
  dvb genesis fork --network cosmos --chain-id test-1 --voting-period 60s --unbonding-time 120s`,
	}

	cmd.AddCommand(
		newGenesisForkCmd(),
	)

	return cmd
}

func newGenesisForkCmd() *cobra.Command {
	opts := &genesisForkOptions{}

	cmd := &cobra.Command{
		Use:   "fork",
		Short: "Fork genesis from a network",
		Long: `Fork genesis from mainnet/testnet and modify for local devnet use.

Sources (in priority order if multiple specified):
  1. --local-path: Use a local genesis file
  2. --snapshot-url: Download snapshot and export genesis (requires --binary-path)
  3. --rpc-url or plugin default: Fetch from RPC endpoint

The forked genesis will have:
  - New chain ID (required)
  - Modified governance voting period (default: 30s)
  - Modified staking unbonding time (default: 60s)

Examples:
  # Fork from mainnet RPC (uses plugin's default endpoint)
  dvb genesis fork --network stable --chain-id my-devnet-1

  # Fork from a specific RPC endpoint
  dvb genesis fork --network cosmos --rpc-url https://rpc.cosmos.network --chain-id test-1

  # Fork from a local genesis file
  dvb genesis fork --network stable --local-path ./mainnet-genesis.json --chain-id my-devnet-1 -o ./devnet-genesis.json

  # Fork from testnet instead of mainnet
  dvb genesis fork --network gaia --network-type testnet --chain-id test-local-1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenesisFork(cmd.Context(), opts)
		},
	}

	// Required flags
	cmd.Flags().StringVar(&opts.network, "network", "", "Network/plugin name (stable, cosmos, gaia) - required")
	cmd.Flags().StringVar(&opts.chainID, "chain-id", "", "New chain ID for the forked genesis - required")
	_ = cmd.MarkFlagRequired("network")
	_ = cmd.MarkFlagRequired("chain-id")

	// Source flags
	cmd.Flags().StringVar(&opts.networkType, "network-type", "mainnet", "Network type (mainnet or testnet)")
	cmd.Flags().StringVar(&opts.rpcURL, "rpc-url", "", "RPC endpoint URL (uses plugin default if not specified)")
	cmd.Flags().StringVar(&opts.snapshotURL, "snapshot-url", "", "Snapshot URL for snapshot mode")
	cmd.Flags().StringVar(&opts.localPath, "local-path", "", "Path to local genesis file")
	cmd.Flags().StringVar(&opts.binaryPath, "binary-path", "", "Binary path (required for snapshot mode)")

	// Output flags
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output file path (default: stdout)")

	// Patch option flags
	cmd.Flags().DurationVar(&opts.votingPeriod, "voting-period", 30*time.Second, "Governance voting period")
	cmd.Flags().DurationVar(&opts.unbondingTime, "unbonding-time", 60*time.Second, "Staking unbonding time")

	// Cache flags
	cmd.Flags().BoolVar(&opts.noCache, "no-cache", false, "Skip cache for snapshots")

	return cmd
}

func runGenesisFork(ctx context.Context, opts *genesisForkOptions) error {
	// Get data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	dataDir := filepath.Join(homeDir, ".devnet-builder")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create logger (quiet for stdout output, verbose for file output)
	var logger *slog.Logger
	if opts.output == "" {
		// Quiet mode for stdout - only errors
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))
	} else {
		// Verbose mode for file output
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	// Create plugin genesis based on network
	pluginGenesis, err := getPluginGenesis(opts.network)
	if err != nil {
		return err
	}

	// Create genesis forker
	forker := provisioner.NewGenesisForker(provisioner.GenesisForkerConfig{
		DataDir:       dataDir,
		PluginGenesis: pluginGenesis,
		Logger:        logger,
	})

	// Determine source mode (priority: local > snapshot > rpc)
	source := determineGenesisSource(opts)

	// Validate snapshot mode requirements
	if source.Mode == types.GenesisModeSnapshot && opts.binaryPath == "" {
		return fmt.Errorf("--binary-path is required for snapshot mode")
	}

	// Print info if outputting to file
	if opts.output != "" {
		fmt.Fprintf(os.Stderr, "Forking genesis...\n")
		fmt.Fprintf(os.Stderr, "  Network:      %s\n", opts.network)
		fmt.Fprintf(os.Stderr, "  Network Type: %s\n", opts.networkType)
		fmt.Fprintf(os.Stderr, "  Source Mode:  %s\n", source.Mode)
		fmt.Fprintf(os.Stderr, "  New Chain ID: %s\n", opts.chainID)
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Fork genesis
	result, err := forker.Fork(ctx, ports.ForkOptions{
		Source: source,
		PatchOpts: types.GenesisPatchOptions{
			ChainID:       opts.chainID,
			VotingPeriod:  opts.votingPeriod,
			UnbondingTime: opts.unbondingTime,
		},
		BinaryPath: opts.binaryPath,
		NoCache:    opts.noCache,
	})
	if err != nil {
		return fmt.Errorf("failed to fork genesis: %w", err)
	}

	// Output result
	if opts.output == "" {
		// Write to stdout
		_, err = os.Stdout.Write(result.Genesis)
		if err != nil {
			return fmt.Errorf("failed to write to stdout: %w", err)
		}
	} else {
		// Write to file
		if err := os.WriteFile(opts.output, result.Genesis, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		// Print success message
		color.Green("Genesis forked successfully!")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "  Output:         %s\n", opts.output)
		fmt.Fprintf(os.Stderr, "  Source Chain:   %s\n", result.SourceChainID)
		fmt.Fprintf(os.Stderr, "  New Chain ID:   %s\n", result.NewChainID)
		fmt.Fprintf(os.Stderr, "  Source Mode:    %s\n", result.SourceMode)
		fmt.Fprintf(os.Stderr, "  Fetched At:     %s\n", result.FetchedAt.Format(time.RFC3339))
	}

	return nil
}

// getPluginGenesis returns the appropriate PluginGenesis for the given network
func getPluginGenesis(network string) (types.PluginGenesis, error) {
	switch network {
	case "stable":
		return cosmos.NewCosmosGenesis("stabled"), nil
	case "cosmos", "gaia":
		return cosmos.NewCosmosGenesis("gaiad"), nil
	default:
		return nil, fmt.Errorf("unknown network: %s (supported: stable, cosmos, gaia)", network)
	}
}

// determineGenesisSource determines the genesis source based on provided options
// Priority: local > snapshot > rpc
func determineGenesisSource(opts *genesisForkOptions) types.GenesisSource {
	source := types.GenesisSource{
		NetworkType: opts.networkType,
		RPCURL:      opts.rpcURL,
		SnapshotURL: opts.snapshotURL,
	}

	// Priority 1: Local file
	if opts.localPath != "" {
		// Convert relative path to absolute
		absPath, err := filepath.Abs(opts.localPath)
		if err != nil {
			// If conversion fails, use the original path
			// The forker will validate it
			absPath = opts.localPath
		}
		source.Mode = types.GenesisModeLocal
		source.LocalPath = absPath
		return source
	}

	// Priority 2: Snapshot
	if opts.snapshotURL != "" {
		source.Mode = types.GenesisModeSnapshot
		return source
	}

	// Priority 3: RPC (default)
	source.Mode = types.GenesisModeRPC
	return source
}
