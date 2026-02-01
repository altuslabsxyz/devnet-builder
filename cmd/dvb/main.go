// cmd/dvb/main.go
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/dvbcontext"
	"github.com/altuslabsxyz/devnet-builder/internal/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	standalone     bool
	daemonClient   *client.Client
	currentContext *dvbcontext.Context
	dimColor       = color.New(color.Faint)
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

			// Try to connect to daemon
			c, err := client.New()
			if err == nil {
				daemonClient = c
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

	// Add commands
	rootCmd.AddCommand(
		newVersionCmd(),
		newDaemonCmd(),
		newUseCmd(),
		newStatusCmd(),
		newGetCmd(),
		newDeleteCmd(),
		newDiffCmd(),
		newBuildCmd(),
		newDeployCmd(), // deprecated
		newListCmd(),
		newDescribeCmd(),
		newStartCmd(),
		newStopCmd(),
		newDestroyCmd(), // deprecated
		newNodeCmd(),
		newUpgradeCmd(),
		newTxCmd(),
		newGovCmd(),
		newGenesisCmd(),
		newProvisionCmd(),
		newPluginsCmd(),
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
		newDaemonLogsCmd(),
	)

	return cmd
}

func newDeployCmd() *cobra.Command {
	var (
		namespace  string
		plugin     string
		validators int
		fullNodes  int
		mode       string
	)

	cmd := &cobra.Command{
		Use:        "deploy [name]",
		Short:      "Deploy a new devnet",
		Deprecated: "use 'dvb provision -f <file>' instead",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			spec := &v1.DevnetSpec{
				Plugin:     plugin,
				Validators: int32(validators),
				FullNodes:  int32(fullNodes),
				Mode:       mode,
			}

			devnet, err := daemonClient.CreateDevnet(cmd.Context(), namespace, name, spec, nil)
			if err != nil {
				return err
			}

			color.Green("✓ Devnet %q created", devnet.Metadata.Name)
			fmt.Printf("  Phase: %s\n", devnet.Status.Phase)
			fmt.Printf("  Plugin: %s\n", devnet.Spec.Plugin)
			fmt.Printf("  Validators: %d\n", devnet.Spec.Validators)

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")
	cmd.Flags().StringVar(&plugin, "plugin", "stable", "Network plugin")
	cmd.Flags().IntVar(&validators, "validators", 4, "Number of validators")
	cmd.Flags().IntVar(&fullNodes, "full-nodes", 0, "Number of full nodes")
	cmd.Flags().StringVar(&mode, "mode", "docker", "Execution mode (docker or local)")

	return cmd
}

func newListCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all devnets",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnets, err := daemonClient.ListDevnets(cmd.Context(), namespace)
			if err != nil {
				return err
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

	return cmd
}

func newStartCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "start [devnet]",
		Short: "Start a stopped devnet",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			if len(args) > 0 {
				explicitDevnet = args[0]
			}

			ns, name, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			devnet, err := daemonClient.StartDevnet(cmd.Context(), ns, name)
			if err != nil {
				return err
			}

			color.Green("✓ Devnet %q starting", devnet.Metadata.Name)
			fmt.Printf("  Phase: %s\n", devnet.Status.Phase)

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func newStopCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "stop [devnet]",
		Short: "Stop a running devnet",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			if len(args) > 0 {
				explicitDevnet = args[0]
			}

			ns, name, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			printContextHeader(explicitDevnet, currentContext)

			devnet, err := daemonClient.StopDevnet(cmd.Context(), ns, name)
			if err != nil {
				return err
			}

			color.Green("✓ Devnet %q stopped", devnet.Metadata.Name)
			fmt.Printf("  Phase: %s\n", devnet.Status.Phase)

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")

	return cmd
}

