// cmd/devnetd/config.go
package main

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage devnetd configuration",
		Long:  `Commands for managing devnetd configuration files.`,
	}

	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigShowCmd())

	return cmd
}
