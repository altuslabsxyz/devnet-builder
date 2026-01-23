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

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
