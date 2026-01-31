// cmd/devnetd/keys.go
package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/auth"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage API keys for remote access",
		Long: `Manage API keys for authenticating remote clients.

API keys allow remote dvb clients to connect to this devnetd instance.
Each key is associated with a name (user identifier) and a list of
namespaces the key can access.

Keys are stored in ~/.devnet-builder/api-keys.yaml`,
	}

	cmd.AddCommand(
		newKeysCreateCmd(),
		newKeysListCmd(),
		newKeysRevokeCmd(),
	)

	return cmd
}

func newKeysCreateCmd() *cobra.Command {
	var (
		name       string
		namespaces string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new API key",
		Long: `Create a new API key for remote client authentication.

The key is displayed only once. Make sure to save it securely.

Examples:
  # Create a key with access to all namespaces
  devnetd keys create --name alice --namespaces "*"

  # Create a key with access to specific namespaces
  devnetd keys create --name bob --namespaces "team-a,team-b"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if namespaces == "" {
				return fmt.Errorf("--namespaces is required")
			}

			// Parse namespaces
			nsList := parseNamespaces(namespaces)
			if len(nsList) == 0 {
				return fmt.Errorf("at least one namespace is required")
			}

			// Load key store
			store := auth.NewFileKeyStore(auth.DefaultKeysPath())
			if err := store.Load(); err != nil {
				return fmt.Errorf("failed to load keys: %w", err)
			}

			// Create the key
			apiKey, err := store.Create(name, nsList)
			if err != nil {
				return fmt.Errorf("failed to create key: %w", err)
			}

			// Save the key store
			if err := store.Save(); err != nil {
				return fmt.Errorf("failed to save keys: %w", err)
			}

			// Output the result
			color.Green("API key created successfully")
			fmt.Println()
			fmt.Printf("Name:       %s\n", apiKey.Name)
			fmt.Printf("Namespaces: %s\n", strings.Join(apiKey.Namespaces, ", "))
			fmt.Printf("Created:    %s\n", apiKey.CreatedAt.Format(time.RFC3339))
			fmt.Println()
			color.Yellow("API Key (save this - it will not be shown again):")
			fmt.Println()
			fmt.Printf("  %s\n", apiKey.Key)
			fmt.Println()
			fmt.Println("To configure dvb client:")
			fmt.Printf("  dvb config set api-key %s\n", apiKey.Key)

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name/identifier for the key owner (required)")
	cmd.Flags().StringVar(&namespaces, "namespaces", "", "Comma-separated list of namespaces, or \"*\" for all (required)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("namespaces")

	return cmd
}

func newKeysListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all API keys",
		Long: `List all API keys with their metadata.

For security, the full key is never shown. Only a masked prefix is displayed.`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load key store
			store := auth.NewFileKeyStore(auth.DefaultKeysPath())
			if err := store.Load(); err != nil {
				return fmt.Errorf("failed to load keys: %w", err)
			}

			keys := store.List()
			if len(keys) == 0 {
				fmt.Println("No API keys found.")
				fmt.Println()
				fmt.Println("Create a key with:")
				fmt.Println("  devnetd keys create --name <name> --namespaces <namespaces>")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tKEY PREFIX\tNAMESPACES\tCREATED")
			for _, key := range keys {
				// Show masked key (first 10 chars + ...)
				maskedKey := maskKey(key.Key)
				namespaces := strings.Join(key.Namespaces, ", ")
				created := key.CreatedAt.Format("2006-01-02 15:04")
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key.Name, maskedKey, namespaces, created)
			}
			w.Flush()

			return nil
		},
	}

	return cmd
}

func newKeysRevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <key>",
		Short: "Revoke an API key",
		Long: `Revoke (delete) an API key.

The full API key must be provided. Once revoked, the key can no longer
be used for authentication.

Examples:
  devnetd keys revoke devnet_abc123...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Validate key format
			if !auth.IsValidKeyFormat(key) {
				return fmt.Errorf("invalid key format: expected devnet_<32-hex-chars>")
			}

			// Load key store
			store := auth.NewFileKeyStore(auth.DefaultKeysPath())
			if err := store.Load(); err != nil {
				return fmt.Errorf("failed to load keys: %w", err)
			}

			// Get key info before revoking (for confirmation message)
			apiKey, ok := store.Get(key)
			if !ok {
				return fmt.Errorf("key not found")
			}
			keyName := apiKey.Name

			// Revoke the key
			if err := store.Revoke(key); err != nil {
				return fmt.Errorf("failed to revoke key: %w", err)
			}

			// Save the key store
			if err := store.Save(); err != nil {
				return fmt.Errorf("failed to save keys: %w", err)
			}

			color.Green("API key revoked: %s", keyName)
			return nil
		},
	}

	return cmd
}

// parseNamespaces parses a comma-separated list of namespaces.
func parseNamespaces(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// maskKey returns a masked version showing only prefix and last 4 chars.
func maskKey(key string) string {
	if len(key) <= 10 {
		return "****"
	}
	return key[:10] + "****"
}
