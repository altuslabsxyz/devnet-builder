package main

import (
	"context"
	"fmt"
	"os"

	"github.com/b-harvest/devnet-builder/internal/domain"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/interactive"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// runInteractiveVersionSelection orchestrates the unified binary source selection flow.
// This function unifies the selection logic for both deploy and upgrade commands,
// implementing User Story 1 (Interactive Source Selection) and User Story 3 (Unified Function Integration).
//
// Flow:
//  1. Prompt user to select binary source (local filesystem or GitHub release)
//  2. If local: Launch filesystem browser with autocomplete (User Story 2)
//  3. If GitHub: Fetch releases and prompt for version selection (existing flow)
//  4. Optionally prompt for network selection (only for upgrade command)
//  5. Return SelectionConfig with all user choices
//
// Parameters:
//   - ctx: Context for cancellation support
//   - cmd: Cobra command for accessing flags and config
//   - includeNetworkSelection: true for upgrade (prompts for network), false for deploy (network from config)
//
// Returns:
//   - *interactive.SelectionConfig: User's selections (source, version, network)
//   - error: User cancellation (exit code 130), validation failure, or system error
//
// Design Decision: This function follows Clean Architecture by depending on port interfaces
// (SourceSelector, FilesystemBrowser) rather than concrete implementations. The actual
// infrastructure adapters (promptui) are injected via constructor functions.
//
// User Stories:
//   - US1: Interactive Source Selection (FR-001, FR-002)
//   - US2: Filesystem Browser with Autocomplete (FR-008, FR-012)
//   - US3: Unified Function Integration (FR-024, SC-004)
//
// Edge Cases:
//   - EC-001: Non-interactive environment → defaults to GitHub releases
//   - EC-005: User cancellation (Ctrl+C/ESC) → returns exit code 130
//   - EC-003: Invalid binary → shows error, allows retry
//
// Deprecated: Use runInteractiveVersionSelectionWithMode instead for new code.
func runInteractiveVersionSelection(
	ctx context.Context,
	cmd *cobra.Command,
	includeNetworkSelection bool,
	network string,
) (*interactive.SelectionConfig, error) {
	// Default to deploy mode (not upgrade)
	// Pass network to ensure correct network is used for cache lookup and display
	return runInteractiveVersionSelectionWithMode(ctx, cmd, includeNetworkSelection, false, network)
}

// runInteractiveVersionSelectionWithMode orchestrates binary source selection with explicit mode control.
//
// Parameters:
//   - ctx: Context for cancellation support
//   - cmd: Cobra command for accessing flags and config
//   - includeNetworkSelection: true for upgrade (prompts for network), false for deploy (network from config)
//   - forUpgrade: true for upgrade command (collects only upgrade target version),
//     false for deploy command (collects export and start versions)
//   - network: Network type from config (used when includeNetworkSelection is false)
//
// Returns:
//   - *interactive.SelectionConfig: User's selections (source, version, network)
//   - error: User cancellation (exit code 130), validation failure, or system error
func runInteractiveVersionSelectionWithMode(
	ctx context.Context,
	cmd *cobra.Command,
	includeNetworkSelection bool,
	forUpgrade bool,
	network string,
) (*interactive.SelectionConfig, error) {
	// Step 1: Source Selection (US1: FR-001, FR-002)
	// Prompt user to choose between local binary and GitHub release
	sourceSelector := interactive.NewSourceSelectorAdapter()
	selectedSourceType, err := sourceSelector.SelectSource(ctx)
	if err != nil {
		// User cancellation (Ctrl+C or ESC) - exit code 130 handled by caller
		// EC-005: User Cancellation
		return nil, handleUserCancellation(err)
	}

	// Initialize SelectionConfig with source type
	config := &interactive.SelectionConfig{
		BinarySource:            domain.NewBinarySource(selectedSourceType),
		IncludeNetworkSelection: includeNetworkSelection,
	}

	// Step 2: Branch based on source type
	switch selectedSourceType {
	case domain.SourceTypeLocal:
		// US2: Filesystem Browser with Autocomplete (FR-008, FR-012)
		// Launch interactive filesystem browser for local binary selection
		if err := handleLocalBinarySelection(ctx, config); err != nil {
			return nil, err
		}
		// For deploy command, set network from config (network selection was skipped)
		if !includeNetworkSelection && network != "" {
			config.Network = network
		}

	case domain.SourceTypeGitHubRelease:
		// Existing flow: Fetch releases and prompt for version selection
		// forUpgrade controls whether to use upgrade flow (single version) or deploy flow (export/start versions)
		// Pass network from config for deploy command (when includeNetworkSelection is false)
		if err := handleGitHubReleaseSelection(ctx, cmd, config, forUpgrade, network); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported source type: %s", selectedSourceType)
	}

	// Step 3: Network Selection (only for upgrade command)
	// FR-026: Network selection is conditional - only for upgrade, not for deploy
	if includeNetworkSelection {
		if err := handleNetworkSelection(ctx, config); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// handleLocalBinarySelection orchestrates filesystem browsing for local binary selection.
// This implements User Story 2: Filesystem Browser with Autocomplete.
//
// Flow:
//  1. Launch filesystem browser with Tab autocomplete (FR-008, FR-012)
//  2. User navigates filesystem and selects binary (FR-009, FR-010)
//  3. Validate selected binary (executable, correct architecture) (FR-013, FR-014)
//  4. Update BinarySource with validated path
//
// Parameters:
//   - ctx: Context for cancellation
//   - config: SelectionConfig to populate with selected path
//
// Returns:
//   - error: Validation failure, user cancellation, or system error
//
// Edge Cases:
//   - EC-003: Invalid binary → shows error, allows retry (not implemented yet, placeholder)
//   - EC-005: Symbolic links → resolves to target with max depth 10
func handleLocalBinarySelection(ctx context.Context, config *interactive.SelectionConfig) error {
	// T022-T029: Filesystem Browser with Autocomplete (Phase 4: User Story 2)
	// Create PathCompleter and FilesystemBrowser adapters
	pathCompleter := interactive.NewPathCompleterAdapter()
	browser := interactive.NewFilesystemBrowserAdapter(pathCompleter)

	// Launch interactive filesystem browser with Tab autocomplete (FR-008, FR-012)
	// User navigates filesystem and selects binary (FR-009, FR-010)
	// Browser validates selection (executable, correct architecture) (FR-013, FR-014)
	selectedPath, err := browser.BrowsePath(ctx, "/usr/local/bin/")
	if err != nil {
		// User cancellation (Ctrl+C or ESC) or validation failure
		// EC-005: User Cancellation
		return handleUserCancellation(err)
	}

	// Update BinarySource with validated path
	config.BinarySource.SelectedPath = selectedPath
	config.BinarySource.MarkValid()

	return nil
}

// handleGitHubReleaseSelection orchestrates GitHub release fetching and version selection.
// This reuses the existing release selection flow.
//
// Flow:
//  1. Setup GitHub client with credentials and cache
//  2. Fetch releases using FetchReleasesWithCache (FR-003)
//  3. Present version list with promptui (existing flow)
//  4. Update SelectionConfig with selected version
//
// Parameters:
//   - ctx: Context for cancellation
//   - cmd: Cobra command for config access
//   - config: SelectionConfig to populate with version info
//   - forUpgrade: true for upgrade command (single version), false for deploy (export/start versions)
//
// Returns:
//   - error: Network failure, user cancellation, or API error
//
// Design Decision: Reuses existing Selector.RunVersionSelectionFlow for deploy
// and Selector.RunUpgradeSelectionFlow for upgrade to avoid code duplication.
// This aligns with US3: Unified Function Integration (SC-004).
//
// Parameters:
//   - network: Network type from config (used when config.IncludeNetworkSelection is false, i.e., deploy command)
func handleGitHubReleaseSelection(ctx context.Context, cmd *cobra.Command, config *interactive.SelectionConfig, forUpgrade bool, network string) error {
	fileCfg := GetLoadedFileConfig()

	// Use unified GitHub client setup (eliminates code duplication)
	client := setupGitHubClient(homeDir, fileCfg)

	// Create selector and run version selection flow
	selector := interactive.NewSelector(client)

	if forUpgrade {
		// Upgrade mode: use dedicated upgrade selection flow
		// This only prompts for single upgrade target version (no export/start distinction)
		upgradeConfig, err := selector.RunUpgradeSelectionFlow(ctx)
		if err != nil {
			return handleUserCancellation(err)
		}

		// Map UpgradeSelectionConfig to SelectionConfig
		// For upgrade, we store the target version in StartVersion (which is used by upgrade.go)
		config.StartVersion = upgradeConfig.UpgradeVersion
		config.StartIsCustomRef = upgradeConfig.IsCustomRef
		// ExportVersion is not used in upgrade mode, but we set it to empty to avoid confusion
		config.ExportVersion = ""
		config.ExportIsCustomRef = false

		// Mark BinarySource as valid (download will happen later in upgrade flow)
		config.BinarySource.MarkValid()

		return nil
	}

	// Deploy mode: use existing version selection flow (export + start versions)
	// For deploy: network is passed from config (when includeNetworkSelection is false)
	// For upgrade: network will be selected in Step 3 (handleNetworkSelection), so empty string is passed
	// The network parameter is now passed from the caller instead of being hardcoded

	// Run existing version selection flow with the network from config
	versionConfig, err := selector.RunVersionSelectionFlow(ctx, network)
	if err != nil {
		return handleUserCancellation(err)
	}

	// Populate SelectionConfig with version information
	config.Network = versionConfig.Network
	config.ExportVersion = versionConfig.ExportVersion
	config.StartVersion = versionConfig.StartVersion
	config.ExportIsCustomRef = versionConfig.ExportIsCustomRef
	config.StartIsCustomRef = versionConfig.StartIsCustomRef

	// Mark BinarySource as valid (download will happen later in deployment flow)
	config.BinarySource.MarkValid()

	return nil
}

// handleNetworkSelection prompts user to select network for upgrade command.
// This implements FR-026: Network selection is optional and only for upgrade.
//
// Flow:
//  1. Present network options (mainnet, testnet)
//  2. User selects with arrow keys
//  3. Update SelectionConfig.Network
//
// Parameters:
//   - ctx: Context for cancellation
//   - config: SelectionConfig to populate with network choice
//
// Returns:
//   - error: User cancellation or system error
func handleNetworkSelection(ctx context.Context, config *interactive.SelectionConfig) error {
	// Use existing network selection options from types.go
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Name | cyan }} - {{ .Description }}",
		Inactive: "  {{ .Name }} - {{ .Description }}",
		Selected: "✓ {{ .Name | green }}",
	}

	prompt := promptui.Select{
		Label:     "Select network:",
		Items:     interactive.Networks,
		Templates: templates,
		Size:      len(interactive.Networks),
	}

	index, _, err := prompt.Run()
	if err != nil {
		return handleUserCancellation(err)
	}

	config.Network = interactive.Networks[index].Name
	return nil
}

// handleUserCancellation checks if error is user cancellation (Ctrl+C or ESC).
// Returns appropriate error for exit code 130 (user cancellation).
//
// This implements EC-005: User Cancellation edge case handling.
//
// Parameters:
//   - err: Error from promptui.Select or promptui.Prompt
//
// Returns:
//   - error: Same error if not cancellation, wrapped error with exit code 130 context
func handleUserCancellation(err error) error {
	if err == promptui.ErrInterrupt {
		// Ctrl+C pressed
		fmt.Fprintln(os.Stderr, "\n✗ Selection cancelled by user")
		return fmt.Errorf("user cancelled selection (exit code 130): %w", err)
	}
	if err == promptui.ErrEOF {
		// ESC pressed
		fmt.Fprintln(os.Stderr, "\n✗ Selection cancelled by user")
		return fmt.Errorf("user cancelled selection (exit code 130): %w", err)
	}
	return err
}
