package docker

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/domain/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/persistence"
)

// PortAllocatorImpl implements the PortAllocator interface
type PortAllocatorImpl struct {
	registry *persistence.PortRegistryFile
}

// NewPortAllocator creates a new port allocator
func NewPortAllocator(homeDir string) *PortAllocatorImpl {
	return &PortAllocatorImpl{
		registry: persistence.NewPortRegistryFile(homeDir),
	}
}

// AllocateRange reserves a contiguous port range for a devnet
func (p *PortAllocatorImpl) AllocateRange(ctx context.Context, devnetName string, validatorCount int) (*ports.PortAllocation, error) {
	// Validate inputs
	if err := p.validateInputs(devnetName, validatorCount); err != nil {
		return nil, err
	}

	var allocation *ports.PortAllocation

	// Use lock to ensure atomic read-modify-write
	err := p.registry.WithLock(ctx, func() error {
		// Load existing allocations
		allocations, err := p.registry.Load(ctx)
		if err != nil {
			return fmt.Errorf("failed to load allocations: %w", err)
		}

		// Check if devnet already has allocation
		for _, alloc := range allocations {
			if alloc.DevnetName == devnetName {
				return fmt.Errorf("devnet %s already has port allocation", devnetName)
			}
		}

		// Calculate required range size
		rangeSize := validatorCount * ports.PortsPerValidator

		// Find available range
		rangeStart, err := p.findAvailableRange(allocations, rangeSize)
		if err != nil {
			return err
		}

		// Create new allocation
		allocation = &ports.PortAllocation{
			DevnetName:     devnetName,
			NetworkName:    fmt.Sprintf("devnet-%s-network", devnetName),
			Subnet:         "", // Will be filled by network manager
			PortRangeStart: rangeStart,
			PortRangeEnd:   rangeStart + rangeSize - 1,
			ValidatorCount: validatorCount,
			AllocatedAt:    time.Now(),
		}

		// Add to allocations
		allocations = append(allocations, allocation)

		// Save updated allocations
		return p.registry.Save(ctx, allocations)
	})

	if err != nil {
		return nil, err
	}

	return allocation, nil
}

// ReleaseRange frees a previously allocated port range
func (p *PortAllocatorImpl) ReleaseRange(ctx context.Context, devnetName string) error {
	return p.registry.WithLock(ctx, func() error {
		// Load existing allocations
		allocations, err := p.registry.Load(ctx)
		if err != nil {
			return fmt.Errorf("failed to load allocations: %w", err)
		}

		// Find and remove allocation
		found := false
		newAllocations := make([]*ports.PortAllocation, 0, len(allocations))
		for _, alloc := range allocations {
			if alloc.DevnetName == devnetName {
				found = true
				continue
			}
			newAllocations = append(newAllocations, alloc)
		}

		if !found {
			return fmt.Errorf("no port allocation found for devnet %s", devnetName)
		}

		// Save updated allocations
		return p.registry.Save(ctx, newAllocations)
	})
}

// GetAllocation retrieves allocation details for a devnet
func (p *PortAllocatorImpl) GetAllocation(ctx context.Context, devnetName string) (*ports.PortAllocation, error) {
	allocations, err := p.registry.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load allocations: %w", err)
	}

	for _, alloc := range allocations {
		if alloc.DevnetName == devnetName {
			return alloc, nil
		}
	}

	return nil, nil
}

// ListAllocations returns all active port allocations
func (p *PortAllocatorImpl) ListAllocations(ctx context.Context) ([]*ports.PortAllocation, error) {
	allocations, err := p.registry.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load allocations: %w", err)
	}

	// Sort by port range start
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].PortRangeStart < allocations[j].PortRangeStart
	})

	return allocations, nil
}

