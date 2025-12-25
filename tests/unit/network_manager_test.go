package unit

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/b-harvest/devnet-builder/internal/infrastructure/docker"
)

// TestNetworkManager_CreateNetwork tests basic network creation
func TestNetworkManager_CreateNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network manager test in short mode (requires Docker)")
	}

	ctx := context.Background()
	manager := docker.NewNetworkManager()

	tests := []struct {
		name       string
		devnetName string
		wantErr    bool
	}{
		{
			name:       "create network with valid name",
			devnetName: "test-network-1",
			wantErr:    false,
		},
		{
			name:       "create network with alphanumeric name",
			devnetName: "testnet123",
			wantErr:    false,
		},
		{
			name:       "create network with hyphens",
			devnetName: "my-test-network",
			wantErr:    false,
		},
		{
			name:       "reject empty name",
			devnetName: "",
			wantErr:    true,
		},
		{
			name:       "reject name with special characters",
			devnetName: "test@network",
			wantErr:    true,
		},
		{
			name:       "reject name with spaces",
			devnetName: "test network",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkID, subnet, err := manager.CreateNetwork(ctx, tt.devnetName)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Cleanup on success
			defer func() {
				if networkID != "" {
					manager.DeleteNetwork(ctx, networkID)
				}
			}()

			// Verify network ID is returned
			if networkID == "" {
				t.Error("networkID is empty")
			}

			// Verify subnet is in expected format (172.X.0.0/16)
			if !isValidSubnet(subnet) {
				t.Errorf("subnet = %v, want format 172.X.0.0/16", subnet)
			}

			// Verify network exists
			exists, err := manager.NetworkExists(ctx, networkID)
			if err != nil {
				t.Fatalf("NetworkExists() error = %v", err)
			}
			if !exists {
				t.Error("network was not created")
			}

			// Verify network name format
			// Note: We'd need to inspect the network to verify the exact name
			// For now, just verify it was created successfully
			_ = "devnet-" + tt.devnetName + "-network" // expected format
		})
	}
}

// TestNetworkManager_SubnetAutoIncrement tests subnet auto-increment on conflicts
func TestNetworkManager_SubnetAutoIncrement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subnet auto-increment test in short mode (requires Docker)")
	}

	ctx := context.Background()
	manager := docker.NewNetworkManager()

	// Create first network
	networkID1, subnet1, err := manager.CreateNetwork(ctx, "subnet-test-1")
	if err != nil {
		t.Fatalf("First CreateNetwork() failed: %v", err)
	}
	defer manager.DeleteNetwork(ctx, networkID1)

	// Create second network
	networkID2, subnet2, err := manager.CreateNetwork(ctx, "subnet-test-2")
	if err != nil {
		t.Fatalf("Second CreateNetwork() failed: %v", err)
	}
	defer manager.DeleteNetwork(ctx, networkID2)

	// Verify different subnets were allocated
	if subnet1 == subnet2 {
		t.Errorf("Expected different subnets, both got %v", subnet1)
	}

	// Verify both are in 172.X.0.0/16 format
	if !isValidSubnet(subnet1) {
		t.Errorf("subnet1 = %v, want format 172.X.0.0/16", subnet1)
	}
	if !isValidSubnet(subnet2) {
		t.Errorf("subnet2 = %v, want format 172.X.0.0/16", subnet2)
	}

	// Extract octet numbers
	octet1 := extractSecondOctet(subnet1)
	octet2 := extractSecondOctet(subnet2)

	// Verify second subnet has higher octet (auto-increment)
	if octet2 <= octet1 {
		t.Errorf("Expected subnet2 octet (%d) > subnet1 octet (%d)", octet2, octet1)
	}
}

// TestNetworkManager_DeleteNetwork tests network deletion
func TestNetworkManager_DeleteNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network deletion test in short mode (requires Docker)")
	}

	ctx := context.Background()
	manager := docker.NewNetworkManager()

	// Create network
	networkID, _, err := manager.CreateNetwork(ctx, "delete-test")
	if err != nil {
		t.Fatalf("CreateNetwork() failed: %v", err)
	}

	// Verify exists
	exists, err := manager.NetworkExists(ctx, networkID)
	if err != nil {
		t.Fatalf("NetworkExists() error = %v", err)
	}
	if !exists {
		t.Fatal("network was not created")
	}

	// Delete network
	err = manager.DeleteNetwork(ctx, networkID)
	if err != nil {
		t.Fatalf("DeleteNetwork() failed: %v", err)
	}

	// Verify removed
	exists, err = manager.NetworkExists(ctx, networkID)
	if err != nil {
		t.Fatalf("NetworkExists() after delete error = %v", err)
	}
	if exists {
		t.Error("network still exists after deletion")
	}

	// Attempting to delete again should fail
	err = manager.DeleteNetwork(ctx, networkID)
	if err == nil {
		t.Error("Expected error when deleting non-existent network")
	}
}

