// cmd/dvb/whoami.go
package main

import (
	"fmt"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newWhoAmICmd() *cobra.Command {
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
  dvb whoami`,
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
