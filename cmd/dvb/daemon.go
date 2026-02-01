// cmd/dvb/daemon.go
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the devnetd daemon",
		Long: `Manage the devnetd daemon.

The daemon runs in the background and manages devnet lifecycle.
Use these subcommands to check status, view logs, and manage plugins.

Examples:
  # Check daemon status and connectivity
  dvb daemon status

  # View daemon logs
  dvb daemon logs -f

  # Show current authentication context
  dvb daemon whoami

  # List available plugins
  dvb daemon plugins list`,
	}

	cmd.AddCommand(
		newDaemonStatusCmd(),
		newDaemonLogsCmd(),
		newDaemonWhoAmICmd(),
		newPluginsCmd(),
	)

	return cmd
}

// newDaemonStatusCmd creates the 'daemon status' subcommand with connectivity info.
func newDaemonStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check daemon status and connectivity",
		Long: `Check daemon status including connectivity information.

Shows whether the daemon is running, and if so, displays:
  - Connection endpoint (socket or remote server)
  - Server version
  - Connection latency
  - Connection mode (local/remote)

Examples:
  # Check daemon status
  dvb daemon status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemonStatus(cmd)
		},
	}

	return cmd
}

func runDaemonStatus(cmd *cobra.Command) error {
	// Load config for remote server info
	cfg, err := client.LoadConfig()
	if err != nil {
		// Config loading failed, assume local
		cfg = &client.ClientConfig{}
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

	// Check if daemon is running (for local) or try to connect (for remote)
	if isRemote {
		return runDaemonStatusRemote(cmd, serverAddr, cfg.APIKey)
	}
	return runDaemonStatusLocal(cmd, serverAddr)
}

func runDaemonStatusLocal(cmd *cobra.Command, socketPath string) error {
	// Check if daemon is running
	if !client.IsDaemonRunning() {
		color.Yellow("○ Daemon is not running")
		fmt.Println()
		fmt.Println("Start the daemon with:")
		fmt.Println("  devnetd &")
		fmt.Println()
		fmt.Println("Or run in foreground:")
		fmt.Println("  devnetd")
		return nil
	}

	// Try to connect and get server info
	start := time.Now()
	c, err := client.New()
	if err != nil {
		color.Green("● Daemon is running")
		fmt.Printf("  Socket:  %s\n", socketPath)
		fmt.Printf("  Error:   %v\n", err)
		return nil
	}
	defer c.Close()

	// Ping to get version and latency
	resp, err := c.Ping(cmd.Context())
	latency := time.Since(start)

	if err != nil {
		color.Green("● Daemon is running")
		fmt.Printf("  Socket:  %s\n", socketPath)
		fmt.Printf("  Error:   ping failed: %v\n", err)
		return nil
	}

	// Success - show full status
	color.Green("● Daemon is running")
	fmt.Println()
	fmt.Printf("  Socket:  %s\n", socketPath)
	fmt.Printf("  Version: %s\n", resp.ServerVersion)
	fmt.Printf("  Latency: %s\n", latency.Round(time.Microsecond))
	fmt.Printf("  Mode:    local (trusted)\n")

	return nil
}

func runDaemonStatusRemote(cmd *cobra.Command, serverAddr, apiKey string) error {
	if apiKey == "" {
		color.Yellow("○ Remote server configured but not connected")
		fmt.Println()
		fmt.Printf("  Server:  %s\n", serverAddr)
		fmt.Println("  Error:   No API key configured")
		fmt.Println()
		fmt.Println("Set your API key with:")
		fmt.Println("  dvb config set api-key <your-api-key>")
		return nil
	}

	// Try to connect
	start := time.Now()
	c, err := client.NewRemote(serverAddr, apiKey)
	if err != nil {
		color.Red("○ Remote server unreachable")
		fmt.Println()
		fmt.Printf("  Server:  %s\n", serverAddr)
		fmt.Printf("  Error:   %v\n", err)
		return nil
	}
	defer c.Close()

	// Ping to get version and latency
	resp, err := c.Ping(cmd.Context())
	latency := time.Since(start)

	if err != nil {
		color.Red("○ Remote server connection failed")
		fmt.Println()
		fmt.Printf("  Server:  %s\n", serverAddr)
		fmt.Printf("  Error:   ping failed: %v\n", err)
		return nil
	}

	// Success - show full status
	color.Green("● Connected to remote daemon")
	fmt.Println()
	fmt.Printf("  Server:  %s\n", serverAddr)
	fmt.Printf("  Version: %s\n", resp.ServerVersion)
	fmt.Printf("  Latency: %s\n", latency.Round(time.Microsecond))
	fmt.Printf("  Mode:    remote (authenticated)\n")

	return nil
}

// newDaemonWhoAmICmd creates the 'daemon whoami' subcommand.
func newDaemonWhoAmICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show authenticated user information",
		Long: `Show information about the currently authenticated user.

For remote connections, displays:
  - User name (from API key)
  - Allowed namespaces

For local connections (Unix socket), no authentication is required
and the user has full access to all namespaces.

Examples:
  dvb daemon whoami`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, err := client.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Determine connection type
			if cfg.Server != "" {
				// Remote connection
				if cfg.APIKey == "" {
					return fmt.Errorf("remote server configured but no API key set")
				}

				// Create remote client and call WhoAmI
				c, err := client.NewRemote(cfg.Server, cfg.APIKey)
				if err != nil {
					return fmt.Errorf("failed to connect: %w", err)
				}
				defer c.Close()

				resp, err := c.WhoAmI(cmd.Context())
				if err != nil {
					return fmt.Errorf("whoami failed: %w", err)
				}

				color.Green("Authenticated")
				fmt.Println()
				fmt.Printf("Name:       %s\n", resp.Name)
				fmt.Printf("Namespaces: %s\n", formatNamespaces(resp.Namespaces))
				fmt.Printf("Server:     %s\n", cfg.Server)
				fmt.Printf("Connection: remote (TLS)\n")
				return nil
			}

			// Local connection
			if !client.IsDaemonRunning() {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			// Create local client and call WhoAmI
			c, err := client.New()
			if err != nil {
				return fmt.Errorf("failed to connect: %w", err)
			}
			defer c.Close()

			resp, err := c.WhoAmI(cmd.Context())
			if err != nil {
				return fmt.Errorf("whoami failed: %w", err)
			}

			color.Green("Authenticated")
			fmt.Println()
			fmt.Printf("Name:       %s\n", resp.Name)
			fmt.Printf("Namespaces: %s\n", formatNamespaces(resp.Namespaces))
			fmt.Printf("Connection: Unix socket (trusted)\n")

			return nil
		},
	}

	return cmd
}

// formatNamespaces formats the namespace list for display.
func formatNamespaces(namespaces []string) string {
	if len(namespaces) == 0 {
		return "(none)"
	}
	for _, ns := range namespaces {
		if ns == "*" {
			return "* (all)"
		}
	}
	return strings.Join(namespaces, ", ")
}
