package config

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// EffectiveConfig represents the final merged configuration after applying priority chain.
type EffectiveConfig struct {
	// Global settings
	Home    StringValue
	NoColor BoolValue
	Verbose BoolValue
	JSON    BoolValue

	// Start command settings
	Network           StringValue // Network source: "mainnet" or "testnet"
	BlockchainNetwork StringValue // Network module: "stable", "ault", etc.
	Validators        IntValue
	Mode              StringValue
	StableVersion     StringValue // Deprecated: use NetworkVersion for new code
	NetworkVersion    StringValue // Version for the selected blockchain network
	NoCache           BoolValue
	Accounts          IntValue

	// GitHub/Cache settings
	GitHubToken StringValue
	CacheTTL    StringValue

	// Metadata
	ConfigFilePath string // Path to loaded config file (empty if none)
}

// NewEffectiveConfig creates a new EffectiveConfig with default values.
func NewEffectiveConfig(defaultHomeDir string) *EffectiveConfig {
	return &EffectiveConfig{
		Home:              NewStringValue(defaultHomeDir),
		NoColor:           NewBoolValue(false),
		Verbose:           NewBoolValue(false),
		JSON:              NewBoolValue(false),
		Network:           NewStringValue("mainnet"),
		BlockchainNetwork: NewStringValue("stable"), // Default to stable for backward compatibility
		Validators:        NewIntValue(4),
		Mode:              NewStringValue("docker"),
		StableVersion:     NewStringValue("latest"),
		NetworkVersion:    NewStringValue(""),       // Empty means use network module default
		NoCache:           NewBoolValue(false),
		Accounts:          NewIntValue(0),
		GitHubToken:       NewStringValue(""),
		CacheTTL:          NewStringValue("1h"),
	}
}

// ToTable writes the configuration as a formatted table.
func (c *EffectiveConfig) ToTable(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")
	fmt.Fprintf(tw, "home\t%s\t%s\n", c.Home.Value, c.Home.Source)
	fmt.Fprintf(tw, "no_color\t%t\t%s\n", c.NoColor.Value, c.NoColor.Source)
	fmt.Fprintf(tw, "verbose\t%t\t%s\n", c.Verbose.Value, c.Verbose.Source)
	fmt.Fprintf(tw, "json\t%t\t%s\n", c.JSON.Value, c.JSON.Source)
	fmt.Fprintf(tw, "network\t%s\t%s\n", c.Network.Value, c.Network.Source)
	fmt.Fprintf(tw, "blockchain_network\t%s\t%s\n", c.BlockchainNetwork.Value, c.BlockchainNetwork.Source)
	fmt.Fprintf(tw, "validators\t%d\t%s\n", c.Validators.Value, c.Validators.Source)
	fmt.Fprintf(tw, "mode\t%s\t%s\n", c.Mode.Value, c.Mode.Source)
	fmt.Fprintf(tw, "stable_version\t%s\t%s\n", c.StableVersion.Value, c.StableVersion.Source)
	fmt.Fprintf(tw, "network_version\t%s\t%s\n", c.NetworkVersion.Value, c.NetworkVersion.Source)
	fmt.Fprintf(tw, "no_cache\t%t\t%s\n", c.NoCache.Value, c.NoCache.Source)
	fmt.Fprintf(tw, "accounts\t%d\t%s\n", c.Accounts.Value, c.Accounts.Source)
	fmt.Fprintf(tw, "github_token\t%s\t%s\n", maskToken(c.GitHubToken.Value), c.GitHubToken.Source)
	fmt.Fprintf(tw, "cache_ttl\t%s\t%s\n", c.CacheTTL.Value, c.CacheTTL.Source)
	tw.Flush()
}

// maskToken masks a GitHub token for display, showing only first 4 and last 4 chars.
func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
