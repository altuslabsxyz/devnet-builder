package config

// ConfigSource represents the origin of a configuration value.
type ConfigSource string

const (
	SourceDefault     ConfigSource = "default"
	SourceConfigFile  ConfigSource = "config.toml"
	SourceEnvironment ConfigSource = "environment"
	SourceFlag        ConfigSource = "flag"
)

// String returns the string representation of the ConfigSource.
func (s ConfigSource) String() string {
	return string(s)
}
