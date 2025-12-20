package main

import (
	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/helpers"
	"github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/internal/provision"
)

// LoadedDevnet contains the result of loading a devnet.
// This is a convenience wrapper that provides typed access to the loaded components.
type LoadedDevnet struct {
	Metadata *devnet.DevnetMetadata
	Devnet   *devnet.Devnet
}

// newDevnetLoader creates a DevnetLoader configured for this CLI.
// It injects the real devnet package functions as callbacks.
func newDevnetLoader(logger *output.Logger) *helpers.DevnetLoader {
	return &helpers.DevnetLoader{
		HomeDir: homeDir,
		Logger:  logger,
		ExistsCheck: func(path string) bool {
			return devnet.DevnetExists(path)
		},
		MetadataLoader: func(path string) (interface{}, error) {
			return devnet.LoadDevnetMetadata(path)
		},
		NodesLoader: func(path string, meta interface{}) (interface{}, error) {
			// meta is already *devnet.DevnetMetadata from MetadataLoader
			return devnet.LoadDevnetWithNodes(path, logger)
		},
	}
}

// loadDevnetOrFail loads the devnet with all components or returns an error.
// This is the single point of truth for devnet loading in commands.
//
// Usage:
//
//	loaded, err := loadDevnetOrFail(logger)
//	if err != nil {
//	    return err // Already formatted with homeDir context
//	}
//	// Use loaded.Devnet and loaded.Metadata
func loadDevnetOrFail(logger *output.Logger) (*LoadedDevnet, error) {
	loader := newDevnetLoader(logger)
	result, err := loader.LoadOrFail()
	if err != nil {
		return nil, err
	}

	return &LoadedDevnet{
		Metadata: result.Metadata.(*devnet.DevnetMetadata),
		Devnet:   result.Devnet.(*devnet.Devnet),
	}, nil
}

// loadMetadataOrFail loads only the devnet metadata or returns an error.
// Use this when you only need metadata (e.g., for node index validation).
//
// Usage:
//
//	metadata, err := loadMetadataOrFail(logger)
//	if err != nil {
//	    return err
//	}
func loadMetadataOrFail(logger *output.Logger) (*devnet.DevnetMetadata, error) {
	loader := newDevnetLoader(logger)
	meta, err := loader.LoadMetadataOrFail()
	if err != nil {
		return nil, err
	}
	return meta.(*devnet.DevnetMetadata), nil
}

// resolveBinaryPath returns the binary path for local execution mode.
// Uses the custom path from metadata if set, otherwise defaults to homeDir/bin/{binaryName}.
func resolveBinaryPath(metadata *devnet.DevnetMetadata) string {
	return helpers.ResolveBinaryPath(metadata.CustomBinaryPath, metadata.HomeDir, metadata.GetBinaryName())
}

// createNodeManagerFactory creates a NodeManagerFactory from devnet metadata.
// This is the single point of truth for creating node managers in cmd/ layer.
func createNodeManagerFactory(metadata *devnet.DevnetMetadata, logger *output.Logger) *node.NodeManagerFactory {
	// Convert devnet.ExecutionMode to node.ExecutionMode
	var mode node.ExecutionMode
	switch metadata.ExecutionMode {
	case devnet.ModeDocker:
		mode = node.ModeDocker
	case devnet.ModeLocal:
		mode = node.ModeLocal
	}

	// Build factory config
	config := node.FactoryConfig{
		Mode:        mode,
		BinaryPath:  resolveBinaryPath(metadata),
		DockerImage: provision.GetDockerImage(metadata.StableVersion),
		EVMChainID:  node.ExtractEVMChainID(metadata.ChainID),
		Logger:      logger,
	}

	return node.NewNodeManagerFactory(config)
}
