package unit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/b-harvest/devnet-builder/internal/domain/ports"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/docker"
)

// TestPortAllocator_AllocateRange tests basic port allocation
func TestPortAllocator_AllocateRange(t *testing.T) {
	tests := []struct {
		name           string
		devnetName     string
		validatorCount int
		wantErr        bool
		expectedPorts  int
	}{
		{
			name:           "allocate for 1 validator",
			devnetName:     "test-1",
			validatorCount: 1,
			wantErr:        false,
			expectedPorts:  100,
		},
		{
			name:           "allocate for 4 validators",
			devnetName:     "test-4",
			validatorCount: 4,
			wantErr:        false,
			expectedPorts:  400,
		},
		{
			name:           "allocate for 10 validators",
			devnetName:     "test-10",
			validatorCount: 10,
			wantErr:        false,
			expectedPorts:  1000,
		},
		{
			name:           "allocate for 100 validators (max)",
			devnetName:     "test-100",
			validatorCount: 100,
			wantErr:        false,
			expectedPorts:  10000,
		},
		{
			name:           "reject 0 validators",
			devnetName:     "test-0",
			validatorCount: 0,
			wantErr:        true,
		},
		{
			name:           "reject 101 validators",
			devnetName:     "test-101",
			validatorCount: 101,
			wantErr:        true,
		},
		{
			name:           "reject empty devnet name",
			devnetName:     "",
			validatorCount: 4,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for registry
			tmpDir := t.TempDir()
			allocator := docker.NewPortAllocator(tmpDir)
			ctx := context.Background()

			// Allocate range
			allocation, err := allocator.AllocateRange(ctx, tt.devnetName, tt.validatorCount)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("AllocateRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify allocation
			if allocation == nil {
				t.Fatal("allocation is nil")
			}

			if allocation.DevnetName != tt.devnetName {
				t.Errorf("DevnetName = %v, want %v", allocation.DevnetName, tt.devnetName)
			}

			if allocation.ValidatorCount != tt.validatorCount {
				t.Errorf("ValidatorCount = %v, want %v", allocation.ValidatorCount, tt.validatorCount)
			}

			actualPorts := allocation.PortRangeEnd - allocation.PortRangeStart + 1
			if actualPorts != tt.expectedPorts {
				t.Errorf("Port range size = %v, want %v", actualPorts, tt.expectedPorts)
			}

			// Verify allocation is persisted
			retrieved, err := allocator.GetAllocation(ctx, tt.devnetName)
			if err != nil {
				t.Fatalf("GetAllocation() error = %v", err)
			}

			if retrieved == nil {
				t.Fatal("retrieved allocation is nil")
			}

			if retrieved.PortRangeStart != allocation.PortRangeStart {
				t.Errorf("Retrieved PortRangeStart = %v, want %v", retrieved.PortRangeStart, allocation.PortRangeStart)
			}
		})
	}
}

// TestPortAllocator_DuplicateDevnet tests that allocating the same devnet twice fails
func TestPortAllocator_DuplicateDevnet(t *testing.T) {
	tmpDir := t.TempDir()
	allocator := docker.NewPortAllocator(tmpDir)
	ctx := context.Background()

	// First allocation should succeed
	_, err := allocator.AllocateRange(ctx, "duplicate-test", 4)
	if err != nil {
		t.Fatalf("First allocation failed: %v", err)
	}

	// Second allocation for same devnet should fail
	_, err = allocator.AllocateRange(ctx, "duplicate-test", 4)
	if err == nil {
		t.Error("Expected error for duplicate devnet, got nil")
	}
}

