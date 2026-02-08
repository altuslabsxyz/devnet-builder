// cmd/dvb/interactive.go
package main

import (
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/tui"
)

var (
	// flagYes auto-confirms all confirmation prompts.
	flagYes bool
	// flagNonInteractive disables all interactive UI elements (pickers, wizards, TUI).
	flagNonInteractive bool
)

// IsNonInteractive returns true if interactive mode should be disabled.
// Checks the --non-interactive flag, DVB_NON_INTERACTIVE=1 / CI=true env vars, or non-TTY.
func IsNonInteractive() bool {
	return flagNonInteractive ||
		os.Getenv("DVB_NON_INTERACTIVE") == "1" ||
		os.Getenv("CI") == "true" ||
		!tui.IsInteractive()
}

// ShouldSkipConfirm returns true if confirmation prompts should be auto-accepted.
// Checks --yes flag, --non-interactive flag, env vars, or non-TTY.
func ShouldSkipConfirm() bool {
	return flagYes || IsNonInteractive()
}
