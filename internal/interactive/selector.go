package interactive

import (
	"context"
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/stablelabs/stable-devnet/internal/github"
)

// Selector handles the interactive selection workflow.
type Selector struct {
	client *github.Client
}

// NewSelector creates a new interactive selector.
func NewSelector(client *github.Client) *Selector {
	return &Selector{
		client: client,
	}
}

// RunSelectionFlow runs the complete interactive selection workflow for start command.
// Returns the selection config and any error (including cancellation).
func (s *Selector) RunSelectionFlow(ctx context.Context) (*SelectionConfig, error) {
	config := &SelectionConfig{}

	// Step 1: Select network
	network, err := SelectNetwork()
	if err != nil {
		return nil, handleInterruptError(err)
	}
	config.Network = network

	// Step 2: Fetch available versions
	releases, fromCache, err := s.client.FetchReleasesWithCache(ctx)
	if err != nil {
		// Check if it's a warning (stale data)
		if warning, ok := err.(*github.StaleDataWarning); ok {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning.Message)
		} else {
			return nil, fmt.Errorf("failed to fetch versions: %w", err)
		}
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no versions available. Check your network connection or GitHub token")
	}

	if fromCache {
		fmt.Println("(Using cached version data)")
	}

	// Step 3: Select export version
	exportVersion, exportIsCustom, err := SelectVersion("Select export binary version", releases, "")
	if err != nil {
		return nil, handleInterruptError(err)
	}
	config.ExportVersion = exportVersion
	config.ExportIsCustomRef = exportIsCustom

	// Step 4: Select start version (default to export version)
	startVersion, startIsCustom, err := SelectVersion("Select node start binary version", releases, exportVersion)
	if err != nil {
		return nil, handleInterruptError(err)
	}
	config.StartVersion = startVersion
	config.StartIsCustomRef = startIsCustom

	// Step 5: Confirm selection
	confirmed, err := ConfirmSelection(config)
	if err != nil {
		return nil, handleInterruptError(err)
	}
	if !confirmed {
		return nil, &CancellationError{Message: "Operation cancelled by user"}
	}

	return config, nil
}

// RunUpgradeSelectionFlow runs the interactive selection workflow for upgrade command.
// Returns the upgrade selection config and any error (including cancellation).
func (s *Selector) RunUpgradeSelectionFlow(ctx context.Context) (*UpgradeSelectionConfig, error) {
	config := &UpgradeSelectionConfig{}

	// Step 1: Fetch available versions
	releases, fromCache, err := s.client.FetchReleasesWithCache(ctx)
	if err != nil {
		// Check if it's a warning (stale data)
		if warning, ok := err.(*github.StaleDataWarning); ok {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning.Message)
		} else {
			return nil, fmt.Errorf("failed to fetch versions: %w", err)
		}
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no versions available. Check your network connection or GitHub token")
	}

	if fromCache {
		fmt.Println("(Using cached version data)")
	}

	// Step 2: Select upgrade version (the binary to upgrade to)
	upgradeVersion, isCustomRef, err := SelectVersion("Select upgrade target version", releases, "")
	if err != nil {
		return nil, handleInterruptError(err)
	}
	config.UpgradeVersion = upgradeVersion
	config.IsCustomRef = isCustomRef

	// Step 3: Prompt for upgrade name (handler name)
	upgradeName, err := PromptUpgradeName(upgradeVersion)
	if err != nil {
		return nil, handleInterruptError(err)
	}
	config.UpgradeName = upgradeName

	// Step 4: Confirm selection
	confirmed, err := ConfirmUpgradeSelection(config)
	if err != nil {
		return nil, handleInterruptError(err)
	}
	if !confirmed {
		return nil, &CancellationError{Message: "Operation cancelled by user"}
	}

	return config, nil
}

// handleInterruptError converts promptui errors to appropriate error types.
func handleInterruptError(err error) error {
	if err == promptui.ErrInterrupt {
		return &CancellationError{Message: "Operation cancelled"}
	}
	if err == promptui.ErrEOF {
		return &CancellationError{Message: "Operation cancelled (EOF)"}
	}
	return err
}

// CancellationError indicates the user cancelled the operation.
type CancellationError struct {
	Message string
}

func (e *CancellationError) Error() string {
	return e.Message
}

// IsCancellation returns true if the error is a cancellation error.
func IsCancellation(err error) bool {
	_, ok := err.(*CancellationError)
	return ok
}
