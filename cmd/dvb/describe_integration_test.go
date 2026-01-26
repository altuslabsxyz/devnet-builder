//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
)

func TestDescribeIntegration(t *testing.T) {
	c, err := client.New()
	if err != nil {
		t.Skip("daemon not running, skipping integration test")
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	devnets, err := c.ListDevnets(ctx)
	if err != nil {
		t.Fatalf("failed to list devnets: %v", err)
	}

	if len(devnets) == 0 {
		t.Skip("no devnets found, skipping")
	}

	// Get first devnet
	devnet, err := c.GetDevnet(ctx, devnets[0].Metadata.Name)
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	// Verify we have conditions (after the controller changes)
	if len(devnet.Status.Conditions) == 0 {
		t.Log("warning: no conditions found on devnet")
	}
}
