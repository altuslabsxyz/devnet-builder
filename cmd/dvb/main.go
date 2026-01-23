// cmd/dvb/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	standalone   bool
	daemonClient *client.Client
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dvb",
		Short: "Devnet Builder CLI",
		Long:  `dvb is a CLI for managing blockchain development networks.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip daemon connection for daemon subcommand
			if cmd.Name() == "daemon" || cmd.Parent() != nil && cmd.Parent().Name() == "daemon" {
				return nil
			}

			// Skip if standalone mode
			if standalone {
				return nil
			}

			// Try to connect to daemon
			c, err := client.New()
			if err == nil {
				daemonClient = c
				return nil
			}

			// Daemon not running - fall back to standalone
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient != nil {
				return daemonClient.Close()
			}
			return nil
		},
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&standalone, "standalone", false, "Force standalone mode (don't connect to daemon)")

	// Add commands
	rootCmd.AddCommand(
		newVersionCmd(),
		newDaemonCmd(),
		newStatusCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("dvb version 0.1.0")
			if daemonClient != nil {
				fmt.Println("Mode: daemon")
			} else {
				fmt.Println("Mode: standalone")
			}
		},
	}
}

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the devnetd daemon",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Check daemon status",
			Run: func(cmd *cobra.Command, args []string) {
				if client.IsDaemonRunning() {
					color.Green("● Daemon is running")
					fmt.Printf("  Socket: %s\n", client.DefaultSocketPath())
				} else {
					color.Yellow("○ Daemon is not running")
					fmt.Println("  Start with: devnetd")
				}
			},
		},
	)

	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [devnet]",
		Short: "Show devnet status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient != nil {
				return statusViaDaemon(cmd.Context(), args)
			}
			return statusStandalone(cmd.Context(), args)
		},
	}
}

func statusViaDaemon(ctx context.Context, args []string) error {
	fmt.Println("Status via daemon (not implemented yet)")
	return nil
}

func statusStandalone(ctx context.Context, args []string) error {
	fmt.Println("Status in standalone mode (not implemented yet)")
	return nil
}
