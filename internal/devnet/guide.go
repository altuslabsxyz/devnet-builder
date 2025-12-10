package devnet

import (
	"fmt"
	"path/filepath"
)

// ConfigGuide contains information for the config modification guide.
type ConfigGuide struct {
	ConfigPaths      map[string]string
	ModifiableParams []ParamInfo
	NextCommand      string
}

// ParamInfo describes a modifiable parameter.
type ParamInfo struct {
	File         string // config.toml, app.toml, genesis.json
	Path         string // TOML/JSON path
	Description  string
	DefaultValue string
}

// GenerateConfigGuide creates a configuration guide based on provision result.
func GenerateConfigGuide(metadata *DevnetMetadata) *ConfigGuide {
	devnetDir := filepath.Join(metadata.HomeDir, "devnet")
	node0Dir := filepath.Join(devnetDir, "node0", "config")

	guide := &ConfigGuide{
		ConfigPaths: map[string]string{
			"config.toml":  filepath.Join(node0Dir, "config.toml"),
			"app.toml":     filepath.Join(node0Dir, "app.toml"),
			"genesis.json": metadata.GenesisPath,
		},
		NextCommand: "devnet-builder run",
		ModifiableParams: []ParamInfo{
			// config.toml parameters
			{
				File:         "config.toml",
				Path:         "consensus.timeout_commit",
				Description:  "Block commit timeout (affects block time)",
				DefaultValue: "1s",
			},
			{
				File:         "config.toml",
				Path:         "p2p.persistent_peers",
				Description:  "Persistent peer connections",
				DefaultValue: "(auto-configured)",
			},
			{
				File:         "config.toml",
				Path:         "p2p.max_num_inbound_peers",
				Description:  "Maximum inbound peer connections",
				DefaultValue: "40",
			},
			{
				File:         "config.toml",
				Path:         "mempool.size",
				Description:  "Maximum transactions in mempool",
				DefaultValue: "5000",
			},
			{
				File:         "config.toml",
				Path:         "log_level",
				Description:  "Logging verbosity level",
				DefaultValue: "info",
			},
			// app.toml parameters
			{
				File:         "app.toml",
				Path:         "api.enable",
				Description:  "Enable REST API server",
				DefaultValue: "true (node0)",
			},
			{
				File:         "app.toml",
				Path:         "grpc.enable",
				Description:  "Enable gRPC server",
				DefaultValue: "true",
			},
			{
				File:         "app.toml",
				Path:         "json-rpc.enable",
				Description:  "Enable EVM JSON-RPC",
				DefaultValue: "true (node0)",
			},
			{
				File:         "app.toml",
				Path:         "minimum-gas-prices",
				Description:  "Minimum gas price for transactions",
				DefaultValue: "0ustbl",
			},
			{
				File:         "app.toml",
				Path:         "pruning",
				Description:  "State pruning strategy",
				DefaultValue: "default",
			},
		},
	}

	return guide
}

// PrintGuide prints the configuration guide to stdout.
func (g *ConfigGuide) PrintGuide() {
	fmt.Println()
	fmt.Println("Configuration files:")
	for name, path := range g.ConfigPaths {
		fmt.Printf("  %-14s %s\n", name+":", path)
	}
	fmt.Println()

	fmt.Println("Modifiable parameters:")
	fmt.Println()

	// Group by file
	currentFile := ""
	for _, param := range g.ModifiableParams {
		if param.File != currentFile {
			fmt.Printf("  %s:\n", param.File)
			currentFile = param.File
		}
		fmt.Printf("    %-35s %s (default: %s)\n",
			param.Path, param.Description, param.DefaultValue)
	}
	fmt.Println()

	fmt.Printf("Next step: Run '%s' to start the nodes\n", g.NextCommand)
	fmt.Println()
}

// ToJSON returns the guide as a JSON-serializable struct.
func (g *ConfigGuide) ToJSON() map[string]interface{} {
	params := make([]map[string]string, len(g.ModifiableParams))
	for i, p := range g.ModifiableParams {
		params[i] = map[string]string{
			"file":          p.File,
			"path":          p.Path,
			"description":   p.Description,
			"default_value": p.DefaultValue,
		}
	}

	return map[string]interface{}{
		"config_paths":      g.ConfigPaths,
		"modifiable_params": params,
		"next_command":      g.NextCommand,
	}
}
