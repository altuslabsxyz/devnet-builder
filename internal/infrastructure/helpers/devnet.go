package helpers

import "fmt"

// DevnetLoadError represents an error during devnet loading.
type DevnetLoadError struct {
	HomeDir string
	Stage   string // "exists", "metadata", "nodes"
	Wrapped error
}

func (e *DevnetLoadError) Error() string {
	switch e.Stage {
	case "exists":
		return fmt.Sprintf("no devnet found at %s", e.HomeDir)
	case "metadata":
		return fmt.Sprintf("failed to load devnet metadata: %v", e.Wrapped)
	case "nodes":
		return fmt.Sprintf("failed to load devnet: %v", e.Wrapped)
	default:
		return fmt.Sprintf("devnet error at %s: %v", e.HomeDir, e.Wrapped)
	}
}

func (e *DevnetLoadError) Unwrap() error {
	return e.Wrapped
}

// LoadResult contains the loaded devnet components.
// Uses interface{} to avoid importing internal/devnet and prevent circular imports.
type LoadResult struct {
	Metadata interface{}
	Devnet   interface{}
}

// DevnetLoader provides devnet loading with callback-based design.
// This design avoids circular imports by accepting callbacks from the cmd/ layer.
type DevnetLoader struct {
	HomeDir string
	Logger  interface{} // Accepts *output.Logger or nil

	// Callbacks injected by caller (from cmd/ layer which can import both helpers and devnet)
	ExistsCheck    func(string) bool
	MetadataLoader func(string) (interface{}, error)
	NodesLoader    func(string, interface{}) (interface{}, error)
}

// LoadOrFail loads devnet with all components, failing early on error.
// Uses injected callbacks to avoid importing internal/devnet.
//
// Returns LoadResult containing:
//   - Metadata: the devnet metadata (caller should type assert to *devnet.DevnetMetadata)
//   - Devnet: the full devnet with nodes (caller should type assert to *devnet.Devnet)
func (l *DevnetLoader) LoadOrFail() (*LoadResult, error) {
	// Step 1: Check if devnet exists
	if l.ExistsCheck != nil && !l.ExistsCheck(l.HomeDir) {
		return nil, &DevnetLoadError{
			HomeDir: l.HomeDir,
			Stage:   "exists",
		}
	}

	// Step 2: Load metadata
	var metadata interface{}
	if l.MetadataLoader != nil {
		var err error
		metadata, err = l.MetadataLoader(l.HomeDir)
		if err != nil {
			return nil, &DevnetLoadError{
				HomeDir: l.HomeDir,
				Stage:   "metadata",
				Wrapped: err,
			}
		}
	}

	// Step 3: Load nodes
	var devnet interface{}
	if l.NodesLoader != nil {
		var err error
		devnet, err = l.NodesLoader(l.HomeDir, metadata)
		if err != nil {
			return nil, &DevnetLoadError{
				HomeDir: l.HomeDir,
				Stage:   "nodes",
				Wrapped: err,
			}
		}
	}

	return &LoadResult{
		Metadata: metadata,
		Devnet:   devnet,
	}, nil
}

// LoadMetadataOrFail loads only metadata, failing if devnet doesn't exist.
// Returns the metadata as interface{} (caller should type assert to *devnet.DevnetMetadata).
func (l *DevnetLoader) LoadMetadataOrFail() (interface{}, error) {
	// Step 1: Check if devnet exists
	if l.ExistsCheck != nil && !l.ExistsCheck(l.HomeDir) {
		return nil, &DevnetLoadError{
			HomeDir: l.HomeDir,
			Stage:   "exists",
		}
	}

	// Step 2: Load metadata
	if l.MetadataLoader != nil {
		metadata, err := l.MetadataLoader(l.HomeDir)
		if err != nil {
			return nil, &DevnetLoadError{
				HomeDir: l.HomeDir,
				Stage:   "metadata",
				Wrapped: err,
			}
		}
		return metadata, nil
	}

	return nil, nil
}
