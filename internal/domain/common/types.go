// Package common provides shared domain types and value objects.
package common

import (
	"fmt"
	"regexp"
)

// ExecutionMode defines how nodes are executed.
type ExecutionMode string

const (
	ModeDocker ExecutionMode = "docker"
	ModeLocal  ExecutionMode = "local"
)

// IsValid checks if the execution mode is valid.
func (m ExecutionMode) IsValid() bool {
	return m == ModeDocker || m == ModeLocal
}

// String returns the string representation.
func (m ExecutionMode) String() string {
	return string(m)
}

// ChainID represents a Cosmos chain identifier.
type ChainID string

// chainIDPattern matches both devnet format (stable-devnet-1) and
// forked network format (stable_988-1, stabletestnet_2201-1)
var chainIDPattern = regexp.MustCompile(`^[a-z]+(_\d+-\d+|-devnet-\d+)$`)

// NewChainID creates a new ChainID with validation.
func NewChainID(id string) (ChainID, error) {
	if !chainIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid chain ID format: %s", id)
	}
	return ChainID(id), nil
}

// String returns the string representation.
func (c ChainID) String() string {
	return string(c)
}

// IsValid checks if the chain ID is valid.
func (c ChainID) IsValid() bool {
	return chainIDPattern.MatchString(string(c))
}

// NetworkSource represents the source network for snapshots.
type NetworkSource string

const (
	NetworkSourceMainnet NetworkSource = "mainnet"
	NetworkSourceTestnet NetworkSource = "testnet"
)

// IsValid checks if the network source is valid.
func (n NetworkSource) IsValid() bool {
	return n == NetworkSourceMainnet || n == NetworkSourceTestnet
}

// String returns the string representation.
func (n NetworkSource) String() string {
	return string(n)
}

// Version represents a semantic version or git reference.
type Version string

// String returns the string representation.
func (v Version) String() string {
	return string(v)
}

// IsEmpty checks if the version is empty.
func (v Version) IsEmpty() bool {
	return v == ""
}
