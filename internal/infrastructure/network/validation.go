package network

import (
	"fmt"
	"regexp"
)

// Module name validation pattern: lowercase, alphanumeric with hyphens
var moduleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Semantic version pattern (simplified)
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// ValidateModule checks if a network module has valid configuration.
// This is called during registration to ensure module correctness.
func ValidateModule(m NetworkModule) error {
	if m == nil {
		return &ModuleValidationError{
			ModuleName: "<nil>",
			Reason:     "module is nil",
		}
	}

	name := m.Name()

	// Validate Name
	if name == "" {
		return &ModuleValidationError{
			ModuleName: "<empty>",
			Reason:     "name cannot be empty",
		}
	}
	if !moduleNamePattern.MatchString(name) {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     fmt.Sprintf("name %q must match pattern %s (lowercase, alphanumeric, hyphens)", name, moduleNamePattern.String()),
		}
	}

	// Validate DisplayName
	if m.DisplayName() == "" {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     "display name cannot be empty",
		}
	}

	// Validate Version
	if m.Version() == "" {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     "version cannot be empty",
		}
	}
	if !semverPattern.MatchString(m.Version()) {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     fmt.Sprintf("version %q must follow semantic versioning (e.g., 1.0.0)", m.Version()),
		}
	}

	// Validate BinaryName
	if m.BinaryName() == "" {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     "binary name cannot be empty",
		}
	}

	// Validate BinarySource
	bs := m.BinarySource()
	if bs.Type == "" {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     "binary source type cannot be empty",
		}
	}
	if bs.Type == BinarySourceGitHub {
		if bs.Owner == "" {
			return &ModuleValidationError{
				ModuleName: name,
				Reason:     "GitHub binary source requires owner",
			}
		}
		if bs.Repo == "" {
			return &ModuleValidationError{
				ModuleName: name,
				Reason:     "GitHub binary source requires repo",
			}
		}
	} else if bs.Type == BinarySourceLocal {
		if bs.LocalPath == "" {
			return &ModuleValidationError{
				ModuleName: name,
				Reason:     "local binary source requires path",
			}
		}
	}

	// Validate DefaultBinaryVersion
	if m.DefaultBinaryVersion() == "" {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     "default binary version cannot be empty",
		}
	}

	// DefaultChainID can be empty - it means "preserve original genesis chain-id"
	// This allows using snapshots with matching chain-id from mainnet/testnet

	// Validate Bech32Prefix
	if m.Bech32Prefix() == "" {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     "bech32 prefix cannot be empty",
		}
	}

	// Validate BaseDenom
	if m.BaseDenom() == "" {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     "base denom cannot be empty",
		}
	}

	// Validate DockerImage
	if m.DockerImage() == "" {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     "docker image cannot be empty",
		}
	}

	// Call module's own Validate method
	if err := m.Validate(); err != nil {
		return &ModuleValidationError{
			ModuleName: name,
			Reason:     fmt.Sprintf("module validation failed: %v", err),
		}
	}

	return nil
}

// ValidateModuleCompatibility checks if a module is compatible with the current
// devnet-builder version. This can be extended to check version constraints.
func ValidateModuleCompatibility(m NetworkModule) error {
	// Currently we just validate the module is well-formed.
	// In the future, this could check against a minimum/maximum version range
	// or other compatibility constraints.
	return ValidateModule(m)
}
