package dvbcontext

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
)

// ErrNoNodes is returned when the devnet has no nodes.
var ErrNoNodes = errors.New("no nodes found")

// NodeSelection represents a selected node with both name and index.
type NodeSelection struct {
	Name  string // Short name: "validator-0", "fullnode-1"
	Index int    // Numeric index for gRPC calls
}

// NodeName derives the short node name from a Node's spec and metadata.
// Format: "{role}-{index}" e.g., "validator-0", "fullnode-1"
func NodeName(node *v1.Node) string {
	role := "unknown"
	if node.Spec != nil && node.Spec.Role != "" {
		role = strings.ToLower(node.Spec.Role)
	}

	index := int32(0)
	if node.Metadata != nil {
		index = node.Metadata.Index
	}

	return fmt.Sprintf("%s-%d", role, index)
}

// ResolveNodeName resolves a node name (e.g., "validator-0") to a NodeSelection
// by listing nodes from the daemon and finding the match.
func ResolveNodeName(ctx context.Context, c *client.Client, namespace, devnet, nodeName string) (*NodeSelection, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}

	nodes, err := c.ListNodes(ctx, namespace, devnet)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, n := range nodes {
		if NodeName(n) == nodeName {
			idx := 0
			if n.Metadata != nil {
				idx = int(n.Metadata.Index)
			}
			return &NodeSelection{Name: nodeName, Index: idx}, nil
		}
	}

	var available []string
	for _, n := range nodes {
		available = append(available, NodeName(n))
	}
	return nil, fmt.Errorf("node %q not found in devnet %q (available: %s)",
		nodeName, devnet, strings.Join(available, ", "))
}

// PickNode selects a node from the devnet.
// If there's only one node, it auto-selects.
// If multiple nodes, shows interactive picker.
// Returns a NodeSelection with both name and index.
func PickNode(ctx context.Context, c *client.Client, namespace, devnet string) (*NodeSelection, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}

	nodes, err := c.ListNodes(ctx, namespace, devnet)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("%w in devnet %s", ErrNoNodes, devnet)
	}

	// Auto-select if only one node
	if len(nodes) == 1 {
		idx := 0
		if nodes[0].Metadata != nil {
			idx = int(nodes[0].Metadata.Index)
		}
		return &NodeSelection{
			Name:  NodeName(nodes[0]),
			Index: idx,
		}, nil
	}

	// Show interactive picker for multiple nodes
	picked, err := fuzzyfinder.Find(nodes, func(i int) string {
		return formatNodeDisplay(nodes[i])
	})
	if err != nil {
		return nil, err
	}

	idx := 0
	if nodes[picked].Metadata != nil {
		idx = int(nodes[picked].Metadata.Index)
	}
	return &NodeSelection{
		Name:  NodeName(nodes[picked]),
		Index: idx,
	}, nil
}

// formatNodeDisplay formats a node for display in the picker.
// Format: "validator-0 (Running)"
func formatNodeDisplay(node *v1.Node) string {
	name := NodeName(node)

	phase := "Unknown"
	if node.Status != nil && node.Status.Phase != "" {
		phase = node.Status.Phase
	}

	return fmt.Sprintf("%s (%s)", name, phase)
}
