// cmd/devnetd/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server"
	"github.com/spf13/cobra"
)

func main() {
	var config server.Config

	rootCmd := &cobra.Command{
		Use:   "devnetd",
		Short: "Devnet Builder Daemon",
		Long:  `devnetd is the daemon that manages blockchain development networks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := server.New(&config)
			if err != nil {
				return err
			}
			return srv.Run(context.Background())
		},
	}

	defaults := server.DefaultConfig()
	rootCmd.Flags().StringVar(&config.SocketPath, "socket", defaults.SocketPath, "Unix socket path")
	rootCmd.Flags().StringVar(&config.DataDir, "data-dir", defaults.DataDir, "Data directory")
	rootCmd.Flags().BoolVar(&config.Foreground, "foreground", true, "Run in foreground")
	rootCmd.Flags().IntVar(&config.Workers, "workers", defaults.Workers, "Workers per controller")
	rootCmd.Flags().StringVar(&config.LogLevel, "log-level", defaults.LogLevel, "Log level (debug, info, warn, error)")
	rootCmd.Flags().BoolVar(&config.EnableDocker, "docker", false, "Enable Docker container runtime")
	rootCmd.Flags().StringVar(&config.DockerImage, "docker-image", "stablelabs/stabled:latest", "Default Docker image for nodes")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
