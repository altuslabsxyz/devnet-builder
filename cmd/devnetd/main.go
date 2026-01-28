// cmd/devnetd/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/config"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server"
	"github.com/altuslabsxyz/devnet-builder/internal/version"
	"github.com/spf13/cobra"
)

// Flag variables for CLI overrides
var (
	flagConfigPath  string
	flagSocket      string
	flagDataDir     string
	flagLogLevel    string
	flagWorkers     int
	flagForeground  bool
	flagDocker      bool
	flagDockerImage string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "devnetd",
		Short: "Devnet Builder Daemon",
		Long:  `devnetd is the daemon that manages blockchain development networks.`,
		RunE:  runDaemon,
	}

	defaults := config.DefaultConfig()

	// Config file flag
	rootCmd.Flags().StringVar(&flagConfigPath, "config", "", "Config file path (default: ~/.devnet-builder/devnetd.toml)")

	// Server flags
	rootCmd.Flags().StringVar(&flagSocket, "socket", "", fmt.Sprintf("Unix socket path (default: %s)", defaults.Server.Socket))
	rootCmd.Flags().StringVar(&flagDataDir, "data-dir", "", fmt.Sprintf("Data directory (default: %s)", defaults.Server.DataDir))
	rootCmd.Flags().StringVar(&flagLogLevel, "log-level", "", fmt.Sprintf("Log level: debug, info, warn, error (default: %s)", defaults.Server.LogLevel))
	rootCmd.Flags().IntVar(&flagWorkers, "workers", 0, fmt.Sprintf("Workers per controller (default: %d)", defaults.Server.Workers))
	rootCmd.Flags().BoolVar(&flagForeground, "foreground", true, "Run in foreground")

	// Docker flags
	rootCmd.Flags().BoolVar(&flagDocker, "docker", false, "Enable Docker container runtime")
	rootCmd.Flags().StringVar(&flagDockerImage, "docker-image", "", fmt.Sprintf("Default Docker image (default: %s)", defaults.Docker.Image))

	// Add subcommands
	rootCmd.AddCommand(version.NewCmd("devnet-builder", "devnetd"))
	rootCmd.AddCommand(newConfigCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Determine data directory for loader
	dataDir := config.DefaultDataDir()
	if flagDataDir != "" {
		dataDir = flagDataDir
	}

	// Load config: defaults < file < env
	loader := config.NewLoader(dataDir, flagConfigPath)
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply CLI flag overrides (highest priority)
	applyFlagOverrides(cmd, cfg)

	// Validate final config
	if err := config.Validate(cfg); err != nil {
		return err
	}

	// Convert to server.Config
	serverCfg := &server.Config{
		SocketPath:         cfg.Server.Socket,
		DataDir:            cfg.Server.DataDir,
		Foreground:         cfg.Server.Foreground,
		Workers:            cfg.Server.Workers,
		LogLevel:           cfg.Server.LogLevel,
		EnableDocker:       cfg.Docker.Enabled,
		DockerImage:        cfg.Docker.Image,
		ShutdownTimeout:    cfg.Timeouts.Shutdown,
		HealthCheckTimeout: cfg.Timeouts.HealthCheck,
		GitHubToken:        cfg.GitHub.Token,
	}

	// Set GitHub token in environment for github_factory.go to pick up
	if cfg.GitHub.Token != "" {
		os.Setenv("GITHUB_TOKEN", cfg.GitHub.Token)
	}

	srv, err := server.New(serverCfg)
	if err != nil {
		return err
	}
	return srv.Run(context.Background())
}

// applyFlagOverrides applies CLI flags to config (highest priority).
func applyFlagOverrides(cmd *cobra.Command, cfg *config.Config) {
	if cmd.Flags().Changed("socket") {
		cfg.Server.Socket = flagSocket
	}
	if cmd.Flags().Changed("data-dir") {
		cfg.Server.DataDir = flagDataDir
	}
	if cmd.Flags().Changed("log-level") {
		cfg.Server.LogLevel = flagLogLevel
	}
	if cmd.Flags().Changed("workers") {
		cfg.Server.Workers = flagWorkers
	}
	if cmd.Flags().Changed("foreground") {
		cfg.Server.Foreground = flagForeground
	}
	if cmd.Flags().Changed("docker") {
		cfg.Docker.Enabled = flagDocker
	}
	if cmd.Flags().Changed("docker-image") {
		cfg.Docker.Image = flagDockerImage
	}
}
