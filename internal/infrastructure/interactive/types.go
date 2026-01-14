package interactive

import (
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain"
)

// SelectionConfig represents the user's selection state during interactive mode for start command.
// Enhanced to support unified binary source selection (local filesystem or GitHub release).
type SelectionConfig struct {
	Network          string // "mainnet" or "testnet"
	StartVersion     string // Version for devnet binary (used for both export and start)
	StartIsCustomRef bool   // True if start version is a custom branch/commit
	UpgradeName      string // Upgrade handler name (for upgrade command only)

	// BinarySource represents the user's choice of binary origin (local or GitHub release).
	// This field is populated by the unified selection flow (runInteractiveVersionSelection).
	// If nil, the selection flow will prompt the user to choose a source.
	BinarySource *domain.BinarySource

	// SourceSelectionTimestamp records when the source selection was made.
	// Used for debugging and audit purposes.
	SourceSelectionTimestamp time.Time

	// IncludeNetworkSelection controls whether network selection prompt is shown.
	// - false for deploy command (network pre-determined from config.toml)
	// - true for upgrade command (user selects network interactively)
	IncludeNetworkSelection bool
}

// UpgradeSelectionConfig represents the user's selection state during interactive upgrade mode.
type UpgradeSelectionConfig struct {
	UpgradeName    string // Upgrade handler name
	UpgradeVersion string // Target version for upgrade (tag or custom ref)
	IsCustomRef    bool   // True if upgrade version is a custom branch/commit
}

// VersionItem represents a version item for display in promptui.
type VersionItem struct {
	TagName      string
	PublishedAt  time.Time
	IsPrerelease bool
	IsLatest     bool
	IsCustom     bool // True for custom branch/commit option
}

// String returns display string for promptui.
func (v VersionItem) String() string {
	if v.IsCustom {
		return v.TagName
	}
	suffix := ""
	if v.IsLatest {
		suffix = " (latest)"
	} else if v.IsPrerelease {
		suffix = " (pre-release)"
	}
	return fmt.Sprintf("%s - %s%s", v.TagName, v.PublishedAt.Format("2006-01-02"), suffix)
}

// NetworkOption represents a network option for selection.
type NetworkOption struct {
	Name        string
	Description string
}

// Networks available for selection.
var Networks = []NetworkOption{
	{Name: "mainnet", Description: "Stable mainnet network"},
	{Name: "testnet", Description: "Stable testnet network"},
}

// DockerImageItem represents a docker image version for display in promptui.
type DockerImageItem struct {
	Tag       string
	CreatedAt time.Time
	IsLatest  bool
	IsCustom  bool // True for "Enter custom image..." option
}

// String returns display string for promptui.
func (d DockerImageItem) String() string {
	if d.IsCustom {
		return d.Tag
	}
	suffix := ""
	if d.IsLatest {
		suffix = " (latest)"
	}
	return fmt.Sprintf("%s - %s%s", d.Tag, d.CreatedAt.Format("2006-01-02"), suffix)
}
