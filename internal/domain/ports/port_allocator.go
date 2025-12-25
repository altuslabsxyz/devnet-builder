package ports

import (
	"context"
	"time"
)

// PortAllocator manages port range allocation for devnets to prevent conflicts
type PortAllocator interface {
	// AllocateRange reserves a contiguous port range for a devnet
	// Returns PortAllocation with range details
	// Thread-safe with advisory file locking
	AllocateRange(ctx context.Context, devnetName string, validatorCount int) (*PortAllocation, error)

	// ReleaseRange frees a previously allocated port range
	// Makes ports available for future allocations
	ReleaseRange(ctx context.Context, devnetName string) error

	// GetAllocation retrieves allocation details for a devnet
	// Returns nil if no allocation exists
	GetAllocation(ctx context.Context, devnetName string) (*PortAllocation, error)

	// ValidatePortAvailability checks if allocated ports conflict with host services
	// Returns list of conflicting ports if any
	ValidatePortAvailability(ctx context.Context, allocation *PortAllocation) ([]int, error)

	// ListAllocations returns all active port allocations
	ListAllocations(ctx context.Context) ([]*PortAllocation, error)
}

// PortAllocation represents a reserved port range for a devnet
type PortAllocation struct {
	DevnetName     string    `json:"devnet_name"`
	NetworkName    string    `json:"network_name"`
	Subnet         string    `json:"subnet"`
	PortRangeStart int       `json:"port_range_start"`
	PortRangeEnd   int       `json:"port_range_end"`
	ValidatorCount int       `json:"validator_count"`
	AllocatedAt    time.Time `json:"allocated_at"`
}

// PortsPerValidator is the number of ports each validator needs
const PortsPerValidator = 100

// GlobalPortRangeStart is the start of the allocatable port range
const GlobalPortRangeStart = 26000

// GlobalPortRangeEnd is the end of the allocatable port range
const GlobalPortRangeEnd = 36000
