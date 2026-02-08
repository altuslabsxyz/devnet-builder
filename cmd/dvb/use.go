// cmd/dvb/use.go
package main

import (
	"errors"
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/internal/dvbcontext"
	"github.com/fatih/color"
	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"
)

func newUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use [ref]",
		Short: "Set or show the current devnet context",
		Long: `Set or show the current devnet context.

The context determines the default devnet for commands that operate on a specific devnet.

Usage:
  dvb use              # Show current context, or pick interactively if none set
  dvb use <devnet>     # Set context to devnet in default namespace
  dvb use ns/devnet    # Set context to devnet in specified namespace
  dvb use -            # Clear the current context

Examples:
  # Set context to a devnet in the default namespace
  dvb use stable-testnet

  # Set context to a devnet in a specific namespace
  dvb use staging/my-devnet

  # Show current context
  dvb use

  # Clear context
  dvb use -`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Case 1: dvb use - (clear context)
			if len(args) == 1 && args[0] == "-" {
				if err := dvbcontext.Clear(); err != nil {
					return fmt.Errorf("failed to clear context: %w", err)
				}
				color.Yellow("Context cleared")
				return nil
			}

			// Case 2: dvb use <ref> (set context)
			if len(args) == 1 {
				ref := args[0]
				namespace, devnetName := dvbcontext.ParseRef(ref)

				// Validate devnet exists via daemon
				if err := requireDaemon(); err != nil {
					return err
				}

				_, err := daemonClient.GetDevnet(cmd.Context(), namespace, devnetName)
				if err != nil {
					return fmt.Errorf("devnet %s/%s not found", namespace, devnetName)
				}

				// Save context
				if err := dvbcontext.Save(namespace, devnetName); err != nil {
					return fmt.Errorf("failed to save context: %w", err)
				}

				color.Green("Context set to %s/%s", namespace, devnetName)
				return nil
			}

			// Case 3: dvb use (no args) - show context or interactive picker
			ctx, err := dvbcontext.Load()
			if err != nil {
				return fmt.Errorf("failed to load context: %w", err)
			}

			// If context exists, print it
			if ctx != nil {
				fmt.Println(ctx.String())
				return nil
			}

			// No context set - show interactive picker if daemon is running
			if daemonClient == nil {
				return errors.New("no context set. Run 'dvb use <devnet>' to set context")
			}

			// List all devnets for picker
			devnets, err := daemonClient.ListDevnets(cmd.Context(), "") // empty = all namespaces
			if err != nil {
				return fmt.Errorf("failed to list devnets: %w", err)
			}

			if len(devnets) == 0 {
				return errors.New("no devnets found. Create one with 'dvb provision'")
			}

			// Build list of ref strings
			items := make([]string, len(devnets))
			for i, d := range devnets {
				items[i] = fmt.Sprintf("%s/%s", d.Metadata.Namespace, d.Metadata.Name)
			}

			// Non-interactive: list available devnets and return error
			if IsNonInteractive() {
				fmt.Println("Available devnets:")
				for _, item := range items {
					fmt.Printf("  %s\n", item)
				}
				return errors.New("no context set. Run 'dvb use <devnet>' to set context")
			}

			// Show interactive picker
			idx, err := fuzzyfinder.Find(items, func(i int) string {
				return items[i]
			})
			if errors.Is(err, fuzzyfinder.ErrAbort) {
				return nil // User cancelled
			}
			if err != nil {
				return fmt.Errorf("picker error: %w", err)
			}

			// Save selected context
			selected := devnets[idx]
			namespace := selected.Metadata.Namespace
			devnetName := selected.Metadata.Name

			if err := dvbcontext.Save(namespace, devnetName); err != nil {
				return fmt.Errorf("failed to save context: %w", err)
			}

			color.Green("Context set to %s/%s", namespace, devnetName)
			return nil
		},
	}

	return cmd
}
