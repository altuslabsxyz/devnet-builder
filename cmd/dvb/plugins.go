// cmd/dvb/plugins.go
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage network plugins",
		Long: `Manage network plugins for devnet provisioning.

Network plugins define the blockchain network configuration, including
the binary to use, available versions, and supported network types.

Plugins are discovered from ~/.devnet-builder/plugins/`,
	}

	cmd.AddCommand(
		newPluginsListCmd(),
	)

	return cmd
}

func newPluginsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available network plugins",
		Long: `List all available network plugins discovered by the daemon.

Network plugins are loaded from ~/.devnet-builder/plugins/ and define
the networks that can be used when provisioning a devnet.

Examples:
  # List all available plugins
  dvb plugins list`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginsList(cmd.Context())
		},
	}

	return cmd
}

// runPluginsList lists available network plugins from the daemon
func runPluginsList(ctx context.Context) error {
	if daemonClient == nil {
		return fmt.Errorf("daemon not running - start with: devnetd")
	}

	networks, err := daemonClient.ListNetworks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	if len(networks) == 0 {
		fmt.Println("No network plugins found.")
		fmt.Println()
		fmt.Println("Install plugins to ~/.devnet-builder/plugins/")
		return nil
	}

	fmt.Println("Available Network Plugins:")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDISPLAY NAME\tBINARY\tVERSION\tNETWORKS")
	for _, n := range networks {
		networks := "-"
		if len(n.AvailableNetworks) > 0 {
			networks = fmt.Sprintf("%v", n.AvailableNetworks)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			n.Name,
			n.DisplayName,
			n.BinaryName,
			n.DefaultBinaryVersion,
			networks,
		)
	}
	w.Flush()

	return nil
}