func newDestroyCmd() *cobra.Command {
	var (
		namespace string
		force     bool
		dataDir   string
	)

	cmd := &cobra.Command{
		Use:   "destroy [devnet]",
		Short: "Destroy a devnet and remove all its data",
		Long: `Destroy a devnet by stopping all nodes and removing all associated data.

This is a destructive operation that cannot be undone. You will be asked
to type the devnet name to confirm unless --force is specified.

In standalone mode, this removes the devnet directory. In daemon mode,
it uses the daemon to properly clean up all resources.

Examples:
  # Destroy a devnet (with type-to-confirm prompt)
  dvb destroy my-devnet

  # Destroy without confirmation (use with caution!)
  dvb destroy my-devnet --force

  # List available devnets
  dvb destroy`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine data directory for standalone mode
			baseDataDir := dataDir
			if baseDataDir == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				baseDataDir = filepath.Join(homeDir, ".devnet-builder")
			}
			devnetsDir := filepath.Join(baseDataDir, "devnets")

			// Resolve devnet from args or context
			var explicitDevnet string
			if len(args) > 0 {
				explicitDevnet = args[0]
			}

			ns, name, err := dvbcontext.Resolve(explicitDevnet, namespace, currentContext)
			if err != nil {
				// If no devnet specified and no context, show available devnets
				if explicitDevnet == "" {
					// If daemon is available, use smart suggestions
					if daemonClient != nil {
						suggestion := dvbcontext.SuggestUsage(daemonClient)
						return dvbcontext.NewNoDevnetError(suggestion)
					}
					// Fall back to listing from filesystem
					return listDevnetsForDestroy(devnetsDir)
				}
				return err
			}

			// Try daemon first if available
			if daemonClient != nil && !standalone {
				if !force {
					fmt.Printf("Are you sure you want to destroy devnet %q? [y/N] ", name)
					var response string
					if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
						fmt.Println("Cancelled")
						return nil
					}
				}

				err := daemonClient.DeleteDevnet(cmd.Context(), ns, name)
				if err != nil {
					return err
				}

				color.Green("✓ Devnet %q destroyed", name)
				return nil
			}

			// Standalone mode: destroy locally
			devnetPath := filepath.Join(devnetsDir, name)
			info, err := os.Stat(devnetPath)
			if os.IsNotExist(err) {
				return fmt.Errorf("devnet '%s' not found in %s", name, devnetsDir)
			}
			if err != nil {
				return fmt.Errorf("failed to check devnet: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("'%s' is not a valid devnet directory", name)
			}

			// Confirm destruction with type-to-confirm (safer than y/N)
			if !force {
				confirmed, err := ConfirmDestroy(name)
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			// Remove devnet directory
			fmt.Fprintf(os.Stderr, "Removing devnet data at %s...\n", devnetPath)
			if err := os.RemoveAll(devnetPath); err != nil {
				return fmt.Errorf("failed to remove devnet directory: %w", err)
			}

			color.Green("✔ Devnet '%s' destroyed successfully", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt (dangerous!)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Base data directory (default: ~/.devnet-builder)")

	return cmd
}

// listDevnetsForDestroy lists all available devnets for the destroy command
func listDevnetsForDestroy(devnetsDir string) error {
	entries, err := os.ReadDir(devnetsDir)
	if os.IsNotExist(err) {
		fmt.Println("No devnets found.")
		fmt.Println()
		fmt.Println("Create a devnet with: dvb provision -i")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read devnets directory: %w", err)
	}

	var devnets []string
	for _, entry := range entries {
		if entry.IsDir() {
			devnets = append(devnets, entry.Name())
		}
	}

	if len(devnets) == 0 {
		fmt.Println("No devnets found.")
		fmt.Println()
		fmt.Println("Create a devnet with: dvb provision -i")
		return nil
	}

	fmt.Println("Available devnets:")
	fmt.Println()
	for _, name := range devnets {
		fmt.Printf("  • %s\n", name)
	}
	fmt.Println()
	fmt.Println("To destroy a devnet, run: dvb destroy <name>")

	return nil
}

func newDescribeCmd() *cobra.Command {
	var (
		namespace    string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "describe [devnet]",
		Short: "Show detailed devnet information",
		Long: `Show detailed information about a devnet including status conditions,
recent events, and node details. Similar to kubectl describe.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var explicitDevnet string
			if len(args) > 0 {
				explicitDevnet = args[0]
			}

			ns, name, err := resolveWithSuggestions(explicitDevnet, namespace)
			if err != nil {
				return err
			}

			devnet, err := daemonClient.GetDevnet(cmd.Context(), ns, name)
			if err != nil {
				return err
			}

			nodes, err := daemonClient.ListNodes(cmd.Context(), ns, name)
			if err != nil {
				// Don't fail if nodes can't be listed
				nodes = nil
			}

			// Check plugin availability for troubleshooting
			var pluginAvailable bool
			var registeredNetworks []*v1.NetworkSummary
			if devnet.Spec != nil && devnet.Spec.Plugin != "" {
				registeredNetworks, _ = daemonClient.ListNetworks(cmd.Context())
				for _, n := range registeredNetworks {
					if n.Name == devnet.Spec.Plugin {
						pluginAvailable = true
						break
					}
				}
			}

			if outputFormat == "yaml" {
				return printDescribeYAML(devnet, nodes)
			}

			printContextHeader(explicitDevnet, currentContext)
			formatDescribeOutput(os.Stdout, devnet, nodes, pluginAvailable, registeredNetworks)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (yaml)")
	return cmd
}

func formatDescribeOutput(w io.Writer, d *v1.Devnet, nodes []*v1.Node, pluginAvailable bool, registeredNetworks []*v1.NetworkSummary) {
	if d == nil {
		fmt.Fprintf(w, "No devnet data available\n")
		return
	}
	if d.Status == nil {
		d.Status = &v1.DevnetStatus{}
	}
	if d.Metadata == nil {
		d.Metadata = &v1.DevnetMetadata{}
	}
	if d.Spec == nil {
		d.Spec = &v1.DevnetSpec{}
	}

	// Phase with color
	phase := d.Status.Phase
	switch phase {
	case "Running":
		color.New(color.FgGreen).Fprintf(w, "● %s\n", phase)
	case "Pending", "Provisioning":
		color.New(color.FgYellow).Fprintf(w, "◐ %s\n", phase)
	case "Stopped":
		color.New(color.FgWhite).Fprintf(w, "○ %s\n", phase)
	case "Degraded":
		color.New(color.FgRed).Fprintf(w, "◑ %s\n", phase)
	default:
		fmt.Fprintf(w, "? %s\n", phase)
	}

	// Basic info
	fmt.Fprintf(w, "\nName:         %s\n", d.Metadata.Name)
	fmt.Fprintf(w, "Namespace:    %s\n", d.Metadata.Namespace)
	if d.Metadata.CreatedAt != nil {
		age := time.Since(d.Metadata.CreatedAt.AsTime()).Round(time.Second)
		fmt.Fprintf(w, "Age:          %s\n", age)
	}
	fmt.Fprintf(w, "Plugin:       %s\n", d.Spec.Plugin)
	fmt.Fprintf(w, "Mode:         %s\n", d.Spec.Mode)
	fmt.Fprintf(w, "Validators:   %d\n", d.Spec.Validators)
	if d.Spec.FullNodes > 0 {
		fmt.Fprintf(w, "Full Nodes:   %d\n", d.Spec.FullNodes)
	}

	// Status section
	fmt.Fprintf(w, "\nStatus:\n")
	fmt.Fprintf(w, "  Nodes:        %d/%d ready\n", d.Status.ReadyNodes, d.Status.Nodes)
	if d.Status.CurrentHeight > 0 {
		fmt.Fprintf(w, "  Height:       %d\n", d.Status.CurrentHeight)
	}
	if d.Status.Subnet > 0 {
		fmt.Fprintf(w, "  Subnet:       127.0.%d.0/24\n", d.Status.Subnet)
	}
	if d.Status.SdkVersion != "" {
		fmt.Fprintf(w, "  SDK Version:  %s\n", d.Status.SdkVersion)
	}
	if d.Status.Message != "" {
		fmt.Fprintf(w, "  Message:      %s\n", d.Status.Message)
	}

	// Conditions section
	if len(d.Status.Conditions) > 0 {
		fmt.Fprintf(w, "\nConditions:\n")
		fmt.Fprintf(w, "  %-20s %-8s %-25s %s\n", "TYPE", "STATUS", "REASON", "MESSAGE")
		for _, c := range d.Status.Conditions {
			status := c.Status
			if c.Status == "True" {
				status = color.GreenString("True")
			} else if c.Status == "False" {
				status = color.RedString("False")
			}
			fmt.Fprintf(w, "  %-20s %-8s %-25s %s\n", c.Type, status, c.Reason, c.Message)
		}
	}

	// Nodes section
	if len(nodes) > 0 {
		// Check if any node has an IP address
		hasAddresses := false
		for _, n := range nodes {
			if n.Spec != nil && n.Spec.Address != "" {
				hasAddresses = true
				break
			}
		}

		fmt.Fprintf(w, "\nNodes:\n")
		if hasAddresses {
			fmt.Fprintf(w, "  %-6s %-10s %-14s %-18s %-10s\n", "INDEX", "PHASE", "IP", "RPC", "HEIGHT")
		} else {
			fmt.Fprintf(w, "  %-6s %-10s %-10s %-10s %-8s %s\n", "INDEX", "ROLE", "PHASE", "HEIGHT", "RESTARTS", "MESSAGE")
		}
		for _, n := range nodes {
			nodePhase := n.Status.Phase
			switch nodePhase {
			case "Running":
				nodePhase = color.GreenString(nodePhase)
			case "Pending", "Starting":
				nodePhase = color.YellowString(nodePhase)
			case "Crashed":
				nodePhase = color.RedString(nodePhase)
			}
			if hasAddresses {
				addr := n.Spec.Address
				if addr == "" {
					addr = "-"
				}
				rpc := "-"
				if addr != "-" {
					rpc = fmt.Sprintf("%s:26657", addr)
				}
				fmt.Fprintf(w, "  %-6d %-10s %-14s %-18s %-10d\n",
					n.Metadata.Index,
					nodePhase,
					addr,
					rpc,
					n.Status.BlockHeight,
				)
			} else {
				msg := n.Status.Message
				if len(msg) > 30 {
					msg = msg[:27] + "..."
				}
				fmt.Fprintf(w, "  %-6d %-10s %-10s %-10d %-8d %s\n",
					n.Metadata.Index,
					n.Spec.Role,
					nodePhase,
					n.Status.BlockHeight,
					n.Status.RestartCount,
					msg,
				)
			}
		}
	}

	// Endpoints section - show connection info when we have addresses
	if len(nodes) > 0 && nodes[0].Spec != nil && nodes[0].Spec.Address != "" {
		firstNodeAddr := nodes[0].Spec.Address
		fmt.Fprintf(w, "\nEndpoints:\n")
		fmt.Fprintf(w, "  RPC:  http://%s:26657\n", firstNodeAddr)
		fmt.Fprintf(w, "  REST: http://%s:1317\n", firstNodeAddr)
		fmt.Fprintf(w, "  gRPC: %s:9090\n", firstNodeAddr)

		fmt.Fprintf(w, "\nConnect with CLI:\n")
		fmt.Fprintf(w, "  %s status --node tcp://%s:26657\n", getBinaryNameFromPlugin(d.Spec.Plugin), firstNodeAddr)
	}

	// Events section
	if len(d.Status.Events) > 0 {
		fmt.Fprintf(w, "\nEvents:\n")
		fmt.Fprintf(w, "  %-8s %-20s %-20s %s\n", "TYPE", "REASON", "AGE", "MESSAGE")
		for _, e := range d.Status.Events {
			eventType := e.Type
			if e.Type == "Warning" {
				eventType = color.YellowString("Warning")
			}
			age := "Unknown"
			if e.Timestamp != nil {
				age = time.Since(e.Timestamp.AsTime()).Round(time.Second).String()
			}
			// Truncate and clean message for readability
			msg := e.Message
			// Replace newlines and multiple spaces with single space
			msg = strings.ReplaceAll(msg, "\n", " ")
			msg = strings.ReplaceAll(msg, "\r", " ")
			// Collapse multiple spaces
			for strings.Contains(msg, "  ") {
				msg = strings.ReplaceAll(msg, "  ", " ")
			}
			// Truncate to reasonable length
			if len(msg) > 120 {
				msg = msg[:117] + "..."
			}
			fmt.Fprintf(w, "  %-8s %-20s %-20s %s\n", eventType, e.Reason, age, msg)
		}
	}

	// Troubleshooting section - show when provisioning is stuck or failed
	if phase == "Provisioning" || phase == "Degraded" || phase == "Pending" {
		fmt.Fprintf(w, "\n")
		color.New(color.FgYellow).Fprintf(w, "Troubleshooting:\n")

		// Check if plugin is available
		if d.Spec.Plugin != "" && !pluginAvailable {
			color.New(color.FgRed).Fprintf(w, "  ⚠ Plugin '%s' not found!\n", d.Spec.Plugin)
			fmt.Fprintf(w, "    The network plugin is not registered with the daemon.\n")
			fmt.Fprintf(w, "    \n")
			fmt.Fprintf(w, "    To fix this:\n")
			fmt.Fprintf(w, "      1. Install the plugin: dvb plugin install %s\n", d.Spec.Plugin)
			fmt.Fprintf(w, "      2. Restart the daemon: pkill devnetd && devnetd &\n")
			fmt.Fprintf(w, "      3. Delete and recreate the devnet\n")
			fmt.Fprintf(w, "    \n")
			if len(registeredNetworks) > 0 {
				fmt.Fprintf(w, "    Available plugins: ")
				names := make([]string, len(registeredNetworks))
				for i, n := range registeredNetworks {
					names[i] = n.Name
				}
				fmt.Fprintf(w, "%s\n", strings.Join(names, ", "))
			} else {
				fmt.Fprintf(w, "    No plugins currently installed.\n")
			}
		} else if phase == "Provisioning" {
			// Plugin exists but provisioning is stuck
			fmt.Fprintf(w, "  Provisioning appears to be in progress or stuck.\n")
			fmt.Fprintf(w, "    \n")
			fmt.Fprintf(w, "    Debug steps:\n")
			fmt.Fprintf(w, "      1. Check daemon logs: dvb daemon logs -f\n")
			fmt.Fprintf(w, "      2. Check daemon status: dvb daemon status\n")
			fmt.Fprintf(w, "      3. If stuck, try: dvb delete %s && dvb provision -i\n", d.Metadata.Name)
		} else if phase == "Degraded" {
			// Provisioning failed
			fmt.Fprintf(w, "  Provisioning has failed.\n")
			fmt.Fprintf(w, "    \n")
			fmt.Fprintf(w, "    Debug steps:\n")
			fmt.Fprintf(w, "      1. Check daemon logs for errors: dvb daemon logs --level error\n")
			fmt.Fprintf(w, "      2. Check the conditions above for specific failure reasons\n")
			fmt.Fprintf(w, "      3. Fix the issue and recreate: dvb delete %s && dvb provision -i\n", d.Metadata.Name)
		}
	}
}

func printDescribeYAML(d *v1.Devnet, nodes []*v1.Node) error {
	data := map[string]interface{}{
		"devnet": d,
		"nodes":  nodes,
	}
	out, err := yaml.Marshal(data)
	if err != nil {
		return err
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