// TestPortAllocator_ReleaseRange tests port range release
func TestPortAllocator_ReleaseRange(t *testing.T) {
	tmpDir := t.TempDir()
	allocator := docker.NewPortAllocator(tmpDir)
	ctx := context.Background()

	// Allocate
	allocation, err := allocator.AllocateRange(ctx, "release-test", 4)
	if err != nil {
		t.Fatalf("AllocateRange() failed: %v", err)
	}

	// Verify exists
	retrieved, err := allocator.GetAllocation(ctx, "release-test")
	if err != nil {
		t.Fatalf("GetAllocation() failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Allocation not found before release")
	}

	// Release
	err = allocator.ReleaseRange(ctx, "release-test")
	if err != nil {
		t.Fatalf("ReleaseRange() failed: %v", err)
	}

	// Verify removed
	retrieved, err = allocator.GetAllocation(ctx, "release-test")
	if err != nil {
		t.Fatalf("GetAllocation() after release failed: %v", err)
	}
	if retrieved != nil {
		t.Error("Allocation still exists after release")
	}

	// Should be able to allocate again with same name
	newAllocation, err := allocator.AllocateRange(ctx, "release-test", 4)
	if err != nil {
		t.Errorf("Re-allocation after release failed: %v", err)
	}

	// Verify port range can be reused (could be same or different)
	if newAllocation.PortRangeStart != allocation.PortRangeStart {
		t.Logf("Port range changed after re-allocation: %d -> %d",
			allocation.PortRangeStart, newAllocation.PortRangeStart)
	}
}

// TestPortAllocator_GapFinding tests that gaps between allocations are filled
func TestPortAllocator_GapFinding(t *testing.T) {
	tmpDir := t.TempDir()
	allocator := docker.NewPortAllocator(tmpDir)
	ctx := context.Background()

	// Allocate devnet1
	alloc1, err := allocator.AllocateRange(ctx, "devnet1", 4)
	if err != nil {
		t.Fatalf("First allocation failed: %v", err)
	}

	// Allocate devnet2
	alloc2, err := allocator.AllocateRange(ctx, "devnet2", 4)
	if err != nil {
		t.Fatalf("Second allocation failed: %v", err)
	}

	// Release devnet1 (creates a gap)
	err = allocator.ReleaseRange(ctx, "devnet1")
	if err != nil {
		t.Fatalf("ReleaseRange() failed: %v", err)
	}

	// Allocate devnet3 - should fill the gap
	alloc3, err := allocator.AllocateRange(ctx, "devnet3", 4)
	if err != nil {
		t.Fatalf("Third allocation failed: %v", err)
	}

	// Verify devnet3 uses the gap (should be same as devnet1's old range)
	if alloc3.PortRangeStart != alloc1.PortRangeStart {
		t.Errorf("Gap not filled: devnet3 start = %v, expected %v", alloc3.PortRangeStart, alloc1.PortRangeStart)
	}

	// Verify devnet2 is still intact
	retrieved2, err := allocator.GetAllocation(ctx, "devnet2")
	if err != nil {
		t.Fatalf("GetAllocation(devnet2) failed: %v", err)
	}
	if retrieved2.PortRangeStart != alloc2.PortRangeStart {
		t.Errorf("devnet2 was modified: start = %v, expected %v", retrieved2.PortRangeStart, alloc2.PortRangeStart)
	}
}

// TestPortAllocator_ListAllocations tests listing all allocations
func TestPortAllocator_ListAllocations(t *testing.T) {
	tmpDir := t.TempDir()
	allocator := docker.NewPortAllocator(tmpDir)
	ctx := context.Background()

	// Initially empty
	allocations, err := allocator.ListAllocations(ctx)
	if err != nil {
		t.Fatalf("ListAllocations() failed: %v", err)
	}
	if len(allocations) != 0 {
		t.Errorf("Expected empty list, got %d allocations", len(allocations))
	}

	// Add 3 allocations
	_, err = allocator.AllocateRange(ctx, "devnet-a", 4)
	if err != nil {
		t.Fatalf("Allocation failed: %v", err)
	}
	_, err = allocator.AllocateRange(ctx, "devnet-b", 4)
	if err != nil {
		t.Fatalf("Allocation failed: %v", err)
	}
	_, err = allocator.AllocateRange(ctx, "devnet-c", 4)
	if err != nil {
		t.Fatalf("Allocation failed: %v", err)
	}

	// List all
	allocations, err = allocator.ListAllocations(ctx)
	if err != nil {
		t.Fatalf("ListAllocations() failed: %v", err)
	}
	if len(allocations) != 3 {
		t.Errorf("Expected 3 allocations, got %d", len(allocations))
	}

	// Verify sorted by port range start
	for i := 1; i < len(allocations); i++ {
		if allocations[i].PortRangeStart <= allocations[i-1].PortRangeStart {
			t.Errorf("Allocations not sorted: [%d].start=%v <= [%d].start=%v",
				i, allocations[i].PortRangeStart, i-1, allocations[i-1].PortRangeStart)
		}
	}
}

