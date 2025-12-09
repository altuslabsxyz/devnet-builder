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
	Network       StringValue
	Validators    IntValue
	Mode          StringValue
	StableVersion StringValue
	NoCache       BoolValue
	Accounts      IntValue

	// Metadata
	ConfigFilePath string // Path to loaded config file (empty if none)
}

// NewEffectiveConfig creates a new EffectiveConfig with default values.
func NewEffectiveConfig(defaultHomeDir string) *EffectiveConfig {
	return &EffectiveConfig{
		Home:          NewStringValue(defaultHomeDir),
		NoColor:       NewBoolValue(false),
		Verbose:       NewBoolValue(false),
		JSON:          NewBoolValue(false),
		Network:       NewStringValue("mainnet"),
		Validators:    NewIntValue(4),
		Mode:          NewStringValue("docker"),
		StableVersion: NewStringValue("latest"),
		NoCache:       NewBoolValue(false),
		Accounts:      NewIntValue(0),
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
	fmt.Fprintf(tw, "validators\t%d\t%s\n", c.Validators.Value, c.Validators.Source)
	fmt.Fprintf(tw, "mode\t%s\t%s\n", c.Mode.Value, c.Mode.Source)
	fmt.Fprintf(tw, "stable_version\t%s\t%s\n", c.StableVersion.Value, c.StableVersion.Source)
	fmt.Fprintf(tw, "no_cache\t%t\t%s\n", c.NoCache.Value, c.NoCache.Source)
	fmt.Fprintf(tw, "accounts\t%d\t%s\n", c.Accounts.Value, c.Accounts.Source)
	tw.Flush()
}