// TestNetworkManager_GetNetworkSubnet tests subnet retrieval
func TestNetworkManager_GetNetworkSubnet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping get subnet test in short mode (requires Docker)")
	}

	ctx := context.Background()
	manager := docker.NewNetworkManager()

	// Create network
	networkID, expectedSubnet, err := manager.CreateNetwork(ctx, "subnet-get-test")
	if err != nil {
		t.Fatalf("CreateNetwork() failed: %v", err)
	}
	defer manager.DeleteNetwork(ctx, networkID)

	// Get subnet
	retrievedSubnet, err := manager.GetNetworkSubnet(ctx, networkID)
	if err != nil {
		t.Fatalf("GetNetworkSubnet() error = %v", err)
	}

	// Verify subnet matches
	if retrievedSubnet != expectedSubnet {
		t.Errorf("GetNetworkSubnet() = %v, want %v", retrievedSubnet, expectedSubnet)
	}

	// Test with non-existent network
	fakeSubnet, err := manager.GetNetworkSubnet(ctx, "fake-network-id")
	if err != nil {
		t.Errorf("GetNetworkSubnet() with fake ID should not error: %v", err)
	}
	if fakeSubnet != "" {
		t.Errorf("Expected empty subnet for non-existent network, got %v", fakeSubnet)
	}
}

// TestNetworkManager_ListDevnetNetworks tests listing devnet networks
func TestNetworkManager_ListDevnetNetworks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping list networks test in short mode (requires Docker)")
	}

	ctx := context.Background()
	manager := docker.NewNetworkManager()

	// Create multiple networks
	network1, _, err := manager.CreateNetwork(ctx, "list-test-1")
	if err != nil {
		t.Fatalf("CreateNetwork() 1 failed: %v", err)
	}
	defer manager.DeleteNetwork(ctx, network1)

	network2, _, err := manager.CreateNetwork(ctx, "list-test-2")
	if err != nil {
		t.Fatalf("CreateNetwork() 2 failed: %v", err)
	}
	defer manager.DeleteNetwork(ctx, network2)

	network3, _, err := manager.CreateNetwork(ctx, "list-test-3")
	if err != nil {
		t.Fatalf("CreateNetwork() 3 failed: %v", err)
	}
	defer manager.DeleteNetwork(ctx, network3)

	// List all devnet networks
	networks, err := manager.ListDevnetNetworks(ctx)
	if err != nil {
		t.Fatalf("ListDevnetNetworks() error = %v", err)
	}

	// Should have at least our 3 networks
	if len(networks) < 3 {
		t.Errorf("Expected at least 3 networks, got %d", len(networks))
	}

	// Verify our networks are in the list
	foundIDs := make(map[string]bool)
	for _, net := range networks {
		foundIDs[net.ID] = true

		// Verify network info structure
		if net.ID == "" {
			t.Error("Network ID is empty")
		}
		if net.Name == "" {
			t.Error("Network Name is empty")
		}
		if net.Subnet == "" {
			t.Error("Network Subnet is empty")
		}
		if net.DevnetName == "" {
			t.Error("Network DevnetName is empty")
		}
	}

	// Check that our networks are present
	if !foundIDs[network1] {
		t.Error("network1 not found in list")
	}
	if !foundIDs[network2] {
		t.Error("network2 not found in list")
	}
	if !foundIDs[network3] {
		t.Error("network3 not found in list")
	}
}

// TestNetworkManager_NetworkExists tests network existence check
func TestNetworkManager_NetworkExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network exists test in short mode (requires Docker)")
	}

	ctx := context.Background()
	manager := docker.NewNetworkManager()

	// Test with non-existent network
	exists, err := manager.NetworkExists(ctx, "non-existent-network-id")
	if err != nil {
		t.Fatalf("NetworkExists() error = %v", err)
	}
	if exists {
		t.Error("Expected false for non-existent network")
	}

	// Create network
	networkID, _, err := manager.CreateNetwork(ctx, "exists-test")
	if err != nil {
		t.Fatalf("CreateNetwork() failed: %v", err)
	}
	defer manager.DeleteNetwork(ctx, networkID)

	// Test with existing network
	exists, err = manager.NetworkExists(ctx, networkID)
	if err != nil {
		t.Fatalf("NetworkExists() error = %v", err)
	}
	if !exists {
		t.Error("Expected true for existing network")
	}

	// Test with empty network ID
	exists, err = manager.NetworkExists(ctx, "")
	if err != nil {
		t.Errorf("NetworkExists() with empty ID should not error: %v", err)
	}
	if exists {
		t.Error("Expected false for empty network ID")
	}
}

// Helper functions

func isValidSubnet(subnet string) bool {
	// Check format: 172.X.0.0/16 where X is 20-254
	pattern := `^172\.(2[0-9]|[3-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-4])\.0\.0/16$`
	matched, _ := regexp.MatchString(pattern, subnet)
	return matched
}

func extractSecondOctet(subnet string) int {
	// Extract second octet from 172.X.0.0/16
	parts := strings.Split(subnet, ".")
	if len(parts) < 2 {
		return 0
	}
	var octet int
	fmt.Sscanf(parts[1], "%d", &octet)
	return octet
}
