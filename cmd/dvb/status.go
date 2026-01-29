// cmd/dvb/status.go
package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/dvbcontext"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current devnet status",
		Long: `Show the current devnet status based on context.

When context is set (via dvb use <devnet>), shows detailed status including:
  - Devnet phase (Running/Stopped/etc.)
  - Node list with status
  - Quick actions available

When no context is set, shows available devnets and how to set context.

Examples:
  # Show status of current context
  dvb status

  # Set context first, then show status
  dvb use my-devnet
  dvb status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd)
		},
	}

	return cmd
}

func runStatus(cmd *cobra.Command) error {
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

	// Case 2: No context set
	if ctx == nil {
		return printStatusNoContext(cmd)
	}

	// Case 3: Context set and daemon running
	return printStatusWithContext(cmd, ctx)
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

// printStatusWithContext handles the case when context is set and daemon is running
func printStatusWithContext(cmd *cobra.Command, ctx *dvbcontext.Context) error {
	// Print context header
	fmt.Print("Context: ")
	if ctx.Namespace == "default" {
		color.New(color.Bold).Println(ctx.Devnet)
	} else {
		color.New(color.Bold).Printf("%s/%s\n", ctx.Namespace, ctx.Devnet)
	}
	fmt.Println()

	// Get devnet details
	devnet, err := daemonClient.GetDevnet(cmd.Context(), ctx.Namespace, ctx.Devnet)
	if err != nil {
		color.Red("Error: devnet not found")
		fmt.Println()
		fmt.Println("The devnet in your context no longer exists.")
		fmt.Println("Clear context with: dvb use -")
		fmt.Println("Or set a new context: dvb use <devnet>")
		return nil
	}

	// Print devnet status
	printDevnetStatus(devnet)

	// Get and print nodes
	nodes, err := daemonClient.ListNodes(cmd.Context(), ctx.Namespace, ctx.Devnet)
	if err == nil && len(nodes) > 0 {
		fmt.Println()
		printNodesSummary(nodes)
	}

	// Print quick actions
	fmt.Println()
	printQuickActions(devnet)

	return nil
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

	// Show plugin/network info
	if d.Spec != nil && d.Spec.Plugin != "" {
		fmt.Printf("Plugin: %s\n", d.Spec.Plugin)
	}
}

// printNodesSummary prints a compact node status table
func printNodesSummary(nodes []*v1.Node) {
	fmt.Println("Nodes:")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  INDEX\tROLE\tSTATUS\tHEIGHT")

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

		fmt.Fprintf(w, "  %d\t%s\t%s\t%s\n",
			n.Metadata.Index,
			n.Spec.Role,
			statusStr,
			height,
		)
	}
	w.Flush()
}

// printQuickActions prints suggested actions based on devnet state
func printQuickActions(d *v1.Devnet) {
	fmt.Println("Quick actions:")

	switch d.Status.Phase {
	case "Running":
		fmt.Println("  dvb stop          # Stop the devnet")
		fmt.Println("  dvb logs -f       # Follow logs")
		fmt.Println("  dvb describe      # Show detailed info")
		fmt.Println("  dvb node list     # List all nodes")
	case "Stopped":
		fmt.Println("  dvb start         # Start the devnet")
		fmt.Println("  dvb describe      # Show detailed info")
		fmt.Println("  dvb delete        # Delete the devnet")
	case "Pending", "Provisioning":
		fmt.Println("  dvb describe      # Show detailed status")
		fmt.Println("  dvb daemon logs   # Check daemon logs")
	case "Degraded":
		fmt.Println("  dvb describe      # Show detailed status and troubleshooting")
		fmt.Println("  dvb daemon logs   # Check daemon logs for errors")
		fmt.Println("  dvb delete        # Delete and recreate")
	default:
		fmt.Println("  dvb describe      # Show detailed info")
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
