package core

import (
	"encoding/json"
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

// NetworkInfo represents information about a registered network.
type NetworkInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Version     string `json:"version"`
	BinaryName  string `json:"binary_name"`
	Bech32      string `json:"bech32_prefix"`
	BaseDenom   string `json:"base_denom"`
	ChainID     string `json:"default_chain_id"`
	DockerImage string `json:"docker_image"`
}

// NetworksResult represents the JSON output for the networks command.
type NetworksResult struct {
	Networks []NetworkInfo `json:"networks"`
	Default  string        `json:"default"`
}

// NewNetworksCmd creates the networks command.
func NewNetworksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "networks",
		Short: "List available blockchain networks",
		Long: `List all registered blockchain network modules.

Each network module provides configuration for a specific Cosmos SDK blockchain.
Use the --blockchain flag with deploy/init commands to select a network.

Examples:
  # List available networks
  devnet-builder networks

  # List in JSON format
  devnet-builder networks --json

  # Deploy with a specific network
  devnet-builder deploy --blockchain ault`,
		RunE: runNetworks,
	}

	return cmd
}

func runNetworks(cmd *cobra.Command, args []string) error {
	jsonMode := shared.GetJSONMode()
	networks := network.List()

	if jsonMode {
		return outputNetworksJSON(networks)
	}

	return outputNetworksText(networks)
}

func outputNetworksText(networkNames []string) error {
	output.Bold("Available Blockchain Networks")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println()

	for _, name := range networkNames {
		mod, err := network.Get(name)
		if err != nil {
			continue
		}

		defaultMarker := ""
		if name == "stable" {
			defaultMarker = " (default)"
		}

		output.Bold("%s%s", mod.DisplayName(), defaultMarker)
		fmt.Printf("  Name:         %s\n", mod.Name())
		fmt.Printf("  Binary:       %s\n", mod.BinaryName())
		fmt.Printf("  Bech32:       %s\n", mod.Bech32Prefix())
		fmt.Printf("  Base Denom:   %s\n", mod.BaseDenom())
		fmt.Printf("  Chain ID:     %s\n", "(from genesis)")
		fmt.Printf("  Docker:       %s\n", mod.DockerImage())
		fmt.Println()
	}

	fmt.Println("Usage: devnet-builder deploy --blockchain <network>")
	return nil
}

func outputNetworksJSON(networkNames []string) error {
	result := NetworksResult{
		Networks: make([]NetworkInfo, 0, len(networkNames)),
		Default:  "stable",
	}

	for _, name := range networkNames {
		mod, err := network.Get(name)
		if err != nil {
			continue
		}

		result.Networks = append(result.Networks, NetworkInfo{
			Name:        mod.Name(),
			DisplayName: mod.DisplayName(),
			Version:     mod.Version(),
			BinaryName:  mod.BinaryName(),
			Bech32:      mod.Bech32Prefix(),
			BaseDenom:   mod.BaseDenom(),
			ChainID:     "(from genesis)",
			DockerImage: mod.DockerImage(),
		})
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}
