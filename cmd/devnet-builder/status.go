package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/node"
	"github.com/stablelabs/stable-devnet/internal/output"
)

// StatusResult represents the JSON output for the status command.
type StatusResult struct {
	ChainID        string             `json:"chain_id"`
	Network        string             `json:"network"`
	Mode           string             `json:"mode"`
	DockerImage    string             `json:"docker_image,omitempty"`
	InitialVersion string             `json:"initial_version,omitempty"`
	CurrentVersion string             `json:"current_version,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	Status         string             `json:"status"`
	Nodes          []NodeStatusResult `json:"nodes"`
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
	logger := output.DefaultLogger

	// Load devnet using consolidated helper
	loaded, err := loadDevnetOrFail(logger)
	if err != nil {
		if jsonMode {
			return outputStatusError(err)
		}
		return err
	}
	d := loaded.Devnet

	// Backward compatibility: if version not in metadata, try to read from genesis
	if d.Metadata.InitialVersion == "" {
		if err := d.Metadata.SetInitialVersionFromGenesis(); err == nil {
			// Save updated metadata with version info
			d.Metadata.Save()
		}
	}

	// Get health status of all nodes
	health := d.GetHealth(ctx)

	// Update overall status based on node health
	overallStatus := d.Metadata.Status
	runningCount := 0
	for _, h := range health {
		if h.Status == node.NodeStatusRunning || h.Status == node.NodeStatusSyncing {
			runningCount++
		}
	}

	if runningCount == len(d.Nodes) {
		overallStatus = devnet.StatusRunning
	} else if runningCount > 0 {
		// Partial running
		overallStatus = devnet.StatusRunning
	} else {
		overallStatus = devnet.StatusStopped
	}

	if jsonMode {
		return outputStatusJSON(d, health, overallStatus)
	}

	return outputStatusText(d, health, overallStatus)
}

func outputStatusText(d *devnet.Devnet, health []*node.NodeHealth, status devnet.DevnetStatus) error {
	output.Bold("Devnet Status")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println()

	fmt.Printf("Chain ID:     %s\n", d.Metadata.ChainID)
	fmt.Printf("Network:      %s\n", d.Metadata.NetworkSource)
	fmt.Printf("Mode:         %s\n", d.Metadata.ExecutionMode)
	if d.Metadata.DockerImage != "" {
		fmt.Printf("Docker Image: %s\n", d.Metadata.DockerImage)
	}

	// Version info
	if d.Metadata.InitialVersion != "" {
		fmt.Printf("Version:      %s", d.Metadata.CurrentVersion)
		if d.Metadata.CurrentVersion != d.Metadata.InitialVersion {
			fmt.Printf(" (initial: %s)", d.Metadata.InitialVersion)
		}
		fmt.Println()
	}

	fmt.Printf("Created:      %s\n", d.Metadata.CreatedAt.Format("2006-01-02 15:04:05 MST"))

	// Status with color
	statusStr := string(status)
	switch status {
	case devnet.StatusRunning:
		statusStr = color.GreenString("running")
	case devnet.StatusStopped:
		statusStr = color.YellowString("stopped")
	case devnet.StatusError:
		statusStr = color.RedString("error")
	}
	fmt.Printf("Status:       %s\n", statusStr)
	fmt.Println()

	output.Bold("Nodes:")
	for _, h := range health {
		nodeStatus := formatNodeStatus(h)
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

func formatNodeStatus(h *node.NodeHealth) string {
	switch h.Status {
	case node.NodeStatusRunning:
		return color.GreenString("running")
	case node.NodeStatusSyncing:
		return color.CyanString("syncing")
	case node.NodeStatusStopped:
		return color.YellowString("stopped")
	case node.NodeStatusStarting:
		return color.CyanString("starting")
	case node.NodeStatusError:
		return color.RedString("error")
	default:
		return string(h.Status)
	}
}

func outputStatusJSON(d *devnet.Devnet, health []*node.NodeHealth, status devnet.DevnetStatus) error {
	result := StatusResult{
		ChainID:        d.Metadata.ChainID,
		Network:        d.Metadata.NetworkSource,
		Mode:           string(d.Metadata.ExecutionMode),
		DockerImage:    d.Metadata.DockerImage,
		InitialVersion: d.Metadata.InitialVersion,
		CurrentVersion: d.Metadata.CurrentVersion,
		CreatedAt:      d.Metadata.CreatedAt,
		Status:         string(status),
		Nodes:          make([]NodeStatusResult, len(health)),
	}

	for i, h := range health {
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
