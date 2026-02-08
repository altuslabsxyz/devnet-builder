// cmd/dvb/status.go
package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/dvbcontext"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// statusOptions holds options for the status command
type statusOptions struct {
	verbose   bool
	events    bool
	namespace string
}

func newStatusCmd() *cobra.Command {
	opts := &statusOptions{}

	cmd := &cobra.Command{
		Use:   "status [devnet]",
		Short: "Show current devnet status",
		Long: `Show the current devnet status based on context.

When context is set (via dvb use <devnet>), shows detailed status including:
  - Devnet phase (Running/Stopped/etc.)
  - Node list with status
  - Quick actions available

When no context is set, shows available devnets and how to set context.

Use --verbose/-v for detailed output including conditions, events, and troubleshooting.

Examples:
  # Show status of current context
  dvb status

  # Show detailed status with conditions and events
  dvb status -v

  # Show status of a specific devnet
  dvb status my-devnet

  # Set context first, then show status
  dvb use my-devnet
  dvb status`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var explicitDevnet string
			if len(args) > 0 {
				explicitDevnet = args[0]
			}
			return runStatus(cmd, explicitDevnet, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Show detailed output (conditions, events, troubleshooting)")
	cmd.Flags().BoolVar(&opts.events, "events", false, "Show recent events")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Namespace (defaults to context or server default)")

	return cmd
}

func runStatus(cmd *cobra.Command, explicitDevnet string, opts *statusOptions) error {
	// Load context
	ctx, err := dvbcontext.Load()
	if err != nil {
		return fmt.Errorf("failed to load context: %w", err)
	}

	// Check daemon status
	daemonRunning := client.IsDaemonRunning()

	// Case 1: Daemon not running
	if !daemonRunning {
		return printStatusDaemonNotRunning(ctx)
	}

	// Case 2: Explicit devnet provided or context set
	if explicitDevnet != "" || ctx != nil {
		ns, name, err := resolveWithSuggestions(explicitDevnet, opts.namespace)
		if err != nil {
			return err
		}
		printContextHeader(explicitDevnet, currentContext)
		return printStatusForDevnet(cmd, ns, name, opts)
	}

	// Case 3: No context set
	return printStatusNoContext(cmd)
}

// printStatusDaemonNotRunning handles the case when daemon is not running
func printStatusDaemonNotRunning(ctx *dvbcontext.Context) error {
	// Show context if set
	if ctx != nil {
		fmt.Print("Context: ")
		if ctx.Namespace == "default" {
			fmt.Println(ctx.Devnet)
		} else {
			fmt.Printf("%s/%s\n", ctx.Namespace, ctx.Devnet)
		}
		fmt.Println()
	} else {
		fmt.Println("Context: (not set)")
		fmt.Println()
	}

	color.Yellow("Daemon: Not running")
	fmt.Println()
	fmt.Println("Start the daemon with:")
	fmt.Println("  devnetd &")
	fmt.Println()
	fmt.Println("Or run in foreground:")
	fmt.Println("  devnetd")

	return nil
}

// printStatusNoContext handles the case when no context is set
func printStatusNoContext(cmd *cobra.Command) error {
	fmt.Println("Context: (not set)")
	fmt.Println()

	// List available devnets
	devnets, err := daemonClient.ListDevnets(cmd.Context(), "")
	if err != nil {
		return fmt.Errorf("failed to list devnets: %w", err)
	}

	if len(devnets) == 0 {
		fmt.Println("No devnets found.")
		fmt.Println()
		fmt.Println("Create a devnet with:")
		fmt.Println("  dvb provision -i")
		return nil
	}

	fmt.Println("Available devnets:")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tSTATUS\tNODES")
	for _, d := range devnets {
		name := d.Metadata.Name
		if d.Metadata.Namespace != "default" {
			name = fmt.Sprintf("%s/%s", d.Metadata.Namespace, d.Metadata.Name)
		}
		phase := formatPhase(d.Status.Phase)
		nodes := fmt.Sprintf("%d/%d ready", d.Status.ReadyNodes, d.Status.Nodes)
		fmt.Fprintf(w, "  %s\t%s\t%s\n", name, phase, nodes)
	}
	w.Flush()

	fmt.Println()
	fmt.Println("Set context with:")
	fmt.Println("  dvb use <devnet>")

	return nil
}

// printStatusForDevnet handles showing status for a specific devnet
func printStatusForDevnet(cmd *cobra.Command, namespace, name string, opts *statusOptions) error {
	// Get devnet details
	devnet, err := daemonClient.GetDevnet(cmd.Context(), namespace, name)
	if err != nil {
		color.Red("Error: devnet not found")
		fmt.Println()
		fmt.Println("The devnet in your context no longer exists.")
		fmt.Println("Clear context with: dvb use -")
		fmt.Println("Or set a new context: dvb use <devnet>")
		return nil
	}

	// Get nodes
	nodes, err := daemonClient.ListNodes(cmd.Context(), namespace, name)
	if err != nil {
		nodes = nil
	}

	// Verbose mode: show full describe-style output
	if opts.verbose {
		return printVerboseStatus(cmd, devnet, nodes)
	}

	// Print devnet status
	printDevnetStatus(devnet)

	// Print nodes summary
	if len(nodes) > 0 {
		fmt.Println()
		printNodesSummary(nodes)
	}

	// Show events if requested
	if opts.events && devnet.Status != nil && len(devnet.Status.Events) > 0 {
		fmt.Println()
		printEvents(devnet.Status.Events)
	}

	// Print quick actions
	fmt.Println()
	printQuickActions(devnet)

	return nil
}

// printVerboseStatus shows detailed status similar to describe command
func printVerboseStatus(cmd *cobra.Command, devnet *v1.Devnet, nodes []*v1.Node) error {
	if devnet == nil {
		fmt.Println("No devnet data available")
		return nil
	}
	if devnet.Status == nil {
		devnet.Status = &v1.DevnetStatus{}
	}
	if devnet.Metadata == nil {
		devnet.Metadata = &v1.DevnetMetadata{}
	}
	if devnet.Spec == nil {
		devnet.Spec = &v1.DevnetSpec{}
	}

	// Phase with color
	phase := devnet.Status.Phase
	switch phase {
	case "Running":
		color.Green("● %s", phase)
	case "Pending", "Provisioning":
		color.Yellow("◐ %s", phase)
	case "Stopped":
		color.White("○ %s", phase)
	case "Degraded":
		color.Red("◑ %s", phase)
	default:
		fmt.Printf("? %s", phase)
	}
	fmt.Println()

	// Basic info
	fmt.Printf("\nName:         %s\n", devnet.Metadata.Name)
	fmt.Printf("Namespace:    %s\n", devnet.Metadata.Namespace)
	if devnet.Metadata.CreatedAt != nil {
		age := time.Since(devnet.Metadata.CreatedAt.AsTime()).Round(time.Second)
		fmt.Printf("Age:          %s\n", formatAge(age))
	}
	fmt.Printf("Plugin:       %s\n", devnet.Spec.Plugin)
	fmt.Printf("Mode:         %s\n", devnet.Spec.Mode)
	fmt.Printf("Validators:   %d\n", devnet.Spec.Validators)
	if devnet.Spec.FullNodes > 0 {
		fmt.Printf("Full Nodes:   %d\n", devnet.Spec.FullNodes)
	}

	// Status section
	fmt.Printf("\nStatus:\n")
	fmt.Printf("  Nodes:        %d/%d ready\n", devnet.Status.ReadyNodes, devnet.Status.Nodes)
	if devnet.Status.CurrentHeight > 0 {
		fmt.Printf("  Height:       %d\n", devnet.Status.CurrentHeight)
	}
	if devnet.Status.Subnet > 0 {
		fmt.Printf("  Subnet:       127.0.%d.0/24\n", devnet.Status.Subnet)
	}
	if devnet.Status.SdkVersion != "" {
		fmt.Printf("  SDK Version:  %s\n", devnet.Status.SdkVersion)
	}
	if devnet.Status.Message != "" {
		fmt.Printf("  Message:      %s\n", devnet.Status.Message)
	}

	// Conditions section
	if len(devnet.Status.Conditions) > 0 {
		fmt.Printf("\nConditions:\n")
		fmt.Printf("  %-20s %-8s %-25s %s\n", "TYPE", "STATUS", "REASON", "MESSAGE")
		for _, c := range devnet.Status.Conditions {
			status := c.Status
			if c.Status == "True" {
				status = color.GreenString("True")
			} else if c.Status == "False" {
				status = color.RedString("False")
			}
			fmt.Printf("  %-20s %-8s %-25s %s\n", c.Type, status, c.Reason, c.Message)
		}
	}

	// Nodes section
	if len(nodes) > 0 {
		printVerboseNodes(nodes)
	}

	// Endpoints section
	if len(nodes) > 0 && nodes[0].Spec != nil && nodes[0].Spec.Address != "" {
		firstNodeAddr := nodes[0].Spec.Address
		fmt.Printf("\nEndpoints:\n")
		fmt.Printf("  RPC:  http://%s:26657\n", firstNodeAddr)
		fmt.Printf("  REST: http://%s:1317\n", firstNodeAddr)
		fmt.Printf("  gRPC: %s:9090\n", firstNodeAddr)

		fmt.Printf("\nConnect with CLI:\n")
		fmt.Printf("  %s status --node tcp://%s:26657\n", getBinaryNameFromPlugin(devnet.Spec.Plugin), firstNodeAddr)
	}

	// Events section
	if len(devnet.Status.Events) > 0 {
		fmt.Println()
		printEvents(devnet.Status.Events)
	}

	// Troubleshooting section
	if phase == "Provisioning" || phase == "Degraded" || phase == "Pending" {
		printTroubleshooting(cmd, devnet)
	}

	return nil
}

// printVerboseNodes prints detailed node table
func printVerboseNodes(nodes []*v1.Node) {
	hasAddresses := false
	for _, n := range nodes {
		if n.Spec != nil && n.Spec.Address != "" {
			hasAddresses = true
			break
		}
	}

	fmt.Printf("\nNodes:\n")
	if hasAddresses {
		fmt.Printf("  %-6s %-10s %-14s %-18s %-10s\n", "INDEX", "PHASE", "IP", "RPC", "HEIGHT")
	} else {
		fmt.Printf("  %-6s %-10s %-10s %-10s %-8s %s\n", "INDEX", "ROLE", "PHASE", "HEIGHT", "RESTARTS", "MESSAGE")
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
			fmt.Printf("  %-6d %-10s %-14s %-18s %-10d\n",
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
			fmt.Printf("  %-6d %-10s %-10s %-10d %-8d %s\n",
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

// printEvents prints the events section
func printEvents(events []*v1.Event) {
	fmt.Printf("Events:\n")
	fmt.Printf("  %-8s %-20s %-20s %s\n", "TYPE", "REASON", "AGE", "MESSAGE")
	for _, e := range events {
		eventType := e.Type
		if e.Type == "Warning" {
			eventType = color.YellowString("Warning")
		}
		age := "Unknown"
		if e.Timestamp != nil {
			age = time.Since(e.Timestamp.AsTime()).Round(time.Second).String()
		}
		msg := e.Message
		// Clean up message
		msg = strings.ReplaceAll(msg, "\n", " ")
		msg = strings.ReplaceAll(msg, "\r", " ")
		for strings.Contains(msg, "  ") {
			msg = strings.ReplaceAll(msg, "  ", " ")
		}
		if len(msg) > 120 {
			msg = msg[:117] + "..."
		}
		fmt.Printf("  %-8s %-20s %-20s %s\n", eventType, e.Reason, age, msg)
	}
}

// printTroubleshooting prints troubleshooting suggestions
func printTroubleshooting(cmd *cobra.Command, devnet *v1.Devnet) {
	phase := devnet.Status.Phase

	// Check plugin availability
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

	fmt.Println()
	color.Yellow("Troubleshooting:")

	if devnet.Spec.Plugin != "" && !pluginAvailable {
		color.Red("  ⚠ Plugin '%s' not found!", devnet.Spec.Plugin)
		fmt.Println("    The network plugin is not registered with the daemon.")
		fmt.Println()
		fmt.Println("    To fix this:")
		fmt.Printf("      1. Install the plugin: dvb daemon plugins install %s\n", devnet.Spec.Plugin)
		fmt.Println("      2. Restart the daemon: pkill devnetd && devnetd &")
		fmt.Println("      3. Delete and recreate the devnet")
		fmt.Println()
		if len(registeredNetworks) > 0 {
			names := make([]string, len(registeredNetworks))
			for i, n := range registeredNetworks {
				names[i] = n.Name
			}
			fmt.Printf("    Available plugins: %s\n", strings.Join(names, ", "))
		} else {
			fmt.Println("    No plugins currently installed.")
		}
	} else if phase == "Provisioning" {
		fmt.Println("  Provisioning appears to be in progress or stuck.")
		fmt.Println()
		fmt.Println("    Debug steps:")
		fmt.Println("      1. Check daemon logs: dvb daemon logs -f")
		fmt.Println("      2. Check daemon status: dvb daemon status")
		fmt.Printf("      3. If stuck, try: dvb delete %s && dvb provision\n", devnet.Metadata.Name)
	} else if phase == "Degraded" {
		fmt.Println("  Provisioning has failed.")
		fmt.Println()
		fmt.Println("    Debug steps:")
		fmt.Println("      1. Check daemon logs for errors: dvb daemon logs --level error")
		fmt.Println("      2. Check the conditions above for specific failure reasons")
		fmt.Printf("      3. Fix the issue and recreate: dvb delete %s && dvb provision\n", devnet.Metadata.Name)
	}
}

// printDevnetStatus prints the devnet status section
func printDevnetStatus(d *v1.Devnet) {
	if d.Status == nil {
		d.Status = &v1.DevnetStatus{}
	}

	// Status line with icon
	phase := d.Status.Phase
	switch phase {
	case "Running":
		color.Green("Status: Running")
	case "Stopped":
		color.White("Status: Stopped")
	case "Pending", "Provisioning":
		color.Yellow("Status: %s", phase)
	case "Degraded":
		color.Red("Status: Degraded")
	default:
		fmt.Printf("Status: %s\n", phase)
	}

	// Show additional info
	if d.Status.CurrentHeight > 0 {
		fmt.Printf("Height: %d\n", d.Status.CurrentHeight)
	}

	// Show age
	if d.Metadata != nil && d.Metadata.CreatedAt != nil {
		age := time.Since(d.Metadata.CreatedAt.AsTime()).Round(time.Second)
		fmt.Printf("Age:    %s\n", formatAge(age))
	}

	// Show subnet info
	if d.Status.Subnet > 0 {
		fmt.Printf("Subnet: 127.0.%d.0/24\n", d.Status.Subnet)
	}

	// Show plugin/network info
	if d.Spec != nil && d.Spec.Plugin != "" {
		fmt.Printf("Plugin: %s\n", d.Spec.Plugin)
	}
}

// printNodesSummary prints a compact node status table
func printNodesSummary(nodes []*v1.Node) {
	fmt.Println("Nodes:")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Check if any node has an address to determine table format
	hasAddresses := false
	for _, n := range nodes {
		if n.Spec != nil && n.Spec.Address != "" {
			hasAddresses = true
			break
		}
	}

	if hasAddresses {
		fmt.Fprintln(w, "  INDEX\tSTATUS\tIP\tRPC\tHEIGHT")
	} else {
		fmt.Fprintln(w, "  INDEX\tROLE\tSTATUS\tHEIGHT")
	}

	for _, n := range nodes {
		phase := n.Status.Phase
		var statusStr string
		switch phase {
		case "Running":
			statusStr = color.GreenString("Running")
		case "Stopped":
			statusStr = color.WhiteString("Stopped")
		case "Pending", "Starting":
			statusStr = color.YellowString(phase)
		case "Crashed":
			statusStr = color.RedString("Crashed")
		default:
			statusStr = phase
		}

		height := "-"
		if n.Status.BlockHeight > 0 {
			height = fmt.Sprintf("%d", n.Status.BlockHeight)
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
			fmt.Fprintf(w, "  %d\t%s\t%s\t%s\t%s\n",
				n.Metadata.Index,
				statusStr,
				addr,
				rpc,
				height,
			)
		} else {
			fmt.Fprintf(w, "  %d\t%s\t%s\t%s\n",
				n.Metadata.Index,
				n.Spec.Role,
				statusStr,
				height,
			)
		}
	}
	w.Flush()
}

// printQuickActions prints suggested actions based on devnet state
func printQuickActions(d *v1.Devnet) {
	fmt.Println("Quick actions:")

	switch d.Status.Phase {
	case "Running":
		fmt.Println("  dvb node stop --all   # Stop all nodes")
		fmt.Println("  dvb logs -f           # Follow logs")
		fmt.Println("  dvb status -v         # Show detailed info")
		fmt.Println("  dvb node list         # List all nodes")
	case "Stopped":
		fmt.Println("  dvb node start --all  # Start all nodes")
		fmt.Println("  dvb status -v         # Show detailed info")
		fmt.Println("  dvb delete            # Delete the devnet")
	case "Pending", "Provisioning":
		fmt.Println("  dvb status -v      # Show detailed status")
		fmt.Println("  dvb daemon logs   # Check daemon logs")
	case "Degraded":
		fmt.Println("  dvb status -v      # Show detailed status and troubleshooting")
		fmt.Println("  dvb daemon logs   # Check daemon logs for errors")
		fmt.Println("  dvb delete        # Delete and recreate")
	default:
		fmt.Println("  dvb status -v      # Show detailed info")
	}
}

// formatPhase returns a colored phase string
func formatPhase(phase string) string {
	switch phase {
	case "Running":
		return color.GreenString("Running")
	case "Stopped":
		return color.WhiteString("Stopped")
	case "Pending", "Provisioning":
		return color.YellowString(phase)
	case "Degraded":
		return color.RedString("Degraded")
	default:
		return phase
	}
}

// formatAge formats a duration into a human-readable age string
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
}
