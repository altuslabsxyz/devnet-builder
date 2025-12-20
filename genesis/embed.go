package genesis

import (
	"embed"
	"fmt"
)

//go:embed stable/*.json
var genesisFS embed.FS

// SupportedBlockchains returns the list of supported blockchain modules.
var SupportedBlockchains = []string{"stable", "ault"}

// SupportedNetworks returns the list of supported networks.
var SupportedNetworks = []string{"mainnet", "testnet"}

// GetGenesisData returns the embedded genesis file content for the given blockchain and network.
func GetGenesisData(blockchain, network string) ([]byte, error) {
	path := fmt.Sprintf("%s/%s-genesis.json", blockchain, network)
	data, err := genesisFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no genesis file for blockchain %s, network %s: %w", blockchain, network, err)
	}
	return data, nil
}

// HasGenesis checks if a genesis file exists for the given blockchain and network.
func HasGenesis(blockchain, network string) bool {
	path := fmt.Sprintf("%s/%s-genesis.json", blockchain, network)
	_, err := genesisFS.ReadFile(path)
	return err == nil
}

// ListAvailableGenesis returns all available blockchain/network combinations.
func ListAvailableGenesis() []string {
	var available []string
	for _, bc := range SupportedBlockchains {
		for _, net := range SupportedNetworks {
			if HasGenesis(bc, net) {
				available = append(available, fmt.Sprintf("%s/%s", bc, net))
			}
		}
	}
	return available
}
