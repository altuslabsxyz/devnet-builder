// cmd/dvb/ping.go
package main

import (
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newPingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Test connection to devnetd server",
		Long: `Test connectivity and authentication to the devnetd server.

This command verifies that:
  1. The server is reachable
  2. TLS connection succeeds (for remote servers)
  3. API key authentication succeeds (for remote servers)

For local connections (Unix socket), no authentication is required.

Examples:
  # Test connection to configured server
  dvb ping

  # Test connection to specific server
  dvb ping --server devnetd.example.com:9000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, err := client.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Determine connection type
			var serverAddr string
			var isRemote bool

			if cfg.Server != "" {
				serverAddr = cfg.Server
				isRemote = true
			} else {
				serverAddr = client.DefaultSocketPath()
				isRemote = false
			}

			// Perform the ping
			start := time.Now()

			if isRemote {
				// Remote connection - require API key
				if cfg.APIKey == "" {
					color.Red("Connection failed")
					fmt.Println()
					fmt.Println("Remote server configured but no API key set.")
					fmt.Println("Set your API key with:")
					fmt.Println("  dvb config set api-key <your-api-key>")
					return fmt.Errorf("missing api-key for remote connection")
				}

				// Create remote client and ping
				c, err := client.NewRemote(serverAddr, cfg.APIKey)
				if err != nil {
					color.Red("Connection failed")
					fmt.Println()
					fmt.Printf("Error: %v\n", err)
					return err
				}
				defer c.Close()

				resp, err := c.Ping(cmd.Context())
				latency := time.Since(start)

				if err != nil {
					color.Red("Connection failed")
					fmt.Println()
					fmt.Printf("Error: %v\n", err)
					return err
				}

				// Success
				color.Green("Connected to devnetd")
				fmt.Println()
				fmt.Printf("  Server:  %s\n", serverAddr)
				fmt.Printf("  Version: %s\n", resp.ServerVersion)
				fmt.Printf("  Latency: %s\n", latency.Round(time.Microsecond))
				fmt.Printf("  Mode:    remote (authenticated)\n")
				return nil
			}

			// Local connection - check if daemon is running
			if !client.IsDaemonRunning() {
				color.Red("Connection failed")
				fmt.Println()
				fmt.Println("Local daemon is not running.")
				fmt.Println("Start the daemon with:")
				fmt.Println("  devnetd &")
				return fmt.Errorf("daemon not running")
			}

			// Try to connect
			c, err := client.New()
			if err != nil {
				color.Red("Connection failed")
				fmt.Println()
				fmt.Printf("Error: %v\n", err)
				return err
			}
			defer c.Close()

			// Call actual Ping RPC
			resp, err := c.Ping(cmd.Context())
			latency := time.Since(start)

			if err != nil {
				color.Red("Connection failed")
				fmt.Println()
				fmt.Printf("Error: %v\n", err)
				return err
			}

			// Success
			color.Green("Connected to devnetd")
			fmt.Println()
			fmt.Printf("  Socket:  %s\n", serverAddr)
			fmt.Printf("  Version: %s\n", resp.ServerVersion)
			fmt.Printf("  Latency: %s\n", latency.Round(time.Microsecond))
			fmt.Printf("  Mode:    local (trusted)\n")

			return nil
		},
	}

	return cmd
}
