// cmd/dvb/tx.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tx",
		Short:   "Manage transactions",
		Aliases: []string{"transaction"},
	}

	cmd.AddCommand(
		newTxSubmitCmd(),
		newTxListCmd(),
		newTxStatusCmd(),
		newTxCancelCmd(),
	)

	return cmd
}

func newTxSubmitCmd() *cobra.Command {
	var (
		txType  string
		signer  string
		payload string
	)

	cmd := &cobra.Command{
		Use:   "submit [devnet]",
		Short: "Submit a transaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			devnet := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var payloadBytes []byte
			if payload != "" {
				payloadBytes = []byte(payload)
			}

			tx, err := daemonClient.SubmitTransaction(cmd.Context(), devnet, txType, signer, payloadBytes)
			if err != nil {
				return err
			}

			color.Green("✓ Transaction submitted: %s", tx.Name)
			fmt.Printf("  Phase: %s\n", tx.Phase)
			fmt.Printf("  Type:  %s\n", tx.TxType)
			fmt.Printf("  Signer: %s\n", tx.Signer)

			return nil
		},
	}

	cmd.Flags().StringVar(&txType, "type", "", "Transaction type (required)")
	cmd.Flags().StringVar(&signer, "signer", "", "Transaction signer (required)")
	cmd.Flags().StringVar(&payload, "payload", "", "JSON payload")
	cmd.MarkFlagRequired("type")
	cmd.MarkFlagRequired("signer")

	return cmd
}

func newTxListCmd() *cobra.Command {
	var (
		txType string
		phase  string
		limit  int
	)

	cmd := &cobra.Command{
		Use:     "list [devnet]",
		Short:   "List transactions for a devnet",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			devnet := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			txs, err := daemonClient.ListTransactions(cmd.Context(), devnet, txType, phase, limit)
			if err != nil {
				return err
			}

			if len(txs) == 0 {
				fmt.Println("No transactions found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tPHASE\tSIGNER\tTX_HASH")
			for _, tx := range txs {
				hash := tx.TxHash
				if len(hash) > 16 {
					hash = hash[:16] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					tx.Name, tx.TxType, tx.Phase, tx.Signer, hash)
			}
			w.Flush()

			return nil
		},
	}

	cmd.Flags().StringVar(&txType, "type", "", "Filter by transaction type")
	cmd.Flags().StringVar(&phase, "phase", "", "Filter by phase")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max transactions to return")

	return cmd
}

func newTxStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show transaction status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			tx, err := daemonClient.GetTransaction(cmd.Context(), name)
			if err != nil {
				return err
			}

			printTxStatus(tx)
			return nil
		},
	}
}

func newTxCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel [name]",
		Short: "Cancel a pending transaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			tx, err := daemonClient.CancelTransaction(cmd.Context(), name)
			if err != nil {
				return err
			}

			color.Green("✓ Transaction cancelled: %s", tx.Name)
			fmt.Printf("  Phase: %s\n", tx.Phase)

			return nil
		},
	}
}

func newGovCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gov",
		Short: "Governance operations",
	}

	cmd.AddCommand(
		newGovVoteCmd(),
		newGovProposeCmd(),
	)

	return cmd
}

func newGovVoteCmd() *cobra.Command {
	var (
		proposalID uint64
		voter      string
		option     string
	)

	cmd := &cobra.Command{
		Use:   "vote [devnet]",
		Short: "Submit a governance vote",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			devnet := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			tx, err := daemonClient.SubmitGovVote(cmd.Context(), devnet, proposalID, voter, option)
			if err != nil {
				return err
			}

			color.Green("✓ Vote submitted: %s", tx.Name)
			fmt.Printf("  Proposal: %d\n", proposalID)
			fmt.Printf("  Voter:    %s\n", voter)
			fmt.Printf("  Option:   %s\n", option)
			fmt.Printf("  Phase:    %s\n", tx.Phase)

			return nil
		},
	}

	cmd.Flags().Uint64Var(&proposalID, "proposal", 0, "Proposal ID (required)")
	cmd.Flags().StringVar(&voter, "voter", "", "Voter address (required)")
	cmd.Flags().StringVar(&option, "option", "", "Vote option: yes, no, abstain, veto (required)")
	cmd.MarkFlagRequired("proposal")
	cmd.MarkFlagRequired("voter")
	cmd.MarkFlagRequired("option")

	return cmd
}

func newGovProposeCmd() *cobra.Command {
	var (
		proposer     string
		proposalType string
		title        string
		description  string
		contentFile  string
	)

	cmd := &cobra.Command{
		Use:   "propose [devnet]",
		Short: "Submit a governance proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			devnet := args[0]

			if daemonClient == nil {
				return fmt.Errorf("daemon not running - start with: devnetd")
			}

			var content []byte
			if contentFile != "" {
				var err error
				content, err = os.ReadFile(contentFile)
				if err != nil {
					return fmt.Errorf("failed to read content file: %w", err)
				}
			}

			tx, err := daemonClient.SubmitGovProposal(cmd.Context(), devnet, proposer, proposalType, title, description, content)
			if err != nil {
				return err
			}

			color.Green("✓ Proposal submitted: %s", tx.Name)
			fmt.Printf("  Title:    %s\n", title)
			fmt.Printf("  Type:     %s\n", proposalType)
			fmt.Printf("  Proposer: %s\n", proposer)
			fmt.Printf("  Phase:    %s\n", tx.Phase)

			return nil
		},
	}

	cmd.Flags().StringVar(&proposer, "proposer", "", "Proposer address (required)")
	cmd.Flags().StringVar(&proposalType, "type", "text", "Proposal type")
	cmd.Flags().StringVar(&title, "title", "", "Proposal title (required)")
	cmd.Flags().StringVar(&description, "description", "", "Proposal description")
	cmd.Flags().StringVar(&contentFile, "content", "", "Path to content JSON file")
	cmd.MarkFlagRequired("proposer")
	cmd.MarkFlagRequired("title")

	return cmd
}

func printTxStatus(tx *v1.Transaction) {
	// Phase with color
	phase := tx.Phase
	switch phase {
	case "Confirmed":
		color.Green("✓ %s", phase)
	case "Pending", "Building", "Signing", "Submitted":
		color.Yellow("◐ %s", phase)
	case "Failed":
		color.Red("✗ %s", phase)
	default:
		fmt.Printf("? %s", phase)
	}

	fmt.Printf("\nName:     %s\n", tx.Name)
	fmt.Printf("Devnet:   %s\n", tx.DevnetRef)
	fmt.Printf("Type:     %s\n", tx.TxType)
	fmt.Printf("Signer:   %s\n", tx.Signer)

	if tx.TxHash != "" {
		fmt.Printf("TxHash:   %s\n", tx.TxHash)
	}
	if tx.Height > 0 {
		fmt.Printf("Height:   %d\n", tx.Height)
	}
	if tx.GasUsed > 0 {
		fmt.Printf("Gas Used: %d\n", tx.GasUsed)
	}
	if tx.Message != "" {
		fmt.Printf("Message:  %s\n", tx.Message)
	}
	if tx.Error != "" {
		color.Red("Error:    %s\n", tx.Error)
	}

	if len(tx.Payload) > 0 {
		var payload map[string]interface{}
		if err := json.Unmarshal(tx.Payload, &payload); err == nil {
			fmt.Println("Payload:")
			for k, v := range payload {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}
	}
}
