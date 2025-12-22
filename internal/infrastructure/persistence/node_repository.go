package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// NodeFileRepository implements NodeRepository using the filesystem.
type NodeFileRepository struct {
	metadataFilename string
}

// NewNodeFileRepository creates a new NodeFileRepository.
func NewNodeFileRepository() *NodeFileRepository {
	return &NodeFileRepository{
		metadataFilename: "node.json",
	}
}

// nodeDir returns the node directory path.
func (r *NodeFileRepository) nodeDir(homeDir string, index int) string {
	return filepath.Join(homeDir, "devnet", fmt.Sprintf("node%d", index))
}

// metadataPath returns the path to the node metadata file.
func (r *NodeFileRepository) metadataPath(homeDir string, index int) string {
	return filepath.Join(r.nodeDir(homeDir, index), r.metadataFilename)
}

// nodesBaseDir returns the base nodes directory.
func (r *NodeFileRepository) nodesBaseDir(homeDir string) string {
	return filepath.Join(homeDir, "devnet")
}

// Save persists a node's metadata to storage.
// Note: node.HomeDir should be the full path to the node directory
// (e.g., ~/.devnet-builder/devnet/node0)
func (r *NodeFileRepository) Save(ctx context.Context, node *ports.NodeMetadata) error {
	if node == nil {
		return fmt.Errorf("node metadata is nil")
	}

	// node.HomeDir is already the full path to the node directory
	if err := os.MkdirAll(node.HomeDir, 0755); err != nil {
		return fmt.Errorf("failed to create node directory: %w", err)
	}

	stored := r.toStoredFormat(node)
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal node metadata: %w", err)
	}

	// Write node.json directly to the node's home directory
	path := filepath.Join(node.HomeDir, r.metadataFilename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write node metadata: %w", err)
	}

	return nil
}

// Load retrieves a node's metadata from storage.
func (r *NodeFileRepository) Load(ctx context.Context, homeDir string, index int) (*ports.NodeMetadata, error) {
	path := r.metadataPath(homeDir, index)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{Path: path}
		}
		return nil, fmt.Errorf("failed to read node metadata: %w", err)
	}

	var stored storedNodeMetadata
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to parse node metadata: %w", err)
	}

	return r.fromStoredFormat(&stored), nil
}

// LoadAll retrieves all nodes for a devnet.
func (r *NodeFileRepository) LoadAll(ctx context.Context, homeDir string) ([]*ports.NodeMetadata, error) {
	nodesDir := r.nodesBaseDir(homeDir)

	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No nodes yet
		}
		return nil, fmt.Errorf("failed to read nodes directory: %w", err)
	}

	var nodes []*ports.NodeMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse node index from directory name (node0, node1, etc.)
		var index int
		if _, err := fmt.Sscanf(entry.Name(), "node%d", &index); err != nil {
			continue // Skip non-node directories
		}

		node, err := r.Load(ctx, homeDir, index)
		if err != nil {
			if IsNotFound(err) {
				continue // Skip nodes without metadata
			}
			return nil, fmt.Errorf("failed to load node %d: %w", index, err)
		}

		nodes = append(nodes, node)
	}

	// Sort by index
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Index < nodes[j].Index
	})

	return nodes, nil
}

// Delete removes a node's data from storage.
func (r *NodeFileRepository) Delete(ctx context.Context, homeDir string, index int) error {
	dir := r.nodeDir(homeDir, index)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to delete node directory: %w", err)
	}
	return nil
}

// storedNodeMetadata is the JSON storage format.
type storedNodeMetadata struct {
	Index       int                `json:"index"`
	Name        string             `json:"name"`
	HomeDir     string             `json:"home_dir"`
	ChainID     string             `json:"chain_id"`
	NodeID      string             `json:"node_id"`
	PID         *int               `json:"pid,omitempty"`
	ContainerID string             `json:"container_id,omitempty"`
	Ports       storedPortConfig   `json:"ports"`
}

type storedPortConfig struct {
	RPC     int `json:"rpc"`
	P2P     int `json:"p2p"`
	GRPC    int `json:"grpc"`
	API     int `json:"api"`
	EVM     int `json:"evm,omitempty"`
	EVMWS   int `json:"evm_ws,omitempty"`
	PProf   int `json:"pprof,omitempty"`
	Rosetta int `json:"rosetta,omitempty"`
}

// toStoredFormat converts ports.NodeMetadata to storage format.
func (r *NodeFileRepository) toStoredFormat(n *ports.NodeMetadata) *storedNodeMetadata {
	return &storedNodeMetadata{
		Index:       n.Index,
		Name:        n.Name,
		HomeDir:     n.HomeDir,
		ChainID:     n.ChainID,
		NodeID:      n.NodeID,
		PID:         n.PID,
		ContainerID: n.ContainerID,
		Ports: storedPortConfig{
			RPC:     n.Ports.RPC,
			P2P:     n.Ports.P2P,
			GRPC:    n.Ports.GRPC,
			API:     n.Ports.API,
			EVM:     n.Ports.EVM,
			EVMWS:   n.Ports.EVMWS,
			PProf:   n.Ports.PProf,
			Rosetta: n.Ports.Rosetta,
		},
	}
}

// fromStoredFormat converts storage format to ports.NodeMetadata.
func (r *NodeFileRepository) fromStoredFormat(s *storedNodeMetadata) *ports.NodeMetadata {
	return &ports.NodeMetadata{
		Index:       s.Index,
		Name:        s.Name,
		HomeDir:     s.HomeDir,
		ChainID:     s.ChainID,
		NodeID:      s.NodeID,
		PID:         s.PID,
		ContainerID: s.ContainerID,
		Ports: ports.PortConfig{
			RPC:     s.Ports.RPC,
			P2P:     s.Ports.P2P,
			GRPC:    s.Ports.GRPC,
			API:     s.Ports.API,
			EVM:     s.Ports.EVM,
			EVMWS:   s.Ports.EVMWS,
			PProf:   s.Ports.PProf,
			Rosetta: s.Ports.Rosetta,
		},
	}
}

// Ensure NodeFileRepository implements NodeRepository.
var _ ports.NodeRepository = (*NodeFileRepository)(nil)
