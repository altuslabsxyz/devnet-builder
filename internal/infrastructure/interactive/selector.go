package interactive

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/github"
	"github.com/manifoldco/promptui"
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

	// Step 2-5: Run version selection flow
	return s.runVersionSelection(ctx, config)
}

// RunVersionSelectionFlow runs version selection workflow with pre-determined network.
// This is used when network is already configured (e.g., from config.toml or flags).
// Returns the selection config and any error (including cancellation).
func (s *Selector) RunVersionSelectionFlow(ctx context.Context, network string) (*SelectionConfig, error) {
	config := &SelectionConfig{
		Network: network,
	}
	return s.runVersionSelection(ctx, config)
}

// runVersionSelection is the internal implementation for version selection steps.
// This method now includes binary source selection (local or GitHub release).
func (s *Selector) runVersionSelection(ctx context.Context, config *SelectionConfig) (*SelectionConfig, error) {
	// Step 1: Select binary source (local filesystem or GitHub release)
	sourceSelector := NewSourceSelectorAdapter()
	sourceType, err := sourceSelector.SelectSource(ctx)
	if err != nil {
		return nil, handleInterruptError(err)
	}

	// Record selection timestamp for debugging/audit
	config.SourceSelectionTimestamp = time.Now()

	// Handle local binary selection
	if sourceType == domain.SourceTypeLocal {
		// Create filesystem browser with path completer
		pathCompleter := NewPathCompleterAdapter()
		browser := NewFilesystemBrowserAdapter(pathCompleter)

		// Browse for binary file
		binaryPath, err := browser.BrowsePath(ctx, "")
		if err != nil {
			return nil, handleInterruptError(err)
		}

		// Create and populate BinarySource
		binarySource := domain.NewBinarySource(domain.SourceTypeLocal)
		binarySource.SelectedPath = binaryPath
		binarySource.MarkValid() // Path was validated by BrowsePath

		config.BinarySource = binarySource
		// For local binary, version is determined from the binary itself
		config.StartVersion = "local"
		config.StartIsCustomRef = false

		// Confirm selection
		confirmed, err := ConfirmLocalBinarySelection(config)
		if err != nil {
			return nil, handleInterruptError(err)
		}
		if !confirmed {
			return nil, &CancellationError{Message: "Operation cancelled by user"}
		}

		return config, nil
	}

	// Handle GitHub release selection (existing flow)
	binarySource := domain.NewBinarySource(domain.SourceTypeGitHubRelease)
	config.BinarySource = binarySource

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

	// Note: fromCache is only true when API failed and we fell back to cache
	// StaleDataWarning is already displayed in that case, so no additional message needed
	_ = fromCache

	// Step 3: Select devnet binary version (used for both export and start)
	startVersion, startIsCustom, err := SelectVersion("Select devnet binary version", releases, "")
	if err != nil {
		return nil, handleInterruptError(err)
	}
	config.StartVersion = startVersion
	config.StartIsCustomRef = startIsCustom

	// Step 4: Confirm selection
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
// When skipUpgradeName is true (e.g., --skip-gov mode), it skips the upgrade name prompt
// since there's no governance proposal requiring a handler name.
// Returns the upgrade selection config and any error (including cancellation).
func (s *Selector) RunUpgradeSelectionFlow(ctx context.Context, skipUpgradeName bool) (*UpgradeSelectionConfig, error) {
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

	// Note: fromCache is only true when API failed and we fell back to cache
	// StaleDataWarning is already displayed in that case, so no additional message needed
	_ = fromCache

	// Step 2: Select upgrade version (the binary to upgrade to)
	upgradeVersion, isCustomRef, err := SelectVersion("Select upgrade target version", releases, "")
	if err != nil {
		return nil, handleInterruptError(err)
	}
	config.UpgradeVersion = upgradeVersion
	config.IsCustomRef = isCustomRef

	// Step 3: Prompt for upgrade name (handler name) - skip if skipUpgradeName is true
	if !skipUpgradeName {
		upgradeName, err := PromptUpgradeName(upgradeVersion)
		if err != nil {
			return nil, handleInterruptError(err)
		}
		config.UpgradeName = upgradeName
	}

	// Step 4: Confirm selection
	confirmed, err := ConfirmUpgradeSelection(config, skipUpgradeName)
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
