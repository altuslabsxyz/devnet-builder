// Package tomlutil provides utilities for TOML file manipulation.
package tomlutil

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// MergeTOML merges override TOML bytes into base TOML bytes.
// The override values take precedence over base values.
// Nested maps are recursively merged (not replaced).
// Returns the merged TOML as bytes.
func MergeTOML(base, override []byte) ([]byte, error) {
	if len(override) == 0 {
		return base, nil
	}
	if len(base) == 0 {
		return override, nil
	}

	// Parse base TOML into a map
	var baseMap map[string]any
	if err := toml.Unmarshal(base, &baseMap); err != nil {
		return nil, fmt.Errorf("failed to parse base TOML: %w", err)
	}

	// Parse override TOML into a map
	var overrideMap map[string]any
	if err := toml.Unmarshal(override, &overrideMap); err != nil {
		return nil, fmt.Errorf("failed to parse override TOML: %w", err)
	}

	// Deep merge: override values take precedence, preserving non-overridden keys
	deepMerge(baseMap, overrideMap)

	// Marshal back to TOML
	result, err := toml.Marshal(baseMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged TOML: %w", err)
	}

	return result, nil
}

// deepMerge recursively merges override into base.
// For nested maps, individual keys are merged rather than replacing the entire map.
func deepMerge(base, override map[string]any) {
	for key, overrideVal := range override {
		baseVal, exists := base[key]
		if !exists {
			base[key] = overrideVal
			continue
		}

		// If both are maps, recursively merge
		baseMap, baseIsMap := baseVal.(map[string]any)
		overrideMap, overrideIsMap := overrideVal.(map[string]any)
		if baseIsMap && overrideIsMap {
			deepMerge(baseMap, overrideMap)
		} else {
			// Override non-map values
			base[key] = overrideVal
		}
	}
}

// MergeAndWriteTOML reads base TOML from a file, merges with override, and writes back.
func MergeAndWriteTOML(filePath string, override []byte, readFile func(string) ([]byte, error), writeFile func(string, []byte) error) error {
	if len(override) == 0 {
		return nil // No overrides to apply
	}

	base, err := readFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	merged, err := MergeTOML(base, override)
	if err != nil {
		return fmt.Errorf("failed to merge %s: %w", filePath, err)
	}

	if err := writeFile(filePath, merged); err != nil {
		return fmt.Errorf("failed to write %s: %w", filePath, err)
	}

	return nil
}
