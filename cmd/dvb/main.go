// cmd/dvb/main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"github.com/altuslabsxyz/devnet-builder/internal/dvbcontext"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/internal/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	standalone     bool
	daemonClient   *client.Client
	currentContext *dvbcontext.Context
	dimColor       = color.New(color.Faint)

	// Remote connection flags
	flagServer string
	flagAPIKey string
	flagLocal  bool
)

// printContextHeader prints the current context being used.
// explicit: the devnet specified via args/flags (empty if using context)
// ctx: the loaded context (may be nil)
func printContextHeader(explicit string, ctx *dvbcontext.Context) {
	// Print nothing if both explicit and context are empty
	if explicit == "" && ctx == nil {
		return
	}

	var usingDevnet string
	var contextDevnet string
	var usingNamespace string
	var contextNamespace string

	// Determine what we're using
	if explicit != "" {
		usingDevnet = explicit
	}
	if ctx != nil {
		contextDevnet = ctx.Devnet
		contextNamespace = ctx.Namespace
		if usingDevnet == "" {
			usingDevnet = ctx.Devnet
			usingNamespace = ctx.Namespace
		}
	}

	// Nothing to show if we still don't have a devnet
	if usingDevnet == "" {
		return
	}

	// Build the display string
	var display string
	if usingNamespace != "" && usingNamespace != "default" {
		display = fmt.Sprintf("%s/%s", usingNamespace, usingDevnet)
	} else {
		display = usingDevnet
	}

	// Check if explicit differs from context
	if explicit != "" && ctx != nil && explicit != contextDevnet {
		var contextDisplay string
		if contextNamespace != "" && contextNamespace != "default" {
			contextDisplay = fmt.Sprintf("%s/%s", contextNamespace, contextDevnet)
		} else {
			contextDisplay = contextDevnet
		}
		dimColor.Printf("Using: %s (context: %s)\n", display, contextDisplay)
	} else {
		dimColor.Printf("Using: %s\n", display)
	}
}

