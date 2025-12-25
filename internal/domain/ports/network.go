package ports

import (
	"context"
	"time"
)

// NetworkManager manages Docker networks for devnet isolation
type NetworkManager interface {
	// CreateNetwork creates an isolated bridge network for a devnet
	// Returns NetworkID and subnet CIDR that was allocated
	// Auto-increments subnet if conflicts detected (172.20.0.0/16 → 172.21.0.0/16)
	CreateNetwork(ctx context.Context, devnetName string) (networkID string, subnet string, err error)

	// DeleteNetwork removes a Docker network
	// Returns error if network doesn't exist or has attached containers
	DeleteNetwork(ctx context.Context, networkID string) error

	// NetworkExists checks if a network with given ID exists
	NetworkExists(ctx context.Context, networkID string) (bool, error)

	// GetNetworkSubnet retrieves the subnet CIDR for a network
	// Returns empty string if network doesn't exist
	GetNetworkSubnet(ctx context.Context, networkID string) (string, error)

	// ListDevnetNetworks returns all networks created by devnet-builder
	// Filters by label: app=devnet-builder
	ListDevnetNetworks(ctx context.Context) ([]NetworkInfo, error)
}

// NetworkInfo represents a Docker network created by devnet-builder
type NetworkInfo struct {
	ID         string    // Docker network ID
	Name       string    // Network name (devnet-{name}-network)
	Subnet     string    // CIDR notation (e.g., 172.20.0.0/16)
	Gateway    string    // Gateway IP address
	DevnetName string    // Devnet this network belongs to
	CreatedAt  time.Time // Network creation timestamp
}