// TestPortAllocator_ConcurrentAccess tests file locking with concurrent access
func TestPortAllocator_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent access test in short mode")
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Run 5 concurrent allocations
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(index int) {
			allocator := docker.NewPortAllocator(tmpDir)
			devnetName := fmt.Sprintf("concurrent-devnet-%d", index)
			_, err := allocator.AllocateRange(ctx, devnetName, 2)
			done <- err
		}(i)
	}

	// Collect results - all allocations should succeed with proper locking
	successCount := 0
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent allocation failed: %v", err)
		} else {
			successCount++
		}
	}

	// Verify successful allocations were persisted
	allocator := docker.NewPortAllocator(tmpDir)
	allocations, err := allocator.ListAllocations(ctx)
	if err != nil {
		t.Fatalf("ListAllocations() failed: %v", err)
	}

	// Should have exactly as many allocations as successful operations
	if len(allocations) != successCount {
		t.Errorf("Expected %d allocations (matching successful operations), got %d", successCount, len(allocations))
	}

	// Main assertion: with proper file locking, all 5 should succeed
	if successCount != 5 {
		t.Errorf("Expected all 5 concurrent allocations to succeed, only %d succeeded", successCount)
	}

	// Verify no overlapping port ranges
	for i := 0; i < len(allocations); i++ {
		for j := i + 1; j < len(allocations); j++ {
			if rangesOverlap(allocations[i], allocations[j]) {
				t.Errorf("Allocations %d and %d have overlapping port ranges", i, j)
			}
		}
	}
}

// TestPortAllocator_RegistryPersistence tests that allocations survive process restart
func TestPortAllocator_RegistryPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create allocator and allocate
	allocator1 := docker.NewPortAllocator(tmpDir)
	alloc, err := allocator1.AllocateRange(ctx, "persist-test", 4)
	if err != nil {
		t.Fatalf("AllocateRange() failed: %v", err)
	}

	// Verify registry file exists
	registryPath := filepath.Join(tmpDir, "port-registry.json")
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Error("Registry file not created")
	}

	// Create new allocator instance (simulates process restart)
	allocator2 := docker.NewPortAllocator(tmpDir)

	// Retrieve allocation
	retrieved, err := allocator2.GetAllocation(ctx, "persist-test")
	if err != nil {
		t.Fatalf("GetAllocation() failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Allocation not persisted")
	}

	// Verify details match
	if retrieved.PortRangeStart != alloc.PortRangeStart {
		t.Errorf("PortRangeStart = %v, want %v", retrieved.PortRangeStart, alloc.PortRangeStart)
	}
	if retrieved.PortRangeEnd != alloc.PortRangeEnd {
		t.Errorf("PortRangeEnd = %v, want %v", retrieved.PortRangeEnd, alloc.PortRangeEnd)
	}
	if retrieved.ValidatorCount != alloc.ValidatorCount {
		t.Errorf("ValidatorCount = %v, want %v", retrieved.ValidatorCount, alloc.ValidatorCount)
	}
}

// Helper functions

func rangesOverlap(a1, a2 *ports.PortAllocation) bool {
	return !(a1.PortRangeEnd < a2.PortRangeStart || a2.PortRangeEnd < a1.PortRangeStart)
}
