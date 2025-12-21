package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// StatusResult represents the JSON output for the status command.
type StatusResult struct {
	ChainID           string             `json:"chain_id"`
	Network           string             `json:"network"`
	BlockchainNetwork string             `json:"blockchain_network"`
	Mode              string             `json:"mode"`
	DockerImage       string             `json:"docker_image,omitempty"`
	InitialVersion    string             `json:"initial_version,omitempty"`
	CurrentVersion    string             `json:"current_version,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	Status            string             `json:"status"`
	Nodes             []NodeStatusResult `json:"nodes"`
}

// NodeStatusResult represents a node status in the JSON output.
type NodeStatusResult struct {
	Index       int    `json:"index"`
	Status      string `json:"status"`
	BlockHeight int64  `json:"block_height"`
	PeerCount   int    `json:"peer_count"`
	CatchingUp  bool   `json:"catching_up"`
	Error       string `json:"error,omitempty"`
}

func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show devnet status",
		Long: `Show the current status of the devnet including node health.

This command displays information about the devnet and all running nodes,
including block height, peer count, and sync status.

Examples:
  # Show status
  devnet-builder status

  # Show status in JSON format
  devnet-builder status --json`,
		RunE: runStatus,
	}

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	svc, err := getCleanService()
	if err != nil {
		return outputStatusError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Get status using service
	status, err := svc.GetStatus(ctx)
	if err != nil {
		if jsonMode {
			return outputStatusError(err)
		}
		return err
	}

	if jsonMode {
		return outputStatusJSON(status)
	}

	return outputStatusText(status)
}

func outputStatusText(status *dto.StatusOutput) error {
	d := status.Devnet

	output.Bold("Devnet Status")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println()

	fmt.Printf("Chain ID:     %s\n", d.ChainID)
	fmt.Printf("Network:      %s\n", d.NetworkSource)
	fmt.Printf("Blockchain:   %s\n", d.BlockchainNetwork)
	fmt.Printf("Mode:         %s\n", d.ExecutionMode)
	if d.DockerImage != "" {
		fmt.Printf("Docker Image: %s\n", d.DockerImage)
	}

	// Version info
	if d.InitialVersion != "" {
		fmt.Printf("Version:      %s", d.CurrentVersion)
		if d.CurrentVersion != d.InitialVersion {
			fmt.Printf(" (initial: %s)", d.InitialVersion)
		}
		fmt.Println()
	}

	fmt.Printf("Created:      %s\n", d.CreatedAt.Format("2006-01-02 15:04:05 MST"))

	// Status with color
	statusStr := status.OverallStatus
	switch status.OverallStatus {
	case "running":
		statusStr = color.GreenString("running")
	case "stopped":
		statusStr = color.YellowString("stopped")
	case "partial":
		statusStr = color.CyanString("partial")
	case "error":
		statusStr = color.RedString("error")
	}
	fmt.Printf("Status:       %s\n", statusStr)
	fmt.Println()

	output.Bold("Nodes:")
	for _, h := range status.Nodes {
		nodeStatus := formatNodeStatusDTO(h)
		heightStr := fmt.Sprintf("height=%d", h.BlockHeight)
		peersStr := fmt.Sprintf("peers=%d", h.PeerCount)
		catchingUpStr := fmt.Sprintf("catching_up=%v", h.CatchingUp)

		fmt.Printf("  Node %d [%s]  %s  %s  %s\n",
			h.Index, nodeStatus, heightStr, peersStr, catchingUpStr)

		if h.Error != "" {
			fmt.Printf("         Error: %s\n", color.RedString(h.Error))
		}
	}

	return nil
}

func formatNodeStatusDTO(h dto.NodeHealthStatus) string {
	switch h.Status {
	case ports.NodeStatusRunning:
		return color.GreenString("running")
	case ports.NodeStatusSyncing:
		return color.CyanString("syncing")
	case ports.NodeStatusStopped:
		return color.YellowString("stopped")
	case ports.NodeStatusError:
		return color.RedString("error")
	default:
		return string(h.Status)
	}
}

func outputStatusJSON(status *dto.StatusOutput) error {
	d := status.Devnet

	result := StatusResult{
		ChainID:           d.ChainID,
		Network:           d.NetworkSource,
		BlockchainNetwork: d.BlockchainNetwork,
		Mode:              d.ExecutionMode,
		DockerImage:       d.DockerImage,
		InitialVersion:    d.InitialVersion,
		CurrentVersion:    d.CurrentVersion,
		CreatedAt:         d.CreatedAt,
		Status:            status.OverallStatus,
		Nodes:             make([]NodeStatusResult, len(status.Nodes)),
	}

	for i, h := range status.Nodes {
		result.Nodes[i] = NodeStatusResult{
			Index:       h.Index,
			Status:      string(h.Status),
			BlockHeight: h.BlockHeight,
			PeerCount:   h.PeerCount,
			CatchingUp:  h.CatchingUp,
			Error:       h.Error,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputStatusError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "DEVNET_NOT_FOUND",
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}
