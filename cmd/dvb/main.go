// cmd/dvb/main.go
package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	standalone   bool
	daemonClient *client.Client
)

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
				return nil
			}

			// Daemon not running - fall back to standalone
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
		newApplyCmd(),
		newGetCmd(),
		newDeleteCmd(),
		newDiffCmd(),
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
		Deprecated: "use 'dvb apply -f <file>' instead",
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnet, err := daemonClient.StartDevnet(cmd.Context(), namespace, name)
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnet, err := daemonClient.StopDevnet(cmd.Context(), namespace, name)
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
	)

	cmd := &cobra.Command{
		Use:        "destroy [devnet]",
		Short:      "Destroy a devnet",
		Deprecated: "use 'dvb delete devnet <name>' or 'dvb delete -f <file>' instead",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			if !force {
				fmt.Printf("Are you sure you want to destroy devnet %q? [y/N] ", name)
				var response string
				if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
					fmt.Println("Cancelled")
					return nil
				}
			}

			err := daemonClient.DeleteDevnet(cmd.Context(), namespace, name)
			if err != nil {
				return err
			}

			color.Green("✓ Devnet %q destroyed", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

	return cmd
}

func newDescribeCmd() *cobra.Command {
	var (
		namespace    string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "describe <devnet>",
		Short: "Show detailed devnet information",
		Long: `Show detailed information about a devnet including status conditions,
recent events, and node details. Similar to kubectl describe.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnet, err := daemonClient.GetDevnet(cmd.Context(), namespace, name)
			if err != nil {
				return err
			}

			nodes, err := daemonClient.ListNodes(cmd.Context(), namespace, name)
			if err != nil {
				// Don't fail if nodes can't be listed
				nodes = nil
			}

			if outputFormat == "yaml" {
				return printDescribeYAML(devnet, nodes)
			}

			formatDescribeOutput(os.Stdout, devnet, nodes)
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (defaults to server default)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (yaml)")
	return cmd
}

func formatDescribeOutput(w io.Writer, d *v1.Devnet, nodes []*v1.Node) {
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
		fmt.Fprintf(w, "\nNodes:\n")
		fmt.Fprintf(w, "  %-6s %-10s %-10s %-10s %-8s %s\n", "INDEX", "ROLE", "PHASE", "HEIGHT", "RESTARTS", "MESSAGE")
		for _, n := range nodes {
			phase := n.Status.Phase
			switch phase {
			case "Running":
				phase = color.GreenString(phase)
			case "Pending", "Starting":
				phase = color.YellowString(phase)
			case "Crashed":
				phase = color.RedString(phase)
			}
			msg := n.Status.Message
			if len(msg) > 30 {
				msg = msg[:27] + "..."
			}
			fmt.Fprintf(w, "  %-6d %-10s %-10s %-10d %-8d %s\n",
				n.Metadata.Index,
				n.Spec.Role,
				phase,
				n.Status.BlockHeight,
				n.Status.RestartCount,
				msg,
			)
		}
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
			fmt.Fprintf(w, "  %-8s %-20s %-20s %s\n", eventType, e.Reason, age, e.Message)
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
