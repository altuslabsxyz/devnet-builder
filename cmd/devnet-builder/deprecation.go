package main

import (
	"fmt"
	"os"
)

// DeprecationEnvVar is the environment variable to suppress deprecation warnings.
const DeprecationEnvVar = "STABLE_DEVNET_NO_DEPRECATION"

// DeprecationWarningFormat is the format string for deprecation warnings.
const DeprecationWarningFormat = "WARNING: '%s' is deprecated. Use '%s' instead.\n"

// CommandMapping maps old command names to their new equivalents.
var CommandMapping = map[string]string{
	"run":       "start",
	"up":        "start",
	"down":      "stop",
	"provision": "init",
	"clean":     "destroy",
}

// PrintDeprecationWarning prints a deprecation warning to stderr.
// The warning is suppressed if:
// - JSON mode is enabled (machine output)
// - STABLE_DEVNET_NO_DEPRECATION environment variable is set
func PrintDeprecationWarning(oldCmd, newCmd string) {
	// Suppress in JSON mode
	if jsonMode {
		return
	}

	// Suppress if environment variable is set
	if os.Getenv(DeprecationEnvVar) != "" {
		return
	}

	fmt.Fprintf(os.Stderr, DeprecationWarningFormat, oldCmd, newCmd)
}

// IsDeprecationSuppressed returns true if deprecation warnings are suppressed.
func IsDeprecationSuppressed() bool {
	return jsonMode || os.Getenv(DeprecationEnvVar) != ""
}

// GetNewCommand returns the new command name for a deprecated command.
// Returns empty string if the command is not deprecated.
func GetNewCommand(oldCmd string) string {
	return CommandMapping[oldCmd]
}
