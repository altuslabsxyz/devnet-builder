package config

// FileConfig represents the raw config.toml file contents.
// All fields are pointers to distinguish "not set" from "set to zero/false".
type FileConfig struct {
	// Global settings
	Home    *string `toml:"home"`
	NoColor *bool   `toml:"no_color"`
	Verbose *bool   `toml:"verbose"`
	JSON    *bool   `toml:"json"`

	// Start command settings
	Network           *string `toml:"network"`            // Network source: "mainnet" or "testnet"
	BlockchainNetwork *string `toml:"blockchain_network"` // Network module: "stable", "ault", etc.
	Validators        *int    `toml:"validators"`
	Mode              *string `toml:"mode"`
	StableVersion     *string `toml:"stable_version"`   // Deprecated: use NetworkVersion instead
	NetworkVersion    *string `toml:"network_version"`  // Version for the selected blockchain network
	NoCache           *bool   `toml:"no_cache"`
	Accounts          *int    `toml:"accounts"`

	// GitHub API settings
	GitHubToken *string `toml:"github_token"` // GHP token for private repos
	CacheTTL    *string `toml:"cache_ttl"`    // Cache TTL (default: "1h")
}

// IsEmpty returns true if no configuration values are set.
func (f *FileConfig) IsEmpty() bool {
	return f.Home == nil &&
		f.NoColor == nil &&
		f.Verbose == nil &&
		f.JSON == nil &&
		f.Network == nil &&
		f.BlockchainNetwork == nil &&
		f.Validators == nil &&
		f.Mode == nil &&
		f.StableVersion == nil &&
		f.NetworkVersion == nil &&
		f.NoCache == nil &&
		f.Accounts == nil &&
		f.GitHubToken == nil &&
		f.CacheTTL == nil
}
