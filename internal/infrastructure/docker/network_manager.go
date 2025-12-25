package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain/ports"
)

// NetworkManager errors
var (
	ErrDockerDaemonUnavailable = errors.New("docker daemon not accessible")
	ErrInvalidDevnetName       = errors.New("devnet name contains invalid characters")
	ErrNoAvailableSubnets      = errors.New("no available subnets in 172.x.0.0/16 range")
	ErrNetworkCreationFailed   = errors.New("docker network create command failed")
	ErrNetworkNotFound         = errors.New("docker network not found")
	ErrNetworkHasContainers    = errors.New("network has attached containers, cannot delete")
	ErrNetworkDeletionFailed   = errors.New("docker network rm command failed")
)

// NetworkManagerImpl implements the NetworkManager interface
type NetworkManagerImpl struct{}

// NewNetworkManager creates a new network manager
func NewNetworkManager() *NetworkManagerImpl {
	return &NetworkManagerImpl{}
}

// CreateNetwork creates an isolated bridge network for a devnet
func (m *NetworkManagerImpl) CreateNetwork(ctx context.Context, devnetName string) (string, string, error) {
	// Validate devnet name
	if err := m.validateDevnetName(devnetName); err != nil {
		return "", "", err
	}

	// Find available subnet
	subnet, err := m.findAvailableSubnet(ctx)
	if err != nil {
		return "", "", err
	}

	// Generate network name
	networkName := fmt.Sprintf("devnet-%s-network", devnetName)

	// Create network using Docker CLI
	cmd := exec.CommandContext(ctx, "docker", "network", "create",
		"--driver", "bridge",
		"--subnet", subnet,
		"--label", "app=devnet-builder",
		"--label", fmt.Sprintf("devnet-name=%s", devnetName),
		networkName,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("%w: %s", ErrNetworkCreationFailed, string(output))
	}

	// Extract network ID from output (Docker returns the network ID)
	networkID := strings.TrimSpace(string(output))

	return networkID, subnet, nil
}

// DeleteNetwork removes a Docker network
func (m *NetworkManagerImpl) DeleteNetwork(ctx context.Context, networkID string) error {
	// Check if network exists
	exists, err := m.NetworkExists(ctx, networkID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNetworkNotFound
	}

	// Attempt to delete network
	cmd := exec.CommandContext(ctx, "docker", "network", "rm", networkID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if error is due to attached containers
		if strings.Contains(string(output), "has active endpoints") {
			return ErrNetworkHasContainers
		}
		return fmt.Errorf("%w: %s", ErrNetworkDeletionFailed, string(output))
	}

	return nil
}

// NetworkExists checks if a network with given ID exists
func (m *NetworkManagerImpl) NetworkExists(ctx context.Context, networkID string) (bool, error) {
	if networkID == "" {
		return false, nil
	}

	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", networkID)
	err := cmd.Run()
	if err != nil {
		// Check if error is "network not found"
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		// Other errors (e.g., Docker daemon unavailable)
		return false, ErrDockerDaemonUnavailable
	}

	return true, nil
}

// GetNetworkSubnet retrieves the subnet CIDR for a network
func (m *NetworkManagerImpl) GetNetworkSubnet(ctx context.Context, networkID string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", networkID,
		"--format", "{{range .IPAM.Config}}{{.Subnet}}{{end}}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Network doesn't exist, return empty string
		return "", nil
	}

	subnet := strings.TrimSpace(string(output))
	return subnet, nil
}

// ListDevnetNetworks returns all networks created by devnet-builder
func (m *NetworkManagerImpl) ListDevnetNetworks(ctx context.Context) ([]ports.NetworkInfo, error) {
	// List networks with devnet-builder label
	cmd := exec.CommandContext(ctx, "docker", "network", "ls",
		"--filter", "label=app=devnet-builder",
		"--format", "{{.ID}}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	// Parse network IDs
	networkIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(networkIDs) == 1 && networkIDs[0] == "" {
		// No networks found
		return []ports.NetworkInfo{}, nil
	}

	// Get details for each network
	networks := make([]ports.NetworkInfo, 0, len(networkIDs))
	for _, networkID := range networkIDs {
		info, err := m.getNetworkInfo(ctx, networkID)
		if err != nil {
			// Skip networks that can't be inspected
			continue
		}
		networks = append(networks, info)
	}

	return networks, nil
}

// validateDevnetName ensures name is alphanumeric + hyphens
func (m *NetworkManagerImpl) validateDevnetName(name string) error {
	if name == "" {
		return ErrInvalidDevnetName
	}

	// Allow alphanumeric characters and hyphens
	validName := regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	if !validName.MatchString(name) {
		return ErrInvalidDevnetName
	}

	return nil
}

// findAvailableSubnet finds an available subnet in 172.X.0.0/16 range
func (m *NetworkManagerImpl) findAvailableSubnet(ctx context.Context) (string, error) {
	// Get existing subnets
	existingSubnets, err := m.listExistingSubnets(ctx)
	if err != nil {
		return "", err
	}

	// Try subnets from 172.20.0.0/16 to 172.254.0.0/16
	for octet := 20; octet < 255; octet++ {
		subnet := fmt.Sprintf("172.%d.0.0/16", octet)
		if !contains(existingSubnets, subnet) {
			return subnet, nil
		}
	}

	return "", ErrNoAvailableSubnets
}

// listExistingSubnets gets all existing Docker network subnets
func (m *NetworkManagerImpl) listExistingSubnets(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "network", "ls", "--format", "{{.ID}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, ErrDockerDaemonUnavailable
	}

	networkIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
	subnets := []string{}

	for _, networkID := range networkIDs {
		if networkID == "" {
			continue
		}

		subnet, err := m.GetNetworkSubnet(ctx, networkID)
		if err == nil && subnet != "" {
			subnets = append(subnets, subnet)
		}
	}

	return subnets, nil
}

// getNetworkInfo retrieves detailed information about a network
func (m *NetworkManagerImpl) getNetworkInfo(ctx context.Context, networkID string) (ports.NetworkInfo, error) {
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", networkID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ports.NetworkInfo{}, fmt.Errorf("failed to inspect network: %w", err)
	}

	// Parse JSON output
	var inspectResults []struct {
		ID      string `json:"Id"`
		Name    string `json:"Name"`
		Created string `json:"Created"`
		IPAM    struct {
			Config []struct {
				Subnet  string `json:"Subnet"`
				Gateway string `json:"Gateway"`
			} `json:"Config"`
		} `json:"IPAM"`
		Labels map[string]string `json:"Labels"`
	}

	if err := json.Unmarshal(output, &inspectResults); err != nil {
		return ports.NetworkInfo{}, fmt.Errorf("failed to parse network inspect: %w", err)
	}

	if len(inspectResults) == 0 {
		return ports.NetworkInfo{}, ErrNetworkNotFound
	}

	result := inspectResults[0]

	// Extract subnet and gateway
	subnet := ""
	gateway := ""
	if len(result.IPAM.Config) > 0 {
		subnet = result.IPAM.Config[0].Subnet
		gateway = result.IPAM.Config[0].Gateway
	}

	// Parse created timestamp
	createdAt, _ := time.Parse(time.RFC3339, result.Created)

	return ports.NetworkInfo{
		ID:         result.ID,
		Name:       result.Name,
		Subnet:     subnet,
		Gateway:    gateway,
		DevnetName: result.Labels["devnet-name"],
		CreatedAt:  createdAt,
	}, nil
}

// contains checks if a slice contains a value
func contains(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
