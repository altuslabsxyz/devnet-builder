package dvbcontext

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
)

// ErrNoNodes is returned when the devnet has no nodes.
var ErrNoNodes = errors.New("no nodes found")

// PickNode selects a node from the devnet.
// If there's only one node, it auto-selects.
// If multiple nodes, shows interactive picker.
// Returns node index and error.
func PickNode(c *client.Client, namespace, devnet string) (int, error) {
	if c == nil {
		return -1, errors.New("client is nil")
	}

	nodes, err := c.ListNodes(context.Background(), namespace, devnet)
	if err != nil {
		return -1, fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes) == 0 {
		return -1, fmt.Errorf("%w in devnet %s", ErrNoNodes, devnet)
	}

	// Auto-select if only one node
	if len(nodes) == 1 {
		if nodes[0].Metadata == nil {
			return 0, nil // Default to index 0 if metadata is missing
		}
		return int(nodes[0].Metadata.Index), nil
	}

	// Show interactive picker for multiple nodes
	idx, err := fuzzyfinder.Find(nodes, func(i int) string {
		return formatNodeDisplay(nodes[i])
	})
	if err != nil {
		// Return the error as-is (including fuzzyfinder.ErrAbort)
		return -1, err
	}

	if nodes[idx].Metadata == nil {
		return idx, nil // Default to selected index if metadata is missing
	}
	return int(nodes[idx].Metadata.Index), nil
}

// formatNodeDisplay formats a node for display in the picker.
// Format: "0: validator (Running)"
func formatNodeDisplay(node *v1.Node) string {
	role := "unknown"
	if node.Spec != nil && node.Spec.Role != "" {
		role = node.Spec.Role
	}

	phase := "Unknown"
	if node.Status != nil && node.Status.Phase != "" {
		phase = node.Status.Phase
	}

	index := int32(0)
	if node.Metadata != nil {
		index = node.Metadata.Index
	}

	return fmt.Sprintf("%d: %s (%s)", index, role, phase)
}