// resolveWithSuggestions wraps dvbcontext.Resolve and enhances the error with
// suggestions when the daemon client is available.
func resolveWithSuggestions(explicitDevnet, explicitNamespace string) (namespace, devnet string, err error) {
	namespace, devnet, err = dvbcontext.Resolve(explicitDevnet, explicitNamespace, currentContext)
	if errors.Is(err, dvbcontext.ErrNoDevnet) && daemonClient != nil {
		suggestion := dvbcontext.SuggestUsage(daemonClient)
		return "", "", dvbcontext.NewNoDevnetError(suggestion)
	}
	return namespace, devnet, err
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "dvb",
		Short: "Devnet Builder CLI",
		Long:  `dvb is a CLI for managing blockchain development networks.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip daemon connection for certain commands
			if cmd.Name() == "daemon" || cmd.Parent() != nil && cmd.Parent().Name() == "daemon" {
				return nil
			}

			// Skip if standalone mode
			if standalone {
				return nil
			}

			// Connection precedence:
			// 1. --local flag -> use Unix socket
			// 2. --server flag -> use specified remote
			// 3. ~/.dvb/config.yaml server -> use configured remote
			// 4. Default -> try local Unix socket

			var c *client.Client
			var err error

			if flagLocal {
				// Force local Unix socket connection
				c, err = client.New()
				if err == nil {
					daemonClient = c
				}
			} else if flagServer != "" {
				// Use --server flag
				apiKey := flagAPIKey
				if apiKey == "" {
					// Try to get API key from config if not provided via flag
					cfg, cfgErr := client.LoadConfig()
					if cfgErr == nil && cfg.APIKey != "" {
						apiKey = cfg.APIKey
					}
				}
				c, err = client.NewRemoteClient(flagServer, apiKey)
				if err != nil {
					return fmt.Errorf("failed to connect to remote server: %w", err)
				}
				daemonClient = c
			} else {
				// Check config file for remote server
				cfg, cfgErr := client.LoadConfig()
				if cfgErr == nil && cfg.Server != "" {
					// Use configured remote server
					apiKey := flagAPIKey
					if apiKey == "" {
						apiKey = cfg.APIKey
					}
					c, err = client.NewRemoteClient(cfg.Server, apiKey)
					if err != nil {
						return fmt.Errorf("failed to connect to remote server: %w", err)
					}
					daemonClient = c
				} else {
					// Default: try local Unix socket
					c, err = client.New()
					if err == nil {
						daemonClient = c
					}
				}
			}

			// Load context (ignore errors, context is optional)
			currentContext, _ = dvbcontext.Load()

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
	rootCmd.PersistentFlags().StringVar(&flagServer, "server", "", "Remote devnetd server address (e.g., devnetd.example.com:9000)")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key for remote server authentication")
	rootCmd.PersistentFlags().BoolVar(&flagLocal, "local", false, "Force local Unix socket connection (ignore config)")
	rootCmd.PersistentFlags().BoolVarP(&flagYes, "yes", "y", false, "Auto-confirm all prompts (skip confirmations)")
	rootCmd.PersistentFlags().BoolVar(&flagNonInteractive, "non-interactive", false, "Disable all interactive UI elements (pickers, wizards)")

	// Add commands
	rootCmd.AddCommand(
		newVersionCmd(),
		newDaemonCmd(),
		newUseCmd(),
		newStatusCmd(),
		newGetCmd(),
		newDeleteCmd(),
		newListCmd(),
		newNodeCmd(),
		newUpgradeCmd(),
		newTxCmd(),
		newGovCmd(),
		newGenesisCmd(),
		newProvisionCmd(),
		newConfigCmd(),
		newCompletionCmd(),
		newDeprecatedStartCmd(),
		newDeprecatedStopCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	var (
		long       bool
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print version information including build details. Use --long for detailed dependency info.",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := version.NewInfo("devnet-builder", "dvb")

			if long {
				info = info.WithBuildDeps()
			}

			if jsonOutput {
				output, err := info.JSON()
				if err != nil {
					return err
				}
				fmt.Println(output)
				return nil
			}

			if long {
				fmt.Print(info.LongString())
			} else {
				fmt.Print(info.String())
			}

			// Show connection mode (dvb-specific feature)
			if daemonClient != nil {
				fmt.Println("mode: daemon")
			} else {
				fmt.Println("mode: standalone")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&long, "long", false, "Show detailed version info including build dependencies")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output version info in JSON format")

	return cmd
}

func newListCmd() *cobra.Command {
	var (
		namespace string
		output    string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all devnets",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDaemon(); err != nil {
				return err
			}

			devnets, err := daemonClient.ListDevnets(cmd.Context(), namespace)
			if err != nil {
				return err
			}

			if output == "json" {
				return printJSON(devnets)
			}

			if len(devnets) == 0 {
				fmt.Println("No devnets found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAMESPACE\tNAME\tPHASE\tNODES\tREADY\tHEIGHT")
			for _, d := range devnets {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\n",
					d.Metadata.Namespace,
					d.Metadata.Name,
					d.Status.Phase,
					d.Status.Nodes,
					d.Status.ReadyNodes,
					d.Status.CurrentHeight)
			}
			w.Flush()

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Filter by namespace (empty = all namespaces)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: json")

	return cmd
}

// newDeprecatedStartCmd returns a hidden "start" command that tells users to use "dvb node start --all".
func newDeprecatedStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "start",
		Short:      "Deprecated: use 'dvb node start --all'",
		Hidden:     true,
		Deprecated: "use 'dvb node start --all' instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("'dvb start' has been replaced by 'dvb node start --all'\n\nUsage:\n  dvb node start --all            # start all nodes\n  dvb node start validator-0      # start a single node")
		},
	}
	return cmd
}

// newDeprecatedStopCmd returns a hidden "stop" command that tells users to use "dvb node stop --all".
func newDeprecatedStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "stop",
		Short:      "Deprecated: use 'dvb node stop --all'",
		Hidden:     true,
		Deprecated: "use 'dvb node stop --all' instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("'dvb stop' has been replaced by 'dvb node stop --all'\n\nUsage:\n  dvb node stop --all             # stop all nodes\n  dvb node stop validator-0       # stop a single node")
		},
	}
	return cmd
}

// printJSON marshals v to indented JSON and writes it to stdout.
func printJSON(v interface{}) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

// getBinaryNameFromPlugin returns the CLI binary name for a given plugin.
// Falls back to "gaiad" if plugin is unknown.
func getBinaryNameFromPlugin(plugin string) string {
	switch plugin {
	case "stable":
		return "stabled"
	case "cosmos", "gaia":
		return "gaiad"
	case "osmosis":
		return "osmosisd"
	default:
		return "gaiad"
	}
}

// pollStartStatus polls devnet status until Running phase is reached.
func pollStartStatus(ctx context.Context, ns, name string) error {
	return pollStartStatusWithClient(ctx, ns, name, daemonClient, 2*time.Second)
}

// pollStartStatusWithClient is the testable implementation of pollStartStatus.
func pollStartStatusWithClient(ctx context.Context, ns, name string, client devnetGetter, pollInterval time.Duration) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	spinner := output.NewStatusSpinner()
	spinner.Start(fmt.Sprintf("Phase: Pending | Starting %s...", name))
	defer spinner.Stop()

	for {
		select {
		case <-ctx.Done():
			spinner.StopWithNewline()
			return ctx.Err()
		case <-ticker.C:
			devnet, err := client.GetDevnet(ctx, ns, name)
			if err != nil {
				spinner.StopWithNewline()
				return err
			}

			// Update spinner message
			msg := devnet.Status.Message
			if msg == "" {
				msg = devnet.Status.Phase
			}
			spinner.Update(fmt.Sprintf("Phase: %s | %s | Nodes: %d/%d",
				devnet.Status.Phase, msg, devnet.Status.ReadyNodes, devnet.Status.Nodes))

			switch devnet.Status.Phase {
			case types.PhaseRunning:
				spinner.StopWithNewline()
				fmt.Fprintf(os.Stderr, "\n")
				color.Green("✓ Devnet %q is running", name)
				return nil
			case types.PhaseDegraded:
				spinner.StopWithNewline()
				return fmt.Errorf("devnet degraded: %s", devnet.Status.Message)
			case types.PhaseStopped:
				spinner.StopWithNewline()
				return fmt.Errorf("devnet stopped unexpectedly: %s", devnet.Status.Message)
			case types.PhasePending, types.PhaseProvisioning:
				// Transitional states - continue polling
			}
		}
	}
}

// runStartTUI shows interactive progress using TUI.
// Uses spinner via pollStartStatus. Can be enhanced with BubbleTea later.
func runStartTUI(ctx context.Context, ns, name string) error {
	// Uses spinner via pollStartStatus -> pollStartStatusWithClient
	// Can be enhanced with BubbleTea TUI later following provision.go:693
	return pollStartStatus(ctx, ns, name)
}
