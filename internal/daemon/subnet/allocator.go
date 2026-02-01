// Package subnet provides subnet allocation for loopback network aliasing.
package subnet

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
)

// minSubnet is the minimum allocatable subnet (0 is reserved).
const minSubnet uint8 = 1

// maxSubnet is the maximum allocatable subnet (255 is reserved for broadcast).
const maxSubnet uint8 = 254

// Allocator manages subnet allocations for devnets, ensuring each devnet
// gets a unique subnet in the 127.0.X.0/24 range for loopback aliasing.
type Allocator struct {
	path        string
	allocations map[uint8]string // subnet -> "namespace/devnetName"
	mu          sync.RWMutex
}

// persistedState represents the JSON structure for persistence.
type persistedState struct {
	Allocations map[string]string `json:"allocations"` // subnet (as string) -> "namespace/devnetName"
}

// LoadOrCreate loads an existing allocator from the given path, or creates
// a new one if the file doesn't exist.
func LoadOrCreate(path string) (*Allocator, error) {
	a := &Allocator{
		path:        path,
		allocations: make(map[uint8]string),
	}

	// Check if file exists
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		// Save empty state
		if err := a.save(); err != nil {
			return nil, fmt.Errorf("failed to initialize allocator: %w", err)
		}
		return a, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read allocator file: %w", err)
	}

	// Parse existing state
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse allocator file: %w", err)
	}

	// Convert string keys to uint8
	for subnetStr, devnetKey := range state.Allocations {
		var subnet uint8
		if _, err := fmt.Sscanf(subnetStr, "%d", &subnet); err != nil {
			return nil, fmt.Errorf("invalid subnet key %q: %w", subnetStr, err)
		}
		a.allocations[subnet] = devnetKey
	}

	return a, nil
}

// devnetKey returns the canonical key for a devnet.
func devnetKey(namespace, devnetName string) string {
	return namespace + "/" + devnetName
}

// hashToSubnet computes a preferred subnet using FNV-32a hash.
// Maps the hash to range [1, 254].
func hashToSubnet(key string) uint8 {
	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()

	// Map to range [1, 254] (254 possible values)
	// Using modulo to distribute across the range
	return uint8((hash % uint32(maxSubnet-minSubnet+1)) + uint32(minSubnet))
}

// Allocate allocates a subnet for the given devnet. If the devnet already
// has an allocation, returns the existing subnet. Uses a hash-based approach
// with auto-increment collision handling.
func (a *Allocator) Allocate(namespace, devnetName string) (uint8, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := devnetKey(namespace, devnetName)

	// Check if already allocated
	for subnet, existingKey := range a.allocations {
		if existingKey == key {
			return subnet, nil
		}
	}

	// Compute preferred subnet from hash
	preferred := hashToSubnet(key)

	// Find available subnet starting from preferred
	subnet := preferred
	for {
		if _, taken := a.allocations[subnet]; !taken {
			// Found available subnet
			a.allocations[subnet] = key
			if err := a.save(); err != nil {
				delete(a.allocations, subnet)
				return 0, fmt.Errorf("failed to persist allocation: %w", err)
			}
			return subnet, nil
		}

		// Move to next subnet with wraparound
		subnet++
		if subnet > maxSubnet {
			subnet = minSubnet
		}

		// Check if we've wrapped around completely (all subnets taken)
		if subnet == preferred {
			return 0, fmt.Errorf("no available subnets")
		}
	}
}

// Release releases the subnet allocation for the given devnet.
func (a *Allocator) Release(namespace, devnetName string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := devnetKey(namespace, devnetName)

	// Find and remove the allocation
	for subnet, existingKey := range a.allocations {
		if existingKey == key {
			delete(a.allocations, subnet)
			if err := a.save(); err != nil {
				// Restore on failure
				a.allocations[subnet] = key
				return fmt.Errorf("failed to persist release: %w", err)
			}
			return nil
		}
	}

	// Not found is not an error - idempotent release
	return nil
}

// GetSubnet returns the allocated subnet for a devnet, if one exists.
func (a *Allocator) GetSubnet(namespace, devnetName string) (uint8, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	key := devnetKey(namespace, devnetName)

	for subnet, existingKey := range a.allocations {
		if existingKey == key {
			return subnet, true
		}
	}

	return 0, false
}

// save persists the current allocations to the JSON file.
func (a *Allocator) save() error {
	state := persistedState{
		Allocations: make(map[string]string),
	}

	for subnet, key := range a.allocations {
		state.Allocations[fmt.Sprintf("%d", subnet)] = key
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically using temp file
	tmpPath := a.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, a.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// NodeIP returns the loopback IP address for a node in the given subnet.
// Format: 127.0.{subnet}.{nodeIndex+1}
// nodeIndex is 0-based, so node 0 gets .1, node 1 gets .2, etc.
func NodeIP(subnet uint8, nodeIndex int) string {
	return fmt.Sprintf("127.0.%d.%d", subnet, nodeIndex+1)
}

// ListAllocations returns a copy of all current allocations.
// Useful for debugging and status reporting.
func (a *Allocator) ListAllocations() map[uint8]string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[uint8]string, len(a.allocations))
	for subnet, key := range a.allocations {
		result[subnet] = key
	}
	return result
}
