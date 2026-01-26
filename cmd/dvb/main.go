// cmd/dvb/main.go
package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
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
		newDeployCmd(),
		newListCmd(),
		newStatusCmd(),
		newStartCmd(),
		newStopCmd(),
		newDestroyCmd(),
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
		plugin     string
		validators int
		fullNodes  int
		mode       string
	)

	cmd := &cobra.Command{
		Use:   "deploy [name]",
		Short: "Deploy a new devnet",
		Args:  cobra.ExactArgs(1),
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

			devnet, err := daemonClient.CreateDevnet(cmd.Context(), name, spec, nil)
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

	cmd.Flags().StringVar(&plugin, "plugin", "stable", "Network plugin")
	cmd.Flags().IntVar(&validators, "validators", 4, "Number of validators")
	cmd.Flags().IntVar(&fullNodes, "full-nodes", 0, "Number of full nodes")
	cmd.Flags().StringVar(&mode, "mode", "docker", "Execution mode (docker or local)")

	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all devnets",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnets, err := daemonClient.ListDevnets(cmd.Context())
			if err != nil {
				return err
			}

			if len(devnets) == 0 {
				fmt.Println("No devnets found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPHASE\tNODES\tREADY\tHEIGHT")
			for _, d := range devnets {
				fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n",
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
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [devnet]",
		Short: "Show devnet status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnet, err := daemonClient.GetDevnet(cmd.Context(), name)
			if err != nil {
				return err
			}

			printDevnetStatus(devnet)
			return nil
		},
	}
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [devnet]",
		Short: "Start a stopped devnet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnet, err := daemonClient.StartDevnet(cmd.Context(), name)
			if err != nil {
				return err
			}

			color.Green("✓ Devnet %q starting", devnet.Metadata.Name)
			fmt.Printf("  Phase: %s\n", devnet.Status.Phase)

			return nil
		},
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [devnet]",
		Short: "Stop a running devnet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			devnet, err := daemonClient.StopDevnet(cmd.Context(), name)
			if err != nil {
				return err
			}

			color.Green("✓ Devnet %q stopped", devnet.Metadata.Name)
			fmt.Printf("  Phase: %s\n", devnet.Status.Phase)

			return nil
		},
	}
}

func newDestroyCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "destroy [devnet]",
		Short: "Destroy a devnet",
		Args:  cobra.ExactArgs(1),
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

			err := daemonClient.DeleteDevnet(cmd.Context(), name)
			if err != nil {
				return err
			}

			color.Green("✓ Devnet %q destroyed", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

	return cmd
}

func printDevnetStatus(d *v1.Devnet) {
	// Phase with color
	phase := d.Status.Phase
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

	fmt.Printf("\nName:       %s\n", d.Metadata.Name)
	fmt.Printf("Plugin:     %s\n", d.Spec.Plugin)
	fmt.Printf("Mode:       %s\n", d.Spec.Mode)
	fmt.Printf("Validators: %d\n", d.Spec.Validators)
	if d.Spec.FullNodes > 0 {
		fmt.Printf("Full Nodes: %d\n", d.Spec.FullNodes)
	}
	fmt.Printf("Nodes:      %d/%d ready\n", d.Status.ReadyNodes, d.Status.Nodes)
	if d.Status.CurrentHeight > 0 {
		fmt.Printf("Height:     %d\n", d.Status.CurrentHeight)
	}
	if d.Status.SdkVersion != "" {
		fmt.Printf("SDK:        %s\n", d.Status.SdkVersion)
	}
	if d.Status.Message != "" {
		fmt.Printf("Message:    %s\n", d.Status.Message)
	}
}
