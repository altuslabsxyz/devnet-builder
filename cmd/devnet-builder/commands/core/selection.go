package core

import (
	"context"
	"fmt"
	"os"

	"github.com/b-harvest/devnet-builder/cmd/devnet-builder/shared"
	"github.com/b-harvest/devnet-builder/internal/domain"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/interactive"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// RunInteractiveVersionSelection orchestrates the unified binary source selection flow.
// This function unifies the selection logic for both deploy and upgrade commands.
//
// Parameters:
//   - ctx: Context for cancellation support
//   - cmd: Cobra command for accessing flags and config
//   - includeNetworkSelection: true for upgrade (prompts for network), false for deploy (network from config)
//   - network: Network type from config (used when includeNetworkSelection is false)
//
// Returns:
//   - *interactive.SelectionConfig: User's selections (source, version, network)
//   - error: User cancellation (exit code 130), validation failure, or system error
func RunInteractiveVersionSelection(
	ctx context.Context,
	cmd *cobra.Command,
	includeNetworkSelection bool,
	network string,
) (*interactive.SelectionConfig, error) {
	// Default to deploy mode (not upgrade), with skipUpgradeName = false
	return RunInteractiveVersionSelectionWithMode(ctx, cmd, includeNetworkSelection, false, network, false)
}

// RunInteractiveVersionSelectionWithMode orchestrates binary source selection with explicit mode control.
//
// Parameters:
//   - ctx: Context for cancellation support
//   - cmd: Cobra command for accessing flags and config
//   - includeNetworkSelection: true for upgrade (prompts for network), false for deploy (network from config)
//   - forUpgrade: true for upgrade command (collects only upgrade target version),
//     false for deploy command (collects export and start versions)
//   - network: Network type from config (used when includeNetworkSelection is false)
//   - skipUpgradeName: true to skip upgrade name prompt (for --skip-gov mode)
//
// Returns:
//   - *interactive.SelectionConfig: User's selections (source, version, network)
//   - error: User cancellation (exit code 130), validation failure, or system error
func RunInteractiveVersionSelectionWithMode(
	ctx context.Context,
	cmd *cobra.Command,
	includeNetworkSelection bool,
	forUpgrade bool,
	network string,
	skipUpgradeName bool,
) (*interactive.SelectionConfig, error) {
	// Step 1: Source Selection
	sourceSelector := interactive.NewSourceSelectorAdapter()
	selectedSourceType, err := sourceSelector.SelectSource(ctx)
	if err != nil {
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
		if err := handleLocalBinarySelection(ctx, config); err != nil {
			return nil, err
		}
		// For deploy command, set network from config (network selection was skipped)
		if !includeNetworkSelection && network != "" {
			config.Network = network
		}

	case domain.SourceTypeGitHubRelease:
		if err := handleGitHubReleaseSelection(ctx, cmd, config, forUpgrade, network, skipUpgradeName); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported source type: %s", selectedSourceType)
	}

	// Step 3: Network Selection (only for upgrade command)
	if includeNetworkSelection {
		if err := handleNetworkSelection(ctx, config); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// handleLocalBinarySelection orchestrates filesystem browsing for local binary selection.
func handleLocalBinarySelection(ctx context.Context, config *interactive.SelectionConfig) error {
	pathCompleter := interactive.NewPathCompleterAdapter()
	browser := interactive.NewFilesystemBrowserAdapter(pathCompleter)

	selectedPath, err := browser.BrowsePath(ctx, "/usr/local/bin/")
	if err != nil {
		return handleUserCancellation(err)
	}

	config.BinarySource.SelectedPath = selectedPath
	config.BinarySource.MarkValid()

	return nil
}

// handleGitHubReleaseSelection orchestrates GitHub release fetching and version selection.
// When skipUpgradeName is true (e.g., --skip-gov mode), it skips the upgrade name prompt.
func handleGitHubReleaseSelection(ctx context.Context, cmd *cobra.Command, config *interactive.SelectionConfig, forUpgrade bool, network string, skipUpgradeName bool) error {
	fileCfg := shared.GetLoadedFileConfig()
	homeDir := shared.GetHomeDir()

	// Use unified GitHub client setup
	client := SetupGitHubClient(homeDir, fileCfg)

	// Create selector and run version selection flow
	selector := interactive.NewSelector(client)

	if forUpgrade {
		upgradeConfig, err := selector.RunUpgradeSelectionFlow(ctx, skipUpgradeName)
		if err != nil {
			return handleUserCancellation(err)
		}

		config.StartVersion = upgradeConfig.UpgradeVersion
		config.StartIsCustomRef = upgradeConfig.IsCustomRef
		config.UpgradeName = upgradeConfig.UpgradeName
		config.BinarySource.MarkValid()

		return nil
	}

	// Deploy mode
	versionConfig, err := selector.RunVersionSelectionFlow(ctx, network)
	if err != nil {
		return handleUserCancellation(err)
	}

	config.Network = versionConfig.Network
	config.StartVersion = versionConfig.StartVersion
	config.StartIsCustomRef = versionConfig.StartIsCustomRef
	config.BinarySource.MarkValid()

	return nil
}

// handleNetworkSelection prompts user to select network for upgrade command.
func handleNetworkSelection(ctx context.Context, config *interactive.SelectionConfig) error {
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
func handleUserCancellation(err error) error {
	if err == promptui.ErrInterrupt {
		fmt.Fprintln(os.Stderr, "\n✗ Selection cancelled by user")
		return fmt.Errorf("user cancelled selection (exit code 130): %w", err)
	}
	if err == promptui.ErrEOF {
		fmt.Fprintln(os.Stderr, "\n✗ Selection cancelled by user")
		return fmt.Errorf("user cancelled selection (exit code 130): %w", err)
	}
	return err
}
