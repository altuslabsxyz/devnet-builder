package network

import (
	"errors"
	"fmt"
)

// Sentinel errors for network operations.
var (
	// ErrUnknownNetwork indicates the requested network is not registered.
	ErrUnknownNetwork = errors.New("unknown network")

	// ErrDuplicateNetwork indicates a network with the same name is already registered.
	ErrDuplicateNetwork = errors.New("network already registered")

	// ErrInvalidModule indicates the network module failed validation.
	ErrInvalidModule = errors.New("invalid network module")

	// ErrNoDefaultNetwork indicates no default network is available.
	ErrNoDefaultNetwork = errors.New("no default network available")
)

// UnknownNetworkError provides detailed information about an unknown network request.
type UnknownNetworkError struct {
	RequestedNetwork  string
	AvailableNetworks []string
}

// Error implements the error interface.
func (e *UnknownNetworkError) Error() string {
	return fmt.Sprintf("unknown network: %s (available: %v)", e.RequestedNetwork, e.AvailableNetworks)
}

// Is implements errors.Is for UnknownNetworkError.
func (e *UnknownNetworkError) Is(target error) bool {
	return target == ErrUnknownNetwork
}

// Unwrap returns the underlying error for errors.Unwrap.
func (e *UnknownNetworkError) Unwrap() error {
	return ErrUnknownNetwork
}

// DuplicateNetworkError provides detailed information about a duplicate registration.
type DuplicateNetworkError struct {
	NetworkName string
}

// Error implements the error interface.
func (e *DuplicateNetworkError) Error() string {
	return fmt.Sprintf("network already registered: %s", e.NetworkName)
}

// Is implements errors.Is for DuplicateNetworkError.
func (e *DuplicateNetworkError) Is(target error) bool {
	return target == ErrDuplicateNetwork
}

// Unwrap returns the underlying error for errors.Unwrap.
func (e *DuplicateNetworkError) Unwrap() error {
	return ErrDuplicateNetwork
}

// ModuleValidationError provides detailed information about a module validation failure.
type ModuleValidationError struct {
	ModuleName string
	Reason     string
}

// Error implements the error interface.
func (e *ModuleValidationError) Error() string {
	return fmt.Sprintf("invalid network module %q: %s", e.ModuleName, e.Reason)
}

// Is implements errors.Is for ModuleValidationError.
func (e *ModuleValidationError) Is(target error) bool {
	return target == ErrInvalidModule
}

// Unwrap returns the underlying error for errors.Unwrap.
func (e *ModuleValidationError) Unwrap() error {
	return ErrInvalidModule
}
