package config

import "github.com/spf13/cobra"

// ApplyStringConfig applies a config file string value if the flag was not explicitly set
// and the config value is present. Returns the effective value and its source.
func ApplyStringConfig(cmd *cobra.Command, flagName string, currentValue string, configValue *string) (string, ConfigSource) {
	// If flag was explicitly set on command line, use it
	if cmd.Flags().Changed(flagName) {
		return currentValue, SourceFlag
	}

	// If config file has a value, use it
	if configValue != nil {
		return *configValue, SourceConfigFile
	}

	// Otherwise, keep current value (which is the default)
	return currentValue, SourceDefault
}

// ApplyIntConfig applies a config file int value if the flag was not explicitly set
// and the config value is present. Returns the effective value and its source.
func ApplyIntConfig(cmd *cobra.Command, flagName string, currentValue int, configValue *int) (int, ConfigSource) {
	// If flag was explicitly set on command line, use it
	if cmd.Flags().Changed(flagName) {
		return currentValue, SourceFlag
	}

	// If config file has a value, use it
	if configValue != nil {
		return *configValue, SourceConfigFile
	}

	// Otherwise, keep current value (which is the default)
	return currentValue, SourceDefault
}

// ApplyBoolConfig applies a config file bool value if the flag was not explicitly set
// and the config value is present. Returns the effective value and its source.
// This is critical for preventing boolean false from overriding config true values.
func ApplyBoolConfig(cmd *cobra.Command, flagName string, currentValue bool, configValue *bool) (bool, ConfigSource) {
	// If flag was explicitly set on command line, use it
	if cmd.Flags().Changed(flagName) {
		return currentValue, SourceFlag
	}

	// If config file has a value, use it
	if configValue != nil {
		return *configValue, SourceConfigFile
	}

	// Otherwise, keep current value (which is the default)
	return currentValue, SourceDefault
}

// ApplyEnvString applies an environment variable string value if set and flag was not changed.
// This handles the priority: config.toml < env < flag
func ApplyEnvString(cmd *cobra.Command, flagName string, currentValue string, envValue string, currentSource ConfigSource) (string, ConfigSource) {
	// If flag was explicitly set on command line, keep it
	if cmd.Flags().Changed(flagName) {
		return currentValue, SourceFlag
	}

	// If env variable is set, use it (overrides config.toml)
	if envValue != "" {
		return envValue, SourceEnvironment
	}

	// Otherwise, keep current value and source
	return currentValue, currentSource
}

// ApplyEnvBool applies an environment variable bool value if set and flag was not changed.
func ApplyEnvBool(cmd *cobra.Command, flagName string, currentValue bool, envSet bool, currentSource ConfigSource) (bool, ConfigSource) {
	// If flag was explicitly set on command line, keep it
	if cmd.Flags().Changed(flagName) {
		return currentValue, SourceFlag
	}

	// If env variable is set, use true (env vars for bools typically mean "enable")
	if envSet {
		return true, SourceEnvironment
	}

	// Otherwise, keep current value and source
	return currentValue, currentSource
}