// ValidatePortAvailability checks if allocated ports conflict with host services
func (p *PortAllocatorImpl) ValidatePortAvailability(ctx context.Context, allocation *ports.PortAllocation) ([]int, error) {
	if allocation == nil {
		return nil, errors.New("allocation cannot be nil")
	}

	// Get list of ports in use on host
	inUsePorts, err := p.getHostPortsInUse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host ports: %w", err)
	}

	// Check for conflicts
	conflicts := []int{}
	for port := allocation.PortRangeStart; port <= allocation.PortRangeEnd; port++ {
		if containsInt(inUsePorts, port) {
			conflicts = append(conflicts, port)
		}
	}

	return conflicts, nil
}

// validateInputs validates devnet name and validator count
func (p *PortAllocatorImpl) validateInputs(devnetName string, validatorCount int) error {
	if devnetName == "" {
		return errors.New("devnet name cannot be empty")
	}

	if validatorCount < 1 || validatorCount > 100 {
		return fmt.Errorf("validator count must be 1-100, got %d", validatorCount)
	}

	return nil
}

// findAvailableRange finds an available port range of the given size
func (p *PortAllocatorImpl) findAvailableRange(allocations []*ports.PortAllocation, size int) (int, error) {
	// Sort allocations by start port
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].PortRangeStart < allocations[j].PortRangeStart
	})

	// Check gap before first allocation
	if len(allocations) == 0 {
		return ports.GlobalPortRangeStart, nil
	}

	if allocations[0].PortRangeStart-ports.GlobalPortRangeStart >= size {
		return ports.GlobalPortRangeStart, nil
	}

	// Check gaps between allocations
	for i := 0; i < len(allocations)-1; i++ {
		gapStart := allocations[i].PortRangeEnd + 1
		gapEnd := allocations[i+1].PortRangeStart - 1
		gapSize := gapEnd - gapStart + 1

		if gapSize >= size {
			return gapStart, nil
		}
	}

	// Check gap after last allocation
	lastEnd := allocations[len(allocations)-1].PortRangeEnd
	if ports.GlobalPortRangeEnd-lastEnd >= size {
		return lastEnd + 1, nil
	}

	return 0, fmt.Errorf("no available port range of size %d in %d-%d", size, ports.GlobalPortRangeStart, ports.GlobalPortRangeEnd)
}

// getHostPortsInUse returns list of ports currently in use on the host
func (p *PortAllocatorImpl) getHostPortsInUse(ctx context.Context) ([]int, error) {
	// Try lsof first (works on macOS and Linux)
	ports, err := p.getPortsWithLsof(ctx)
	if err == nil {
		return ports, nil
	}

	// Fallback to netstat (works on Linux)
	ports, err = p.getPortsWithNetstat(ctx)
	if err == nil {
		return ports, nil
	}

	// If both fail, return empty list (assume no conflicts)
	return []int{}, nil
}

// getPortsWithLsof gets ports in use using lsof command
func (p *PortAllocatorImpl) getPortsWithLsof(ctx context.Context) ([]int, error) {
	cmd := exec.CommandContext(ctx, "lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return p.parsePortsFromOutput(string(output))
}

// getPortsWithNetstat gets ports in use using netstat command
func (p *PortAllocatorImpl) getPortsWithNetstat(ctx context.Context) ([]int, error) {
	cmd := exec.CommandContext(ctx, "netstat", "-tuln")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return p.parsePortsFromOutput(string(output))
}

// parsePortsFromOutput extracts port numbers from command output
func (p *PortAllocatorImpl) parsePortsFromOutput(output string) ([]int, error) {
	ports := []int{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// Look for port numbers in the line
		fields := strings.Fields(line)
		for _, field := range fields {
			// Check if field contains a colon (address:port format)
			if strings.Contains(field, ":") {
				parts := strings.Split(field, ":")
				if len(parts) >= 2 {
					portStr := parts[len(parts)-1]
					if port, err := strconv.Atoi(portStr); err == nil {
						ports = append(ports, port)
					}
				}
			}
		}
	}

	return ports, nil
}

// containsInt checks if an int slice contains a value
func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
