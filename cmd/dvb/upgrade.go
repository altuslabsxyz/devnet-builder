// cmd/dvb/upgrade.go
package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Manage chain upgrades",
		Long:  `Manage chain upgrades for devnets, including creating, listing, and monitoring upgrade progress.`,
	}

	cmd.AddCommand(
		newUpgradeCreateCmd(),
		newUpgradeListCmd(),
		newUpgradeStatusCmd(),
		newUpgradeCancelCmd(),
		newUpgradeRetryCmd(),
		newUpgradeDeleteCmd(),
	)

	return cmd
}

func newUpgradeCreateCmd() *cobra.Command {
	var (
		devnet       string
		upgradeName  string
		targetHeight int64
		binaryType   string
		binaryPath   string
		version      string
		autoVote     bool
		withExport   bool
	)

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new chain upgrade",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			if devnet == "" {
				return fmt.Errorf("--devnet is required")
			}
			if upgradeName == "" {
				return fmt.Errorf("--upgrade-name is required")
			}

			spec := &v1.UpgradeSpec{
				DevnetRef:    devnet,
				UpgradeName:  upgradeName,
				TargetHeight: targetHeight,
				AutoVote:     autoVote,
				WithExport:   withExport,
				NewBinary: &v1.BinarySource{
					Type:    binaryType,
					Path:    binaryPath,
					Version: version,
				},
			}

			upgrade, err := daemonClient.CreateUpgrade(cmd.Context(), name, spec)
			if err != nil {
				return err
			}

			color.Green("✓ Upgrade %q created", upgrade.Metadata.Name)
			fmt.Printf("  Devnet:       %s\n", upgrade.Spec.DevnetRef)
			fmt.Printf("  Upgrade Name: %s\n", upgrade.Spec.UpgradeName)
			fmt.Printf("  Phase:        %s\n", upgrade.Status.Phase)
			if upgrade.Spec.TargetHeight > 0 {
				fmt.Printf("  Target Height: %d\n", upgrade.Spec.TargetHeight)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&devnet, "devnet", "", "Name of the devnet to upgrade (required)")
	cmd.Flags().StringVar(&upgradeName, "upgrade-name", "", "Name for the on-chain upgrade proposal (required)")
	cmd.Flags().Int64Var(&targetHeight, "target-height", 0, "Target block height for upgrade (0 = auto-calculate)")
	cmd.Flags().StringVar(&binaryType, "binary-type", "cache", "Binary source type (cache, path, docker)")
	cmd.Flags().StringVar(&binaryPath, "binary-path", "", "Path to new binary (for path type)")
	cmd.Flags().StringVar(&version, "version", "", "Version of new binary")
	cmd.Flags().BoolVar(&autoVote, "auto-vote", true, "Automatically vote yes on the upgrade proposal")
	cmd.Flags().BoolVar(&withExport, "with-export", false, "Export state before and after upgrade")

	cmd.MarkFlagRequired("devnet")
	cmd.MarkFlagRequired("upgrade-name")

	return cmd
}

func newUpgradeListCmd() *cobra.Command {
	var devnet string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List upgrades",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			upgrades, err := daemonClient.ListUpgrades(cmd.Context(), devnet)
			if err != nil {
				return err
			}

			if len(upgrades) == 0 {
				fmt.Println("No upgrades found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tDEVNET\tUPGRADE\tPHASE\tPROGRESS")
			for _, u := range upgrades {
				progress := ""
				switch u.Status.Phase {
				case "Voting":
					progress = fmt.Sprintf("%d/%d votes", u.Status.VotesReceived, u.Status.VotesRequired)
				case "Waiting":
					if u.Status.CurrentHeight > 0 && u.Spec.TargetHeight > 0 {
						remaining := u.Spec.TargetHeight - u.Status.CurrentHeight
						if remaining > 0 {
							progress = fmt.Sprintf("%d blocks remaining", remaining)
						}
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					u.Metadata.Name,
					u.Spec.DevnetRef,
					u.Spec.UpgradeName,
					u.Status.Phase,
					progress)
			}
			w.Flush()

			return nil
		},
	}

	cmd.Flags().StringVar(&devnet, "devnet", "", "Filter by devnet name")

	return cmd
}

func newUpgradeStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show upgrade status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			upgrade, err := daemonClient.GetUpgrade(cmd.Context(), name)
			if err != nil {
				return err
			}

			printUpgradeStatus(upgrade)
			return nil
		},
	}
}

func newUpgradeCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel [name]",
		Short: "Cancel a running upgrade",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			upgrade, err := daemonClient.CancelUpgrade(cmd.Context(), name)
			if err != nil {
				return err
			}

			color.Yellow("✓ Upgrade %q cancelled", upgrade.Metadata.Name)
			fmt.Printf("  Phase: %s\n", upgrade.Status.Phase)

			return nil
		},
	}
}

func newUpgradeRetryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retry [name]",
		Short: "Retry a failed upgrade",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			upgrade, err := daemonClient.RetryUpgrade(cmd.Context(), name)
			if err != nil {
				return err
			}

			color.Green("✓ Upgrade %q retrying", upgrade.Metadata.Name)
			fmt.Printf("  Phase: %s\n", upgrade.Status.Phase)

			return nil
		},
	}
}

func newUpgradeDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete an upgrade",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			if !force {
				fmt.Printf("Are you sure you want to delete upgrade %q? [y/N] ", name)
				var response string
				if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
					fmt.Println("Cancelled")
					return nil
				}
			}

			err := daemonClient.DeleteUpgrade(cmd.Context(), name)
			if err != nil {
				return err
			}

			color.Green("✓ Upgrade %q deleted", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

	return cmd
}

func printUpgradeStatus(u *v1.Upgrade) {
	// Phase with color
	phase := u.Status.Phase
	switch phase {
	case "Completed":
		color.Green("● %s", phase)
	case "Pending", "Proposing", "Voting", "Waiting", "Switching", "Verifying":
		color.Yellow("◐ %s", phase)
	case "Failed":
		color.Red("✗ %s", phase)
	default:
		fmt.Printf("? %s", phase)
	}

	fmt.Printf("\nName:         %s\n", u.Metadata.Name)
	fmt.Printf("Devnet:       %s\n", u.Spec.DevnetRef)
	fmt.Printf("Upgrade Name: %s\n", u.Spec.UpgradeName)

	if u.Spec.TargetHeight > 0 {
		fmt.Printf("Target Height: %d\n", u.Spec.TargetHeight)
	}

	if u.Status.CurrentHeight > 0 {
		fmt.Printf("Current Height: %d\n", u.Status.CurrentHeight)
	}

	if u.Status.ProposalId > 0 {
		fmt.Printf("Proposal ID:  %d\n", u.Status.ProposalId)
	}

	if u.Status.VotesRequired > 0 {
		fmt.Printf("Votes:        %d/%d\n", u.Status.VotesReceived, u.Status.VotesRequired)
	}

	if u.Spec.NewBinary != nil && u.Spec.NewBinary.Version != "" {
		fmt.Printf("New Version:  %s\n", u.Spec.NewBinary.Version)
	}

	if u.Status.Message != "" {
		fmt.Printf("Message:      %s\n", u.Status.Message)
	}

	if u.Status.Error != "" {
		color.Red("Error:        %s\n", u.Status.Error)
	}

	if u.Status.PreExportPath != "" {
		fmt.Printf("Pre-export:   %s\n", u.Status.PreExportPath)
	}

	if u.Status.PostExportPath != "" {
		fmt.Printf("Post-export:  %s\n", u.Status.PostExportPath)
	}
}
