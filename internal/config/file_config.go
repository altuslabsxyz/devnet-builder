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
	Network       *string `toml:"network"`
	Validators    *int    `toml:"validators"`
	Mode          *string `toml:"mode"`
	StableVersion *string `toml:"stable_version"`
	NoCache       *bool   `toml:"no_cache"`
	Accounts      *int    `toml:"accounts"`
}

// IsEmpty returns true if no configuration values are set.
func (f *FileConfig) IsEmpty() bool {
	return f.Home == nil &&
		f.NoColor == nil &&
		f.Verbose == nil &&
		f.JSON == nil &&
		f.Network == nil &&
		f.Validators == nil &&
		f.Mode == nil &&
		f.StableVersion == nil &&
		f.NoCache == nil &&
		f.Accounts == nil
}
