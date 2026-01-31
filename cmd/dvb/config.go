// cmd/dvb/config.go
package main

import (
	"fmt"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage dvb client configuration",
		Long: `Manage dvb client configuration for connecting to devnetd.

Configuration is stored in ~/.dvb/config.yaml and includes:
  - server:    Remote devnetd server address (optional)
  - api-key:   API key for authentication (required for remote)
  - namespace: Default namespace for commands

Examples:
  dvb config set server devnetd.example.com:9000
  dvb config set api-key devnet_xxx
  dvb config set namespace team-a
  dvb config get server
  dvb config list`,
	}

	cmd.AddCommand(
		newConfigGetCmd(),
		newConfigSetCmd(),
		newConfigListCmd(),
	)

	return cmd
}

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value.

Supported keys:
  server    - Remote devnetd server address (e.g., devnetd.example.com:9000)
  api-key   - API key for authentication
  namespace - Default namespace for commands

Examples:
  dvb config set server devnetd.example.com:9000
  dvb config set api-key devnet_abc123...
  dvb config set namespace team-a`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			// Load existing config
			cfg, err := client.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Set the value
			if err := cfg.Set(key, value); err != nil {
				return err
			}

			// Save the config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			// Show confirmation (mask api-key)
			displayValue := value
			if key == "api-key" {
				displayValue = maskAPIKey(value)
			}
			color.Green("Set %s = %s", key, displayValue)
			return nil
		},
	}

	return cmd
}

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get a configuration value.

Supported keys:
  server    - Remote devnetd server address
  api-key   - API key for authentication (masked for security)
  namespace - Default namespace for commands

Examples:
  dvb config get server
  dvb config get api-key
  dvb config get namespace`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Validate key
			validKeys := []string{"server", "api-key", "namespace"}
			valid := false
			for _, k := range validKeys {
				if k == key {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("unknown config key: %s (supported: %s)", key, strings.Join(validKeys, ", "))
			}

			// Load config
			cfg, err := client.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			value := cfg.Get(key)
			if value == "" {
				fmt.Printf("%s: (not set)\n", key)
				return nil
			}

			// Mask api-key for security
			if key == "api-key" {
				value = maskAPIKey(value)
			}

			fmt.Printf("%s: %s\n", key, value)
			return nil
		},
	}

	return cmd
}

func newConfigListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long: `List all configuration values.

API keys are masked for security.`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := client.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Println("dvb configuration (~/.dvb/config.yaml):")
			fmt.Println()

			// Server
			server := cfg.Server
			if server == "" {
				server = "(not set - using local socket)"
			}
			fmt.Printf("  server:    %s\n", server)

			// API Key (masked)
			apiKey := cfg.APIKey
			if apiKey == "" {
				apiKey = "(not set)"
			} else {
				apiKey = maskAPIKey(apiKey)
			}
			fmt.Printf("  api-key:   %s\n", apiKey)

			// Namespace
			namespace := cfg.Namespace
			if namespace == "" {
				namespace = "(not set - using default)"
			}
			fmt.Printf("  namespace: %s\n", namespace)

			return nil
		},
	}

	return cmd
}

// maskAPIKey masks an API key, showing only the prefix and last 4 characters.
func maskAPIKey(key string) string {
	const prefix = "devnet_"
	if len(key) <= len(prefix)+4 {
		return prefix + "****"
	}
	// Show: devnet_****<last4>
	return prefix + "****" + key[len(key)-4:]
}
