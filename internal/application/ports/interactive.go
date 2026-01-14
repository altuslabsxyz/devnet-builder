package ports

// SelectionConfig represents the user's selection for devnet start.
type SelectionConfig struct {
	Network          string // "mainnet" or "testnet"
	StartVersion     string // Version for devnet binary (used for both export and start)
	StartIsCustomRef bool   // True if start version is a custom branch/commit
}

// UpgradeSelectionConfig represents the user's selection for upgrade.
type UpgradeSelectionConfig struct {
	UpgradeName    string // Upgrade handler name
	UpgradeVersion string // Target version for upgrade
	IsCustomRef    bool   // True if upgrade version is a custom ref
}

// NetworkOption represents a network option for selection.
type NetworkOption struct {
	Name        string
	Description string
}

// InteractiveSelector defines operations for interactive user prompts.
type InteractiveSelector interface {
	// SelectNetwork prompts user to select a network.
	SelectNetwork() (string, error)

	// SelectVersion prompts user to select a version from releases.
	// prompt is the display prompt, releases are available options,
	// defaultVersion is pre-selected if provided.
	SelectVersion(prompt string, releases []GitHubRelease, defaultVersion string) (string, bool, error)

	// SelectDockerImage prompts user to select a docker image version.
	SelectDockerImage(prompt string, versions []ImageVersion, defaultVersion string) (string, bool, error)

	// PromptUpgradeName prompts user for an upgrade name.
	PromptUpgradeName(defaultName string) (string, error)

	// ConfirmSelection asks user to confirm their selection.
	ConfirmSelection(config *SelectionConfig) (bool, error)

	// ConfirmUpgradeSelection asks user to confirm upgrade selection.
	ConfirmUpgradeSelection(config *UpgradeSelectionConfig) (bool, error)

	// ConfirmAction asks user to confirm a generic action.
	ConfirmAction(message string) (bool, error)
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
